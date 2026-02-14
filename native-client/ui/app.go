package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/martavoi/krts/native-client/internal"
)

type screen int

const (
	screenMenu screen = iota
	screenPhone
	screenCode
	screenSuccess
	screenError
	screenWhoamiToken
	screenWhoamiResult
	screenLog
)

// FlowType indicates which auth flow to run.
type FlowType int

const (
	FlowNone   FlowType = iota
	FlowSignup
	FlowSignin
	FlowWhoami
)

// AppModel is the main Bubble Tea model.
type AppModel struct {
	kratos    *internal.KratosClient
	flowType  FlowType
	screen    screen
	list      list.Model
	phoneInput textinput.Model
	codeInput  textinput.Model
	tokenInput textinput.Model

	// State
	regState   *internal.RegistrationState
	loginState *internal.LoginState

	// Results
	sessionToken string
	whoamiResult string
	errMsg       string
	width        int
	height       int

	// API log view (Bubbles viewport)
	logViewport   viewport.Model
	logCopyStatus string // "Copied to clipboard" or error, cleared on key/leave
	// Who am I result viewport for scrollable session JSON
	whoamiViewport viewport.Model
	prevScreen     screen // screen to return to when closing log
}

var listItems = []list.Item{
	item{"Sign up", "Register with phone + SMS OTP"},
	item{"Sign in", "Login with phone + SMS OTP"},
	item{"Who am I?", "Check session with token"},
	item{"Quit", "Exit the application"},
}

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

func NewAppModel(kratos *internal.KratosClient, initialFlow FlowType) AppModel {
	delegate := list.NewDefaultDelegate()
	l := list.New(listItems, delegate, 0, 0)
	l.Title = "Kratos Demo — Phone + SMS OTP"
	l.SetShowStatusBar(false)
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))

	phone := textinput.New()
	phone.Placeholder = "+15551234567"
	phone.Prompt = "Phone: "
	phone.CharLimit = 20
	phone.Width = 30

	code := textinput.New()
	code.Placeholder = "123456"
	code.Prompt = "Code: "
	code.CharLimit = 10
	code.Width = 12

	token := textinput.New()
	token.Placeholder = "session token or leave empty"
	token.Prompt = "Token: "
	token.CharLimit = 200
	token.Width = 50

	m := AppModel{
		kratos:      kratos,
		flowType:    initialFlow,
		screen:      screenMenu,
		list:        l,
		phoneInput:  phone,
		codeInput:   code,
		tokenInput:  token,
	}
	if initialFlow == FlowWhoami {
		m.screen = screenWhoamiToken
		m.tokenInput.Focus()
	}
	return m
}

func (m AppModel) Init() tea.Cmd {
	if m.flowType == FlowWhoami && m.screen == screenWhoamiToken {
		return textinput.Blink
	}
	if m.flowType == FlowSignup && m.screen == screenMenu {
		return m.cmdCreateRegFlow()
	}
	if m.flowType == FlowSignin && m.screen == screenMenu {
		return m.cmdCreateLoginFlow()
	}
	return nil
}

// Messages
type (
	regFlowCreatedMsg struct {
		state *internal.RegistrationState
		err   error
	}
	regPhoneSentMsg  struct{ err error }
	regCodeResultMsg struct {
		token string
		err   error
	}
	loginFlowCreatedMsg struct {
		state *internal.LoginState
		err   error
	}
	loginPhoneSentMsg  struct{ err error }
	loginCodeResultMsg struct {
		token string
		err   error
	}
	whoamiResultMsg struct {
		result string
		err    error
	}
	logCopyResultMsg struct{ err error }
)

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-4)
		if m.screen == screenLog {
			m = m.refreshLogViewport()
		}
		if m.screen == screenWhoamiResult {
			m = m.refreshWhoamiViewport()
		}
		return m, nil
	case tea.KeyMsg:
		if key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c"))) {
			return m, tea.Quit
		}
		// Ctrl+L: toggle API log view from any screen
		if key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+l"))) {
			if m.screen == screenLog {
				m.screen = m.prevScreen
				m.logCopyStatus = ""
				return m, nil
			}
			m.prevScreen = m.screen
			m.screen = screenLog
			m = m.refreshLogViewport()
			return m, nil
		}
		switch m.screen {
		case screenMenu:
			return m.handleMenuUpdate(msg)
		case screenPhone:
			return m.handlePhoneUpdate(msg)
		case screenCode:
			return m.handleCodeUpdate(msg)
		case screenSuccess:
			return m, tea.Quit
		case screenError:
			return m, tea.Quit
		case screenWhoamiToken:
			return m.handleWhoamiTokenUpdate(msg)
		case screenWhoamiResult:
			return m.handleWhoamiResultUpdate(msg)
		case screenLog:
			return m.handleLogUpdate(msg)
		}
	case logCopyResultMsg:
		if msg.err != nil {
			m.logCopyStatus = "Copy failed: " + msg.err.Error()
		} else {
			m.logCopyStatus = "Copied to clipboard"
		}
		return m, nil
	case regFlowCreatedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil
		}
		m.regState = msg.state
		m.screen = screenPhone
		m.phoneInput.Focus()
		return m, textinput.Blink
	case regPhoneSentMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil
		}
		m.screen = screenCode
		m.codeInput.Focus()
		m.codeInput.Reset()
		return m, textinput.Blink
	case regCodeResultMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil
		}
		m.sessionToken = msg.token
		m.screen = screenSuccess
		return m, nil
	case loginFlowCreatedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil
		}
		m.loginState = msg.state
		m.screen = screenPhone
		m.phoneInput.Focus()
		return m, textinput.Blink
	case loginPhoneSentMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil
		}
		m.screen = screenCode
		m.codeInput.Focus()
		m.codeInput.Reset()
		return m, textinput.Blink
	case loginCodeResultMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil
		}
		m.sessionToken = msg.token
		m.screen = screenSuccess
		return m, nil
	case whoamiResultMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.screen = screenError
			return m, nil
		}
		m.whoamiResult = msg.result
		m.screen = screenWhoamiResult
		m = m.refreshWhoamiViewport()
		return m, nil
	case tea.MouseMsg:
		if m.screen == screenWhoamiResult {
			var cmd tea.Cmd
			m.whoamiViewport, cmd = m.whoamiViewport.Update(msg)
			return m, cmd
		}
		return m, nil
	}
	return m, nil
}

func (m AppModel) handleMenuUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	i, ok := m.list.SelectedItem().(item)
	if !ok {
		return m, nil
	}
		if key.Matches(msg, key.NewBinding(key.WithKeys("enter"))) {
		switch strings.TrimSpace(i.title) {
		case "Sign up":
			m.flowType = FlowSignup
			return m, m.cmdCreateRegFlow()
		case "Sign in":
			m.flowType = FlowSignin
			return m, m.cmdCreateLoginFlow()
		case "Who am I?":
			m.flowType = FlowWhoami
			m.screen = screenWhoamiToken
			m.tokenInput.Reset()
			m.tokenInput.Focus()
			return m, textinput.Blink
		case "Quit":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m AppModel) handlePhoneUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, key.NewBinding(key.WithKeys("enter"))) {
		phone := strings.TrimSpace(m.phoneInput.Value())
		if phone == "" {
			return m, nil
		}
		if m.flowType == FlowSignup {
			return m, m.cmdSubmitRegPhone(phone)
		}
		return m, m.cmdSubmitLoginPhone(phone)
	}
	var cmd tea.Cmd
	m.phoneInput, cmd = m.phoneInput.Update(msg)
	return m, cmd
}

func (m AppModel) handleCodeUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, key.NewBinding(key.WithKeys("enter"))) {
		code := strings.TrimSpace(m.codeInput.Value())
		if code == "" {
			return m, nil
		}
		if m.flowType == FlowSignup {
			return m, m.cmdSubmitRegCode(code)
		}
		return m, m.cmdSubmitLoginCode(code)
	}
	var cmd tea.Cmd
	m.codeInput, cmd = m.codeInput.Update(msg)
	return m, cmd
}

func (m AppModel) handleWhoamiTokenUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, key.NewBinding(key.WithKeys("enter"))) {
		tok := strings.TrimSpace(m.tokenInput.Value())
		if tok == "" {
			m.errMsg = "session token required"
			m.screen = screenError
			return m, nil
		}
		return m, m.cmdWhoami(tok)
	}
	var cmd tea.Cmd
	m.tokenInput, cmd = m.tokenInput.Update(msg)
	return m, cmd
}

func (m AppModel) refreshLogViewport() AppModel {
	w := m.width - 4
	h := m.height - 6
	if w < 20 {
		w = 20
	}
	if h < 8 {
		h = 8
	}
	m.logViewport = viewport.New(w, h)
	m.logViewport.MouseWheelEnabled = true
	lines := internal.GetAPILogLines()
	content := strings.Join(lines, "\n")
	if content == "" {
		content = "No API requests yet.\n\nUse Sign up / Sign in / Who am I to generate traffic.\n\nPress Esc or Ctrl+L to go back."
	}
	m.logViewport.SetContent(content)
	m.logViewport.GotoBottom()
	return m
}

// refreshWhoamiViewport sizes the Who am I viewport and sets its content so the full session JSON can be scrolled.
func (m AppModel) refreshWhoamiViewport() AppModel {
	w := m.width - 4
	h := m.height - 6
	if w < 20 {
		w = 20
	}
	if h < 8 {
		h = 8
	}
	m.whoamiViewport = viewport.New(w, h)
	m.whoamiViewport.MouseWheelEnabled = true
	content := m.whoamiResult
	if content == "" {
		content = "No session data."
	}
	m.whoamiViewport.SetContent(content)
	m.whoamiViewport.GotoTop()
	return m
}

func (m AppModel) handleLogUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, key.NewBinding(key.WithKeys("esc"))) {
		m.screen = m.prevScreen
		m.logCopyStatus = ""
		return m, nil
	}
	if key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+y"))) {
		return m, copyLogToClipboardCmd()
	}
	m.logCopyStatus = "" // clear status on any other key
	var cmd tea.Cmd
	m.logViewport, cmd = m.logViewport.Update(msg)
	return m, cmd
}

func (m AppModel) handleWhoamiResultUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, key.NewBinding(key.WithKeys("esc"))) {
		m.screen = screenMenu
		return m, nil
	}
	var cmd tea.Cmd
	m.whoamiViewport, cmd = m.whoamiViewport.Update(msg)
	return m, cmd
}

func (m AppModel) View() string {
	doc := strings.Builder{}
	docStyle := lipgloss.NewStyle().Margin(1, 2)

	helpLine := lipgloss.NewStyle().Faint(true).Render("Ctrl+L API log")
	switch m.screen {
	case screenMenu:
		doc.WriteString(m.list.View())
		doc.WriteString("\n\n" + helpLine)
	case screenPhone:
		doc.WriteString("Enter your phone number (e.g. +15551234567)\n\n")
		doc.WriteString(m.phoneInput.View())
		doc.WriteString("\n\nPress Enter to submit.\n\n" + helpLine)
	case screenCode:
		doc.WriteString("Check webhook.site for the OTP code.\n\n")
		doc.WriteString(m.codeInput.View())
		doc.WriteString("\n\nPress Enter to submit.\n\n" + helpLine)
	case screenSuccess:
		doc.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true).Render("Success!\n\n"))
		doc.WriteString("Session token:\n")
		doc.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(m.sessionToken))
		doc.WriteString("\n\nPress Ctrl+C to quit.")
	case screenError:
		doc.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true).Render("Error\n\n"))
		doc.WriteString(m.errMsg)
		doc.WriteString("\n\nPress Ctrl+C to quit.")
	case screenWhoamiToken:
		doc.WriteString("Enter your session token (from signup/signin)\n\n")
		doc.WriteString(m.tokenInput.View())
		doc.WriteString("\n\nPress Enter to submit.\n\n" + helpLine)
	case screenWhoamiResult:
		doc.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true).Render("Session"))
		doc.WriteString("  ")
		doc.WriteString(lipgloss.NewStyle().Faint(true).Render("↑/↓ scroll  Esc back  Ctrl+C quit"))
		doc.WriteString("\n\n")
		doc.WriteString(m.whoamiViewport.View())
	case screenLog:
		doc.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true).Render("Kratos API log"))
		doc.WriteString("  ↑/↓ scroll  ")
		doc.WriteString(lipgloss.NewStyle().Faint(true).Render("Ctrl+Y copy  Esc or Ctrl+L back"))
		doc.WriteString("\n\n")
		doc.WriteString(m.logViewport.View())
		if m.logCopyStatus != "" {
			doc.WriteString("\n\n")
			if strings.HasPrefix(m.logCopyStatus, "Copy failed") {
				doc.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.logCopyStatus))
			} else {
				doc.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(m.logCopyStatus))
			}
		}
	}

	return docStyle.Render(doc.String())
}

// Commands that run API calls async
func (m AppModel) cmdCreateRegFlow() tea.Cmd {
	return func() tea.Msg {
		state, err := internal.CreateRegistrationFlow(context.Background(), m.kratos)
		return regFlowCreatedMsg{state: state, err: err}
	}
}

func (m AppModel) cmdSubmitRegPhone(phone string) tea.Cmd {
	return func() tea.Msg {
		err := internal.SubmitRegistrationPhone(context.Background(), m.kratos, m.regState, phone)
		return regPhoneSentMsg{err: err}
	}
}

func (m AppModel) cmdSubmitRegCode(code string) tea.Cmd {
	return func() tea.Msg {
		token, err := internal.SubmitRegistrationCode(context.Background(), m.kratos, m.regState, code)
		return regCodeResultMsg{token: token, err: err}
	}
}

func (m AppModel) cmdCreateLoginFlow() tea.Cmd {
	return func() tea.Msg {
		state, err := internal.CreateLoginFlow(context.Background(), m.kratos)
		return loginFlowCreatedMsg{state: state, err: err}
	}
}

func (m AppModel) cmdSubmitLoginPhone(phone string) tea.Cmd {
	return func() tea.Msg {
		err := internal.SubmitLoginPhone(context.Background(), m.kratos, m.loginState, phone)
		return loginPhoneSentMsg{err: err}
	}
}

func (m AppModel) cmdSubmitLoginCode(code string) tea.Cmd {
	return func() tea.Msg {
		token, err := internal.SubmitLoginCode(context.Background(), m.kratos, m.loginState, code)
		return loginCodeResultMsg{token: token, err: err}
	}
}

func (m AppModel) cmdWhoami(tok string) tea.Cmd {
	return func() tea.Msg {
		session, err := internal.WhoAmI(context.Background(), m.kratos, tok)
		if err != nil {
			return whoamiResultMsg{result: "", err: err}
		}
		return whoamiResultMsg{result: internal.FormatSession(session), err: nil}
	}
}

// copyLogToClipboardCmd copies the full API log to the system clipboard.
func copyLogToClipboardCmd() tea.Cmd {
	return func() tea.Msg {
		lines := internal.GetAPILogLines()
		content := strings.Join(lines, "\n")
		err := clipboard.WriteAll(content)
		return logCopyResultMsg{err: err}
	}
}

var _ = fmt.Sprint
