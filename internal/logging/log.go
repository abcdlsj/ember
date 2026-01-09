package logging

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
)

var (
	logger  *log.Logger
	enabled = true

	homeDir, _ = os.UserHomeDir()
)

func init() {
	logFile := filepath.Join(homeDir, ".ember", "ember.log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	logger = log.NewWithOptions(f, log.Options{
		ReportTimestamp: true,
		ReportCaller:    true,
		Level:           log.DebugLevel,
	})
}

func SetEnabled(v bool) {
	enabled = v
	if enabled {
		logger.SetLevel(log.DebugLevel)
	} else {
		logger.SetLevel(log.InfoLevel)
	}
}

func IsEnabled() bool {
	return enabled
}

func MPV(path string, args []string) {
	if !enabled {
		return
	}

	logger.Debug("MPV command",
		"path", path,
		"args", args,
	)
}

func HTTP(method, url string, status int, body string) {
	if !enabled {
		return
	}

	logger.Debug("HTTP request",
		"method", method,
		"url", url,
		"status", status,
		"body", body,
	)
}
