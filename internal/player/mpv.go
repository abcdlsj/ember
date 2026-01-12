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
	mpvPath = findMPVPath()
}

func findMPVPath() string {
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), "Applications/mpv.app/Contents/MacOS/mpv"),
		"/Applications/mpv.app/Contents/MacOS/mpv",
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	if path, err := exec.LookPath("mpv"); err == nil {
		return path
	}
	return ""
}

func Available() bool {
	return mpvPath != ""
}

type PlayResult struct {
	Err         error
	PositionSec int64
}

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

	args := buildMPVArgs(title, subtitleURLs, urls, startPositionSec, startIndex)
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

	lastPositionSec := readPlaybackPosition(stdout, startPositionSec)
	runErr := cmd.Wait()

	return PlayResult{Err: runErr, PositionSec: lastPositionSec}
}

func buildMPVArgs(title string, subtitleURLs, urls []string, startPositionSec int64, startIndex int) []string {
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
	args = append(args, urls...)

	return args
}

var posRegex = regexp.MustCompile(`POS:(\d+(?:\.\d+)?)`)

func readPlaybackPosition(r interface{ Read([]byte) (int, error) }, fallback int64) int64 {
	var lastPos int64
	buf := make([]byte, 256)

	for {
		n, err := r.Read(buf)
		if err != nil {
			break
		}
		if pos := parsePositionFromOutput(buf[:n]); pos > 0 {
			lastPos = pos
		}
	}

	if lastPos == 0 {
		return fallback
	}
	return lastPos
}

func parsePositionFromOutput(data []byte) int64 {
	matches := posRegex.FindAllStringSubmatch(string(data), -1)
	if len(matches) == 0 {
		return 0
	}

	last := matches[len(matches)-1]
	if len(last) < 2 {
		return 0
	}

	sec, err := strconv.ParseFloat(last[1], 64)
	if err != nil {
		return 0
	}
	return int64(sec)
}
