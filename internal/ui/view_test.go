package ui

import (
	"regexp"
	"strings"
	"testing"

	"ember/internal/api"
	"ember/internal/service"
	"ember/internal/storage"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var terminalBackgroundSGR = regexp.MustCompile(`\x1b\[(?:4[0-7]|10[0-7])m`)

func TestViewFitsViewport(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"wide", 140, 42},
		{"compact", 86, 30},
		{"small", 52, 18},
		{"tiny", 32, 12},
	} {
		t.Run(size.name, func(t *testing.T) {
			model := testModel(size.width, size.height)
			view := model.View()
			if got := lipgloss.Height(view); got != size.height {
				headerHeight := minInt(2, model.height)
				bodyHeight := maxInt(1, model.height-headerHeight)
				cw, ch := model.coverFrame(model.width, bodyHeight)
				t.Fatalf("view height = %d, viewport height = %d (header=%d body=%d carousel=%d cover=%d/%d info=%d footer=%d)", got, size.height, lipgloss.Height(model.renderHeader(model.width, headerHeight)), bodyHeight, lipgloss.Height(model.renderCarousel(model.width, bodyHeight)), ch, lipgloss.Height(model.renderCover(model.items[0], cw, ch, true)), lipgloss.Height(model.renderItemInfo(model.items[0], model.width)), lipgloss.Height(model.renderMediaFooter(model.width)))
			}
			for lineNumber, line := range strings.Split(view, "\n") {
				if got := lipgloss.Width(line); got != size.width {
					t.Fatalf("line %d width = %d, want exact viewport width %d", lineNumber+1, got, size.width)
				}
			}
		})
	}
}

func TestModalViewsFitCompactViewport(t *testing.T) {
	for _, state := range []State{StateSearching, StateServerManage, StateServerEdit, StatePlaylistSelect, StatePlaylistEdit} {
		model := testModel(52, 24)
		model.state = state
		if state == StateServerEdit {
			model.initServerInputs("Cinema", "http://localhost:8096", "viewer", "")
		}
		view := model.View()
		if got := lipgloss.Height(view); got != model.height {
			t.Fatalf("state %d height = %d, want %d", state, got, model.height)
		}
		for lineNumber, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got != model.width {
				t.Fatalf("state %d line %d width = %d, want exact viewport width %d", state, lineNumber+1, got, model.width)
			}
		}
	}
}

func TestViewDoesNotOverrideTerminalBackground(t *testing.T) {
	view := testModel(140, 42).View()
	if strings.Contains(view, "\x1b[48;") || terminalBackgroundSGR.MatchString(view) {
		t.Fatal("main UI emitted a terminal background color")
	}
}

func TestSidebarUsesOneAlignmentGrid(t *testing.T) {
	model := testModel(140, 42)
	sidebar := ansi.Strip(model.renderStatus(model.sidebarWidth(), model.height-2))
	if strings.Contains(sidebar, "Browse") || strings.Contains(sidebar, "Server\n") {
		t.Fatalf("sidebar still contains redundant headings:\n%s", sidebar)
	}
	for _, expected := range []string{"EMBER", "Continue", "Favorites", "History", "Search", "Playlists", "● ", "m Servers", "? Help"} {
		if !strings.Contains(sidebar, expected) {
			t.Fatalf("sidebar is missing row %q:\n%s", expected, sidebar)
		}
	}
	if strings.Contains(sidebar, "titles") {
		t.Fatalf("sidebar still shows title count header:\n%s", sidebar)
	}
	for _, indent := range []string{"  1 Continue", "  2 Favorites"} {
		if strings.Contains(sidebar, indent) {
			t.Fatalf("sidebar rows should not be indented, got %q:\n%s", indent, sidebar)
		}
	}
	// Section rows should place the digit at column 0 and label at the right edge.
	for _, section := range []string{"1", "2", "3", "4", "5"} {
		row := ""
		for _, line := range strings.Split(sidebar, "\n") {
			trimmed := strings.TrimLeft(line, " ")
			if strings.HasPrefix(trimmed, section+" ") {
				row = trimmed
				break
			}
		}
		if row == "" {
			t.Fatalf("could not find sidebar row for section %s:\n%s", section, sidebar)
		}
		if !strings.HasPrefix(row, section) {
			t.Fatalf("section %s row should start with the key: %q", section, row)
		}
	}
}

func TestPrimaryPlayHintIsNotDuplicated(t *testing.T) {
	view := strings.ToLower(ansi.Strip(testModel(140, 42).View()))
	if count := strings.Count(view, "p play"); count != 1 {
		t.Fatalf("p play hint appears %d times, want exactly once", count)
	}
}

func TestWrapTextCapsLines(t *testing.T) {
	lines := wrapText("A deliberately long overview that should wrap without leaking into the rest of the sidebar", 18, 3)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	for _, line := range lines {
		if len([]rune(line)) > 18 {
			t.Fatalf("wrapped line exceeds width: %q", line)
		}
	}
}

func testModel(width, height int) *Model {
	store := new(storage.Store)
	svc := service.NewMediaService(api.New(""), store)
	model := New(svc)
	model.width = width
	model.height = height
	model.state = StateBrowsing
	model.status = ""
	model.items = []service.MediaItem{{
		ID:           "film-1",
		Name:         "In the Mood for Ember",
		Type:         "Movie",
		Year:         2026,
		Overview:     "A quiet film about rediscovering a personal cinema collection.",
		RunTimeTicks: 7200 * 10000000,
		Playable:     true,
		UserData: &service.UserData{
			PlaybackPositionPct: 37,
		},
	}}
	model.totalItems = 18
	return model
}
