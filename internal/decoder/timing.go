package decoder

import (
	"sync"
	"time"
)

type TimingDecoder struct {
	wpm         float64
	ditMs       float64
	symbolBuf   []Symbol
	lastSymTime time.Time
	adaptive    []float64 // rolling window of last 8 dit durations
	mu          sync.Mutex
	onChar      func(rune)
	onWord      func()
}

func NewTimingDecoder(wpm float64, onChar func(rune), onWord func()) *TimingDecoder {
	ditMs := 1200.0 / wpm
	return &TimingDecoder{
		wpm:    wpm,
		ditMs:  ditMs,
		onChar: onChar,
		onWord: onWord,
	}
}

// AddSymbol adds a dit or dah with the given duration in milliseconds.
func (td *TimingDecoder) AddSymbol(sym Symbol, durationMs float64) {
	td.mu.Lock()
	defer td.mu.Unlock()

	// Update adaptive timing for dits
	if sym == SymDit {
		td.adaptive = append(td.adaptive, durationMs)
		if len(td.adaptive) > 8 {
			td.adaptive = td.adaptive[1:]
		}
		// Recompute dit duration as average
		sum := 0.0
		for _, v := range td.adaptive {
			sum += v
		}
		td.ditMs = sum / float64(len(td.adaptive))
	}

	td.symbolBuf = append(td.symbolBuf, sym)
	td.lastSymTime = time.Now()
}

// AddPause processes a pause of pauseMs milliseconds.
// Returns true if a letter was emitted.
func (td *TimingDecoder) AddPause(pauseMs float64) bool {
	td.mu.Lock()
	defer td.mu.Unlock()

	ditMs := td.ditMs

	if pauseMs >= 7*ditMs {
		// word space
		td.flushLetter()
		if td.onWord != nil {
			td.onWord()
		}
		return true
	} else if pauseMs >= 3*ditMs {
		// letter space
		td.flushLetter()
		return true
	}
	// inter-element gap — do nothing
	return false
}

// Flush forces decoding of any buffered symbols (call on long pause/timeout).
func (td *TimingDecoder) Flush() {
	td.mu.Lock()
	defer td.mu.Unlock()
	td.flushLetter()
}

// GetDitMs returns current dit duration estimate.
func (td *TimingDecoder) GetDitMs() float64 {
	td.mu.Lock()
	defer td.mu.Unlock()
	return td.ditMs
}

// GetWPM returns estimated WPM based on current dit duration.
func (td *TimingDecoder) GetWPM() float64 {
	td.mu.Lock()
	defer td.mu.Unlock()
	return 1200.0 / td.ditMs
}

func (td *TimingDecoder) flushLetter() {
	if len(td.symbolBuf) == 0 {
		return
	}
	if r, ok := Decode(td.symbolBuf); ok {
		if td.onChar != nil {
			td.onChar(r)
		}
	}
	td.symbolBuf = nil
}
