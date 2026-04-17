package ui

import (
	"fmt"
	"strings"

	"ember/internal/player"
	"ember/internal/service"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	statusWidth := 32
	if m.width < 100 {
		statusWidth = 28
	}
	contentWidth := m.width - statusWidth

	content := m.renderCarousel(contentWidth, m.height)
	status := m.renderStatus(statusWidth, m.height)

	return lipgloss.JoinHorizontal(lipgloss.Top, status, content)
}

func (m *Model) renderCarousel(width, height int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Height(height)

	if m.helpVisible {
		return style.Align(lipgloss.Center, lipgloss.Center).Render(m.renderHelp(width - 6))
	}

	if m.state == StateServerManage {
		return style.Align(lipgloss.Center, lipgloss.Center).Render(m.renderServerManage())
	}

	if m.state == StateServerEdit {
		return style.Align(lipgloss.Center, lipgloss.Center).Render(m.renderServerEdit())
	}

	if m.state == StateSearching {
		return style.Align(lipgloss.Center, lipgloss.Center).Render(m.renderSearch())
	}

	if m.state == StateLoading {
		return style.Align(lipgloss.Center, lipgloss.Center).Render(m.spinner.View() + " Loading...")
	}

	if len(m.items) == 0 {
		parts := []string{lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(m.emptyStateText())}
		if header := m.renderContentHeader(width); header != "" {
			parts = append([]string{header}, parts...)
		}
		empty := lipgloss.JoinVertical(lipgloss.Left, parts...)
		return style.Align(lipgloss.Center, lipgloss.Center).Render(empty)
	}

	coverWidth, coverHeight := m.coverFrame(width, height)

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
		Render(fmt.Sprintf("< %d / %d >  Page %d  Total %d", m.cursor+1, len(m.items), m.page+1, m.totalItems))

	coverBlock := lipgloss.NewStyle().
		Width(width).
		Height(coverHeight).
		Align(lipgloss.Center, lipgloss.Top).
		Render(cover)

	parts := []string{coverBlock, "", info, nav}
	if header := m.renderContentHeader(width); header != "" {
		parts = append([]string{header}, parts...)
	}
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return style.Align(lipgloss.Center, lipgloss.Top).Render(content)
}

func (m *Model) renderCover(item service.MediaItem, width, height int, selected bool) string {
	if img, ok := m.coverCache[item.ID]; ok && img != "" {
		imgStyle := lipgloss.NewStyle().
			Width(width).
			MaxWidth(width).
			Align(lipgloss.Center, lipgloss.Top)
		return imgStyle.Render(img)
	}

	return m.renderPlaceholder(item, width, height, selected)
}

func (m *Model) renderPlaceholder(item service.MediaItem, width, height int, selected bool) string {
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

func (m *Model) renderItemInfo(item service.MediaItem, width int) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("255")).
		Width(width).
		Align(lipgloss.Center)

	lineStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Width(width).
		Align(lipgloss.Center)

	lines := []string{titleStyle.Render(truncateText(itemPrimaryTitle(item), width-2))}
	meta := strings.Join(itemMeta(item), "  ")
	lines = append(lines, lineStyle.Render(truncateText(meta, width-2)))

	return lipgloss.NewStyle().
		Width(width).
		Height(2).
		Align(lipgloss.Center, lipgloss.Bottom).
		Render(lipgloss.JoinVertical(lipgloss.Center, lines...))
}

func (m *Model) renderSearch() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).MarginBottom(1).Render("Search")
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	inputLabelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

	queryLine := lipgloss.JoinHorizontal(lipgloss.Left, inputLabelStyle.Render("Query:")+" ", m.searchInput.View())
	lines := []string{title, queryLine, labelStyle.Render("Search by title or keyword.")}
	if strings.TrimSpace(m.lastSearchQuery) != "" {
		lines = append(lines, labelStyle.Render(`Last query: "`+m.lastSearchQuery+`"`))
	}
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).MarginTop(1).Render(
		"[Enter] search  [Esc] cancel",
	)
	lines = append(lines, hint)
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *Model) renderServerManage() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).MarginBottom(1).Render("Server Management")

	servers := m.svc.GetServers()
	if len(servers) == 0 {
		emptyMsg := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("No servers configured")
		hint := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).MarginTop(1).Render("[a]dd  [esc] back")
		return lipgloss.JoinVertical(lipgloss.Center, title, emptyMsg, hint)
	}

	activeIdx := m.svc.Store().GetActiveServerIndex()
	activePrefix := ""
	if srv := m.svc.GetActiveServer(); srv != nil {
		activePrefix = srv.Prefix
	}

	lines := make([]string, len(servers))
	for i, srv := range servers {
		lines[i] = m.renderServerLine(i, srv, activeIdx, activePrefix)
	}

	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).MarginTop(1).Render(
		"[a]dd  [e]dit  [d]elete  [p]ing  [enter] connect  [esc] back",
	)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.JoinVertical(lipgloss.Center, title, content, hint)
}

func (m *Model) renderServerLine(idx int, srv service.ServerInfo, activeIdx int, activePrefix string) string {
	prefix := "  "
	if idx == activeIdx {
		prefix = "* "
	}

	style := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	if idx == m.serverCursor {
		style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	}

	name := srv.Name
	if name == "" {
		name = srv.URL
	}

	line := style.Render(prefix + name)

	if lat, ok := m.serverLatencies[idx]; ok {
		line += renderLatency(lat.Milliseconds())
	} else if srv.Prefix == activePrefix && m.pingInProgress {
		line += lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(" ...")
	}

	return line
}

func renderLatency(lat int64) string {
	color := "82"
	if lat > 1000 {
		color = "196"
	} else if lat > 500 {
		color = "214"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(fmt.Sprintf(" %dms", lat))
}

func (m *Model) renderServerEdit() string {
	title := "Add Server"
	if m.editingServer >= 0 {
		title = "Edit Server"
	}
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).MarginBottom(1).Render(title)

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(12)
	var fields []string
	labels := []string{"Name:", "URL:", "Username:", "Password:"}
	for i, input := range m.serverInputs {
		label := labelStyle.Render(labels[i])
		fields = append(fields, lipgloss.JoinHorizontal(lipgloss.Left, label, input.View()))
	}

	tip := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true).MarginTop(1).Render(
		"Name: same prefix = shared data (e.g. HomeNAS Main, HomeNAS Backup)",
	)

	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).MarginTop(1).Render(
		"[Tab] next  [Enter] save  [Esc] cancel",
	)

	content := lipgloss.JoinVertical(lipgloss.Left, fields...)
	return lipgloss.JoinVertical(lipgloss.Center, titleStyle, content, tip, hint)
}

func (m *Model) renderStatus(width, height int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(1, 2)

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Render("EMBER")

	divider := lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render(strings.Repeat("─", width-4))

	var serverName string
	if srv := m.svc.GetActiveServer(); srv != nil {
		serverName = srv.Name
		if serverName == "" {
			serverName = srv.URL
		}
		if len(serverName) > width-6 {
			serverName = serverName[:width-9] + "..."
		}
	} else {
		serverName = "(no server)"
	}

	sections := []struct {
		key  string
		name string
		sec  Section
	}{
		{"1", "Continue", SectionResume},
		{"2", "Favorites", SectionFavorites},
		{"3", "History", SectionHistory},
		{"4", "Search", SectionSearch},
	}

	var navItems []string
	for _, s := range sections {
		line := fmt.Sprintf(" %s  %s", s.key, s.name)
		if m.section == s.sec {
			line = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(line)
		}
		navItems = append(navItems, line)
	}

	latency := renderLatency(int64(m.latency / 1000000))

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
		dimStyle.Render(serverName),
		divider,
		dimStyle.Render("Navigation:"),
	}
	lines = append(lines, navItems...)
	lines = append(lines,
		"",
		divider,
		dimStyle.Render("Status:"),
		dimStyle.Render(" Latency:")+latency,
		dimStyle.Render(" MPV:")+mpvStatus,
		dimStyle.Render(" Log:")+logStatus,
		"",
		dimStyle.Render(m.status),
	)

	if path := m.currentBreadcrumb(); path != "" {
		lines = append(lines, dimStyle.Render(" Path:")+highlightStyle.Render(" "+truncateText(path, width-11)))
	}

	if m.cursor < len(m.items) {
		curItem := m.items[m.cursor]
		lines = append(lines, "", divider)
		lines = append(lines, highlightStyle.Render("Current:"))
		meta := strings.Join(itemMeta(curItem), " · ")
		if meta != "" {
			lines = append(lines, dimStyle.Render(truncateText(meta, width-4)))
		}
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
			lines = append(lines, highlightStyle.Render("Subtitles:"))
			subLine := strings.Join(subs, " ")
			if len(subLine) > width-4 {
				subLine = subLine[:width-4]
			}
			lines = append(lines, dimStyle.Render(subLine))
		}
	}

	if m.lastPlayPosition > 0 {
		lines = append(lines, "", divider)
		lines = append(lines, highlightStyle.Render("Last Play:"))
		lines = append(lines, dimStyle.Render(formatDuration(m.lastPlayPosition)))
		reportStatus := "OK"
		if !m.lastReportOK {
			reportStatus = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("FAIL")
		}
		lines = append(lines, dimStyle.Render("Report: ")+reportStatus)
	}

	lines = append(lines,
		"",
		divider,
		dimStyle.Render("Actions:"),
	)
	for _, action := range m.statusActions() {
		lines = append(lines, dimStyle.Render(action))
	}

	return style.Render(strings.Join(lines, "\n"))
}

func (m *Model) renderContentHeader(width int) string {
	path := m.currentBreadcrumb()
	if path == "" {
		return ""
	}
	breadcrumb := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Width(width).
		Render(path)

	return breadcrumb
}

func (m *Model) renderHelp(width int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("99")).
		Padding(1, 2)

	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117")).Render("Help"),
		"",
		"Navigation",
		"  1/2/3 switch sections",
		"  4 or / open search",
		"  left/right move or change page",
		"  enter open item",
		"  esc/backspace go back",
		"",
		"Playback",
		"  p play current item",
		"  R replay from beginning",
		"  c continuous play for episode",
		"",
		"Actions",
		"  f toggle favorite",
		"  s jump to season",
		"  S jump to series",
		"  r refresh current view",
		"  m manage servers",
		"  d toggle debug log",
		"",
		"Press ? or Esc to close",
	}

	return style.Render(strings.Join(lines, "\n"))
}

func (m *Model) currentBreadcrumb() string {
	if m.state == StateSearching {
		if strings.TrimSpace(m.lastSearchQuery) == "" {
			return ""
		}
		return `Search / "` + m.lastSearchQuery + `"`
	}
	parts := make([]string, 0, 2)
	switch m.view.mode {
	case viewSearch:
		if strings.TrimSpace(m.lastSearchQuery) != "" {
			parts = append(parts, "Search", `"`+m.lastSearchQuery+`"`)
		}
	case viewItems:
		if m.currentLib != nil && strings.TrimSpace(m.currentLib.Name) != "" {
			parts = append(parts, m.currentLib.Name)
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

func itemPrimaryTitle(item service.MediaItem) string {
	if name := strings.TrimSpace(item.Name); name != "" {
		return name
	}
	if item.Type == "Episode" && item.IndexNumber > 0 {
		return fmt.Sprintf("EP %02d", item.IndexNumber)
	}
	if item.SeasonName != "" {
		return item.SeasonName
	}
	if item.SeriesName != "" {
		return item.SeriesName
	}
	if item.Type != "" {
		return item.Type
	}
	return "Unknown"
}

func itemMeta(item service.MediaItem) []string {
	parts := []string{item.Type}
	if item.Year > 0 {
		parts = append(parts, fmt.Sprintf("%d", item.Year))
	}
	if item.RunTimeTicks > 0 {
		parts = append(parts, formatDuration(item.RunTimeTicks/10000000))
	}
	if item.UserData != nil {
		switch {
		case item.UserData.Played:
			parts = append(parts, "Played")
		case item.UserData.PlaybackPositionPct > 0:
			parts = append(parts, fmt.Sprintf("%d%% watched", item.UserData.PlaybackPositionPct))
		}
		if item.UserData.IsFavorite {
			parts = append(parts, "Favorite")
		}
	}
	return parts
}

func truncateText(text string, max int) string {
	text = strings.TrimSpace(text)
	if text == "" || max <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

func (m *Model) emptyStateText() string {
	switch m.view.mode {
	case viewResume:
		return "Nothing to continue"
	case viewFavorites:
		return "No favorites yet"
	case viewHistory:
		return "No watch history"
	case viewSearch:
		if strings.TrimSpace(m.lastSearchQuery) == "" {
			return "Enter a keyword to search"
		}
		return `No results for "` + m.lastSearchQuery + `"`
	case viewItems:
		return "Library is empty"
	case viewSeasons:
		return "No seasons"
	case viewEpisodes:
		return "No episodes"
	default:
		return "Nothing here"
	}
}

func (m *Model) loadErrorText(err error) string {
	switch m.view.mode {
	case viewSearch:
		return "Search failed: " + err.Error()
	case viewResume:
		return "Failed to load continue list: " + err.Error()
	case viewFavorites:
		return "Failed to load favorites: " + err.Error()
	case viewHistory:
		return "Failed to load history: " + err.Error()
	case viewItems:
		return "Failed to load library: " + err.Error()
	case viewSeasons:
		return "Failed to load seasons: " + err.Error()
	case viewEpisodes:
		return "Failed to load episodes: " + err.Error()
	default:
		return "Load failed: " + err.Error()
	}
}

func (m *Model) loadedItemsText(total int) string {
	switch m.view.mode {
	case viewSearch:
		return fmt.Sprintf("%d results", total)
	case viewResume:
		return fmt.Sprintf("%d in continue list", total)
	case viewFavorites:
		return fmt.Sprintf("%d favorites", total)
	case viewHistory:
		return fmt.Sprintf("%d history items", total)
	default:
		return fmt.Sprintf("%d items", total)
	}
}

func (m *Model) statusActions() []string {
	actions := []string{
		" ←→  move",
		" ↵   open",
		" esc back",
	}

	item, ok := m.currentItem()
	if ok {
		if item.Playable {
			actions = append(actions, " p   play", " R   replay")
		}
		if item.Type == "Episode" {
			actions = append(actions, " c   continuous", " s   season", " S   series")
		} else if item.Type == "Season" {
			actions = append(actions, " S   series")
		}
		actions = append(actions, " f   toggle fav")
	}

	actions = append(actions, " r   refresh", " 4,/ search", " ?   help", " q   quit")
	return actions
}

func (m *Model) currentItem() (service.MediaItem, bool) {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return service.MediaItem{}, false
	}
	return m.items[m.cursor], true
}

func (m *Model) coverFrame(width, height int) (int, int) {
	coverWidth := width - 4
	if coverWidth < 1 {
		coverWidth = 1
	}

	reserved := lipgloss.Height(m.renderContentHeader(width)) + 4

	coverHeight := height - reserved
	if coverHeight < 1 {
		coverHeight = 1
	}
	if coverHeight > height {
		coverHeight = height
	}

	return coverWidth, coverHeight
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
