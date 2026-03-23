package groups

import (
	"math/rand"
	"testing"
)

func TestNewTrainerValidation(t *testing.T) {
	if _, err := NewTrainer(Config{GroupSize: 0, Symbols: []rune{'A'}}); err == nil {
		t.Fatalf("expected error for group size <= 0")
	}
	if _, err := NewTrainer(Config{GroupSize: 5, Symbols: nil}); err == nil {
		t.Fatalf("expected error for empty symbols")
	}
}

func TestNextPromptUsesConfiguredAlphabetAndSize(t *testing.T) {
	tr, err := NewTrainer(Config{
		GroupSize: 5,
		Symbols:   []rune{'A', 'B', 'C'},
		Rand:      rand.New(rand.NewSource(1)),
	})
	if err != nil {
		t.Fatalf("NewTrainer() error: %v", err)
	}

	p := tr.NextPrompt()
	if len(p) != 5 {
		t.Fatalf("expected prompt len 5, got %d", len(p))
	}

	allowed := map[rune]bool{'A': true, 'B': true, 'C': true}
	for _, r := range p {
		if !allowed[r] {
			t.Fatalf("unexpected symbol in prompt: %q", r)
		}
	}
}

func TestEvaluateStrictWholeGroup(t *testing.T) {
	tr, err := NewTrainer(Config{
		GroupSize: 5,
		Symbols:   []rune{'A', 'B', 'C', 'D', 'E'},
		Rand:      rand.New(rand.NewSource(1)),
	})
	if err != nil {
		t.Fatalf("NewTrainer() error: %v", err)
	}

	cases := []struct {
		name    string
		prompt  []rune
		answer  []rune
		correct bool
	}{
		{name: "exact", prompt: []rune("ABCDE"), answer: []rune("ABCDE"), correct: true},
		{name: "wrong-position", prompt: []rune("ABCDE"), answer: []rune("ABCED"), correct: false},
		{name: "short", prompt: []rune("ABCDE"), answer: []rune("ABCD"), correct: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			round := tr.Evaluate(tc.prompt, tc.answer)
			if round.Correct != tc.correct {
				t.Fatalf("Evaluate() correct=%v, want %v", round.Correct, tc.correct)
			}
		})
	}
}

func TestStatsUpdateAndReset(t *testing.T) {
	tr, err := NewTrainer(Config{
		GroupSize: 3,
		Symbols:   []rune{'A', 'B', 'C'},
		Rand:      rand.New(rand.NewSource(1)),
	})
	if err != nil {
		t.Fatalf("NewTrainer() error: %v", err)
	}

	tr.Evaluate([]rune("ABC"), []rune("ABC"))
	tr.Evaluate([]rune("ABC"), []rune("ABD"))

	stats := tr.Stats()
	if stats.Rounds != 2 {
		t.Fatalf("Rounds=%d, want 2", stats.Rounds)
	}
	if stats.CorrectRounds != 1 {
		t.Fatalf("CorrectRounds=%d, want 1", stats.CorrectRounds)
	}
	if stats.Accuracy != 0.5 {
		t.Fatalf("Accuracy=%f, want 0.5", stats.Accuracy)
	}

	tr.Reset()
	stats = tr.Stats()
	if stats.Rounds != 0 || stats.CorrectRounds != 0 || stats.Accuracy != 0 {
		t.Fatalf("stats after reset = %+v, want zero", stats)
	}
}
