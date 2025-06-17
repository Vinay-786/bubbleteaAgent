package main

import (
	"encoding/json"
	"io"
	"os"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/ai"

	"github.com/davecgh/go-spew/spew"
)

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

	ti := textarea.New()
	ti.Placeholder = "Enter search term"
	ti.Focus()
	ti.ShowLineNumbers = false

	return model{
		textarea: ti,
		dump:     dump,
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if m.dump != nil {
		spew.Fdump(m.dump, msg)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.textarea.SetWidth(msg.Width)

	case ai.AIRunParamsBodyTextGenerationMessage:
		m.conversation = append(m.conversation, msg)
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
			m.state = receiving
			m.textarea.Reset()
			return m, handleLLMResponse(usermsg, m.conversation)
		}
	}

	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
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
