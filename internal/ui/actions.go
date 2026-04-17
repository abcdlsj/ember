package ui

import (
	"fmt"
	"strings"
	"time"

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
	durationTicks := streamInfo.Duration
	startPosSec := streamInfo.PositionSec
	subtitleURLs := streamInfo.SubtitleURLs
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
			_ = m.svc.ReportPlaybackStart(itemID, mediaSourceID, sessionID, startPosSec)
		})
		err := m.svc.ReportPlaybackStopped(itemID, mediaSourceID, sessionID, result.PositionSec, durationTicks)

		return playDoneMsg{
			itemID:        itemID,
			positionSec:   result.PositionSec,
			durationTicks: durationTicks,
			reportOK:      err == nil,
			err:           result.Err,
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
		plan, err := m.svc.BuildContinuousPlayback(item)
		if err != nil {
			return playDoneMsg{err: err}
		}

		startPosSec := plan.StreamInfo.PositionSec
		playSessionID := strings.ReplaceAll(uuid.New().String(), "-", "")
		result := player.PlayMultipleWithHook(plan.URLs, plan.Title, nil, startPosSec, plan.StartIndex, func() {
			_ = m.svc.ReportPlaybackStart(plan.CurrentItem.ID, plan.StreamInfo.MediaSourceID, playSessionID, startPosSec)
		})

		durationTicks := plan.CurrentItem.RunTimeTicks
		reportOK := result.Err == nil
		if plan.CurrentItem.ID != "" && result.PositionSec > 0 {
			reportOK = m.svc.ReportPlaybackStopped(plan.CurrentItem.ID, plan.StreamInfo.MediaSourceID, playSessionID, result.PositionSec, durationTicks) == nil && reportOK
		}

		return playDoneMsg{
			itemID:        plan.CurrentItem.ID,
			positionSec:   result.PositionSec,
			durationTicks: durationTicks,
			reportOK:      reportOK,
			err:           result.Err,
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
		list, seriesID, seasonID, err := m.svc.ResolveSeason(item)
		if err != nil {
			return itemsMsg{err: err}
		}

		return itemsMsg{
			items: list.Items,
			total: list.Total,
			view:  &viewState{mode: viewEpisodes, seriesID: seriesID, seasonID: seasonID},
		}
	}
}

func (m *Model) goToSeries(item service.MediaItem) tea.Cmd {
	return func() tea.Msg {
		list, seriesID, err := m.svc.ResolveSeries(item)
		if err != nil {
			return itemsMsg{err: err}
		}

		return itemsMsg{
			items: list.Items,
			total: list.Total,
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
