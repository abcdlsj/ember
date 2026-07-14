package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"ember/internal/service"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Foreground-only terminal palette. Ember deliberately inherits the user's
// terminal background so it works with dark, light and transparent themes.
const (
	colorCanvas  = ""
	colorLine    = "240"
	colorText    = "252"
	colorMuted   = "244"
	colorFaint   = "241"
	colorAccent  = "205"
	colorAccent2 = "99"
	colorContext = "81"
	colorGood    = "82"
	colorBad     = "196"
)

var sections = []struct {
	key   string
	label string
	sec   Section
}{
	{"1", "Continue", SectionResume},
	{"2", "Favorites", SectionFavorites},
	{"3", "History", SectionHistory},
	{"4", "Search", SectionSearch},
	{"5", "Playlists", SectionPlaylists},
}

func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render("Starting Ember…")
	}

	if m.useSidebar() {
		sidebarWidth := m.sidebarWidth()
		contentWidth := maxInt(1, m.width-sidebarWidth-1)
		sidebar := newSurface(sidebarWidth, m.height, colorCanvas).Render(m.renderStatus(sidebarWidth, m.height))
		divider := newSurface(1, m.height, colorCanvas).Render(verticalRule(m.height))
		content := newSurface(contentWidth, m.height, colorCanvas).Render(m.renderCarousel(contentWidth, m.height))
		return joinHorizontal(m.height, sidebar, divider, content)
	}

	headerHeight := minInt(2, m.height)
	bodyHeight := maxInt(1, m.height-headerHeight)
	header := newSurface(m.width, headerHeight, colorCanvas).Render(m.renderHeader(m.width, headerHeight))
	body := newSurface(m.width, bodyHeight, colorCanvas).Render(m.renderCarousel(m.width, bodyHeight))
	return joinVertical(header, body)
}

func (m *Model) renderHeader(width, height int) string {
	brand := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent2)).Render("EMBER")

	server := "OFFLINE"
	if srv := m.svc.GetActiveServer(); srv != nil {
		server = srv.Name
		if server == "" {
			server = srv.URL
		}
	}
	latency := m.latency.Milliseconds()
	connection := fmt.Sprintf("%s  ·  %dms", truncateText(server, 22), latency)

	if width < 48 {
		connection = ""
	}
	right := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render(connection)
	line := spread(brand, right, width)
	line = lipgloss.NewStyle().Width(width).Render(line)

	if height <= 1 {
		return line
	}
	nav := m.renderCompactNav(width)
	return lipgloss.JoinVertical(lipgloss.Left, line, nav)
}

func (m *Model) renderCompactNav(width int) string {
	var entries []string
	for _, section := range sections {
		label := section.key + " " + section.label
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
		if section.sec == m.activeSection() {
			style = style.Bold(true).Foreground(lipgloss.Color(colorAccent))
			label = "› " + label
		} else {
			label = "  " + label
		}
		entries = append(entries, style.Render(label))
	}
	nav := strings.Join(entries, "   ")
	if lipgloss.Width(nav) > width {
		active := sectionLabel(m.activeSection())
		nav = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent)).Render("› " + active)
	}
	return lipgloss.NewStyle().Width(width).Render(nav)
}

func (m *Model) renderCarousel(width, height int) string {
	canvas := lipgloss.NewStyle().
		Width(width).
		Height(height)

	if m.helpVisible {
		return canvas.Align(lipgloss.Center, lipgloss.Center).Render(m.renderHelp(minInt(72, width-4)))
	}
	if m.state == StateServerManage {
		return canvas.Align(lipgloss.Center, lipgloss.Center).Render(m.renderServerManage())
	}
	if m.state == StateServerEdit {
		return canvas.Align(lipgloss.Center, lipgloss.Center).Render(m.renderServerEdit())
	}
	if m.state == StateSearching {
		return canvas.Align(lipgloss.Center, lipgloss.Center).Render(m.renderSearch())
	}
	if m.state == StatePlaylistSelect {
		return canvas.Align(lipgloss.Center, lipgloss.Center).Render(m.renderPlaylistSelect())
	}
	if m.state == StatePlaylistEdit {
		return canvas.Align(lipgloss.Center, lipgloss.Center).Render(m.renderPlaylistEdit())
	}
	if m.state == StateLoading {
		loading := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent)).Render(m.spinner.View()) +
			lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render("  Loading…")
		return canvas.Align(lipgloss.Center, lipgloss.Center).Render(loading)
	}

	if len(m.items) == 0 {
		empty := lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorText)).Render("Nothing here"),
			lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render(m.emptyStateText()),
			"",
			lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render("r refresh  ·  / search"),
		)
		return canvas.Align(lipgloss.Center, lipgloss.Center).Render(empty)
	}

	coverWidth, coverHeight := m.coverFrame(width, height)
	item, ok := m.currentItem()
	cover := m.renderEmptyCover(coverWidth, coverHeight)
	if ok {
		cover = m.renderCover(item, coverWidth, coverHeight, true)
	}

	coverBlock := lipgloss.NewStyle().
		Width(width).
		Height(coverHeight).
		Align(lipgloss.Center, lipgloss.Center).
		Render(cover)
	info := m.renderItemInfo(item, width)
	footer := m.renderMediaFooter(width)

	return canvas.Render(lipgloss.JoinVertical(lipgloss.Left, coverBlock, info, footer))
}

func (m *Model) renderCover(item service.MediaItem, width, height int, selected bool) string {
	if img, ok := m.coverCache[item.ID]; ok && img != "" {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			MaxWidth(width).
			Align(lipgloss.Center, lipgloss.Center).
			Render(img)
	}
	return m.renderPlaceholder(item, width, height, selected)
}

func (m *Model) renderPlaceholder(item service.MediaItem, width, height int, selected bool) string {
	typeLabels := map[string]string{
		"Movie": "FEATURE", "Series": "SERIES", "Season": "SEASON", "Episode": "EPISODE",
		"Playlist": "PLAYLIST", "CollectionFolder": "LIBRARY", "Folder": "FOLDER", "BoxSet": "COLLECTION",
	}
	label := typeLabels[item.Type]
	if label == "" {
		label = strings.ToUpper(item.Type)
	}
	glyph := "◇"
	if selected {
		glyph = "◆"
	}
	text := lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent2)).Render(glyph),
		lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render(label),
	)
	if height < 4 {
		text = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent2)).Render(glyph)
	}
	return lipgloss.NewStyle().
		Width(maxInt(1, width-2)).
		Height(maxInt(1, height-2)).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(colorLine)).
		Align(lipgloss.Center, lipgloss.Center).
		Render(text)
}

func (m *Model) renderEmptyCover(width, height int) string {
	return lipgloss.NewStyle().Width(width).Height(height).Render("")
}

func (m *Model) renderItemInfo(item service.MediaItem, width int) string {
	inner := maxInt(1, width-4)
	title := item.Name
	if item.IndexNumber > 0 {
		title = fmt.Sprintf("%02d  /  %s", item.IndexNumber, item.Name)
	}
	context := itemContext(item)
	if context != "" {
		context = strings.ToUpper(context)
	}

	titleLine := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorText)).Render(truncateText(title, inner))
	meta := strings.Join(itemMeta(item), "  ·  ")
	metaLine := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render(truncateText(meta, inner))
	if context != "" {
		metaLine = lipgloss.NewStyle().Foreground(lipgloss.Color(colorContext)).Render(truncateText(context, inner)) + "   " + metaLine
	}

	progress := ""
	if item.UserData != nil && item.UserData.PlaybackPositionPct > 0 && !item.UserData.Played {
		progress = renderProgress(inner, item.UserData.PlaybackPositionPct)
	}
	content := lipgloss.JoinVertical(lipgloss.Center, titleLine, metaLine, progress)
	return lipgloss.NewStyle().
		Width(width).
		Height(4).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)
}

func (m *Model) renderMediaFooter(width int) string {
	count := fmt.Sprintf("%02d / %02d", m.cursor+1, len(m.items))
	if m.totalItems > len(m.items) {
		count += fmt.Sprintf("  ·  PAGE %d  ·  %d TITLES", m.page+1, m.totalItems)
	}
	left := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render("← " + count + " →")
	hints := "enter open  ·  p play  ·  ? help"
	if width < 64 {
		hints = "enter open  ·  p play"
	}
	if width < 44 {
		hints = "? help"
	}
	if strings.TrimSpace(m.status) != "" {
		hints = m.status
	}
	right := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render(hints)
	separator := lipgloss.NewStyle().Foreground(lipgloss.Color(colorLine)).Render(strings.Repeat("─", width))
	row := spread(" "+left, right+" ", width)
	return lipgloss.JoinVertical(lipgloss.Left, separator, row)
}

var emberLogo = []string{
	"█▀▀ █▄▀▄█ █▀▄ █▀▀ █▀▄",
	"█▀▀ █░▀░█ █▀▄ █▀▀ █▀▄",
	"▀▀▀ ▀░░░▀ ▀▀░ ▀▀▀ ▀░▀",
}

const emberLogoWidth = 21

func (m *Model) renderStatus(width, height int) string {
	inner := maxInt(1, width-2)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	accent := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent))
	good := lipgloss.NewStyle().Foreground(lipgloss.Color(colorGood))
	key := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFaint))
	line := lipgloss.NewStyle().Foreground(lipgloss.Color(colorLine))
	divider := line.Render(strings.Repeat("─", inner))
	brandStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent2))

	var lines []string
	if inner >= emberLogoWidth {
		for _, row := range emberLogo {
			lines = append(lines, brandStyle.Render(row))
		}
	} else {
		lines = append(lines, brandStyle.Render("EMBER"))
	}
	lines = append(lines, "", divider, "")

	for _, section := range sections {
		style := dim
		if section.sec == m.activeSection() {
			style = accent
		}
		lines = append(lines, style.Render(padLeftRight(section.key, section.label, inner)))
	}

	server := "NO SERVER"
	if active := m.svc.GetActiveServer(); active != nil {
		server = active.Name
		if server == "" {
			server = active.URL
		}
	}
	latency := ""
	if m.latency > 0 {
		latency = renderLatency(m.latency.Milliseconds())
	}
	serverLine := good.Render("● ") + dim.Render(truncateText(server, maxInt(1, inner-2-lipgloss.Width(latency)-1)))
	if latency != "" {
		serverLine = spread(serverLine, latency, inner)
	}
	bottom := []string{
		divider,
		serverLine,
		"",
		key.Render("m Servers"),
		key.Render("? Help"),
	}
	padding := maxInt(1, height-len(lines)-len(bottom)-2)
	lines = append(lines, make([]string, padding)...)
	lines = append(lines, bottom...)
	return lipgloss.NewStyle().Width(inner).Margin(1, 1, 0, 1).Render(strings.Join(lines, "\n"))
}

// padLeftRight puts `left` at column 0 and `right` at the right edge of a
// width-wide row, filling the middle with spaces.
func padLeftRight(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m *Model) renderSearch() string {
	const panelWidth = 56
	inner := m.modalContentWidth(panelWidth)
	description := "Type a title or keyword"
	if strings.TrimSpace(m.lastSearchQuery) != "" {
		description = `Previous: “` + m.lastSearchQuery + `”`
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		modalTitle("Search"),
		lipgloss.NewStyle().Foreground(lipgloss.Color(colorLine)).Render(strings.Repeat("─", inner)),
		"",
		m.inputFrame(m.searchInput.View(), inner),
		"",
		muted(description),
		"",
		spread(muted("enter  search"), muted("esc  cancel"), inner),
	)
	return m.modalCard(body, panelWidth)
}

func (m *Model) renderPlaylistSelect() string {
	lines := []string{modalEyebrow("COLLECTIONS"), modalTitle("Add to playlist")}
	if m.pendingPlaylistItem != nil {
		lines = append(lines, muted(truncateText(m.pendingPlaylistItem.Name, 48)))
	}
	lines = append(lines, "")
	choices := append([]service.MediaItem{{Name: "+  Create new playlist"}}, m.playlistChoices...)
	for i, playlist := range choices {
		prefix := "  "
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
		if i == m.playlistCursor {
			prefix = "▌ "
			style = style.Bold(true).Foreground(lipgloss.Color(colorAccent2))
		}
		lines = append(lines, style.Render(prefix+truncateText(playlist.Name, 46)))
	}
	lines = append(lines, "", modalHint("ENTER  CHOOSE", "N  NEW", "ESC  CANCEL"))
	return m.modalCard(strings.Join(lines, "\n"), 56)
}

func (m *Model) renderPlaylistEdit() string {
	title := "Create a playlist"
	if m.editingPlaylistID != "" {
		title = "Rename playlist"
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		modalEyebrow("COLLECTIONS"), modalTitle(title), "",
		m.inputFrame(m.playlistInput.View(), 48), "",
		modalHint("ENTER  SAVE", "ESC  CANCEL"),
	)
	return m.modalCard(body, 56)
}

func (m *Model) renderServerManage() string {
	lines := []string{modalEyebrow("CONNECTIONS"), modalTitle("Media servers"), ""}
	servers := m.svc.GetServers()
	if len(servers) == 0 {
		lines = append(lines, muted("No servers configured."), "")
	} else {
		activeIdx := m.svc.Store().GetActiveServerIndex()
		activePrefix := ""
		if srv := m.svc.GetActiveServer(); srv != nil {
			activePrefix = srv.Prefix
		}
		for i, srv := range servers {
			lines = append(lines, m.renderServerLine(i, srv, activeIdx, activePrefix))
		}
		lines = append(lines, "")
	}
	lines = append(lines, modalHint("ENTER  CONNECT", "A  ADD", "E  EDIT", "D  DELETE", "P  PING", "ESC  BACK"))
	return m.modalCard(strings.Join(lines, "\n"), 64)
}

func (m *Model) renderServerLine(idx int, srv service.ServerInfo, activeIdx int, activePrefix string) string {
	name := srv.Name
	if name == "" {
		name = srv.URL
	}
	marker := "  "
	if idx == activeIdx {
		marker = "● "
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	if idx == m.serverCursor {
		marker = "▌ "
		style = style.Bold(true).Foreground(lipgloss.Color(colorAccent2))
	}
	line := style.Render(marker + truncateText(name, 42))
	if lat, ok := m.serverLatencies[idx]; ok {
		line += "  " + renderLatency(lat.Milliseconds())
	} else if srv.Prefix == activePrefix && m.pingInProgress {
		line += muted("  PINGING…")
	}
	return line
}

func renderLatency(lat int64) string {
	color := colorGood
	if lat > 1000 {
		color = colorBad
	} else if lat > 500 {
		color = colorAccent
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(fmt.Sprintf("%dms", lat))
}

func (m *Model) renderServerEdit() string {
	title := "Add a server"
	if m.editingServer >= 0 {
		title = "Edit server"
	}
	labels := []string{"DISPLAY NAME", "SERVER URL", "USERNAME", "PASSWORD"}
	lines := []string{modalEyebrow("CONNECTIONS"), modalTitle(title), ""}
	for i, input := range m.serverInputs {
		lines = append(lines, modalEyebrow(labels[i]), m.inputFrame(input.View(), 48), "")
	}
	lines = append(lines, muted("Matching name prefixes share local metadata."), "", modalHint("TAB  NEXT", "ENTER  SAVE", "ESC  CANCEL"))
	return m.modalCard(strings.Join(lines, "\n"), 58)
}

func (m *Model) renderHelp(width int) string {
	columns := lipgloss.JoinHorizontal(lipgloss.Top,
		helpColumn("NAVIGATE", []string{"← → / H L   Move", "ENTER       Open", "ESC         Back", "1—5         Sections", "/           Search"}, 25),
		helpColumn("PLAYBACK", []string{"P           Play", "R           Restart", "C           Continue season", "F           Favorite", "A           Add to playlist"}, 27),
	)
	manage := helpColumn("MANAGE", []string{"N / E / X   New · Rename · Delete", "S / ⇧S      Season · Series", "M           Servers", "R           Refresh", "Q           Quit"}, 52)
	body := lipgloss.JoinVertical(lipgloss.Left,
		modalEyebrow("COMMAND PALETTE"), modalTitle("Keyboard shortcuts"), "", columns, "", manage, "", modalHint("? / ESC  CLOSE"),
	)
	return m.modalCard(body, minInt(64, maxInt(36, width)))
}

func helpColumn(title string, rows []string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, append([]string{modalEyebrow(title), ""}, rows...)...))
}

func (m *Model) activeSection() Section {
	if m.state == StateSearching {
		return SectionSearch
	}
	return m.section
}

func (m *Model) currentBreadcrumb() string {
	if m.state == StateSearching {
		if strings.TrimSpace(m.lastSearchQuery) == "" {
			return ""
		}
		return `Search / “` + m.lastSearchQuery + `”`
	}
	parts := make([]string, 0, 2)
	switch m.view.mode {
	case viewSearch:
		if strings.TrimSpace(m.lastSearchQuery) != "" {
			parts = append(parts, "Search", `“`+m.lastSearchQuery+`”`)
		}
	case viewItems:
		if m.currentLib != nil && strings.TrimSpace(m.currentLib.Name) != "" {
			parts = append(parts, m.currentLib.Name)
		}
	case viewPlaylists:
		parts = append(parts, "Playlists")
	case viewPlaylistItems:
		parts = append(parts, "Playlists")
		if strings.TrimSpace(m.view.playlistName) != "" {
			parts = append(parts, m.view.playlistName)
		} else if name := m.svc.GetPlaylistName(m.view.playlistID); name != "" {
			parts = append(parts, name)
		}
	case viewSeasons:
		if len(m.items) > 0 && strings.TrimSpace(m.items[0].SeriesName) != "" {
			parts = append(parts, m.items[0].SeriesName)
		}
	case viewEpisodes:
		if len(m.items) > 0 {
			if strings.TrimSpace(m.items[0].SeriesName) != "" {
				parts = append(parts, m.items[0].SeriesName)
			}
			if strings.TrimSpace(m.items[0].SeasonName) != "" {
				parts = append(parts, m.items[0].SeasonName)
			}
		}
	}
	return strings.Join(parts, " / ")
}

func itemContext(item service.MediaItem) string {
	if item.Type == "Episode" {
		parts := make([]string, 0, 2)
		if strings.TrimSpace(item.SeriesName) != "" {
			parts = append(parts, item.SeriesName)
		}
		if strings.TrimSpace(item.SeasonName) != "" {
			parts = append(parts, item.SeasonName)
		}
		return strings.Join(parts, " / ")
	}
	if item.Type == "Season" && strings.TrimSpace(item.SeriesName) != "" {
		return item.SeriesName
	}
	return ""
}

func itemMeta(item service.MediaItem) []string {
	parts := []string{strings.ToUpper(item.Type)}
	if item.Type == "Playlist" && strings.TrimSpace(item.Overview) != "" {
		parts = append(parts, item.Overview)
	}
	if item.Year > 0 {
		parts = append(parts, fmt.Sprintf("%d", item.Year))
	}
	if item.RunTimeTicks > 0 {
		parts = append(parts, formatDuration(item.RunTimeTicks/10000000))
	}
	if item.UserData != nil {
		switch {
		case item.UserData.Played:
			parts = append(parts, "WATCHED")
		case item.UserData.PlaybackPositionPct > 0:
			parts = append(parts, fmt.Sprintf("%d%% WATCHED", item.UserData.PlaybackPositionPct))
		}
		if item.UserData.IsFavorite {
			parts = append(parts, "♥ FAVORITE")
		}
	}
	return parts
}

func truncateText(value string, max int) string {
	value = strings.TrimSpace(value)
	if value == "" || max <= 0 {
		return ""
	}
	if ansi.StringWidth(value) <= max {
		return value
	}
	if max == 1 {
		return "…"
	}
	return ansi.Truncate(value, max, "…")
}

func wrapText(value string, width, maxLines int) []string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" || width <= 0 || maxLines <= 0 {
		return nil
	}
	var lines []string
	for len(value) > 0 && len(lines) < maxLines {
		if utf8.RuneCountInString(value) <= width {
			lines = append(lines, value)
			break
		}
		runes := []rune(value)
		cut := width
		for i := width; i > width/2; i-- {
			if runes[i] == ' ' {
				cut = i
				break
			}
		}
		lines = append(lines, strings.TrimSpace(string(runes[:cut])))
		value = strings.TrimSpace(string(runes[cut:]))
	}
	if value != "" && len(lines) == maxLines && len(lines) > 0 {
		lines[len(lines)-1] = truncateText(lines[len(lines)-1], maxInt(1, width-1)) + "…"
	}
	return lines
}

func (m *Model) emptyStateText() string {
	switch m.view.mode {
	case viewResume:
		return "Nothing in progress. Start something excellent."
	case viewFavorites:
		return "Favorite a title and it will live here."
	case viewHistory:
		return "Your watch history is empty."
	case viewSearch:
		if strings.TrimSpace(m.lastSearchQuery) == "" {
			return "Enter a title or keyword to begin."
		}
		return `No matches for “` + m.lastSearchQuery + `”.`
	case viewItems:
		return "This library has no visible titles."
	case viewPlaylists:
		return "Create a playlist to build your first collection."
	case viewPlaylistItems:
		return "This playlist is waiting for its first title."
	case viewSeasons:
		return "No seasons are available."
	case viewEpisodes:
		return "No episodes are available."
	default:
		return "Nothing here yet."
	}
}

func (m *Model) loadErrorText(err error) string {
	labels := map[viewMode]string{
		viewSearch: "Search failed", viewResume: "Could not load Continue", viewFavorites: "Could not load Favorites",
		viewHistory: "Could not load History", viewItems: "Could not load library", viewPlaylists: "Could not load playlists",
		viewPlaylistItems: "Could not load playlist", viewSeasons: "Could not load seasons", viewEpisodes: "Could not load episodes",
	}
	label := labels[m.view.mode]
	if label == "" {
		label = "Load failed"
	}
	return label + ": " + err.Error()
}

func (m *Model) statusActions() []string {
	actions := []string{"← →   Move", "↵     Open", "ESC   Back"}
	item, ok := m.currentItem()
	if ok {
		if item.Playable {
			actions = append(actions, "P     Play", "R     Restart")
		}
		if item.Type == "Episode" {
			actions = append(actions, "C     Play season", "S/⇧S  Season / series")
		} else if item.Type == "Season" {
			actions = append(actions, "⇧S    Open series")
		}
		if item.Type != "Playlist" {
			actions = append(actions, "F     Favorite")
		}
	}
	if m.view.mode == viewPlaylists {
		actions = append(actions, "N/E/X New / edit / delete")
	} else if m.view.mode == viewPlaylistItems {
		actions = append(actions, "A/D   Add / remove", "⇧P    Play playlist")
	} else if ok && item.Playable {
		actions = append(actions, "A     Add to playlist")
	}
	actions = append(actions, "?     All shortcuts")
	return actions
}

func (m *Model) currentItem() (service.MediaItem, bool) {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return service.MediaItem{}, false
	}
	return m.items[m.cursor], true
}

func (m *Model) useSidebar() bool { return m.width >= 104 && m.height >= 22 }

func (m *Model) sidebarWidth() int {
	if m.width >= 150 {
		return 24
	}
	return 20
}

func (m *Model) mediaViewport() (int, int) {
	if m.useSidebar() {
		return maxInt(1, m.width-m.sidebarWidth()-1), m.height
	}
	return m.width, maxInt(1, m.height-minInt(2, m.height))
}

func (m *Model) coverFrame(width, height int) (int, int) {
	coverWidth := maxInt(1, width-2)
	coverHeight := maxInt(1, height-6)
	return coverWidth, coverHeight
}

func sectionLabel(section Section) string {
	for _, item := range sections {
		if item.sec == section {
			return item.label
		}
	}
	return "EMBER"
}

func renderProgress(width, pct int) string {
	width = maxInt(4, width)
	pct = maxInt(0, minInt(100, pct))
	filled := width * pct / 100
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent2)).Render(strings.Repeat("━", filled)) +
		lipgloss.NewStyle().Foreground(lipgloss.Color(colorLine)).Render(strings.Repeat("━", width-filled))
}

func verticalRule(height int) string {
	if height <= 0 {
		return ""
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorLine)).Render(
		strings.TrimSuffix(strings.Repeat("│\n", height), "\n"),
	)
}

// modalContentWidth is the usable text width inside a modal card — the outer
// Width() minus 2×padding, so callers can size dividers/inputs to this value
// without wrapping.
func (m *Model) modalContentWidth(preferredWidth int) int {
	viewportWidth, _ := m.mediaViewport()
	outerWidth := minInt(preferredWidth, maxInt(12, viewportWidth-4))
	return maxInt(6, outerWidth-6)
}

func (m *Model) modalCard(content string, preferredWidth int) string {
	inner := m.modalContentWidth(preferredWidth)
	return lipgloss.NewStyle().
		Width(inner + 4).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorLine)).
		Foreground(lipgloss.Color(colorText)).
		Render(content)
}

func modalEyebrow(value string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent)).Render(value)
}

func modalTitle(value string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorText)).Render(value)
}

func modalHint(values ...string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render(strings.Join(values, "   "))
}

func muted(value string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render(value)
}

func (m *Model) inputFrame(value string, preferredWidth int) string {
	viewportWidth, _ := m.mediaViewport()
	outerWidth := minInt(preferredWidth, maxInt(8, viewportWidth-10))
	return lipgloss.NewStyle().
		Width(maxInt(4, outerWidth-4)).
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(colorLine)).
		Render(value)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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
