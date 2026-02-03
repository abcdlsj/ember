package ui

import (
	"fmt"
	"strings"
	"time"

	"ember/internal/api"
	"ember/internal/player"
	"ember/internal/service"
	"ember/internal/storage"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

// selectItem handles item selection
func (m *Model) selectItem() (tea.Model, tea.Cmd) {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return m, nil
	}

	item := m.items[m.cursor]

	switch item.Type {
	case "Movie", "Episode", "Video":
		return m.playItem(item)

	case "Series":
		m.pushNav()
		m.state = StateLoading
		return m, m.loadSeasons(item.ID)

	case "Season":
		m.pushNav()
		m.state = StateLoading
		seriesID := item.SeriesID
		if seriesID == "" {
			seriesID = item.ParentID
		}
		return m, m.loadEpisodes(seriesID, item.ID)

	case "CollectionFolder", "Folder", "BoxSet":
		m.pushNav()
		m.currentLib = &item
		m.page = 0
		m.state = StateLoading
		return m, m.loadItems(item.ID, 0)
	}

	return m, nil
}

// playItem plays a media item
func (m *Model) playItem(item service.MediaItem) (tea.Model, tea.Cmd) {
	streamInfo, err := m.svc.GetStreamInfo(item.ID)
	if err != nil {
		m.status = "Cannot play: " + err.Error()
		return m, nil
	}

	itemID := item.ID
	mediaSourceID := streamInfo.MediaSourceID
	sessionID := strings.ReplaceAll(uuid.New().String(), "-", "")
	client := m.svc.Client()
	store := m.svc.Store()
	durationTicks := item.RunTimeTicks

	// Report playback start
	m.svc.ReportPlayback(service.PlaybackRequest{
		Type:          "start",
		ItemID:        itemID,
		PositionTicks: streamInfo.PositionSec * 10000000,
	})

	m.status = "Playing: " + item.Name

	return m, func() tea.Msg {
		result := player.Play(streamInfo.StreamURL, item.Name, []string{}, streamInfo.PositionSec)

		// Save local progress
		store.UpdatePlaybackPosition(itemID, result.PositionSec, durationTicks/10000000)

		// Report to Emby
		err := client.ReportPlaybackStopped(itemID, mediaSourceID, sessionID, result.PositionSec*10_000_000)
		
		return playDoneMsg{
			itemID:        itemID,
			positionSec:   result.PositionSec,
			durationTicks: durationTicks,
			reportOK:      err == nil,
		}
	}
}

// playSeasonContinuously plays episodes continuously from current position
func (m *Model) playSeasonContinuously(item service.MediaItem) tea.Cmd {
	seriesID := item.SeriesID
	seasonID := item.SeasonID
	if seasonID == "" {
		seasonID = item.ParentID
	}

	if seriesID == "" || seasonID == "" {
		m.status = "Cannot play continuously: missing season info"
		return nil
	}

	return func() tea.Msg {
		episodes, err := m.svc.Client().GetEpisodes(seriesID, seasonID)
		if err != nil || len(episodes) == 0 {
			return playDoneMsg{}
		}

		// Find current episode index
		startIdx := -1
		for i, ep := range episodes {
			if ep.ID == item.ID {
				startIdx = i
				break
			}
		}
		if startIdx == -1 {
			startIdx = 0
		}

		// Build playlist
		var urls []string
		var currentItem *service.MediaItem
		var currentItemID string

		for i := startIdx; i < len(episodes); i++ {
			ep := episodes[i]
			epFull, err := m.svc.Client().GetItem(ep.ID)
			if err != nil || len(epFull.MediaSources) == 0 {
				continue
			}

			ms := epFull.MediaSources[0]
			url := m.svc.Client().StreamURL(ep.ID, ms.ID, ms.Container)
			urls = append(urls, url)

			if i == startIdx {
				svcItem := convertRawToService(*epFull)
				currentItem = &svcItem
				currentItemID = ep.ID
			}
		}

		if len(urls) == 0 {
			return playDoneMsg{}
		}

		// Calculate start position
		var startPosSec int64
		if currentItem != nil {
			currentFull, _ := m.svc.Client().GetItem(currentItemID)
			if currentFull.UserData != nil && currentFull.UserData.PlaybackPositionTicks > 0 {
				startPosSec = currentFull.UserData.PlaybackPositionTicks / 10000000
			}
			if startPosSec == 0 {
				startPosSec = m.svc.Store().GetPlaybackPosition(currentItemID)
			}
		}

		title := item.SeriesName
		if title == "" {
			title = item.Name
		}

		playSessionID := strings.ReplaceAll(uuid.New().String(), "-", "")

		// Report playback start
		if currentItemID != "" {
			currentFull, _ := m.svc.Client().GetItem(currentItemID)
			if len(currentFull.MediaSources) > 0 {
				m.svc.Client().ReportPlaybackStart(currentItemID, currentFull.MediaSources[0].ID, playSessionID, startPosSec*10000000)
			}
		}

		// Play with mpv
		result := player.PlayMultiple(urls, title, []string{}, startPosSec, startIdx)

		// Save progress
		var durationTicks int64
		if currentItemID != "" && result.PositionSec > 0 {
			currentFull, _ := m.svc.Client().GetItem(currentItemID)
			if currentFull != nil {
				durationTicks = currentFull.RunTimeTicks
			}
			m.svc.Store().UpdatePlaybackPosition(currentItemID, result.PositionSec, durationTicks/10000000)
			m.svc.Client().ReportPlaybackStopped(currentItemID, "", playSessionID, result.PositionSec*10000000)
		}

		return playDoneMsg{
			itemID:        currentItemID,
			positionSec:   result.PositionSec,
			durationTicks: durationTicks,
			reportOK:      result.Err == nil,
		}
	}
}

// goBack navigates back
func (m *Model) goBack() (tea.Model, tea.Cmd) {
	if len(m.navStack) == 0 {
		return m, nil
	}

	prev := m.navStack[len(m.navStack)-1]
	m.navStack = m.navStack[:len(m.navStack)-1]
	m.section = prev.Section
	m.items = prev.Items
	m.cursor = prev.Cursor
	m.totalItems = len(prev.Items)
	m.status = prev.Title

	return m, m.loadVisibleImages()
}

// goToSeason navigates to season from episode
func (m *Model) goToSeason(item service.MediaItem) tea.Cmd {
	return func() tea.Msg {
		if item.SeriesID == "" {
			// Need to fetch full item
			fullItem, err := m.svc.Client().GetItem(item.ID)
			if err != nil {
				return itemsMsg{err: fmt.Errorf("no series info")}
			}
			item.SeriesID = fullItem.SeriesID
			item.SeasonID = fullItem.SeasonID
			if item.SeasonID == "" {
				item.SeasonID = fullItem.ParentID
			}
		}

		if item.SeriesID == "" || item.SeasonID == "" {
			return itemsMsg{err: fmt.Errorf("no season info")}
		}

		items, err := m.svc.Client().GetEpisodes(item.SeriesID, item.SeasonID)
		if err != nil {
			return itemsMsg{err: err}
		}

		return itemsMsg{items: convertRawItems(items), total: len(items)}
	}
}

// goToSeries navigates to series from episode or season
func (m *Model) goToSeries(item service.MediaItem) tea.Cmd {
	return func() tea.Msg {
		seriesID := item.SeriesID
		if seriesID == "" && item.ParentID != "" {
			seriesID = item.ParentID
		}

		if seriesID == "" {
			// Need to fetch full item
			fullItem, err := m.svc.Client().GetItem(item.ID)
			if err != nil {
				return itemsMsg{err: fmt.Errorf("no series info")}
			}
			seriesID = fullItem.SeriesID
			if seriesID == "" {
				seriesID = fullItem.ParentID
			}
		}

		if seriesID == "" {
			return itemsMsg{err: fmt.Errorf("no series info")}
		}

		items, err := m.svc.Client().GetSeasons(seriesID)
		if err != nil {
			return itemsMsg{err: err}
		}

		return itemsMsg{items: convertRawItems(items), total: len(items)}
	}
}

// pushNav saves current state to navigation stack
func (m *Model) pushNav() {
	m.navStack = append(m.navStack, NavState{
		Section: m.section,
		Items:   m.items,
		Cursor:  m.cursor,
		Title:   m.status,
	})
}

// currentParentID returns the current parent ID for pagination
func (m *Model) currentParentID() string {
	if m.currentLib != nil {
		return m.currentLib.ID
	}
	return ""
}

// resetForServerSwitch resets state when switching servers
func (m *Model) resetForServerSwitch(samePrefix bool) {
	m.status = "Connected"
	m.state = StateLoading
	m.section = SectionResume
	m.navStack = nil
	m.currentLib = nil
	m.page = 0
	m.cursor = 0
	m.sectionCache = make(map[Section][]service.MediaItem)
	m.sectionCursor = make(map[Section]int)
	m.coverCache = make(map[string]string)

	if !samePrefix {
		m.detailCache = make(map[string]*storage.MediaDetail)
		m.serverLatencies = make(map[int]time.Duration)
	}
}

// syncItemState updates item state across all locations
func (m *Model) syncItemState(itemID string, updater func(*service.MediaItem)) {
	// Update current items
	for i := range m.items {
		if m.items[i].ID == itemID {
			updater(&m.items[i])
		}
	}

	// Update sectionCache
	for sec := range m.sectionCache {
		for i := range m.sectionCache[sec] {
			if m.sectionCache[sec][i].ID == itemID {
				updater(&m.sectionCache[sec][i])
			}
		}
	}

	// Update navStack
	for i := range m.navStack {
		for j := range m.navStack[i].Items {
			if m.navStack[i].Items[j].ID == itemID {
				updater(&m.navStack[i].Items[j])
			}
		}
	}
}

// refreshCurrentView reloads the current view
func (m *Model) refreshCurrentView() (tea.Model, tea.Cmd) {
	m.state = StateLoading
	delete(m.sectionCache, m.section)

	switch m.section {
	case SectionResume:
		return m, m.loadResume()

	case SectionFavorites:
		return m, m.loadFavorites()

	case SectionSearch:
		query := strings.TrimSpace(m.searchInput.Value())
		if query != "" {
			return m, m.searchItems(query)
		}
		return m, nil

	default:
		return m, m.loadItems(m.currentParentID(), m.page)
	}
}

// switchSection switches to a different section
func (m *Model) switchSection(target Section, loader func() tea.Cmd) (tea.Model, tea.Cmd) {
	m.sectionCursor[m.section] = m.cursor

	m.section = target
	m.page = 0
	m.navStack = nil
	m.currentLib = nil

	if cached, ok := m.sectionCache[target]; ok && len(cached) > 0 {
		m.items = cached
		m.totalItems = len(cached)
		m.cursor = m.sectionCursor[target]
		m.state = StateBrowsing
		m.status = fmt.Sprintf("%d items", len(cached))
		return m, m.loadVisibleImages()
	}

	m.state = StateLoading
	return m, loader()
}

// pingServers pings all servers with same prefix
func (m *Model) pingServers() tea.Cmd {
	return func() tea.Msg {
		srv := m.svc.GetActiveServer()
		if srv == nil {
			return pingServersMsg{latencies: nil}
		}

		prefix := srv.Prefix
		servers := m.svc.GetServers()

		type pingResult struct {
			idx     int
			latency int64
		}

		var targets []int
		for i, s := range servers {
			if s.Prefix == prefix {
				targets = append(targets, i)
			}
		}

		ch := make(chan pingResult, len(targets))
		for _, idx := range targets {
			go func(i int, url string) {
				latency := m.svc.PingServer(url)
				ch <- pingResult{idx: i, latency: latency}
			}(idx, servers[idx].URL)
		}

		latencies := make(map[int]time.Duration, len(targets))
		for range targets {
			r := <-ch
			latencies[r.idx] = time.Duration(r.latency) * time.Millisecond
		}

		return pingServersMsg{latencies: latencies}
	}
}

// Helper functions
func convertRawItems(items []api.MediaItem) []service.MediaItem {
	result := make([]service.MediaItem, len(items))
	for i, item := range items {
		result[i] = convertRawToService(item)
	}
	return result
}

func convertRawToService(item api.MediaItem) service.MediaItem {
	// This is a simplified conversion - in real usage, service methods handle this
	return service.MediaItem{
		ID:           item.ID,
		Name:         item.Name,
		Type:         item.Type,
		Year:         item.Year,
		SeriesID:     item.SeriesID,
		SeriesName:   item.SeriesName,
		SeasonID:     item.SeasonID,
		SeasonName:   item.SeasonName,
		ParentID:     item.ParentID,
		IndexNumber:  item.IndexNumber,
		Overview:     item.Overview,
		RunTimeTicks: item.RunTimeTicks,
		Playable:     item.Type == "Movie" || item.Type == "Episode" || item.Type == "Video",
		Browsable:    item.Type == "Series" || item.Type == "Season" || item.Type == "CollectionFolder" || item.Type == "Folder" || item.Type == "BoxSet",
	}
}

