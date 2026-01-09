package ui

import (
	"fmt"
	"strings"
	"time"

	"ember/internal/api"
	"ember/internal/logging"
	"ember/internal/player"
	"ember/internal/storage"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
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
)

type Model struct {
	client  *api.Client
	store   *storage.Store
	width   int
	height  int
	section Section
	state   State

	items      []api.MediaItem
	totalItems int
	page       int
	pageSize   int
	cursor     int
	navStack   []NavState
	currentLib *api.MediaItem

	searchInput textinput.Model
	spinner     spinner.Model
	status      string
	latency     time.Duration

	coverCache  map[string]string
	detailCache map[string]*storage.MediaDetail

	// section 级别缓存，避免切换时重复加载
	sectionCache  map[Section][]api.MediaItem
	sectionCursor map[Section]int

	lastPlayPosition int64
	lastReportOK     bool
	loggingEnabled   bool
}

type NavState struct {
	Section  Section
	Items    []api.MediaItem
	Cursor   int
	Title    string
	ParentID string
}

type itemsMsg struct {
	items []api.MediaItem
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

func New(client *api.Client, store *storage.Store) *Model {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 100
	ti.Width = 30

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &Model{
		client:         client,
		store:          store,
		section:        SectionResume,
		state:          StateLoading,
		pageSize:       20,
		searchInput:    ti,
		spinner:        sp,
		status:         "Connecting...",
		coverCache:     make(map[string]string),
		detailCache:    make(map[string]*storage.MediaDetail),
		sectionCache:   make(map[Section][]api.MediaItem),
		sectionCursor:  make(map[Section]int),
		loggingEnabled: true,
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadResume,
		m.pingServer,
		m.spinner.Tick,
	)
}

func (m *Model) loadResume() tea.Msg {
	// 从服务器获取 Continue Watching 列表
	items, err := m.client.GetResumeItems(m.pageSize)
	return itemsMsg{items: items, total: len(items), err: err}
}

func (m *Model) loadLibraries() tea.Msg {
	items, err := m.client.GetLibraries()
	return itemsMsg{items: items, total: len(items), err: err}
}

func (m *Model) loadItems(parentID string, page int) tea.Cmd {
	return func() tea.Msg {
		items, total, err := m.client.GetItems(parentID, page*m.pageSize, m.pageSize)
		return itemsMsg{items: items, total: total, err: err}
	}
}

func (m *Model) searchItems(query string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.Search(query, 50)
		return itemsMsg{items: items, total: len(items), err: err}
	}
}

func (m *Model) loadSeasons(seriesID string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.GetSeasons(seriesID)
		return itemsMsg{items: items, total: len(items), err: err}
	}
}

func (m *Model) loadEpisodes(seriesID, seasonID string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.GetEpisodes(seriesID, seasonID)
		return itemsMsg{items: items, total: len(items), err: err}
	}
}

func (m *Model) loadFavorites() tea.Msg {
	items, err := m.client.GetFavorites(m.pageSize)
	return itemsMsg{items: items, total: len(items), err: err}
}

type favoriteMsg struct {
	itemID string
	isFav  bool
	err    error
}

func (m *Model) toggleFavorite(item api.MediaItem) tea.Cmd {
	return func() tea.Msg {
		isFav := item.UserData != nil && item.UserData.IsFavorite
		var err error
		if isFav {
			err = m.client.RemoveFavorite(item.ID)
		} else {
			err = m.client.AddFavorite(item.ID)
		}
		return favoriteMsg{itemID: item.ID, isFav: !isFav, err: err}
	}
}

func (m *Model) pingServer() tea.Msg {
	return pingMsg(m.client.Ping())
}

func (m *Model) loadImage(item api.MediaItem, width, height int) tea.Cmd {
	return func() tea.Msg {
		url := m.client.ImageURL(item.ID, 800)
		img := RenderImage(url, width, height)
		return imageMsg{id: item.ID, image: img}
	}
}

func (m *Model) loadDetail(itemID string) tea.Cmd {
	return func() tea.Msg {
		if cached, ok := m.store.GetMediaDetail(itemID); ok {
			return detailMsg{id: itemID, detail: &cached}
		}

		item, err := m.client.GetItem(itemID)
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
		m.store.SetMediaDetail(detail)
		return detailMsg{id: itemID, detail: &detail}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// 窗口变化时重新加载图片以自适应
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
			// 缓存当前 section 的数据
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
			return pingMsg(m.client.Ping())
		})

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case playDoneMsg:
		m.lastPlayPosition = msg.positionSec
		m.lastReportOK = msg.reportOK
		m.status = "Playback finished"
		// 同步更新播放进度到所有位置
		if msg.itemID != "" {
			m.syncItemState(msg.itemID, func(item *api.MediaItem) {
				if item.UserData == nil {
					item.UserData = &api.UserData{}
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
		// 同步更新所有位置的 item 状态
		m.syncItemState(msg.itemID, func(item *api.MediaItem) {
			if item.UserData == nil {
				item.UserData = &api.UserData{}
			}
			item.UserData.IsFavorite = msg.isFav
		})
		if msg.isFav {
			m.status = "Added to favorites"
		} else {
			m.status = "Removed from favorites"
		}
		// 只在 Favorites 页面取消收藏时刷新（需要从列表移除）
		if m.section == SectionFavorites && !msg.isFav {
			return m.refreshCurrentView()
		}
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

	// 计算实际的 cover 尺寸
	statusWidth := 24
	contentWidth := m.width - statusWidth
	coverWidth := contentWidth - 2
	coverHeight := m.height - 4

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
			parentID := ""
			if m.currentLib != nil {
				parentID = m.currentLib.ID
			}
			return m, m.loadItems(parentID, m.page)
		}

	case "right", "l":
		if m.cursor < len(m.items)-1 {
			m.cursor++
			return m, m.loadVisibleImages()
		} else if (m.page+1)*m.pageSize < m.totalItems {
			m.page++
			m.state = StateLoading
			parentID := ""
			if m.currentLib != nil {
				parentID = m.currentLib.ID
			}
			return m, m.loadItems(parentID, m.page)
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
		// 连播模式：播放当前季从当前集开始的所有集
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
		// 刷新当前页面
		return m.refreshCurrentView()

	case "s":
		// 跳转到当前季（从单集跳转）
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.Type == "Episode" {
				m.pushNav()
				m.state = StateLoading
				return m, m.goToSeason(item)
			}
		}

	case "S":
		// 跳转到所有季（从单集或单季跳转）
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.Type == "Episode" || item.Type == "Season" {
				m.pushNav()
				m.state = StateLoading
				return m, m.goToSeries(item)
			}
		}
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

type playDoneMsg struct {
	itemID       string
	positionSec  int64
	durationTicks int64
	reportOK     bool
}

func (m *Model) playItem(item api.MediaItem) (tea.Model, tea.Cmd) {
	fullItem, err := m.client.GetItem(item.ID)
	if err != nil || len(fullItem.MediaSources) == 0 {
		m.status = "Cannot play: no media source"
		return m, nil
	}

	ms := fullItem.MediaSources[0]
	streamURL := m.client.StreamURL(item.ID, ms.ID, ms.Container)

	var subtitleURLs []string
	for _, stream := range ms.MediaStreams {
		if stream.Type == "Subtitle" && stream.IsExternal {
			subtitleURLs = append(subtitleURLs, m.client.SubtitleURL(item.ID, ms.ID, stream.Index))
		}
	}

	// 优先读取服务器进度，fallback 到本地进度
	var startPosSec int64
	if fullItem.UserData != nil && fullItem.UserData.PlaybackPositionTicks > 0 {
		startPosSec = fullItem.UserData.PlaybackPositionTicks / 10000000
	}
	if startPosSec == 0 {
		startPosSec = m.store.GetPlaybackPosition(item.ID)
	}

	playSessionID := strings.ReplaceAll(uuid.New().String(), "-", "")

	// 上报播放开始（弱依赖，忽略错误）
	m.client.ReportPlaybackStart(item.ID, ms.ID, playSessionID, startPosSec*10000000)

	m.status = "Playing: " + item.Name

	itemID := item.ID
	mediaSourceID := ms.ID
	sessionID := playSessionID
	client := m.client
	store := m.store
	durationTicks := fullItem.RunTimeTicks

	return m, func() tea.Msg {
		result := player.Play(streamURL, item.Name, subtitleURLs, startPosSec)

		// 保存本地进度
		store.UpdatePlaybackPosition(itemID, result.PositionSec, durationTicks/10000000)

		// 上报 Emby（弱依赖，报错不影响）
		err := client.ReportPlaybackStopped(itemID, mediaSourceID, sessionID, result.PositionSec*10_000_000)
		return playDoneMsg{
			itemID:        itemID,
			positionSec:   result.PositionSec,
			durationTicks: durationTicks,
			reportOK:      err == nil,
		}
	}
}

func (m *Model) playSeasonContinuously(item api.MediaItem) tea.Cmd {
	// 获取当前季的所有剧集
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
		episodes, err := m.client.GetEpisodes(seriesID, seasonID)
		if err != nil || len(episodes) == 0 {
			return playDoneMsg{}
		}

		// 找到当前集的索引
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

		// 构建播放列表：从当前集开始的所有集
		var urls []string
		var currentSubtitleURLs []string
		var currentItem *api.MediaItem

		for i := startIdx; i < len(episodes); i++ {
			ep := episodes[i]
			epFull, err := m.client.GetItem(ep.ID)
			if err != nil || len(epFull.MediaSources) == 0 {
				continue
			}

			ms := epFull.MediaSources[0]
			url := m.client.StreamURL(ep.ID, ms.ID, ms.Container)
			urls = append(urls, url)

			// 保存当前集的字幕（第一集）
			if i == startIdx {
				currentItem = &ep
				for _, stream := range ms.MediaStreams {
					if stream.Type == "Subtitle" && stream.IsExternal {
						currentSubtitleURLs = append(currentSubtitleURLs, m.client.SubtitleURL(ep.ID, ms.ID, stream.Index))
					}
				}
			}
		}

		if len(urls) == 0 {
			return playDoneMsg{}
		}

		// 计算起始位置
		var startPosSec int64
		if currentItem != nil {
			currentFull, _ := m.client.GetItem(currentItem.ID)
			if currentFull.UserData != nil && currentFull.UserData.PlaybackPositionTicks > 0 {
				startPosSec = currentFull.UserData.PlaybackPositionTicks / 10000000
			}
			if startPosSec == 0 {
				startPosSec = m.store.GetPlaybackPosition(currentItem.ID)
			}
		}

		// 生成标题
		title := item.SeriesName
		if title == "" {
			title = item.Name
		}

		playSessionID := strings.ReplaceAll(uuid.New().String(), "-", "")

		// 上报播放开始（弱依赖，忽略错误）
		if currentItem != nil {
			currentFull, _ := m.client.GetItem(currentItem.ID)
			if len(currentFull.MediaSources) > 0 {
				m.client.ReportPlaybackStart(currentItem.ID, currentFull.MediaSources[0].ID, playSessionID, startPosSec*10000000)
			}
		}

		// 传递第一集的标题，mpv 会自动显示播放列表
		result := player.PlayMultiple(urls, title, currentSubtitleURLs, startPosSec, startIdx)

		// 保存进度（只保存当前播放的这一集）
		var currentItemID string
		var durationTicks int64
		if currentItem != nil && result.PositionSec > 0 {
			currentItemID = currentItem.ID
			currentFull, _ := m.client.GetItem(currentItem.ID)
			if currentFull != nil {
				durationTicks = currentFull.RunTimeTicks
			}
			m.store.UpdatePlaybackPosition(currentItem.ID, result.PositionSec, durationTicks/10000000)
			m.client.ReportPlaybackStopped(currentItem.ID, "", playSessionID, result.PositionSec*10000000)
		}

		return playDoneMsg{
			itemID:        currentItemID,
			positionSec:   result.PositionSec,
			durationTicks: durationTicks,
			reportOK:      result.Err == nil,
		}
	}
}

func (m *Model) refreshCurrentView() (tea.Model, tea.Cmd) {
	m.state = StateLoading
	// 清除当前 section 的缓存，强制重新加载
	delete(m.sectionCache, m.section)

	switch m.section {
	case SectionResume:
		return m, m.loadResume

	case SectionFavorites:
		return m, m.loadFavorites

	case SectionSearch:
		query := strings.TrimSpace(m.searchInput.Value())
		if query != "" {
			return m, m.searchItems(query)
		}
		m.state = StateSearching
		m.searchInput.Focus()
		return m, textinput.Blink

	default:
		// 刷新当前列表
		parentID := ""
		if m.currentLib != nil {
			parentID = m.currentLib.ID
		}
		return m, m.loadItems(parentID, m.page)
	}
}

func (m *Model) switchSection(target Section, loader func() tea.Msg) (tea.Model, tea.Cmd) {
	// 保存当前 section 的 cursor
	m.sectionCursor[m.section] = m.cursor

	m.section = target
	m.page = 0
	m.navStack = nil
	m.currentLib = nil

	// 优先使用缓存
	if cached, ok := m.sectionCache[target]; ok && len(cached) > 0 {
		m.items = cached
		m.totalItems = len(cached)
		m.cursor = m.sectionCursor[target]
		m.state = StateBrowsing
		m.status = fmt.Sprintf("%d items", len(cached))
		return m, m.loadVisibleImages()
	}

	// 无缓存则加载
	m.state = StateLoading
	return m, loader
}

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

func (m *Model) goToSeason(item api.MediaItem) tea.Cmd {
	return func() tea.Msg {
		// 如果没有 SeriesID，从 API 获取完整信息
		fullItem := &item
		if item.SeriesID == "" {
			fetched, err := m.client.GetItem(item.ID)
			if err != nil {
				return itemsMsg{err: err}
			}
			fullItem = fetched
		}

		if fullItem.SeriesID == "" {
			return itemsMsg{err: fmt.Errorf("no series info")}
		}

		seasonID := fullItem.SeasonID
		if seasonID == "" {
			seasonID = fullItem.ParentID
		}
		if seasonID == "" {
			return itemsMsg{err: fmt.Errorf("no season info")}
		}

		items, err := m.client.GetEpisodes(fullItem.SeriesID, seasonID)
		return itemsMsg{items: items, total: len(items), err: err}
	}
}

func (m *Model) goToSeries(item api.MediaItem) tea.Cmd {
	return func() tea.Msg {
		// 如果没有 SeriesID，从 API 获取完整信息
		fullItem := &item
		if item.SeriesID == "" && item.ParentID == "" {
			fetched, err := m.client.GetItem(item.ID)
			if err != nil {
				return itemsMsg{err: err}
			}
			fullItem = fetched
		}

		seriesID := fullItem.SeriesID
		if item.Type == "Season" {
			seriesID = fullItem.ParentID
			if seriesID == "" {
				seriesID = fullItem.SeriesID
			}
		}

		if seriesID == "" {
			return itemsMsg{err: fmt.Errorf("no series info")}
		}

		items, err := m.client.GetSeasons(seriesID)
		return itemsMsg{items: items, total: len(items), err: err}
	}
}

func (m *Model) pushNav() {
	m.navStack = append(m.navStack, NavState{
		Section: m.section,
		Items:   m.items,
		Cursor:  m.cursor,
		Title:   m.status,
	})
}

// syncItemState 同步更新所有位置的 item 状态（m.items, sectionCache, navStack）
func (m *Model) syncItemState(itemID string, updater func(*api.MediaItem)) {
	// 更新当前 items
	for i := range m.items {
		if m.items[i].ID == itemID {
			updater(&m.items[i])
		}
	}

	// 更新 sectionCache
	for sec := range m.sectionCache {
		for i := range m.sectionCache[sec] {
			if m.sectionCache[sec][i].ID == itemID {
				updater(&m.sectionCache[sec][i])
			}
		}
	}

	// 更新 navStack
	for i := range m.navStack {
		for j := range m.navStack[i].Items {
			if m.navStack[i].Items[j].ID == itemID {
				updater(&m.navStack[i].Items[j])
			}
		}
	}
}

func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	statusWidth := 24
	contentWidth := m.width - statusWidth

	content := m.renderCarousel(contentWidth, m.height)
	status := m.renderStatus(statusWidth, m.height)

	return lipgloss.JoinHorizontal(lipgloss.Top, status, content)
}

func (m *Model) renderCarousel(width, height int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Height(height)

	if m.state == StateSearching {
		return style.Align(lipgloss.Center, lipgloss.Center).Render(m.renderSearch())
	}

	if m.state == StateLoading {
		return style.Align(lipgloss.Center, lipgloss.Center).Render(m.spinner.View() + " Loading...")
	}

	if len(m.items) == 0 {
		return style.Align(lipgloss.Center, lipgloss.Center).Render("No items")
	}

	// info 2行 + nav 1行 + 间距 1行 = 4行
	coverHeight := height - 4
	coverWidth := width - 2

	var cover string
	if m.cursor < 0 || m.cursor >= len(m.items) {
		cover = m.renderEmptyCover(coverWidth, coverHeight)
	} else {
		cover = m.renderCover(m.items[m.cursor], coverWidth, coverHeight, true)
	}

	var info string
	if m.cursor < len(m.items) {
		item := m.items[m.cursor]
		info = m.renderItemInfo(item, width)
	}

	nav := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Align(lipgloss.Center).
		Width(width).
		Render(fmt.Sprintf("< %d / %d >  Page %d", m.cursor+1, len(m.items), m.page+1))

	content := lipgloss.JoinVertical(lipgloss.Center, cover, info, nav)
	return style.Align(lipgloss.Center, lipgloss.Bottom).Render(content)
}

func (m *Model) renderCover(item api.MediaItem, width, height int, selected bool) string {
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center)

	if img, ok := m.coverCache[item.ID]; ok && img != "" {
		imgStyle := lipgloss.NewStyle().
			Width(width).
			Height(height).
			MaxWidth(width).
			MaxHeight(height).
			Align(lipgloss.Center, lipgloss.Center)
		return style.Render(imgStyle.Render(img))
	}

	placeholder := m.renderPlaceholder(item, width, height, selected)
	return style.Render(placeholder)
}

func (m *Model) renderPlaceholder(item api.MediaItem, width, height int, selected bool) string {
	bgColor := "236"
	fgColor := "244"
	if selected {
		bgColor = "237"
		fgColor = "252"
	}

	typeLabels := map[string]string{
		"Movie":            "MOVIE",
		"Series":           "SERIES",
		"Season":           "SEASON",
		"Episode":          "EP",
		"CollectionFolder": "LIBRARY",
		"Folder":           "FOLDER",
		"BoxSet":           "BOXSET",
	}

	label := typeLabels[item.Type]
	if label == "" {
		label = item.Type
	}

	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Background(lipgloss.Color(bgColor)).
		Foreground(lipgloss.Color(fgColor)).
		Align(lipgloss.Center, lipgloss.Center)

	return style.Render(label)
}

func (m *Model) renderEmptyCover(width, height int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Height(height)

	return style.Render("")
}

func (m *Model) renderItemInfo(item api.MediaItem, width int) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("255")).
		Width(width).
		Align(lipgloss.Center)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Width(width).
		Align(lipgloss.Center)

	title := item.Name
	if item.Year > 0 {
		title = fmt.Sprintf("%s (%d)", item.Name, item.Year)
	}
	if item.IndexNumber > 0 {
		title = fmt.Sprintf("EP %02d - %s", item.IndexNumber, item.Name)
	}

	fav := ""
	if item.UserData != nil && item.UserData.IsFavorite {
		fav = " [FAV]"
	}

	subtitle := item.Type + fav

	return lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render(title),
		subtitleStyle.Render(subtitle),
	)
}

func (m *Model) renderSearch() string {
	title := lipgloss.NewStyle().Bold(true).MarginBottom(1).Render("Search")
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).MarginTop(1).Render("[Enter] search  [Esc] cancel")
	return lipgloss.JoinVertical(lipgloss.Center, title, m.searchInput.View(), hint)
}

func (m *Model) renderStatus(width, height int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(1, 1)

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Padding(0, 1).Render("EMBER")

	sections := []struct {
		key  string
		name string
		sec  Section
	}{
		{"1", "Resume", SectionResume},
		{"2", "Favorites", SectionFavorites},
	}

	var navItems []string
	for _, s := range sections {
		line := fmt.Sprintf(" %s %s", s.key, s.name)
		if m.section == s.sec {
			line = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(line)
		}
		navItems = append(navItems, line)
	}

	latencyColor := "82"
	if m.latency > 500*time.Millisecond {
		latencyColor = "214"
	}
	if m.latency > time.Second {
		latencyColor = "196"
	}

	latency := lipgloss.NewStyle().Foreground(lipgloss.Color(latencyColor)).
		Render(fmt.Sprintf(" %dms", m.latency.Milliseconds()))

	mpvStatus := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(" N/A")
	if player.Available() {
		mpvStatus = " OK"
	}

	logStatus := " OFF"
	if m.loggingEnabled {
		logStatus = " ON"
	}
	logStatus = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render(logStatus)

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))

	lines := []string{
		title,
		"",
		dimStyle.Render(" Nav:"),
	}
	lines = append(lines, navItems...)
	lines = append(lines,
		"",
		dimStyle.Render(" Latency:")+latency,
		dimStyle.Render(" MPV:")+mpvStatus,
		dimStyle.Render(" Log:")+logStatus,
		"",
		dimStyle.Render(" "+m.status),
	)

	if m.cursor < len(m.items) {
		curItem := m.items[m.cursor]
		if detail, ok := m.detailCache[curItem.ID]; ok && len(detail.Subtitles) > 0 {
			var subs []string
			for _, sub := range detail.Subtitles {
				lang := sub.Language
				if lang == "" {
					lang = "?"
				}
				ext := ""
				if sub.IsExternal {
					ext = "*"
				}
				subs = append(subs, lang+ext)
			}
			lines = append(lines, "")
			lines = append(lines, highlightStyle.Render(" Subtitles:"))
			subLine := " " + strings.Join(subs, " ")
			if len(subLine) > width-2 {
				subLine = subLine[:width-2]
			}
			lines = append(lines, dimStyle.Render(subLine))
		}
	}

	if m.lastPlayPosition > 0 {
		lines = append(lines, "")
		lines = append(lines, highlightStyle.Render(" Last Play:"))
		lines = append(lines, dimStyle.Render(fmt.Sprintf(" %s", formatDuration(m.lastPlayPosition))))
		reportStatus := "OK"
		if !m.lastReportOK {
			reportStatus = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("FAIL")
		}
		lines = append(lines, dimStyle.Render(" Report: ")+reportStatus)
	}

	// 显示系列导航信息
	if m.cursor < len(m.items) {
		curItem := m.items[m.cursor]
		if curItem.Type == "Episode" && curItem.SeriesName != "" {
			lines = append(lines, "")
			lines = append(lines, highlightStyle.Render(" Series:"))
			lines = append(lines, dimStyle.Render(" "+curItem.SeriesName))
			if curItem.SeasonName != "" {
				lines = append(lines, dimStyle.Render(" "+curItem.SeasonName))
			}
		}
	}

	lines = append(lines,
		"",
		dimStyle.Render(" Keys:"),
		dimStyle.Render(" j/k  move"),
		dimStyle.Render(" enter select"),
		dimStyle.Render(" esc  back"),
		dimStyle.Render(" f    fav"),
		dimStyle.Render(" s    season"),
		dimStyle.Render(" S    series"),
		dimStyle.Render(" c    continuous"),
		dimStyle.Render(" d    debug"),
		dimStyle.Render(" r    refresh"),
		dimStyle.Render(" /    search"),
		dimStyle.Render(" q    quit"),
	)

	return style.Render(strings.Join(lines, "\n"))
}

func formatDuration(sec int64) string {
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func Run(client *api.Client, store *storage.Store) error {
	p := tea.NewProgram(
		New(client, store),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}
