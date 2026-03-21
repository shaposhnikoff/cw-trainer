package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// KochMsg types sent from the Koch session goroutine to TUI.
type KochMsgType int

const (
	KochMsgPlay    KochMsgType = iota // program is playing this symbol
	KochMsgWaiting                    // waiting for user answer
	KochMsgCorrect                    // user answered correctly
	KochMsgWrong                      // user answered wrong (with correct answer)
	KochMsgLevelUp                    // leveled up
	KochMsgAnswer                     // user typed this char (from decoder)
)

type KochEvent struct {
	Type     KochMsgType
	Symbol   rune // symbol played or correct answer
	Got      rune // what user typed (for KochMsgWrong)
	Level    int
	MaxLevel int
	Accuracy float64
	Recent   int // recent total
	Total    int // session symbols
}

type KochState int

const (
	KochStatePlaying  KochState = iota
	KochStateWaiting
	KochStateCorrect
	KochStateWrong
	KochStateLevelUp
)

type KochModel struct {
	state        KochState
	currentSym   rune
	gotSym       rune
	level        int
	maxLevel     int
	accuracy     float64
	recentTotal  int
	sessionStart time.Time
	sessionDur   time.Duration
	activeSyms   []rune
	wpm          int
	events       <-chan KochEvent
	answerCh     chan<- rune
	onQuit       func()
	width        int
	height       int
}

func NewKochModel(
	level, wpm int,
	activeSyms []rune,
	events <-chan KochEvent,
	answerCh chan<- rune,
	onQuit func(),
) KochModel {
	return KochModel{
		state:        KochStatePlaying,
		level:        level,
		maxLevel:     40,
		wpm:          wpm,
		activeSyms:   activeSyms,
		sessionStart: time.Now(),
		events:       events,
		answerCh:     answerCh,
		onQuit:       onQuit,
	}
}

func (m KochModel) Init() tea.Cmd {
	return tea.Batch(tickCmd(), waitForKochEvent(m.events))
}

func waitForKochEvent(events <-chan KochEvent) tea.Cmd {
	return func() tea.Msg { return <-events }
}

func (m KochModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.onQuit != nil {
				m.onQuit()
			}
			return m, tea.Quit
		}

	case tickMsg:
		m.sessionDur = time.Since(m.sessionStart)
		return m, tickCmd()

	case KochEvent:
		cmds := []tea.Cmd{waitForKochEvent(m.events)}
		switch msg.Type {
		case KochMsgPlay:
			m.state = KochStatePlaying
			m.currentSym = msg.Symbol
			m.level = msg.Level
			m.accuracy = msg.Accuracy
			m.recentTotal = msg.Recent
		case KochMsgWaiting:
			m.state = KochStateWaiting
		case KochMsgCorrect:
			m.state = KochStateCorrect
			m.accuracy = msg.Accuracy
			m.recentTotal = msg.Recent
		case KochMsgWrong:
			m.state = KochStateWrong
			m.gotSym = msg.Got
			m.accuracy = msg.Accuracy
			m.recentTotal = msg.Recent
		case KochMsgLevelUp:
			m.state = KochStateLevelUp
			m.level = msg.Level
			m.currentSym = msg.Symbol // new symbol being introduced
		case KochMsgAnswer:
			// forward to session goroutine
			if m.answerCh != nil {
				select {
				case m.answerCh <- msg.Symbol:
				default:
				}
			}
		}
		return m, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

var (
	kochTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	kochCorrectStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
	kochWrongStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	kochSymbolStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("226")).Padding(0, 1)
	kochDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	kochLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	kochBarStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	kochBorderStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

func (m KochModel) View() string {
	width := m.width
	if width < 50 {
		width = 70
	}

	// Title
	title := kochTitleStyle.Render("Koch Trainer")
	levelStr := fmt.Sprintf("Level %d/%d", m.level, m.maxLevel)
	titleBar := fmt.Sprintf("%s  %s  %s", title, kochDimStyle.Render(levelStr), kochDimStyle.Render("[Q] quit"))

	// Active symbols
	syms := make([]string, len(m.activeSyms))
	for i, r := range m.activeSyms {
		syms[i] = string(r)
	}
	symLine := kochLabelStyle.Render("Symbols: ") + strings.Join(syms, " ")

	// Current symbol display
	var centerLine string
	switch m.state {
	case KochStatePlaying:
		centerLine = "  playing..."
		if m.currentSym != 0 {
			centerLine = "  " + kochSymbolStyle.Render(string(unicode.ToUpper(m.currentSym))) + "  playing..."
		}
	case KochStateWaiting:
		centerLine = "  ? Your answer: _"
	case KochStateCorrect:
		centerLine = kochCorrectStyle.Render("  correct: " + string(unicode.ToUpper(m.currentSym)))
	case KochStateWrong:
		centerLine = kochWrongStyle.Render("  got: "+string(unicode.ToUpper(m.gotSym))) +
			"  " + kochLabelStyle.Render("correct: "+string(unicode.ToUpper(m.currentSym)))
	case KochStateLevelUp:
		centerLine = kochCorrectStyle.Render("  Level up!  New: ") + kochSymbolStyle.Render(string(unicode.ToUpper(m.currentSym)))
	}

	// Accuracy bar
	pct := int(m.accuracy * 100)
	barWidth := 20
	filled := barWidth * pct / 100
	if filled > barWidth {
		filled = barWidth
	}
	bar := kochBarStyle.Render(strings.Repeat("\u2588", filled)) + kochDimStyle.Render(strings.Repeat("\u2591", barWidth-filled))
	accLine := fmt.Sprintf("%s %s %d%%  (goal: 90%%)", kochLabelStyle.Render("Accuracy:"), bar, pct)

	// Progress line
	progressLine := fmt.Sprintf("%s %d / %d  until next level",
		kochLabelStyle.Render("Symbols:"), m.recentTotal, 50)

	// Time
	dur := m.sessionDur
	h := int(dur.Hours())
	mn := int(dur.Minutes()) % 60
	s := int(dur.Seconds()) % 60
	statsLine := fmt.Sprintf("%s %d WPM   %s %02d:%02d:%02d",
		kochLabelStyle.Render("Speed:"), m.wpm,
		kochLabelStyle.Render("Session:"), h, mn, s)

	inner := strings.Join([]string{
		titleBar,
		strings.Repeat("\u2500", width-4),
		"",
		symLine,
		"",
		centerLine,
		"",
		strings.Repeat("\u2500", width-4),
		accLine,
		progressLine,
		statsLine,
	}, "\n")

	return kochBorderStyle.Width(width - 2).Render(inner)
}
