package ui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// surface is the only primitive allowed to place a component into the final
// terminal grid. It gives every component a strict border-box: overflow is
// clipped, missing cells are painted, and ANSI resets cannot leak the user's
// terminal background through the UI.
type surface struct {
	width      int
	height     int
	background string
}

func newSurface(width, height int, background string) surface {
	return surface{
		width:      maxInt(0, width),
		height:     maxInt(0, height),
		background: background,
	}
}

func (s surface) Render(content string) string {
	if s.width == 0 || s.height == 0 {
		return ""
	}

	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	result := make([]string, s.height)
	for row := 0; row < s.height; row++ {
		line := ""
		if row < len(lines) {
			line = ansi.Truncate(lines[row], s.width, "")
		}
		padding := strings.Repeat(" ", maxInt(0, s.width-ansi.StringWidth(line)))
		result[row] = paintBackground(line+padding, s.background)
	}
	return strings.Join(result, "\n")
}

func joinHorizontal(height int, regions ...string) string {
	rows := make([][]string, len(regions))
	for i, region := range regions {
		rows[i] = strings.Split(region, "\n")
	}
	result := make([]string, maxInt(0, height))
	for row := range result {
		var line strings.Builder
		for _, regionRows := range rows {
			if row < len(regionRows) {
				line.WriteString(regionRows[row])
			}
		}
		result[row] = line.String()
	}
	return strings.Join(result, "\n")
}

func joinVertical(regions ...string) string {
	return strings.Join(regions, "\n")
}

// spread places two values at the edges of an exact-width row. Both sides are
// ANSI-aware and the left side yields first when the terminal gets narrow.
func spread(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	right = ansi.Truncate(right, width, "")
	rightWidth := ansi.StringWidth(right)
	leftLimit := width - rightWidth
	if rightWidth > 0 && leftLimit > 0 {
		leftLimit--
	}
	left = ansi.Truncate(left, maxInt(0, leftLimit), "")
	gap := maxInt(0, width-ansi.StringWidth(left)-rightWidth)
	return left + strings.Repeat(" ", gap) + right
}

var sgrPattern = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

func paintBackground(value, color string) string {
	background := trueColorBackground(color)
	if background == "" {
		return value
	}

	painted := sgrPattern.ReplaceAllStringFunc(value, func(sequence string) string {
		params := strings.TrimSuffix(strings.TrimPrefix(sequence, "\x1b["), "m")
		if params == "" || params == "0" || hasSGRParam(params, "49") {
			return sequence + background
		}
		return sequence
	})
	return background + painted + "\x1b[0m"
}

func hasSGRParam(params, target string) bool {
	for _, param := range strings.Split(params, ";") {
		if param == target {
			return true
		}
	}
	return false
}

func trueColorBackground(color string) string {
	color = strings.TrimPrefix(color, "#")
	if len(color) != 6 {
		return ""
	}
	r, errR := strconv.ParseUint(color[0:2], 16, 8)
	g, errG := strconv.ParseUint(color[2:4], 16, 8)
	b, errB := strconv.ParseUint(color[4:6], 16, 8)
	if errR != nil || errG != nil || errB != nil {
		return ""
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}
