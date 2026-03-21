package audio

import (
	"context"
	"io"
	"math"
	"sync/atomic"
	"time"
	"unsafe"

	oto "github.com/hajimehoshi/oto/v2"

	"cw-trainer/internal/decoder"
)

const channelCount = 1

type elementCmd struct {
	toneMs float64
	gapMs  float64
}

type Tone struct {
	player     oto.Player
	pw         *io.PipeWriter
	freqBits   atomic.Uint64
	sampleRate int
	cmdCh      chan elementCmd
}

// Ensure float64 is 8 bytes (compile-time check)
var _ = [1]struct{}{}[unsafe.Sizeof(float64(0))-8]

func NewTone(ctx context.Context, freq int, sampleRate int) (*Tone, error) {
	pr, pw := io.Pipe()

	otoCtx, readyChan, err := oto.NewContextWithOptions(&oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: channelCount,
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		return nil, err
	}
	<-readyChan

	player := otoCtx.NewPlayer(pr)
	player.Play()

	t := &Tone{
		player:     player,
		pw:         pw,
		sampleRate: sampleRate,
		cmdCh:      make(chan elementCmd, 16),
	}
	t.freqBits.Store(math.Float64bits(float64(freq)))

	go t.audioLoop(ctx)
	return t, nil
}

// PlayElement schedules a tone burst of toneMs followed by silence of gapMs.
// Non-blocking: queues and returns immediately.
func (t *Tone) PlayElement(toneMs, gapMs float64) {
	select {
	case t.cmdCh <- elementCmd{toneMs, gapMs}:
	default:
	}
}

func (t *Tone) SetFreq(freq int) {
	t.freqBits.Store(math.Float64bits(float64(freq)))
}

// PlayMorse plays the morse sequence for given symbols at given ditMs.
// Returns total playback duration.
func (t *Tone) PlayMorse(symbols []decoder.Symbol, ditMs float64) time.Duration {
	total := time.Duration(0)
	for i, sym := range symbols {
		toneMs := ditMs
		if sym == decoder.SymDah {
			toneMs = 3 * ditMs
		}
		gapMs := ditMs // inter-element gap
		if i == len(symbols)-1 {
			gapMs = 3 * ditMs // letter space after last symbol
		}
		t.PlayElement(toneMs, gapMs)
		total += time.Duration((toneMs + gapMs) * float64(time.Millisecond))
	}
	return total
}

func (t *Tone) Close() {
	t.pw.Close()
	t.player.Close()
}

func (t *Tone) audioLoop(ctx context.Context) {
	var phase float64
	fadeSamples := int(0.005 * float64(t.sampleRate)) // 5ms envelope

	for {
		select {
		case <-ctx.Done():
			return
		case cmd := <-t.cmdCh:
			freq := math.Float64frombits(t.freqBits.Load())
			sr := float64(t.sampleRate)

			// --- Tone ---
			toneSamples := int(cmd.toneMs * sr / 1000.0)
			buf := make([]byte, toneSamples*2)
			for i := 0; i < toneSamples; i++ {
				env := 1.0
				if i < fadeSamples {
					env = float64(i) / float64(fadeSamples)
				} else if i >= toneSamples-fadeSamples {
					env = float64(toneSamples-1-i) / float64(fadeSamples)
				}
				s := math.Sin(2*math.Pi*freq*phase/sr) * env * 0.5
				phase++
				v := int16(s * math.MaxInt16)
				buf[i*2] = byte(v)
				buf[i*2+1] = byte(v >> 8)
			}
			t.pw.Write(buf) //nolint

			// --- Gap silence ---
			phase = 0 // reset so next tone starts cleanly
			gapSamples := int(cmd.gapMs * sr / 1000.0)
			silence := make([]byte, gapSamples*2)
			t.pw.Write(silence) //nolint
		}
	}
}
