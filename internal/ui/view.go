package ui

import (
	"fmt"
	"strings"

	"ember/internal/player"
	"ember/internal/service"

	"github.com/charmbracelet/lipgloss"
)

// View renders the UI
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
		return style.Align(lipgloss.Center, lipgloss.Center).Render("No items")
	}

	coverHeight := height - 6
	coverWidth := width - 4

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

func (m *Model) renderCover(item service.MediaItem, width, height int, selected bool) string {
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
		{"1", "Resume", SectionResume},
		{"2", "Favorites", SectionFavorites},
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
			lines = append(lines, "", divider)
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

	if m.cursor < len(m.items) {
		curItem := m.items[m.cursor]
		if curItem.Type == "Episode" && curItem.SeriesName != "" {
			lines = append(lines, "", divider)
			lines = append(lines, highlightStyle.Render("Series:"))
			lines = append(lines, dimStyle.Render(curItem.SeriesName))
			if curItem.SeasonName != "" {
				lines = append(lines, dimStyle.Render(curItem.SeasonName))
			}
		}
	}

	lines = append(lines,
		"",
		divider,
		dimStyle.Render("Keys:"),
		dimStyle.Render(" ←→  move"),
		dimStyle.Render(" ↵   select"),
		dimStyle.Render(" esc back"),
		dimStyle.Render(" f   fav"),
		dimStyle.Render(" s   season"),
		dimStyle.Render(" S   series"),
		dimStyle.Render(" c   continuous"),
		dimStyle.Render(" d   debug"),
		dimStyle.Render(" r   refresh"),
		dimStyle.Render(" m   servers"),
		dimStyle.Render(" /   search"),
		dimStyle.Render(" q   quit"),
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
