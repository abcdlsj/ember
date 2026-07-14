package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSurfaceEnforcesBorderBox(t *testing.T) {
	block := newSurface(8, 3, "#123456").Render("中文 abcdefghijk\nshort")
	lines := strings.Split(block, "\n")
	if len(lines) != 3 {
		t.Fatalf("height = %d, want 3", len(lines))
	}
	for row, line := range lines {
		if width := ansi.StringWidth(line); width != 8 {
			t.Fatalf("row %d width = %d, want 8", row, width)
		}
	}
}

func TestSurfaceRestoresBackgroundAfterANSIReset(t *testing.T) {
	block := newSurface(6, 1, "#123456").Render("\x1b[31mRED\x1b[0m")
	resetWithBackground := "\x1b[0m\x1b[48;2;18;52;86m"
	if !strings.Contains(block, resetWithBackground) {
		t.Fatalf("surface did not restore its background after reset: %q", block)
	}
}

func TestJoinHorizontalPreservesRows(t *testing.T) {
	left := newSurface(3, 2, "#000000").Render("A")
	right := newSurface(4, 2, "#ffffff").Render("B")
	joined := joinHorizontal(2, left, right)
	for row, line := range strings.Split(joined, "\n") {
		if width := ansi.StringWidth(line); width != 7 {
			t.Fatalf("row %d width = %d, want 7", row, width)
		}
	}
}

func TestSpreadUsesExactWidth(t *testing.T) {
	row := spread("一部很长的电影标题", "P PLAY", 18)
	if width := ansi.StringWidth(row); width != 18 {
		t.Fatalf("width = %d, want 18", width)
	}
	if !strings.HasSuffix(row, "P PLAY") {
		t.Fatalf("right value was not preserved: %q", row)
	}
}
