package ui

import (
	"ember/internal/service"

	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the TUI application
func Run(svc *service.MediaService) error {
	p := tea.NewProgram(
		New(svc),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}
