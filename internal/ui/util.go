package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) initServerInputs(name, url, username, password string) {
	m.serverInputs = make([]textinput.Model, 4)

	m.serverInputs[0] = textinput.New()
	m.serverInputs[0].Placeholder = "Prefix Description (e.g. HomeNAS Main)"
	m.serverInputs[0].SetValue(name)
	m.serverInputs[0].CharLimit = 50
	m.serverInputs[0].Width = 40

	m.serverInputs[1] = textinput.New()
	m.serverInputs[1].Placeholder = "http://your-server:8096"
	m.serverInputs[1].SetValue(url)
	m.serverInputs[1].CharLimit = 200
	m.serverInputs[1].Width = 40

	m.serverInputs[2] = textinput.New()
	m.serverInputs[2].Placeholder = "Username"
	m.serverInputs[2].SetValue(username)
	m.serverInputs[2].CharLimit = 50
	m.serverInputs[2].Width = 40

	m.serverInputs[3] = textinput.New()
	m.serverInputs[3].Placeholder = "Password"
	m.serverInputs[3].SetValue(password)
	m.serverInputs[3].EchoMode = textinput.EchoPassword
	m.serverInputs[3].CharLimit = 100
	m.serverInputs[3].Width = 40

	for i := range m.serverInputs {
		m.serverInputs[i].TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorText))
		m.serverInputs[i].PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorFaint))
		m.serverInputs[i].Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent))
	}

	m.serverFocused = 0
}
