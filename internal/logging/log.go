package logging

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
)

var (
	logger      *log.Logger
	imageLogger *log.Logger
	enabled     = true

	homeDir, _ = os.UserHomeDir()
)

func init() {
	logDir := filepath.Join(homeDir, ".ember")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return
	}

	mainFile := filepath.Join(logDir, "ember.log")
	f, err := os.OpenFile(mainFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	logger = log.NewWithOptions(f, log.Options{
		ReportTimestamp: true,
		ReportCaller:    true,
		Level:           log.DebugLevel,
	})

	imageFile := filepath.Join(logDir, "image-errors.log")
	imageOutput, err := os.OpenFile(imageFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	imageLogger = log.NewWithOptions(imageOutput, log.Options{
		ReportTimestamp: true,
		ReportCaller:    true,
		Level:           log.DebugLevel,
	})
}

func SetEnabled(v bool) {
	enabled = v
	if enabled {
		if logger != nil {
			logger.SetLevel(log.DebugLevel)
		}
		if imageLogger != nil {
			imageLogger.SetLevel(log.DebugLevel)
		}
		return
	}
	if logger != nil {
		logger.SetLevel(log.InfoLevel)
	}
	if imageLogger != nil {
		imageLogger.SetLevel(log.InfoLevel)
	}
}

func IsEnabled() bool {
	return enabled
}

func MPV(path string, args []string) {
	if !enabled || logger == nil {
		return
	}

	logger.Debug("MPV command",
		"path", path,
		"args", args,
	)
}

func HTTP(method, url string, status int, body string) {
	if !enabled || logger == nil {
		return
	}

	logger.Debug("HTTP request",
		"method", method,
		"url", url,
		"status", status,
		"body", body,
	)
}

func ImageError(url string, status int, contentType string, err error) {
	if !enabled || imageLogger == nil || err == nil {
		return
	}

	imageLogger.Debug("Image request failed",
		"url", url,
		"status", status,
		"content_type", contentType,
		"error", err.Error(),
	)
}
