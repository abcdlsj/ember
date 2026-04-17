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

func (m *Model) selectItem() (tea.Model, tea.Cmd) {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return m, nil
	}

	item := m.items[m.cursor]

	switch item.Type {
	case "Movie", "Episode", "Video":
		return m.playItem(item, false)

	case "Series":
		m.pushNav()
		m.page = 0
		m.state = StateLoading
		m.view = viewState{mode: viewSeasons, seriesID: item.ID}
		return m, m.loadSeasons(item.ID)

	case "Season":
		m.pushNav()
		m.page = 0
		m.state = StateLoading
		seriesID := item.SeriesID
		if seriesID == "" {
			seriesID = item.ParentID
		}
		m.view = viewState{mode: viewEpisodes, seriesID: seriesID, seasonID: item.ID}
		return m, m.loadEpisodes(seriesID, item.ID)

	case "CollectionFolder", "Folder", "BoxSet":
		m.pushNav()
		m.currentLib = &item
		m.page = 0
		m.state = StateLoading
		m.view = viewState{mode: viewItems, parentID: item.ID}
		return m, m.loadItems(item.ID, 0)
	}

	return m, nil
}

func (m *Model) playItem(item service.MediaItem, fromBeginning bool) (tea.Model, tea.Cmd) {
	streamInfo, err := m.svc.GetStreamInfoForItem(item)
	if err != nil {
		m.status = "Cannot play: " + err.Error()
		return m, nil
	}

	itemID := item.ID
	mediaSourceID := streamInfo.MediaSourceID
	sessionID := strings.ReplaceAll(uuid.New().String(), "-", "")
	client := m.svc.Client()
	store := m.svc.Store()
	durationTicks := streamInfo.Duration
	startPosSec := streamInfo.PositionSec
	subtitleURLs := buildSubtitleURLs(client, itemID, mediaSourceID, streamInfo.Subtitles)
	if fromBeginning {
		startPosSec = 0
	}

	if fromBeginning {
		m.status = "Launching MPV from beginning: " + item.Name
	} else {
		m.status = "Launching MPV: " + item.Name
	}

	return m, func() tea.Msg {
		result := player.PlayWithHook(streamInfo.StreamURL, item.Name, subtitleURLs, startPosSec, func() {
			_ = client.ReportPlaybackStart(itemID, mediaSourceID, sessionID, startPosSec*10_000_000)
		})
		store.UpdatePlaybackPosition(itemID, result.PositionSec, durationTicks/10000000)
		err := client.ReportPlaybackStopped(itemID, mediaSourceID, sessionID, result.PositionSec*10_000_000)

		return playDoneMsg{
			itemID:        itemID,
			positionSec:   result.PositionSec,
			durationTicks: durationTicks,
			reportOK:      err == nil,
		}
	}
}

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

		if currentItem == nil {
			return playDoneMsg{}
		}

		title := item.SeriesName
		if title == "" {
			title = item.Name
		}

		streamInfo, err := m.svc.GetStreamInfoForItem(*currentItem)
		if err != nil {
			return playDoneMsg{}
		}

		startPosSec := streamInfo.PositionSec
		playSessionID := strings.ReplaceAll(uuid.New().String(), "-", "")
		result := player.PlayMultipleWithHook(urls, title, nil, startPosSec, startIdx, func() {
			_ = m.svc.Client().ReportPlaybackStart(currentItemID, streamInfo.MediaSourceID, playSessionID, startPosSec*10_000_000)
		})

		durationTicks := currentItem.RunTimeTicks
		reportOK := result.Err == nil
		if currentItemID != "" && result.PositionSec > 0 {
			m.svc.Store().UpdatePlaybackPosition(currentItemID, result.PositionSec, durationTicks/10000000)
			reportOK = m.svc.Client().ReportPlaybackStopped(currentItemID, streamInfo.MediaSourceID, playSessionID, result.PositionSec*10_000_000) == nil && reportOK
		}

		return playDoneMsg{
			itemID:        currentItemID,
			positionSec:   result.PositionSec,
			durationTicks: durationTicks,
			reportOK:      reportOK,
		}
	}
}

func (m *Model) goBack() (tea.Model, tea.Cmd) {
	if len(m.navStack) == 0 {
		return m, nil
	}

	prev := m.navStack[len(m.navStack)-1]
	m.navStack = m.navStack[:len(m.navStack)-1]
	m.section = prev.Section
	m.view = prev.View
	m.items = prev.Items
	m.cursor = prev.Cursor
	m.page = prev.Page
	m.totalItems = len(prev.Items)
	m.status = prev.Title
	m.currentLib = prev.CurrentLib

	return m, m.loadVisibleImages()
}

func (m *Model) goToSeason(item service.MediaItem) tea.Cmd {
	return func() tea.Msg {
		if item.SeriesID == "" {
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

		return itemsMsg{
			items: convertRawItems(items),
			total: len(items),
			view:  &viewState{mode: viewEpisodes, seriesID: item.SeriesID, seasonID: item.SeasonID},
		}
	}
}

func (m *Model) goToSeries(item service.MediaItem) tea.Cmd {
	return func() tea.Msg {
		seriesID := item.SeriesID
		if seriesID == "" && item.ParentID != "" {
			seriesID = item.ParentID
		}

		if seriesID == "" {
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

		return itemsMsg{
			items: convertRawItems(items),
			total: len(items),
			view:  &viewState{mode: viewSeasons, seriesID: seriesID},
		}
	}
}

func (m *Model) pushNav() {
	m.navStack = append(m.navStack, NavState{
		Section:    m.section,
		View:       m.view,
		Items:      m.items,
		Cursor:     m.cursor,
		Page:       m.page,
		Title:      m.status,
		CurrentLib: m.currentLib,
	})
}

func (m *Model) resetForServerSwitch(samePrefix bool) {
	m.status = "Connected"
	m.state = StateLoading
	m.section = SectionResume
	m.view = viewState{mode: viewResume}
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

func (m *Model) syncItemState(itemID string, updater func(*service.MediaItem)) {
	for i := range m.items {
		if m.items[i].ID == itemID {
			updater(&m.items[i])
		}
	}

	for sec := range m.sectionCache {
		for i := range m.sectionCache[sec] {
			if m.sectionCache[sec][i].ID == itemID {
				updater(&m.sectionCache[sec][i])
			}
		}
	}

	for i := range m.navStack {
		for j := range m.navStack[i].Items {
			if m.navStack[i].Items[j].ID == itemID {
				updater(&m.navStack[i].Items[j])
			}
		}
	}
}

func (m *Model) refreshCurrentView() (tea.Model, tea.Cmd) {
	m.state = StateLoading
	m.keepCursor = true
	if m.section == SectionResume || m.section == SectionFavorites {
		delete(m.sectionCache, m.section)
	}

	return m, m.loadActiveView()
}

func (m *Model) loadActiveView() tea.Cmd {
	switch m.view.mode {
	case viewResume:
		return m.loadResume()

	case viewFavorites:
		return m.loadFavorites()

	case viewHistory:
		return m.loadHistory(m.page)

	case viewSearch:
		if m.hasSearchCriteria() {
			return m.searchItems()
		}
		return nil

	case viewSeasons:
		return m.loadSeasons(m.view.seriesID)

	case viewEpisodes:
		return m.loadEpisodes(m.view.seriesID, m.view.seasonID)

	case viewItems:
		return m.loadItems(m.view.parentID, m.page)
	}
	return m.loadResume()
}

func (m *Model) loadCurrentPagedSection() tea.Cmd {
	return m.loadActiveView()
}

func (m *Model) switchSection(target Section, loader func() tea.Cmd) (tea.Model, tea.Cmd) {
	m.sectionCursor[m.section] = m.cursor

	m.section = target
	m.page = 0
	m.navStack = nil
	m.currentLib = nil
	m.keepCursor = false
	switch target {
	case SectionResume:
		m.view = viewState{mode: viewResume}
	case SectionFavorites:
		m.view = viewState{mode: viewFavorites}
	case SectionHistory:
		m.view = viewState{mode: viewHistory}
	case SectionSearch:
		m.view = viewState{mode: viewSearch}
	}

	if (target == SectionResume || target == SectionFavorites) && len(m.navStack) == 0 {
		if cached, ok := m.sectionCache[target]; ok && len(cached) > 0 {
			m.items = cached
			m.totalItems = len(cached)
			m.cursor = m.sectionCursor[target]
			m.state = StateBrowsing
			m.status = fmt.Sprintf("%d items", len(cached))
			return m, m.loadVisibleImages()
		}
	}

	m.state = StateLoading
	return m, loader()
}

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

func convertRawItems(items []api.MediaItem) []service.MediaItem {
	result := make([]service.MediaItem, len(items))
	for i, item := range items {
		result[i] = convertRawToService(item)
	}
	return result
}

func convertRawToService(item api.MediaItem) service.MediaItem {
	var userData *service.UserData
	if item.UserData != nil {
		pct := 0
		if item.RunTimeTicks > 0 && item.UserData.PlaybackPositionTicks > 0 {
			pct = int(float64(item.UserData.PlaybackPositionTicks) / float64(item.RunTimeTicks) * 100)
		}
		userData = &service.UserData{
			PlaybackPositionTicks: item.UserData.PlaybackPositionTicks,
			Played:                item.UserData.Played,
			IsFavorite:            item.UserData.IsFavorite,
			LastPlayedDate:        item.UserData.LastPlayedDate,
			PlaybackPositionPct:   pct,
		}
	}

	mediaSources := make([]service.MediaSource, 0, len(item.MediaSources))
	for _, source := range item.MediaSources {
		subtitles := make([]service.SubtitleInfo, 0, len(source.MediaStreams))
		for _, stream := range source.MediaStreams {
			if stream.Type != "Subtitle" {
				continue
			}
			subtitles = append(subtitles, service.SubtitleInfo{
				Index:      stream.Index,
				Language:   stream.Language,
				Title:      stream.Title,
				IsExternal: stream.IsExternal,
				IsDefault:  stream.IsDefault,
				Codec:      stream.Codec,
			})
		}

		mediaSources = append(mediaSources, service.MediaSource{
			ID:        source.ID,
			Container: source.Container,
			Protocol:  source.Protocol,
			Subtitles: subtitles,
		})
	}

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
		UserData:     userData,
		MediaSources: mediaSources,
		Playable:     item.Type == "Movie" || item.Type == "Episode" || item.Type == "Video",
		Browsable:    item.Type == "Series" || item.Type == "Season" || item.Type == "CollectionFolder" || item.Type == "Folder" || item.Type == "BoxSet",
	}
}

func buildSubtitleURLs(client *api.Client, itemID, sourceID string, subtitles []service.SubtitleInfo) []string {
	urls := make([]string, 0, len(subtitles))
	for _, subtitle := range subtitles {
		if !subtitle.IsExternal {
			continue
		}
		urls = append(urls, client.SubtitleURL(itemID, sourceID, subtitle.Index, subtitle.Codec))
	}
	return urls
}
