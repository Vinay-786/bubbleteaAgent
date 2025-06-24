package main

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if m.state == selectllm {
		return lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("Choose a Model (press esc to cancel):"),
			m.modelList.View(),
		)
	}
	return fmt.Sprintf(
		"%s%s%s%s%s",
		m.currentModel,
		gap,
		m.viewport.View(),
		gap,
		m.textarea.View(),
	)
}
