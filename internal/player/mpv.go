package player

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"

	"ember/internal/logging"
)

var mpvPath string

func init() {
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), "Applications/mpv.app/Contents/MacOS/mpv"),
		"/Applications/mpv.app/Contents/MacOS/mpv",
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			mpvPath = p
			return
		}
	}

	if path, err := exec.LookPath("mpv"); err == nil {
		mpvPath = path
	}
}

func Available() bool {
	return mpvPath != ""
}

type PlayResult struct {
	Err         error
	PositionSec int64
}

var positionRegex = regexp.MustCompile(`(?:AV|A|V):\s*(\d+):(\d+):(\d+)`)

func Play(url, title string, subtitleURLs []string, startPositionSec int64) PlayResult {
	return PlayMultiple([]string{url}, title, subtitleURLs, startPositionSec, 0)
}

func PlayMultiple(urls []string, title string, subtitleURLs []string, startPositionSec int64, startIndex int) PlayResult {
	if mpvPath == "" {
		return PlayResult{Err: exec.ErrNotFound}
	}

	if len(urls) == 0 {
		return PlayResult{Err: fmt.Errorf("no URLs provided")}
	}

	args := []string{
		"--hwdec=auto",
		"--vo=gpu",
		"--fullscreen",
		"--title=" + title,
		"--slang=chi,zho,zh,chs,cht,cn,chinese",
		"--term-playing-msg=",
		"--term-status-msg=POS:${time-pos}",
		"--msg-level=all=no,statusline=status",
	}

	if startPositionSec > 0 {
		args = append(args, fmt.Sprintf("--start=%d", startPositionSec))
	}

	if startIndex > 0 {
		args = append(args, fmt.Sprintf("--playlist-start=%d", startIndex))
	}

	for _, subURL := range subtitleURLs {
		args = append(args, "--sub-file="+subURL)
	}

	for _, url := range urls {
		args = append(args, url)
	}

	logging.MPV(mpvPath, args)

	cmd := exec.Command(mpvPath, args...)
	cmd.Stdin = os.Stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return PlayResult{Err: err}
	}

	if err := cmd.Start(); err != nil {
		return PlayResult{Err: err}
	}

	var lastPositionSec int64
	buf := make([]byte, 256)
	for {
		n, err := stdout.Read(buf)
		if err != nil {
			break
		}
		if pos := parsePositionFromBytes(buf[:n]); pos > 0 {
			lastPositionSec = pos
		}
	}

	runErr := cmd.Wait()

	if lastPositionSec == 0 {
		lastPositionSec = startPositionSec
	}

	return PlayResult{Err: runErr, PositionSec: lastPositionSec}
}

var posRegex = regexp.MustCompile(`POS:(\d+):(\d+):(\d+)`)

func parsePositionFromBytes(data []byte) int64 {
	s := string(data)
	matches := posRegex.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return 0
	}
	last := matches[len(matches)-1]
	if len(last) == 4 {
		h, _ := strconv.ParseInt(last[1], 10, 64)
		m, _ := strconv.ParseInt(last[2], 10, 64)
		sec, _ := strconv.ParseInt(last[3], 10, 64)
		return h*3600 + m*60 + sec
	}
	return 0
}
