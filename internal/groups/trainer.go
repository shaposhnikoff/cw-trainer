package groups

import (
	"errors"
	"math/rand"
	"time"
)

type Config struct {
	GroupSize int
	Symbols   []rune
	Rand      *rand.Rand
}

type Stats struct {
	Rounds        int
	CorrectRounds int
	Accuracy      float64
}

type Round struct {
	Prompt  []rune
	Answer  []rune
	Correct bool
}

type Trainer struct {
	groupSize int
	symbols   []rune
	rng       *rand.Rand
	stats     Stats
}

func NewTrainer(cfg Config) (*Trainer, error) {
	if cfg.GroupSize <= 0 {
		return nil, errors.New("group size must be > 0")
	}
	if len(cfg.Symbols) == 0 {
		return nil, errors.New("symbols must not be empty")
	}

	rng := cfg.Rand
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	symbols := make([]rune, len(cfg.Symbols))
	copy(symbols, cfg.Symbols)

	return &Trainer{
		groupSize: cfg.GroupSize,
		symbols:   symbols,
		rng:       rng,
	}, nil
}

func (t *Trainer) GroupSize() int {
	return t.groupSize
}

func (t *Trainer) NextPrompt() []rune {
	out := make([]rune, t.groupSize)
	for i := 0; i < t.groupSize; i++ {
		out[i] = t.symbols[t.rng.Intn(len(t.symbols))]
	}
	return out
}

func (t *Trainer) Evaluate(prompt, answer []rune) Round {
	correct := len(prompt) == len(answer)
	if correct {
		for i := range prompt {
			if prompt[i] != answer[i] {
				correct = false
				break
			}
		}
	}

	t.stats.Rounds++
	if correct {
		t.stats.CorrectRounds++
	}
	t.stats.Accuracy = float64(t.stats.CorrectRounds) / float64(t.stats.Rounds)

	promptCopy := make([]rune, len(prompt))
	answerCopy := make([]rune, len(answer))
	copy(promptCopy, prompt)
	copy(answerCopy, answer)

	return Round{
		Prompt:  promptCopy,
		Answer:  answerCopy,
		Correct: correct,
	}
}

func (t *Trainer) Stats() Stats {
	return t.stats
}

func (t *Trainer) Reset() {
	t.stats = Stats{}
}
