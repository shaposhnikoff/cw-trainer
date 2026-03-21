package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Msg int

const (
	MsgChar Msg = iota
	MsgWordSpace
	MsgDitOn
	MsgDitOff
	MsgDahOn
	MsgDahOff
	MsgWPMUpdate
	MsgFreqUpdate
	MsgCurrentSeq
)

type TUIEvent struct {
	Type Msg
	Char rune
	WPM  float64
	Freq int
	Seq  string
}

type tickMsg time.Time

type Model struct {
	decoded      []rune
	currentSeq   string
	wpm          float64
	freq         int
	ditActive    bool
	dahActive    bool
	sessionStart time.Time
	sessionDur   time.Duration
	charCount    int
	wordCount    int
	events       <-chan TUIEvent
	onFreqChange func(int)
	onQuit       func()
	width        int
	height       int
}

func NewModel(freq int, wpm float64, events <-chan TUIEvent, onFreqChange func(int), onQuit func()) Model {
	return Model{
		freq:         freq,
		wpm:          wpm,
		sessionStart: time.Now(),
		events:       events,
		onFreqChange: onFreqChange,
		onQuit:       onQuit,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		waitForEvent(m.events),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func waitForEvent(events <-chan TUIEvent) tea.Cmd {
	return func() tea.Msg {
		return <-events
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.onQuit != nil {
				m.onQuit()
			}
			return m, tea.Quit
		case "+", "=":
			m.freq += 10
			if m.freq > 1200 {
				m.freq = 1200
			}
			if m.onFreqChange != nil {
				m.onFreqChange(m.freq)
			}
		case "-":
			m.freq -= 10
			if m.freq < 200 {
				m.freq = 200
			}
			if m.onFreqChange != nil {
				m.onFreqChange(m.freq)
			}
		case "r":
			m.decoded = nil
			m.charCount = 0
			m.wordCount = 0
			m.sessionStart = time.Now()
		}

	case tickMsg:
		m.sessionDur = time.Since(m.sessionStart)
		return m, tickCmd()

	case TUIEvent:
		cmds := []tea.Cmd{waitForEvent(m.events)}
		switch msg.Type {
		case MsgChar:
			m.decoded = append(m.decoded, msg.Char)
			m.charCount++
			m.currentSeq = ""
		case MsgWordSpace:
			m.decoded = append(m.decoded, ' ')
			m.wordCount++
		case MsgDitOn:
			m.ditActive = true
		case MsgDitOff:
			m.ditActive = false
		case MsgDahOn:
			m.dahActive = true
		case MsgDahOff:
			m.dahActive = false
		case MsgWPMUpdate:
			m.wpm = msg.WPM
		case MsgFreqUpdate:
			m.freq = msg.Freq
		case MsgCurrentSeq:
			m.currentSeq = msg.Seq
		}
		return m, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	borderStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	activeStyle   = lipgloss.NewStyle().Background(lipgloss.Color("205")).Foreground(lipgloss.Color("0")).Bold(true).Padding(0, 1)
	inactiveStyle = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(lipgloss.Color("244")).Padding(0, 1)
	decodedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

func (m Model) View() string {
	width := m.width
	if width < 50 {
		width = 70
	}

	// Title bar
	title := titleStyle.Render("CW Trainer")
	quit := dimStyle.Render("[Q] quit  [+/-] freq  [R] reset")
	titleBar := fmt.Sprintf("%s  %s", title, quit)

	// Paddle visualization
	ditLabel := inactiveStyle.Render(" DIT ")
	if m.ditActive {
		ditLabel = activeStyle.Render(" DIT ")
	}
	dahLabel := inactiveStyle.Render(" DAH ")
	if m.dahActive {
		dahLabel = activeStyle.Render(" DAH ")
	}
	paddleRow := fmt.Sprintf("%s  %s", ditLabel, dahLabel)

	seqDisplay := ""
	if m.currentSeq != "" {
		seqDisplay = "  " + dimStyle.Render(m.currentSeq)
	}

	// Decoded text — last 200 chars, wrapped
	decodedStr := string(m.decoded)
	if len(decodedStr) > 200 {
		decodedStr = decodedStr[len(decodedStr)-200:]
	}

	// Word-wrap decoded text
	words := strings.Fields(decodedStr)
	var lines []string
	line := ""
	maxWidth := width - 6
	for _, w := range words {
		if len(line)+len(w)+1 > maxWidth {
			lines = append(lines, line)
			line = w
		} else {
			if line == "" {
				line = w
			} else {
				line += " " + w
			}
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	// Keep last 4 lines
	if len(lines) > 4 {
		lines = lines[len(lines)-4:]
	}
	decodedDisplay := decodedStyle.Render(strings.Join(lines, "\n"))

	// Stats
	dur := m.sessionDur
	h := int(dur.Hours())
	mn := int(dur.Minutes()) % 60
	s := int(dur.Seconds()) % 60
	timeStr := fmt.Sprintf("%02d:%02d:%02d", h, mn, s)

	statsLine := fmt.Sprintf("%s %.0f WPM   %s %d Hz   %s %s",
		labelStyle.Render("Speed:"), m.wpm,
		labelStyle.Render("Freq:"), m.freq,
		labelStyle.Render("Session:"), timeStr,
	)

	charLine := fmt.Sprintf("%s %d   %s %d",
		labelStyle.Render("Chars:"), m.charCount,
		labelStyle.Render("Words:"), m.wordCount,
	)

	// Build layout
	inner := strings.Join([]string{
		titleBar,
		strings.Repeat("─", width-4),
		paddleRow + seqDisplay,
		"",
		labelStyle.Render("Decoded:"),
		decodedDisplay,
		"",
		strings.Repeat("─", width-4),
		statsLine,
		charLine,
	}, "\n")

	return borderStyle.Width(width - 2).Render(inner)
}
