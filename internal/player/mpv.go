package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"

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

type ipcEvent struct {
	Event string `json:"event"`
	Name  string `json:"name"`
	Data  any    `json:"data"`
}

func Play(url, title string, subtitleURLs []string, startPositionSec int64) PlayResult {
	return play([]string{url}, title, subtitleURLs, startPositionSec, 0, nil)
}

func PlayWithHook(url, title string, subtitleURLs []string, startPositionSec int64, onStarted func()) PlayResult {
	return play([]string{url}, title, subtitleURLs, startPositionSec, 0, onStarted)
}

func PlayMultiple(urls []string, title string, subtitleURLs []string, startPositionSec int64, startIndex int) PlayResult {
	return play(urls, title, subtitleURLs, startPositionSec, startIndex, nil)
}

func PlayMultipleWithHook(urls []string, title string, subtitleURLs []string, startPositionSec int64, startIndex int, onStarted func()) PlayResult {
	return play(urls, title, subtitleURLs, startPositionSec, startIndex, onStarted)
}

func play(urls []string, title string, subtitleURLs []string, startPositionSec int64, startIndex int, onStarted func()) PlayResult {
	if mpvPath == "" {
		return PlayResult{Err: exec.ErrNotFound}
	}
	if len(urls) == 0 {
		return PlayResult{Err: fmt.Errorf("no URLs provided")}
	}

	ipcPath := filepath.Join(os.TempDir(), fmt.Sprintf("ember-mpv-%d-%d.sock", os.Getpid(), time.Now().UnixNano()))
	_ = os.Remove(ipcPath)
	defer os.Remove(ipcPath)

	args := buildMPVArgs(title, subtitleURLs, urls, startPositionSec, startIndex, ipcPath)
	logging.MPV(mpvPath, args)

	cmd := exec.Command(mpvPath, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return PlayResult{Err: err}
	}

	if onStarted != nil {
		go onStarted()
	}

	var position atomic.Int64
	position.Store(startPositionSec)
	go observePlaybackPosition(ipcPath, &position)

	runErr := cmd.Wait()
	return PlayResult{
		Err:         runErr,
		PositionSec: position.Load(),
	}
}

func buildMPVArgs(title string, subtitleURLs, urls []string, startPositionSec int64, startIndex int, ipcPath string) []string {
	args := []string{
		"--hwdec=auto",
		"--vo=gpu",
		"--fullscreen",
		"--force-window=immediate",
		"--prefetch-playlist=yes",
		"--terminal=no",
		"--title=" + title,
		"--slang=chi,zho,zh,chs,cht,cn,chinese",
		"--input-ipc-server=" + ipcPath,
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

func observePlaybackPosition(ipcPath string, position *atomic.Int64) {
	conn, err := dialIPC(ipcPath)
	if err != nil {
		return
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(map[string]any{
		"command": []any{"observe_property", 1, "time-pos"},
	}); err != nil {
		return
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 1024), 1024*1024)
	for scanner.Scan() {
		var event ipcEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		if event.Event != "property-change" || event.Name != "time-pos" {
			continue
		}
		sec, ok := event.Data.(float64)
		if !ok || sec < 0 {
			continue
		}
		position.Store(int64(sec))
	}
}

func dialIPC(ipcPath string) (net.Conn, error) {
	deadline := time.Now().Add(3 * time.Second)
	var lastErr error

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", ipcPath, 200*time.Millisecond)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("mpv ipc not available")
	}
	return nil, lastErr
}
