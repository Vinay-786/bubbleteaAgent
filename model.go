package main

import (
	"encoding/json"
	"io"
	"log"
	"os"

	"github.com/charmbracelet/bubbles/list"
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
	chatting uint = iota
	selectllm
)

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
	currentModel string
	modelList    list.Model
	store        *sqliteStore
}

type ModelNames struct {
	sno   int
	name  string
	alias string
}

func (m ModelNames) Title() string       { return m.alias }
func (m ModelNames) Description() string { return m.name }
func (m ModelNames) FilterValue() string { return m.alias }

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

	defaultList := list.New([]list.Item{
		ModelNames{sno: 1, name: "@cf/meta/llama-3.1-8b-instruct-fast", alias: "Llama 3.1"},
		ModelNames{sno: 2, name: "@cf/meta/llama-3.3-70b-instruct-fp8-fast", alias: "Llama 3.3"},
		ModelNames{sno: 3, name: "@cf/google/gemma-3-12b-it", alias: "Google Gemma"},
		ModelNames{sno: 4, name: "@cf/qwen/qwq-32b", alias: "Qwen"},
		ModelNames{sno: 5, name: "@cf/deepseek-ai/deepseek-r1-distill-qwen-32b", alias: "DeepSeek"},
		ModelNames{sno: 6, name: "@cf/mistralai/mistral-small-3.1-24b-instruct", alias: "Mistral"},
	}, list.NewDefaultDelegate(), 0, 0)

	defaultList.Title = "Available Models"
	defaultList.SetShowTitle(true)
	defaultList.SetFilteringEnabled(true)
	defaultList.SetShowHelp(true)

	store := &sqliteStore{}
	if err := store.Init(); err != nil {
		log.Fatal(err)
	}
	store.createTables()

	return model{
		textarea:     ta,
		dump:         dump,
		viewport:     vp,
		conversation: []ai.AIRunParamsBodyTextGenerationMessage{},
		currentModel: "@cf/meta/llama-3.1-8b-instruct-fast", //default model
		err:          nil,
		modelList:    defaultList,
		store:        store,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		tea.EnterAltScreen,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		liCmd tea.Cmd
	)

	// Always update these components
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
		m.modelList.SetWidth(msg.Width / 2)
		m.modelList.SetHeight(msg.Height)

		if len(m.conversation) > 0 {
			m.updateViewportContent()
		}
		m.viewport.GotoBottom()

	case ai.AIRunParamsBodyTextGenerationMessage:
		m.conversation = append(m.conversation, msg)
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case chatting:
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyCtrlS:
				m.store.SaveConversation(m.conversation)
			case tea.KeyCtrlE:
				m.textarea.Blur()
				m.state = selectllm
				return m, nil
			case tea.KeyEnter:
				usermsg := m.textarea.Value()
				m.conversation = append(m.conversation, ai.AIRunParamsBodyTextGenerationMessage{
					Role:    cloudflare.F("user"),
					Content: cloudflare.F(usermsg),
				})
				m.updateViewportContent()
				m.viewport.GotoBottom()
				m.textarea.Reset()
				return m, handleLLMResponse(usermsg, m.conversation, m.currentModel)
			}
		case selectllm:
			m.modelList, liCmd = m.modelList.Update(msg)
			switch msg.Type {
			case tea.KeyEsc:
				m.state = chatting
				m.textarea.Focus()
				return m, nil
			case tea.KeyEnter:
				if item, ok := m.modelList.SelectedItem().(ModelNames); ok {
					m.currentModel = item.name
					m.state = chatting
					m.textarea.Focus()
				}
				return m, nil
			}
			return m, liCmd
		}
	}

	if m.state == selectllm {
		m.modelList, liCmd = m.modelList.Update(msg)
		return m, tea.Batch(tiCmd, vpCmd, liCmd)
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func handleLLMResponse(msg string, c []ai.AIRunParamsBodyTextGenerationMessage, model string) tea.Cmd {
	return func() tea.Msg {
		res, err := callAgent(msg, c, model)
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
