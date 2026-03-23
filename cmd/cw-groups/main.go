//go:build linux

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"cw-trainer/internal/audio"
	"cw-trainer/internal/decoder"
	"cw-trainer/internal/groups"
	"cw-trainer/internal/input"
	"cw-trainer/internal/koch"
	"cw-trainer/internal/ui"
)

func main() {
	devicePath := flag.String("device", "/dev/input/event4", "evdev device path")
	wpm := flag.Int("wpm", 20, "speed in WPM")
	freq := flag.Int("freq", 700, "tone frequency in Hz")
	mode := flag.String("mode", "iambic-a", "keyer mode: iambic-a, iambic-b")
	letterSpaceMult := flag.Float64("letter-space", 4.0, "letter space threshold multiplier (× dit)")
	groupSize := flag.Int("group-size", 5, "symbols in one group")
	level := flag.Int("level", 2, "Koch level")
	flag.Parse()

	if *wpm <= 0 {
		fmt.Fprintln(os.Stderr, "invalid --wpm: must be > 0")
		os.Exit(2)
	}
	if *letterSpaceMult <= 0 || *letterSpaceMult > 7 {
		fmt.Fprintln(os.Stderr, "invalid --letter-space: must be in (0, 7]")
		os.Exit(2)
	}
	if *groupSize <= 0 {
		fmt.Fprintln(os.Stderr, "invalid --group-size: must be > 0")
		os.Exit(2)
	}
	if *level < koch.MinLevel || *level > len(koch.KochOrder) {
		fmt.Fprintf(os.Stderr, "invalid --level: must be in [%d, %d]\n", koch.MinLevel, len(koch.KochOrder))
		os.Exit(2)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	rawEvents := make(chan input.KeyEvent, 32)
	go func() {
		if err := input.ReadEvents(ctx, *devicePath, rawEvents); err != nil && ctx.Err() == nil {
			log.Printf("Input error: %v", err)
		}
	}()

	iambicEvents := make(chan input.KeyEvent, 32)
	go func() {
		for {
			select {
			case ev, ok := <-rawEvents:
				if !ok {
					return
				}
				select {
				case iambicEvents <- ev:
				default:
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	tone, err := audio.NewTone(ctx, *freq, 44100)
	if err != nil {
		log.Printf("Audio init failed: %v (continuing without audio)", err)
	} else {
		defer tone.Close()
	}

	session := koch.NewSession(*level, *wpm, nil)
	active := append([]rune(nil), session.ActiveSymbols()...)
	trainer, err := groups.NewTrainer(groups.Config{
		GroupSize: *groupSize,
		Symbols:   active,
	})
	if err != nil {
		log.Fatalf("trainer init error: %v", err)
	}

	answerCh := make(chan rune, 128)
	resetCh := make(chan struct{}, 1)
	groupsEvents := make(chan ui.GroupsEvent, 64)

	timingDecoder := decoder.NewTimingDecoder(float64(*wpm),
		func(r rune) {
			r = normalizeAnswerRune(r)
			if r == 0 {
				return
			}
			select {
			case answerCh <- r:
			default:
			}
		},
		nil,
	)

	var flushTimer *time.Timer
	onSymbol := func(sym decoder.Symbol, durationMs float64) {
		if flushTimer != nil {
			flushTimer.Stop()
		}
		timingDecoder.AddSymbol(sym, durationMs)
	}
	onElement := func(toneMs, gapMs float64) {
		if tone != nil {
			tone.PlayElement(toneMs, gapMs)
		}
	}

	iambicMode := decoder.ModeA
	if *mode == "iambic-b" {
		iambicMode = decoder.ModeB
	}
	keyer := decoder.NewIambicKeyer(iambicMode, timingDecoder, onSymbol, onElement)
	keyer.OnIdle(func() {
		ditMs := timingDecoder.GetDitMs()
		letterSpace := time.Duration(*letterSpaceMult * ditMs * float64(time.Millisecond))
		wordSpace := time.Duration(7 * ditMs * float64(time.Millisecond))
		if flushTimer != nil {
			flushTimer.Stop()
		}
		flushTimer = time.AfterFunc(letterSpace, func() {
			timingDecoder.Flush()
			time.AfterFunc(wordSpace-letterSpace, func() {
				timingDecoder.AddPause(7 * ditMs)
			})
		})
	})
	go keyer.Run(ctx, iambicEvents)

	go runGroupsSession(ctx, tone, trainer, *level, *wpm, answerCh, resetCh, groupsEvents)

	model := ui.NewGroupsModel(
		*level,
		*groupSize,
		*wpm,
		groupsEvents,
		answerCh,
		cancel,
		func() {
			select {
			case resetCh <- struct{}{}:
			default:
			}
		},
	)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}

func runGroupsSession(
	ctx context.Context,
	tone *audio.Tone,
	trainer *groups.Trainer,
	level int,
	wpm int,
	answerCh chan rune,
	resetCh <-chan struct{},
	events chan<- ui.GroupsEvent,
) {
	ditMs := 1200.0 / float64(wpm)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if drainReset(resetCh) {
			trainer.Reset()
			sendGroupsEvent(ctx, events, ui.GroupsEvent{Type: ui.GroupsMsgStatsReset})
		}

		drainAnswer(answerCh)
		prompt := trainer.NextPrompt()

		sendGroupsEvent(ctx, events, ui.GroupsEvent{
			Type:      ui.GroupsMsgPlaying,
			Level:     level,
			GroupSize: trainer.GroupSize(),
			WPM:       wpm,
		})

		for _, r := range prompt {
			symbols := koch.MorseFor(r)
			if symbols == nil {
				continue
			}
			if tone != nil {
				dur := tone.PlayMorse(symbols, ditMs)
				select {
				case <-time.After(dur):
				case <-ctx.Done():
					return
				}
			}
		}

		sendGroupsEvent(ctx, events, ui.GroupsEvent{Type: ui.GroupsMsgWaiting})

		got := make([]rune, 0, trainer.GroupSize())
		for len(got) < trainer.GroupSize() {
			select {
			case <-ctx.Done():
				return
			case <-resetCh:
				trainer.Reset()
				sendGroupsEvent(ctx, events, ui.GroupsEvent{Type: ui.GroupsMsgStatsReset})
				got = got[:0]
				drainAnswer(answerCh)
				sendGroupsEvent(ctx, events, ui.GroupsEvent{Type: ui.GroupsMsgWaiting})
			case r := <-answerCh:
				norm := normalizeAnswerRune(r)
				if norm == 0 {
					continue
				}
				got = append(got, norm)
				sendGroupsEvent(ctx, events, ui.GroupsEvent{
					Type:  ui.GroupsMsgInputProgress,
					Typed: string(got),
				})
			}
		}

		round := trainer.Evaluate(prompt, got)
		stats := trainer.Stats()

		sendGroupsEvent(ctx, events, ui.GroupsEvent{
			Type:          ui.GroupsMsgResult,
			Expected:      strings.ToUpper(string(round.Prompt)),
			Got:           strings.ToUpper(string(round.Answer)),
			Correct:       round.Correct,
			Rounds:        stats.Rounds,
			CorrectRounds: stats.CorrectRounds,
			Accuracy:      stats.Accuracy,
		})

		select {
		case <-time.After(1200 * time.Millisecond):
		case <-ctx.Done():
			return
		}
	}
}

func sendGroupsEvent(ctx context.Context, events chan<- ui.GroupsEvent, ev ui.GroupsEvent) {
	select {
	case events <- ev:
	case <-ctx.Done():
	}
}

func drainAnswer(ch chan rune) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func drainReset(ch <-chan struct{}) bool {
	drained := false
	for {
		select {
		case <-ch:
			drained = true
		default:
			return drained
		}
	}
}

func normalizeAnswerRune(r rune) rune {
	r = []rune(strings.ToUpper(string(r)))[0]
	if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == ',' || r == '?' || r == '/' {
		return r
	}
	return 0
}
