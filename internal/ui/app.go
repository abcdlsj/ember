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
	SectionHistory
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

type viewMode int

const (
	viewResume viewMode = iota
	viewFavorites
	viewHistory
	viewSearch
	viewItems
	viewSeasons
	viewEpisodes
)

type viewState struct {
	mode     viewMode
	parentID string
	seriesID string
	seasonID string
}

type Model struct {
	svc    *service.MediaService
	width  int
	height int

	section Section
	state   State
	view    viewState

	items      []service.MediaItem
	totalItems int
	page       int
	pageSize   int
	cursor     int
	navStack   []NavState
	currentLib *service.MediaItem
	keepCursor bool

	searchInput     textinput.Model
	lastSearchQuery string
	spinner         spinner.Model
	status          string
	latency         time.Duration

	coverCache  map[string]string
	detailCache map[string]*storage.MediaDetail

	sectionCache  map[Section][]service.MediaItem
	sectionCursor map[Section]int

	lastPlayPosition int64
	lastReportOK     bool
	loggingEnabled   bool
	helpVisible      bool

	serverCursor     int
	serverInputs     []textinput.Model
	serverFocused    int
	editingServer    int
	serverLatencies  map[int]time.Duration
	pingInProgress   bool
	prevServerPrefix string
}

type NavState struct {
	Section    Section
	View       viewState
	Items      []service.MediaItem
	Cursor     int
	Page       int
	Title      string
	CurrentLib *service.MediaItem
}

type itemsMsg struct {
	items []service.MediaItem
	total int
	err   error
	view  *viewState
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
	err           error
}

func New(svc *service.MediaService) *Model {
	inputTextStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	inputPlaceholderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	inputPromptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	inputCursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "Search..."
	ti.CharLimit = 100
	ti.Width = 30
	ti.TextStyle = inputTextStyle
	ti.PlaceholderStyle = inputPlaceholderStyle
	ti.PromptStyle = inputPromptStyle
	ti.Cursor.Style = inputCursorStyle

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
		view:            viewState{mode: viewResume},
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

func (m *Model) loadHistory(page int) tea.Cmd {
	return func() tea.Msg {
		list, err := m.svc.GetHistory(page, m.pageSize)
		if err != nil {
			return itemsMsg{err: err}
		}
		return itemsMsg{items: list.Items, total: list.Total}
	}
}

func (m *Model) searchItems() tea.Cmd {
	return func() tea.Msg {
		list, err := m.svc.SearchWithOptions(service.SearchQuery{
			Query: m.lastSearchQuery,
			Limit: m.pageSize,
			Page:  m.page,
		})
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

func (m *Model) setFavorite(item service.MediaItem, target bool) tea.Cmd {
	return func() tea.Msg {
		result, err := m.svc.SetFavorite(item.ID, target)
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
		if width <= 0 || height <= 0 {
			return imageMsg{id: item.ID, image: ""}
		}

		urls := item.ImageURLs
		if len(urls) == 0 && item.ImageURL != "" {
			urls = []string{item.ImageURL}
		}
		if len(urls) == 0 {
			return imageMsg{id: item.ID, image: ""}
		}

		img := RenderImage(urls, width, height)
		return imageMsg{id: item.ID, image: img}
	}
}

func (m *Model) loadDetail(itemID string) tea.Cmd {
	return func() tea.Msg {
		if cached, ok := m.svc.Store().GetMediaDetail(itemID); ok {
			d := cached
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

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.coverCache = make(map[string]string)
		return m, m.loadVisibleImages()

	case tea.KeyMsg:
		if m.helpVisible {
			if msg.String() == "?" || msg.String() == "esc" {
				m.helpVisible = false
			}
			return m, nil
		}
		if m.state != StateSearching && msg.String() == "?" {
			m.helpVisible = true
			return m, nil
		}
		return m.handleKey(msg)

	case itemsMsg:
		if msg.err != nil {
			m.state = StateBrowsing
			m.keepCursor = false
			m.status = m.loadErrorText(msg.err)
		} else {
			if msg.view != nil {
				m.view = *msg.view
			}
			m.items = msg.items
			m.totalItems = msg.total
			if len(msg.items) == 0 {
				m.cursor = 0
			} else if m.keepCursor && m.cursor < len(msg.items) {
			} else if m.keepCursor {
				m.cursor = len(msg.items) - 1
			} else {
				m.cursor = 0
			}
			m.keepCursor = false
			m.state = StateBrowsing
			m.status = m.loadedItemsText(msg.total)
			if m.section == SectionResume || m.section == SectionFavorites {
				m.sectionCache[m.section] = msg.items
				m.sectionCursor[m.section] = m.cursor
			}
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
		if msg.err != nil {
			m.status = "Playback failed: " + msg.err.Error()
		} else if msg.positionSec > 0 {
			m.status = "Saved progress at " + formatDuration(msg.positionSec)
		} else {
			m.status = "Playback finished"
		}
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
	if len(m.items) == 0 || m.width <= 0 || m.height <= 0 {
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
	coverHeight := m.height - 11
	if coverHeight < 8 {
		coverHeight = m.height - 8
	}
	if coverWidth <= 0 || coverHeight <= 0 {
		return nil
	}

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
			return m, m.loadCurrentPagedSection()
		}

	case "right", "l":
		if m.cursor < len(m.items)-1 {
			m.cursor++
			return m, m.loadVisibleImages()
		} else if (m.page+1)*m.pageSize < m.totalItems {
			m.page++
			m.state = StateLoading
			return m, m.loadCurrentPagedSection()
		}

	case "enter":
		return m.selectItem()

	case "p":
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.Playable {
				return m.playItem(item, false)
			}
		}
		return m.selectItem()

	case "R":
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.Playable {
				return m.playItem(item, true)
			}
		}

	case "backspace", "esc":
		return m.goBack()

	case "1":
		return m.switchSection(SectionResume, m.loadResume)

	case "2":
		return m.switchSection(SectionFavorites, m.loadFavorites)

	case "3":
		return m.switchSection(SectionHistory, func() tea.Cmd { return m.loadHistory(0) })

	case "4", "/":
		m.state = StateSearching
		m.searchInput.SetValue(m.lastSearchQuery)
		return m, tea.Batch(m.searchInput.Focus(), textinput.Blink)

	case "f":
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			return m, m.toggleFavorite(item)
		}

	case "a":
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			return m, m.setFavorite(item, true)
		}

	case "u":
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			return m, m.setFavorite(item, false)
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
				m.page = 0
				m.state = StateLoading
				return m, m.goToSeason(item)
			}
		}

	case "S":
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.Type == "Episode" || item.Type == "Season" {
				m.pushNav()
				m.page = 0
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
		m.lastSearchQuery = strings.TrimSpace(m.searchInput.Value())
		if m.lastSearchQuery == "" {
			m.status = "Enter keyword to search"
			return m, nil
		}
		m.page = 0
		m.state = StateLoading
		m.section = SectionSearch
		m.view = viewState{mode: viewSearch}
		m.searchInput.Blur()
		return m, m.searchItems()
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m *Model) hasSearchCriteria() bool {
	return strings.TrimSpace(m.lastSearchQuery) != ""
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
