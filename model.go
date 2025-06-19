package main

import (
	"encoding/json"
	"io"
	"os"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/ai"

	"github.com/davecgh/go-spew/spew"
)

const gap = "\n\n"

const (
	typing uint = iota
	receiving
)

type Conversation struct {
	user      string
	assistant string
}

type model struct {
	state        uint
	dump         io.Writer
	textarea     textarea.Model
	viewport     viewport.Model
	err          error
	conversation []ai.AIRunParamsBodyTextGenerationMessage
}

func NewModel() model {
	var dump *os.File
	if _, ok := os.LookupEnv("DEBUG"); ok {
		var err error
		dump, err = os.OpenFile("messages.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			os.Exit(1)
		}
	}

	ta := textarea.New()
	ta.Placeholder = "Enter search term"
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 280

	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	ta.ShowLineNumbers = false

	vp := viewport.New(30, 5)
	vp.SetContent(`Welcome to Agent. Ask anything..`)

	ta.KeyMap.InsertNewline.SetEnabled(false)

	return model{
		textarea:     ta,
		dump:         dump,
		viewport:     vp,
		conversation: []ai.AIRunParamsBodyTextGenerationMessage{},
		err:          nil,
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	if m.dump != nil {
		spew.Fdump(m.dump, msg)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.textarea.SetWidth(msg.Width)
		m.viewport.Height = msg.Height - m.textarea.Height() - lipgloss.Height(gap)
		m.textarea.SetWidth(msg.Width)
		m.textarea.SetHeight(3)

		if len(m.conversation) > 0 {
			m.updateViewportContent()
		}
		m.viewport.GotoBottom()

	case ai.AIRunParamsBodyTextGenerationMessage:
		m.conversation = append(m.conversation, msg)

		m.updateViewportContent()
		m.viewport.GotoBottom()
		m.state = typing
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case typing:
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			}
		case receiving:
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "ctrl+t":
				m.state = typing
			}
		}
		switch msg.String() {
		case "enter":
			usermsg := m.textarea.Value()
			m.conversation = append(m.conversation, ai.AIRunParamsBodyTextGenerationMessage{
				Role:    cloudflare.F("user"),
				Content: cloudflare.F(usermsg),
			})
			m.updateViewportContent()
			m.viewport.GotoBottom()
			m.textarea.Reset()
			m.state = receiving
			return m, handleLLMResponse(usermsg, m.conversation)
		}
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func handleLLMResponse(msg string, c []ai.AIRunParamsBodyTextGenerationMessage) tea.Cmd {
	return func() tea.Msg {
		res, err := callAgent(msg, c)
		if err != nil {
			return err
		}

		rawBytes, err := json.Marshal(res)
		if err != nil {
			panic(err.Error())
		}

		var obj ai.AIRunResponseObject
		if res != nil {
			err = json.Unmarshal(rawBytes, &obj)
			return ai.AIRunParamsBodyTextGenerationMessage{
				Role:    cloudflare.F("assistant"),
				Content: cloudflare.F(obj.Response),
			}
			// fmt.Printf("\u001b[93mLlama\u001b[0m: %s\n", obj.Response)
		}

		return nil
	}
}

func (m *model) updateViewportContent() {
	var fullContent string
	boxWidth := max(m.viewport.Width-4, 0)
	for _, msg := range m.conversation {
		if msg.Role.String() == "user" {
			fullContent += lipgloss.NewStyle().
				MarginLeft(1).
				MarginRight(1).
				Padding(1).
				Border(lipgloss.RoundedBorder()).
				Width(boxWidth).
				Render(UserRoleStyle.Render(msg.Role.String())+": "+msg.Content.String()) + "\n"
		} else {
			fullContent += lipgloss.NewStyle().
				MarginLeft(1).
				MarginRight(1).
				Padding(1).
				Border(lipgloss.RoundedBorder()).
				Width(boxWidth).
				Render(AssistantRoleStyle.Render(msg.Role.String())+": "+msg.Content.String()) + "\n"
		}
	}
	m.viewport.SetContent(fullContent)
}

var (
	UserRoleStyle = lipgloss.NewStyle().
			Bold(true).
			Underline(true).
			Foreground(lipgloss.Color("9"))

	AssistantRoleStyle = lipgloss.NewStyle().
				Bold(true).
				Underline(true).
				Foreground(lipgloss.Color("8"))
)
