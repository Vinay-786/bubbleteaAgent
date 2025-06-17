package main

func (m model) View() string {
	output := ""
	for _, msg := range m.conversation {
		role := *&msg.Role.Value
		content := *&msg.Content.Value
		output += role + ": " + content + "\n"
	}
	output += "\n" + m.textarea.View()
	return output
}
