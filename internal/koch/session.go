package koch

import (
	"math/rand"

	"cw-trainer/internal/decoder"
)

var KochOrder = []rune{
	'K', 'M', 'R', 'S', 'U', 'A', 'P', 'T', 'L', 'O',
	'W', 'I', '.', 'N', 'J', 'E', 'F', '0', 'Y', 'V',
	',', 'G', '5', '/', 'Q', '9', 'Z', 'H', '3', '8',
	'B', '?', '4', '2', '7', 'C', '1', 'D', '6', 'X',
}

const (
	SessionSymbols  = 50   // symbols per level check
	LevelUpAccuracy = 0.90 // 90% to advance
	MinLevel        = 2
)

type SymbolStat struct {
	Sent    int `json:"sent"`
	Correct int `json:"correct"`
}

type Session struct {
	Level       int                 `json:"level"`
	WPM         int                 `json:"wpm"`
	SymbolStats map[rune]SymbolStat `json:"symbol_stats"`
	// runtime only (not persisted)
	pending       rune
	recentCorrect int
	recentTotal   int
}

func NewSession(level, wpm int, stats map[rune]SymbolStat) *Session {
	if level < MinLevel {
		level = MinLevel
	}
	if stats == nil {
		stats = make(map[rune]SymbolStat)
	}
	return &Session{Level: level, WPM: wpm, SymbolStats: stats}
}

// ActiveSymbols returns the symbols active at current level.
func (s *Session) ActiveSymbols() []rune {
	n := s.Level
	if n > len(KochOrder) {
		n = len(KochOrder)
	}
	return KochOrder[:n]
}

// NextSymbol picks a random symbol from active set and stores it as pending.
func (s *Session) NextSymbol() rune {
	active := s.ActiveSymbols()
	s.pending = active[rand.Intn(len(active))]
	return s.pending
}

// MorseFor returns the dit/dah sequence for rune r, or nil if unknown.
func MorseFor(r rune) []decoder.Symbol {
	for pattern, ch := range morseMap() {
		if ch == r {
			syms := make([]decoder.Symbol, len(pattern))
			for i, c := range pattern {
				if c == '.' {
					syms[i] = decoder.SymDit
				} else {
					syms[i] = decoder.SymDah
				}
			}
			return syms
		}
	}
	return nil
}

// CheckAnswer checks user's answer against pending symbol.
// Updates stats. Returns true if correct.
func (s *Session) CheckAnswer(got rune) bool {
	correct := got == s.pending
	st := s.SymbolStats[s.pending]
	st.Sent++
	if correct {
		st.Correct++
		s.recentCorrect++
	}
	s.recentTotal++
	s.SymbolStats[s.pending] = st
	return correct
}

// Accuracy returns accuracy over the last SessionSymbols attempts.
func (s *Session) Accuracy() float64 {
	if s.recentTotal == 0 {
		return 0
	}
	return float64(s.recentCorrect) / float64(s.recentTotal)
}

// ShouldLevelUp returns true if accuracy >= 90% over last 50 symbols.
func (s *Session) ShouldLevelUp() bool {
	return s.recentTotal >= SessionSymbols && s.Accuracy() >= LevelUpAccuracy
}

// LevelUp advances to the next level and resets recent counters.
func (s *Session) LevelUp() {
	if s.Level < len(KochOrder) {
		s.Level++
	}
	s.recentCorrect = 0
	s.recentTotal = 0
}

// RecentTotal returns how many symbols have been tried in current level window.
func (s *Session) RecentTotal() int { return s.recentTotal }
