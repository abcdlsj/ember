package ui

import (
	"fmt"
	"strings"
	"time"

	"ember/internal/logging"
	"ember/internal/service"
	"ember/internal/storage"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Section int

const (
	SectionResume Section = iota
	SectionFavorites
	SectionSearch
)

type State int

const (
	StateLoading State = iota
	StateBrowsing
	StateSearching
	StateServerManage
	StateServerEdit
)

// Model represents the TUI application state
type Model struct {
	svc    *service.MediaService
	width  int
	height int

	section Section
	state   State

	items      []service.MediaItem
	totalItems int
	page       int
	pageSize   int
	cursor     int
	navStack   []NavState
	currentLib *service.MediaItem

	searchInput textinput.Model
	spinner     spinner.Model
	status      string
	latency     time.Duration

	coverCache  map[string]string
	detailCache map[string]*storage.MediaDetail

	// section 级别缓存
	sectionCache  map[Section][]service.MediaItem
	sectionCursor map[Section]int

	lastPlayPosition int64
	lastReportOK     bool
	loggingEnabled   bool

	// 服务器管理
	serverCursor    int
	serverInputs    []textinput.Model
	serverFocused   int
	editingServer   int
	serverLatencies map[int]time.Duration
	pingInProgress  bool
	prevServerPrefix string
}

// NavState represents navigation history
type NavState struct {
	Section  Section
	Items    []service.MediaItem
	Cursor   int
	Title    string
	ParentID string
}

// Message types for Bubbletea
type itemsMsg struct {
	items []service.MediaItem
	total int
	err   error
}

type imageMsg struct {
	id    string
	image string
}

type detailMsg struct {
	id     string
	detail *storage.MediaDetail
}

type pingMsg time.Duration

type pingServersMsg struct {
	latencies map[int]time.Duration
}

type favoriteMsg struct {
	itemID string
	isFav  bool
	err    error
}

type connectServerMsg struct {
	err        error
	samePrefix bool
}

type playDoneMsg struct {
	itemID        string
	positionSec   int64
	durationTicks int64
	reportOK      bool
}

// New creates a new TUI model
func New(svc *service.MediaService) *Model {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 100
	ti.Width = 30

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	initialState := StateLoading
	if svc.Store().GetActiveServer() == nil {
		initialState = StateServerManage
	}

	return &Model{
		svc:             svc,
		section:         SectionResume,
		state:           initialState,
		pageSize:        20,
		searchInput:     ti,
		spinner:         sp,
		status:          "Connecting...",
		coverCache:      make(map[string]string),
		detailCache:     make(map[string]*storage.MediaDetail),
		sectionCache:    make(map[Section][]service.MediaItem),
		sectionCursor:   make(map[Section]int),
		loggingEnabled:  true,
		editingServer:   -1,
		serverLatencies: make(map[int]time.Duration),
	}
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	if m.state == StateServerManage {
		return m.spinner.Tick
	}
	return tea.Batch(
		m.loadResume(),
		m.pingServer(),
		m.spinner.Tick,
	)
}

// ==================== Loading Commands ====================

func (m *Model) loadResume() tea.Cmd {
	return func() tea.Msg {
		list, err := m.svc.GetResume(50)
		if err != nil {
			return itemsMsg{err: err}
		}
		return itemsMsg{items: list.Items, total: list.Total}
	}
}

func (m *Model) loadLibraries() tea.Cmd {
	return func() tea.Msg {
		list, err := m.svc.GetLibraries()
		if err != nil {
			return itemsMsg{err: err}
		}
		return itemsMsg{items: list.Items, total: list.Total}
	}
}

func (m *Model) loadItems(parentID string, page int) tea.Cmd {
	return func() tea.Msg {
		list, err := m.svc.GetItems(parentID, page, m.pageSize)
		if err != nil {
			return itemsMsg{err: err}
		}
		return itemsMsg{items: list.Items, total: list.Total}
	}
}

func (m *Model) searchItems(query string) tea.Cmd {
	return func() tea.Msg {
		list, err := m.svc.Search(query, 50)
		if err != nil {
			return itemsMsg{err: err}
		}
		return itemsMsg{items: list.Items, total: list.Total}
	}
}

func (m *Model) loadSeasons(seriesID string) tea.Cmd {
	return func() tea.Msg {
		list, err := m.svc.GetSeasons(seriesID)
		if err != nil {
			return itemsMsg{err: err}
		}
		return itemsMsg{items: list.Items, total: list.Total}
	}
}

func (m *Model) loadEpisodes(seriesID, seasonID string) tea.Cmd {
	return func() tea.Msg {
		list, err := m.svc.GetEpisodes(seriesID, seasonID)
		if err != nil {
			return itemsMsg{err: err}
		}
		return itemsMsg{items: list.Items, total: list.Total}
	}
}

func (m *Model) loadFavorites() tea.Cmd {
	return func() tea.Msg {
		list, err := m.svc.GetFavorites(50)
		if err != nil {
			return itemsMsg{err: err}
		}
		return itemsMsg{items: list.Items, total: list.Total}
	}
}

func (m *Model) toggleFavorite(item service.MediaItem) tea.Cmd {
	return func() tea.Msg {
		result, err := m.svc.ToggleFavorite(item.ID)
		if err != nil {
			return favoriteMsg{itemID: item.ID, err: err}
		}
		return favoriteMsg{itemID: item.ID, isFav: result.IsFavorite}
	}
}

func (m *Model) pingServer() tea.Cmd {
	return func() tea.Msg {
		status := m.svc.GetServerStatus()
		return pingMsg(status.Latency)
	}
}

func (m *Model) loadImage(item service.MediaItem, width, height int) tea.Cmd {
	return func() tea.Msg {
		// For TUI, we still need to fetch and convert image to ASCII/terminal format
		// This is kept from original implementation
		url := item.ImageURL
		if url == "" {
			return imageMsg{id: item.ID, image: ""}
		}
		img := RenderImage(url, width, height)
		return imageMsg{id: item.ID, image: img}
	}
}

func (m *Model) loadDetail(itemID string) tea.Cmd {
	return func() tea.Msg {
		if cached, ok := m.svc.Store().GetMediaDetail(itemID); ok {
			d := cached // copy to avoid reference to loop variable
			return detailMsg{id: itemID, detail: &d}
		}

		item, err := m.svc.GetItemRaw(itemID)
		if err != nil || len(item.MediaSources) == 0 {
			return detailMsg{id: itemID, detail: nil}
		}

		ms := item.MediaSources[0]
		detail := storage.MediaDetail{
			ItemID:    itemID,
			SourceID:  ms.ID,
			Container: ms.Container,
		}
		for _, stream := range ms.MediaStreams {
			if stream.Type == "Subtitle" {
				detail.Subtitles = append(detail.Subtitles, storage.SubtitleInfo{
					Index:      stream.Index,
					Language:   stream.Language,
					Title:      stream.Title,
					IsExternal: stream.IsExternal,
					Codec:      stream.Codec,
				})
			}
		}
		m.svc.Store().SetMediaDetail(detail)
		return detailMsg{id: itemID, detail: &detail}
	}
}

// ==================== Update ====================

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.coverCache = make(map[string]string)
		return m, m.loadVisibleImages()

	case tea.KeyMsg:
		return m.handleKey(msg)

	case itemsMsg:
		if msg.err != nil {
			m.status = "Error: " + msg.err.Error()
		} else {
			m.items = msg.items
			m.totalItems = msg.total
			m.cursor = 0
			m.state = StateBrowsing
			m.status = fmt.Sprintf("%d items", msg.total)
			m.sectionCache[m.section] = msg.items
			m.sectionCursor[m.section] = 0
		}
		return m, m.loadVisibleImages()

	case imageMsg:
		m.coverCache[msg.id] = msg.image
		return m, nil

	case detailMsg:
		if msg.detail != nil {
			m.detailCache[msg.id] = msg.detail
		}
		return m, nil

	case pingMsg:
		m.latency = time.Duration(msg)
		return m, tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
			return pingMsg(m.svc.GetServerStatus().Latency)
		})

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case playDoneMsg:
		m.lastPlayPosition = msg.positionSec
		m.lastReportOK = msg.reportOK
		m.status = "Playback finished"
		if msg.itemID != "" {
			m.syncItemState(msg.itemID, func(item *service.MediaItem) {
				if item.UserData == nil {
					item.UserData = &service.UserData{}
				}
				item.UserData.PlaybackPositionTicks = msg.positionSec * 10000000
			})
		}
		return m, nil

	case favoriteMsg:
		if msg.err != nil {
			m.status = "Favorite error: " + msg.err.Error()
			return m, nil
		}
		m.syncItemState(msg.itemID, func(item *service.MediaItem) {
			if item.UserData == nil {
				item.UserData = &service.UserData{}
			}
			item.UserData.IsFavorite = msg.isFav
		})
		if msg.isFav {
			m.status = "Added to favorites"
		} else {
			m.status = "Removed from favorites"
		}
		if m.section == SectionFavorites && !msg.isFav {
			return m.refreshCurrentView()
		}
		return m, nil

	case connectServerMsg:
		if msg.err != nil {
			m.status = "Connect failed: " + msg.err.Error()
			m.state = StateServerManage
			return m, nil
		}
		m.resetForServerSwitch(msg.samePrefix)
		return m, m.loadResume()

	case pingServersMsg:
		m.pingInProgress = false
		m.serverLatencies = msg.latencies
		m.status = "Ping complete"
		return m, nil
	}

	return m, nil
}

func (m *Model) loadVisibleImages() tea.Cmd {
	if len(m.items) == 0 {
		return nil
	}

	var cmds []tea.Cmd
	start := m.cursor - 2
	end := m.cursor + 3

	if start < 0 {
		start = 0
	}
	if end > len(m.items) {
		end = len(m.items)
	}

	statusWidth := 32
	if m.width < 100 {
		statusWidth = 28
	}
	contentWidth := m.width - statusWidth
	coverWidth := contentWidth - 4
	coverHeight := m.height - 6

	for i := start; i < end; i++ {
		item := m.items[i]
		if _, ok := m.coverCache[item.ID]; !ok {
			cmds = append(cmds, m.loadImage(item, coverWidth, coverHeight))
		}
	}

	if m.cursor < len(m.items) {
		curItem := m.items[m.cursor]
		if _, ok := m.detailCache[curItem.ID]; !ok {
			cmds = append(cmds, m.loadDetail(curItem.ID))
		}
	}

	return tea.Batch(cmds...)
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.state == StateSearching {
		return m.handleSearchKey(msg)
	}
	if m.state == StateServerManage {
		return m.handleServerManageKey(msg)
	}
	if m.state == StateServerEdit {
		return m.handleServerEditKey(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "left", "h":
		if m.cursor > 0 {
			m.cursor--
			return m, m.loadVisibleImages()
		} else if m.page > 0 {
			m.page--
			m.state = StateLoading
			return m, m.loadItems(m.currentParentID(), m.page)
		}

	case "right", "l":
		if m.cursor < len(m.items)-1 {
			m.cursor++
			return m, m.loadVisibleImages()
		} else if (m.page+1)*m.pageSize < m.totalItems {
			m.page++
			m.state = StateLoading
			return m, m.loadItems(m.currentParentID(), m.page)
		}

	case "enter":
		return m.selectItem()

	case "backspace", "esc":
		return m.goBack()

	case "1":
		return m.switchSection(SectionResume, m.loadResume)

	case "2":
		return m.switchSection(SectionFavorites, m.loadFavorites)

	case "3", "/":
		m.state = StateSearching
		m.searchInput.Focus()
		return m, textinput.Blink

	case "f":
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			return m, m.toggleFavorite(item)
		}

	case "c":
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.Type == "Episode" {
				m.status = "Loading playlist..."
				return m, m.playSeasonContinuously(item)
			}
		}

	case "d":
		m.loggingEnabled = !m.loggingEnabled
		logging.SetEnabled(m.loggingEnabled)
		if m.loggingEnabled {
			m.status = "Debug logging: ON"
		} else {
			m.status = "Debug logging: OFF"
		}
		return m, nil

	case "r":
		return m.refreshCurrentView()

	case "s":
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.Type == "Episode" {
				m.pushNav()
				m.state = StateLoading
				return m, m.goToSeason(item)
			}
		}

	case "S":
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.Type == "Episode" || item.Type == "Season" {
				m.pushNav()
				m.state = StateLoading
				return m, m.goToSeries(item)
			}
		}

	case "m":
		m.state = StateServerManage
		m.serverCursor = m.svc.Store().GetActiveServerIndex()
		return m, nil
	}

	return m, nil
}

func (m *Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = StateBrowsing
		m.searchInput.Blur()
		return m, nil

	case "enter":
		query := strings.TrimSpace(m.searchInput.Value())
		if query != "" {
			m.state = StateLoading
			m.section = SectionSearch
			m.searchInput.Blur()
			return m, m.searchItems(query)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m *Model) handleServerManageKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	servers := m.svc.GetServers()

	switch msg.String() {
	case "q", "esc":
		m.state = StateBrowsing
		return m, nil

	case "up", "k":
		if m.serverCursor > 0 {
			m.serverCursor--
		}

	case "down", "j":
		if m.serverCursor < len(servers)-1 {
			m.serverCursor++
		}

	case "enter":
		if len(servers) > 0 && m.serverCursor < len(servers) {
			oldPrefix := ""
			if srv := m.svc.GetActiveServer(); srv != nil {
				oldPrefix = srv.Prefix
			}
			
			err := m.svc.ActivateServer(m.serverCursor)
			if err != nil {
				return m, func() tea.Msg {
					return connectServerMsg{err: err}
				}
			}
			
			newPrefix := ""
			if srv := m.svc.GetActiveServer(); srv != nil {
				newPrefix = srv.Prefix
			}
			
			return m, func() tea.Msg {
				return connectServerMsg{err: nil, samePrefix: oldPrefix != "" && oldPrefix == newPrefix}
			}
		}

	case "a":
		m.editingServer = -1
		m.initServerInputs("", "", "", "")
		m.state = StateServerEdit
		return m, m.serverInputs[0].Focus()

	case "e":
		if len(servers) > 0 && m.serverCursor < len(servers) {
			srv := servers[m.serverCursor]
			m.editingServer = m.serverCursor
			m.initServerInputs(srv.Name, srv.URL, srv.Username, "")
			m.state = StateServerEdit
			return m, m.serverInputs[0].Focus()
		}

	case "d", "delete":
		if len(servers) > 0 && m.serverCursor < len(servers) {
			m.svc.DeleteServer(m.serverCursor)
			if m.serverCursor >= len(m.svc.GetServers()) && m.serverCursor > 0 {
				m.serverCursor--
			}
		}

	case "p":
		if m.pingInProgress {
			return m, nil
		}
		m.pingInProgress = true
		m.serverLatencies = make(map[int]time.Duration)
		m.status = "Pinging servers..."
		return m, m.pingServers()
	}

	return m, nil
}

func (m *Model) handleServerEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = StateServerManage
		return m, nil

	case "tab", "down":
		m.serverInputs[m.serverFocused].Blur()
		m.serverFocused = (m.serverFocused + 1) % len(m.serverInputs)
		return m, m.serverInputs[m.serverFocused].Focus()

	case "shift+tab", "up":
		m.serverInputs[m.serverFocused].Blur()
		m.serverFocused = (m.serverFocused - 1 + len(m.serverInputs)) % len(m.serverInputs)
		return m, m.serverInputs[m.serverFocused].Focus()

	case "enter":
		srv := service.ServerInfo{
			Name:     m.serverInputs[0].Value(),
			URL:      m.serverInputs[1].Value(),
			Username: m.serverInputs[2].Value(),
		}
		password := m.serverInputs[3].Value()
		
		if srv.URL == "" {
			m.status = "URL is required"
			return m, nil
		}

		var err error
		if m.editingServer < 0 {
			err = m.svc.AddServer(srv.Name, srv.URL, srv.Username, password)
			m.serverCursor = len(m.svc.GetServers()) - 1
		} else {
			// For edit, we don't have UpdateServer with password in service yet
			// Using the existing one without password change for now
			err = m.svc.UpdateServer(m.editingServer, srv.Name, srv.URL, srv.Username, password)
		}
		
		if err != nil {
			m.status = "Error: " + err.Error()
			return m, nil
		}
		
		m.state = StateServerManage
		return m, nil
	}

	var cmd tea.Cmd
	m.serverInputs[m.serverFocused], cmd = m.serverInputs[m.serverFocused].Update(msg)
	return m, cmd
}

// ... rest of the file continues with View methods
