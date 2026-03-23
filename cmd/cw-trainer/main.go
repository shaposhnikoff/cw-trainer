//go:build linux

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"cw-trainer/internal/audio"
	"cw-trainer/internal/decoder"
	"cw-trainer/internal/input"
	"cw-trainer/internal/koch"
	"cw-trainer/internal/ui"
)

func main() {
	devicePath := flag.String("device", "/dev/input/event4", "evdev device path")
	wpm := flag.Int("wpm", 15, "initial speed in WPM")
	freq := flag.Int("freq", 700, "tone frequency in Hz")
	mode := flag.String("mode", "iambic-a", "keyer mode: iambic-a, iambic-b")
	letterSpaceMult := flag.Float64("letter-space", 4.0, "letter space threshold multiplier (× dit)")
	debug := flag.Bool("debug", false, "debug mode: print symbols, no TUI")
	kochMode := flag.Bool("koch", false, "Koch trainer mode")
	flag.Parse()

	if *wpm <= 0 {
		fmt.Fprintln(os.Stderr, "invalid --wpm: must be > 0")
		os.Exit(2)
	}
	if *letterSpaceMult <= 0 || *letterSpaceMult > 7 {
		fmt.Fprintln(os.Stderr, "invalid --letter-space: must be in (0, 7]")
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

	if *debug {
		runDebug(ctx, tone, iambicEvents, float64(*wpm), *mode, *letterSpaceMult)
		return
	}

	if *kochMode {
		runKoch(ctx, tone, iambicEvents, *wpm, *letterSpaceMult, cancel)
		return
	}

	tuiEvents := make(chan ui.TUIEvent, 64)

	iambicMode := decoder.ModeA
	if *mode == "iambic-b" {
		iambicMode = decoder.ModeB
	}

	var currentSeq []decoder.Symbol
	timingDecoder := decoder.NewTimingDecoder(float64(*wpm),
		func(r rune) {
			tuiEvents <- ui.TUIEvent{Type: ui.MsgChar, Char: r}
			currentSeq = nil
		},
		func() {
			tuiEvents <- ui.TUIEvent{Type: ui.MsgWordSpace}
		},
	)

	var flushTimer *time.Timer

	onSymbol := func(sym decoder.Symbol, durationMs float64) {
		if flushTimer != nil {
			flushTimer.Stop()
		}
		currentSeq = append(currentSeq, sym)
		seq := ""
		for _, s := range currentSeq {
			if s == decoder.SymDit {
				seq += "."
			} else {
				seq += "-"
			}
		}
		tuiEvents <- ui.TUIEvent{Type: ui.MsgCurrentSeq, Seq: seq}
		timingDecoder.AddSymbol(sym, durationMs)
		tuiEvents <- ui.TUIEvent{Type: ui.MsgWPMUpdate, WPM: timingDecoder.GetWPM()}
	}

	onElement := func(toneMs, gapMs float64) {
		if tone != nil {
			tone.PlayElement(toneMs, gapMs)
		}
		ditMs := timingDecoder.GetDitMs()
		if toneMs > 2*ditMs {
			tuiEvents <- ui.TUIEvent{Type: ui.MsgDahOn}
			time.AfterFunc(time.Duration(toneMs*float64(time.Millisecond)), func() {
				select {
				case tuiEvents <- ui.TUIEvent{Type: ui.MsgDahOff}:
				default:
				}
			})
			return
		}
		tuiEvents <- ui.TUIEvent{Type: ui.MsgDitOn}
		time.AfterFunc(time.Duration(toneMs*float64(time.Millisecond)), func() {
			select {
			case tuiEvents <- ui.TUIEvent{Type: ui.MsgDitOff}:
			default:
			}
		})
	}

	iambicKeyer := decoder.NewIambicKeyer(iambicMode, timingDecoder, onSymbol, onElement)
	iambicKeyer.OnIdle(func() {
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

	go iambicKeyer.Run(ctx, iambicEvents)

	freqVal := *freq
	model := ui.NewModel(freqVal, float64(*wpm), tuiEvents,
		func(f int) {
			freqVal = f
			if tone != nil {
				tone.SetFreq(f)
			}
		},
		cancel,
	)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}

func runKoch(ctx context.Context, tone *audio.Tone, iambicEvents <-chan input.KeyEvent, wpm int, letterSpaceMult float64, cancel context.CancelFunc) {
	prog, err := koch.LoadProgress()
	if err != nil {
		log.Printf("Progress load error: %v, starting fresh", err)
		prog = &koch.Progress{Level: koch.MinLevel, WPM: wpm, SymbolStats: make(map[rune]koch.SymbolStat)}
	}
	if wpm > 0 {
		prog.WPM = wpm
	}
	if prog.WPM <= 0 {
		prog.WPM = 20
	}

	session := koch.NewSession(prog.Level, prog.WPM, prog.SymbolStats)

	kochEvents := make(chan ui.KochEvent, 32)
	answerCh := make(chan rune, 4)

	// Timing decoder for user's paddle input
	timingDecoder := decoder.NewTimingDecoder(float64(prog.WPM),
		func(r rune) {
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

	iambicKeyer := decoder.NewIambicKeyer(decoder.ModeA, timingDecoder, onSymbol, onElement)
	iambicKeyer.OnIdle(func() {
		ditMs := timingDecoder.GetDitMs()
		letterSpace := time.Duration(letterSpaceMult * ditMs * float64(time.Millisecond))
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
	go iambicKeyer.Run(ctx, iambicEvents)

	// Koch session goroutine
	go func() {
		defer func() {
			prog.Level = session.Level
			prog.SymbolStats = session.SymbolStats
			_ = koch.SaveProgress(prog)
		}()

		ditMs := 1200.0 / float64(prog.WPM)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Pick next symbol
			sym := session.NextSymbol()
			symbols := koch.MorseFor(sym)
			if symbols == nil {
				continue
			}

			// Tell TUI we're playing
			select {
			case kochEvents <- ui.KochEvent{
				Type:     ui.KochMsgPlay,
				Symbol:   sym,
				Level:    session.Level,
				Accuracy: session.Accuracy(),
				Recent:   session.RecentTotal(),
			}:
			case <-ctx.Done():
				return
			}

			// Play the symbol
			if tone != nil {
				dur := tone.PlayMorse(symbols, ditMs)
				select {
				case <-time.After(dur):
				case <-ctx.Done():
					return
				}
			}

			// Tell TUI we're waiting for answer
			select {
			case kochEvents <- ui.KochEvent{Type: ui.KochMsgWaiting}:
			case <-ctx.Done():
				return
			}

			// Wait for user answer (timeout 10s)
			var answer rune
			select {
			case answer = <-answerCh:
			case <-time.After(10 * time.Second):
				// timeout — skip this round
				continue
			case <-ctx.Done():
				return
			}

			// Check answer
			correct := session.CheckAnswer(answer)

			var evType ui.KochMsgType
			if correct {
				evType = ui.KochMsgCorrect
			} else {
				evType = ui.KochMsgWrong
			}
			select {
			case kochEvents <- ui.KochEvent{
				Type:     evType,
				Symbol:   sym,
				Got:      answer,
				Accuracy: session.Accuracy(),
				Recent:   session.RecentTotal(),
			}:
			case <-ctx.Done():
				return
			}

			// Pause to show result
			select {
			case <-time.After(1500 * time.Millisecond):
			case <-ctx.Done():
				return
			}

			// Level up?
			if session.ShouldLevelUp() {
				session.LevelUp()
				newSym := session.ActiveSymbols()[len(session.ActiveSymbols())-1]
				newSymMorse := koch.MorseFor(newSym)

				select {
				case kochEvents <- ui.KochEvent{
					Type:   ui.KochMsgLevelUp,
					Symbol: newSym,
					Level:  session.Level,
				}:
				case <-ctx.Done():
					return
				}

				// Play new symbol several times
				if tone != nil && newSymMorse != nil {
					for i := 0; i < 5; i++ {
						dur := tone.PlayMorse(newSymMorse, ditMs)
						select {
						case <-time.After(dur + 500*time.Millisecond):
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}
	}()

	model := ui.NewKochModel(
		session.Level, prog.WPM,
		session.ActiveSymbols(),
		kochEvents,
		func() { cancel() },
	)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
	}
}

func runDebug(ctx context.Context, tone *audio.Tone, iambicEvents <-chan input.KeyEvent, wpm float64, mode string, letterSpaceMult float64) {
	fmt.Println("Debug mode — iambic keyer active. Press Ctrl+C to exit.")

	iambicMode := decoder.ModeA
	if mode == "iambic-b" {
		iambicMode = decoder.ModeB
	}

	onSymbol := func(sym decoder.Symbol, durationMs float64) {
		name := "DIT"
		if sym == decoder.SymDah {
			name = "DAH"
		}
		fmt.Printf("[%s] %s (%.0fms)\n", time.Now().Format("15:04:05.000"), name, durationMs)
	}

	onElement := func(toneMs, gapMs float64) {
		if tone != nil {
			tone.PlayElement(toneMs, gapMs)
		}
	}

	timingDecoder := decoder.NewTimingDecoder(wpm,
		func(r rune) { fmt.Printf(">>> %c\n", r) },
		func() { fmt.Println(">>> [space]") },
	)

	var flushTimer *time.Timer
	origOnSymbol := onSymbol
	onSymbol = func(sym decoder.Symbol, durationMs float64) {
		if flushTimer != nil {
			flushTimer.Stop()
		}
		origOnSymbol(sym, durationMs)
	}

	keyer := decoder.NewIambicKeyer(iambicMode, timingDecoder, onSymbol, onElement)
	keyer.OnIdle(func() {
		ditMs := timingDecoder.GetDitMs()
		letterSpace := time.Duration(letterSpaceMult * ditMs * float64(time.Millisecond))
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
	keyer.Run(ctx, iambicEvents)
}
