package decoder

import (
	"context"
	"time"

	"cw-trainer/internal/input"
)

type IambicMode int

const (
	ModeA IambicMode = iota
	ModeB
)

type IambicKeyer struct {
	mode      IambicMode
	timing    *TimingDecoder
	onSymbol  func(Symbol, float64)
	onElement func(toneMs, gapMs float64) // tone on for toneMs, then silence gapMs
	onIdle    func()                      // called when keyer transitions to idle
}

func NewIambicKeyer(
	mode IambicMode,
	timing *TimingDecoder,
	onSymbol func(Symbol, float64),
	onElement func(toneMs, gapMs float64),
) *IambicKeyer {
	return &IambicKeyer{
		mode:      mode,
		timing:    timing,
		onSymbol:  onSymbol,
		onElement: onElement,
	}
}

func (k *IambicKeyer) OnIdle(fn func()) { k.onIdle = fn }

func (k *IambicKeyer) Run(ctx context.Context, events <-chan input.KeyEvent) {
	type phase int
	const (
		idle    phase = iota
		sending       // timer covers tone + gap
	)

	var (
		ditHeld   bool
		dahHeld   bool
		lastSym   = SymDah
		cur       = idle
		curSym    Symbol
		curToneMs float64
		bLatch    bool
	)

	timer := time.NewTimer(time.Hour)
	timer.Stop()

	ditMs := func() float64 { return k.timing.GetDitMs() }
	elDur := func(sym Symbol) float64 {
		if sym == SymDit {
			return ditMs()
		}
		return 3 * ditMs()
	}

	startElement := func(sym Symbol) {
		curSym = sym
		cur = sending
		bLatch = ditHeld && dahHeld
		toneMs := elDur(sym)
		gapMs := ditMs()
		curToneMs = toneMs
		if k.onElement != nil {
			k.onElement(toneMs, gapMs)
		}
		total := time.Duration((toneMs + gapMs) * float64(time.Millisecond))
		timer.Reset(total)
	}

	pickNext := func() {
		if ditHeld && dahHeld {
			if lastSym == SymDit {
				startElement(SymDah)
			} else {
				startElement(SymDit)
			}
		} else if ditHeld {
			startElement(SymDit)
		} else if dahHeld {
			startElement(SymDah)
		} else {
			cur = idle
			if k.onIdle != nil {
				k.onIdle()
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			timer.Stop()
			return

		case ev, ok := <-events:
			if !ok {
				return
			}
			switch ev.Key {
			case input.DitKey:
				ditHeld = ev.Action == input.Press
			case input.DahKey:
				dahHeld = ev.Action == input.Press
			}
			if cur == sending && ditHeld && dahHeld {
				bLatch = true
			}
			if cur == idle && (ditHeld || dahHeld) {
				pickNext()
			}

		case <-timer.C:
			if k.onSymbol != nil {
				k.onSymbol(curSym, curToneMs)
			}
			lastSym = curSym
			if k.mode == ModeB && bLatch && !ditHeld && !dahHeld {
				next := SymDit
				if curSym == SymDit {
					next = SymDah
				}
				startElement(next)
				continue
			}
			pickNext()
		}
	}
}
