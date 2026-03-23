package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type GroupsMsgType int

const (
	GroupsMsgPlaying GroupsMsgType = iota
	GroupsMsgWaiting
	GroupsMsgInputProgress
	GroupsMsgResult
	GroupsMsgStatsReset
)

type GroupsEvent struct {
	Type          GroupsMsgType
	Level         int
	GroupSize     int
	WPM           int
	Typed         string
	Expected      string
	Got           string
	Correct       bool
	Rounds        int
	CorrectRounds int
	Accuracy      float64
}

type GroupsState int

const (
	GroupsStatePlaying GroupsState = iota
	GroupsStateWaiting
	GroupsStateResult
)

type GroupsModel struct {
	state         GroupsState
	level         int
	groupSize     int
	wpm           int
	typed         string
	expected      string
	got           string
	lastCorrect   bool
	rounds        int
	correctRounds int
	accuracy      float64
	sessionStart  time.Time
	sessionDur    time.Duration
	events        <-chan GroupsEvent
	answerCh      chan<- rune
	onQuit        func()
	onReset       func()
	width         int
	height        int
}

func NewGroupsModel(
	level, groupSize, wpm int,
	events <-chan GroupsEvent,
	answerCh chan<- rune,
	onQuit func(),
	onReset func(),
) GroupsModel {
	return GroupsModel{
		state:        GroupsStatePlaying,
		level:        level,
		groupSize:    groupSize,
		wpm:          wpm,
		sessionStart: time.Now(),
		events:       events,
		answerCh:     answerCh,
		onQuit:       onQuit,
		onReset:      onReset,
	}
}

func (m GroupsModel) Init() tea.Cmd {
	return tea.Batch(tickCmd(), waitForGroupsEvent(m.events))
}

func waitForGroupsEvent(events <-chan GroupsEvent) tea.Cmd {
	return func() tea.Msg { return <-events }
}

func (m GroupsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.onQuit != nil {
				m.onQuit()
			}
			return m, tea.Quit
		case "r":
			if m.onReset != nil {
				m.onReset()
			}
		default:
			if m.state == GroupsStateWaiting && m.answerCh != nil && len(msg.Runes) > 0 {
				r := normalizeAnswerRune(msg.Runes[0])
				if r != 0 {
					select {
					case m.answerCh <- r:
					default:
					}
				}
			}
		}

	case tickMsg:
		m.sessionDur = time.Since(m.sessionStart)
		return m, tickCmd()

	case GroupsEvent:
		cmds := []tea.Cmd{waitForGroupsEvent(m.events)}
		switch msg.Type {
		case GroupsMsgPlaying:
			m.state = GroupsStatePlaying
			m.typed = ""
			m.level = msg.Level
			m.groupSize = msg.GroupSize
			m.wpm = msg.WPM
		case GroupsMsgWaiting:
			m.state = GroupsStateWaiting
			m.typed = ""
		case GroupsMsgInputProgress:
			m.state = GroupsStateWaiting
			m.typed = msg.Typed
		case GroupsMsgResult:
			m.state = GroupsStateResult
			m.expected = msg.Expected
			m.got = msg.Got
			m.lastCorrect = msg.Correct
			m.rounds = msg.Rounds
			m.correctRounds = msg.CorrectRounds
			m.accuracy = msg.Accuracy
		case GroupsMsgStatsReset:
			m.rounds = 0
			m.correctRounds = 0
			m.accuracy = 0
			m.expected = ""
			m.got = ""
			m.typed = ""
		}
		return m, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

var (
	groupsTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	groupsDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	groupsLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	groupsCorrectStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
	groupsWrongStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	groupsBorderStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

func (m GroupsModel) View() string {
	width := m.width
	if width < 50 {
		width = 70
	}

	title := groupsTitleStyle.Render("CW Groups")
	titleBar := fmt.Sprintf(
		"%s  %s  %s",
		title,
		groupsDimStyle.Render(fmt.Sprintf("L%d  group=%d", m.level, m.groupSize)),
		groupsDimStyle.Render("[Q] quit  [R] reset"),
	)

	var status string
	switch m.state {
	case GroupsStatePlaying:
		status = groupsDimStyle.Render("Playing group...")
	case GroupsStateWaiting:
		pad := strings.Repeat("_", max(0, m.groupSize-len(m.typed)))
		status = fmt.Sprintf("Type answer: %s%s", strings.ToUpper(m.typed), pad)
	case GroupsStateResult:
		if m.lastCorrect {
			status = groupsCorrectStyle.Render("Correct")
		} else {
			status = groupsWrongStyle.Render("Wrong")
		}
	}

	resultLine := ""
	if m.state == GroupsStateResult {
		resultLine = fmt.Sprintf(
			"%s %s   %s %s",
			groupsLabelStyle.Render("Expected:"), strings.ToUpper(m.expected),
			groupsLabelStyle.Render("Got:"), strings.ToUpper(m.got),
		)
	}

	dur := m.sessionDur
	h := int(dur.Hours())
	mn := int(dur.Minutes()) % 60
	s := int(dur.Seconds()) % 60
	statsLine := fmt.Sprintf(
		"%s %d   %s %d   %s %.0f%%   %s %d WPM   %s %02d:%02d:%02d",
		groupsLabelStyle.Render("Rounds:"), m.rounds,
		groupsLabelStyle.Render("Correct:"), m.correctRounds,
		groupsLabelStyle.Render("Accuracy:"), m.accuracy*100,
		groupsLabelStyle.Render("Speed:"), m.wpm,
		groupsLabelStyle.Render("Session:"), h, mn, s,
	)

	parts := []string{
		titleBar,
		strings.Repeat("─", width-4),
		"",
		status,
	}
	if resultLine != "" {
		parts = append(parts, resultLine)
	}
	parts = append(parts,
		"",
		strings.Repeat("─", width-4),
		statsLine,
	)

	return groupsBorderStyle.Width(width - 2).Render(strings.Join(parts, "\n"))
}

func normalizeAnswerRune(r rune) rune {
	r = unicode.ToUpper(r)
	if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == ',' || r == '?' || r == '/' {
		return r
	}
	return 0
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
