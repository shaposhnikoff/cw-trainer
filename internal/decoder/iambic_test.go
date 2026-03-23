package decoder

import (
	"context"
	"testing"
	"time"

	"cw-trainer/internal/input"
)

// collectSymbols runs the iambic keyer and collects symbols into a slice.
// Returns the collected symbols channel (buffered).
func runKeyer(t *testing.T, wpm float64, mode IambicMode) (
	keyer *IambicKeyer,
	events chan input.KeyEvent,
	symbols chan Symbol,
	cancel context.CancelFunc,
) {
	t.Helper()
	events = make(chan input.KeyEvent, 64)
	symbols = make(chan Symbol, 64)

	td := NewTimingDecoder(wpm, nil, nil)
	keyer = NewIambicKeyer(mode, td, func(sym Symbol, _ float64) {
		symbols <- sym
	}, nil)

	ctx, c := context.WithCancel(context.Background())
	cancel = c

	go keyer.Run(ctx, events)
	return
}

// press sends a press event for the given key.
func press(ch chan<- input.KeyEvent, k input.Key) {
	ch <- input.KeyEvent{Key: k, Action: input.Press, Timestamp: time.Now()}
}

// release sends a release event for the given key.
func release(ch chan<- input.KeyEvent, k input.Key) {
	ch <- input.KeyEvent{Key: k, Action: input.Release, Timestamp: time.Now()}
}

// collectN collects exactly n symbols with a timeout. Fails the test if timeout.
func collectN(t *testing.T, ch <-chan Symbol, n int, timeout time.Duration) []Symbol {
	t.Helper()
	out := make([]Symbol, 0, n)
	deadline := time.After(timeout)
	for len(out) < n {
		select {
		case s := <-ch:
			out = append(out, s)
		case <-deadline:
			t.Fatalf("timeout waiting for %d symbols, got %d: %v", n, len(out), out)
		}
	}
	return out
}

// symStr converts symbols to dot-dash string for readable assertions.
func symStr(syms []Symbol) string {
	s := ""
	for _, sym := range syms {
		if sym == SymDit {
			s += "."
		} else {
			s += "-"
		}
	}
	return s
}

const testWPM = 200 // ditMs = 6ms — fast for tests

func TestIambic_SingleDit(t *testing.T) {
	_, events, symbols, cancel := runKeyer(t, testWPM, ModeA)
	defer cancel()

	press(events, input.DitKey)
	time.Sleep(5 * time.Millisecond) // hold briefly
	release(events, input.DitKey)

	syms := collectN(t, symbols, 1, 200*time.Millisecond)
	if symStr(syms) != "." {
		t.Errorf("expected '.', got %q", symStr(syms))
	}
}

func TestIambic_SingleDah(t *testing.T) {
	_, events, symbols, cancel := runKeyer(t, testWPM, ModeA)
	defer cancel()

	press(events, input.DahKey)
	time.Sleep(15 * time.Millisecond) // hold briefly
	release(events, input.DahKey)

	syms := collectN(t, symbols, 1, 200*time.Millisecond)
	if symStr(syms) != "-" {
		t.Errorf("expected '-', got %q", symStr(syms))
	}
}

func TestIambic_HoldDit_ThreeDits(t *testing.T) {
	_, events, symbols, cancel := runKeyer(t, testWPM, ModeA)
	defer cancel()

	// ditMs=6ms, cycle=12ms. 3 cycles span t=0..36ms.
	// Release at 33ms: 4th element starts at t=36ms, so we stop before it.
	press(events, input.DitKey)
	time.Sleep(33 * time.Millisecond)
	release(events, input.DitKey)

	syms := collectN(t, symbols, 3, 200*time.Millisecond)
	for i, s := range syms {
		if s != SymDit {
			t.Errorf("symbol[%d] = %v, want Dit", i, s)
		}
	}
}

func TestIambic_HoldDah_ThreeDahs(t *testing.T) {
	_, events, symbols, cancel := runKeyer(t, testWPM, ModeA)
	defer cancel()

	// ditMs=6ms, dahMs=18ms, gap=6ms → dah cycle=24ms.
	// 3 cycles span t=0..72ms. Release at 68ms.
	press(events, input.DahKey)
	time.Sleep(68 * time.Millisecond)
	release(events, input.DahKey)

	syms := collectN(t, symbols, 3, 500*time.Millisecond)
	for i, s := range syms {
		if s != SymDah {
			t.Errorf("symbol[%d] = %v, want Dah", i, s)
		}
	}
}

func TestIambic_Squeeze_Alternates(t *testing.T) {
	_, events, symbols, cancel := runKeyer(t, testWPM, ModeA)
	defer cancel()

	// Press both simultaneously → should alternate dit/dah
	press(events, input.DitKey)
	press(events, input.DahKey)
	time.Sleep(100 * time.Millisecond) // hold for several cycles
	release(events, input.DitKey)
	release(events, input.DahKey)

	syms := collectN(t, symbols, 4, 500*time.Millisecond)
	// Should alternate: . - . - or - . - .
	for i := 1; i < len(syms); i++ {
		if syms[i] == syms[i-1] {
			t.Errorf("squeeze should alternate; got %q", symStr(syms))
			break
		}
	}
}

func TestIambic_DitThenDah(t *testing.T) {
	_, events, symbols, cancel := runKeyer(t, testWPM, ModeA)
	defer cancel()

	press(events, input.DitKey)
	time.Sleep(5 * time.Millisecond)
	release(events, input.DitKey)

	// wait for it to go idle
	time.Sleep(50 * time.Millisecond)

	press(events, input.DahKey)
	time.Sleep(5 * time.Millisecond)
	release(events, input.DahKey)

	syms := collectN(t, symbols, 2, 500*time.Millisecond)
	if symStr(syms) != ".-" {
		t.Errorf("expected '.-', got %q", symStr(syms))
	}
}

func TestIambic_SOS_Symbols(t *testing.T) {
	_, events, symbols, cancel := runKeyer(t, testWPM, ModeA)
	defer cancel()

	// Tap each element individually: press, hold briefly (<ditMs), release, wait for cycle to finish.
	// ditMs=6ms, cycle=dit+gap=12ms; dah cycle=dah+gap=24ms.
	tap := func(key input.Key, waitMs time.Duration) {
		press(events, key)
		time.Sleep(3 * time.Millisecond) // brief touch, keyer latches
		release(events, key)
		time.Sleep(waitMs) // wait for element + gap to finish
	}
	dit := func() { tap(input.DitKey, 15*time.Millisecond) }
	dah := func() { tap(input.DahKey, 27*time.Millisecond) }

	dit()
	dit()
	dit()                             // S
	time.Sleep(20 * time.Millisecond) // inter-letter gap
	dah()
	dah()
	dah() // O
	time.Sleep(20 * time.Millisecond)
	dit()
	dit()
	dit() // S

	syms := collectN(t, symbols, 9, 2*time.Second)
	if symStr(syms) != "...---..." {
		t.Errorf("expected SOS (\"...---...\"), got %q", symStr(syms))
	}
}

func TestIambic_ModeA_NoExtraAfterSqueezeRelease(t *testing.T) {
	_, events, symbols, cancel := runKeyer(t, testWPM, ModeA)
	defer cancel()

	press(events, input.DitKey)
	press(events, input.DahKey)
	time.Sleep(2 * time.Millisecond)
	release(events, input.DitKey)
	release(events, input.DahKey)

	syms := collectN(t, symbols, 1, 200*time.Millisecond)
	if len(syms) != 1 {
		t.Fatalf("expected exactly one symbol, got %d", len(syms))
	}
	select {
	case s := <-symbols:
		t.Fatalf("mode A should stop after first symbol, got extra %q", symStr([]Symbol{s}))
	case <-time.After(80 * time.Millisecond):
	}
}

func TestIambic_ModeB_ExtraAfterSqueezeRelease(t *testing.T) {
	_, events, symbols, cancel := runKeyer(t, testWPM, ModeB)
	defer cancel()

	press(events, input.DitKey)
	press(events, input.DahKey)
	time.Sleep(2 * time.Millisecond)
	release(events, input.DitKey)
	release(events, input.DahKey)

	syms := collectN(t, symbols, 2, 300*time.Millisecond)
	if symStr(syms) != ".-" {
		t.Errorf("expected '.-' for mode B squeeze release, got %q", symStr(syms))
	}
}
