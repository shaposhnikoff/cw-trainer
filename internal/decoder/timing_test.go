package decoder

import "testing"

func TestTimingDecoder_Flush(t *testing.T) {
	var got []rune
	td := NewTimingDecoder(20, func(r rune) { got = append(got, r) }, nil)
	ditMs := td.GetDitMs() // 60ms

	td.AddSymbol(SymDit, ditMs)
	td.AddSymbol(SymDit, ditMs)
	td.AddSymbol(SymDit, ditMs)
	td.Flush()

	if len(got) != 1 || got[0] != 'S' {
		t.Errorf("expected S, got %v", got)
	}
}

func TestTimingDecoder_SOS(t *testing.T) {
	var got []rune
	td := NewTimingDecoder(20, func(r rune) { got = append(got, r) }, nil)
	ditMs := td.GetDitMs()
	dahMs := 3 * ditMs

	// S
	td.AddSymbol(SymDit, ditMs)
	td.AddSymbol(SymDit, ditMs)
	td.AddSymbol(SymDit, ditMs)
	td.AddPause(3 * ditMs) // letter space

	// O
	td.AddSymbol(SymDah, dahMs)
	td.AddSymbol(SymDah, dahMs)
	td.AddSymbol(SymDah, dahMs)
	td.AddPause(3 * ditMs)

	// S
	td.AddSymbol(SymDit, ditMs)
	td.AddSymbol(SymDit, ditMs)
	td.AddSymbol(SymDit, ditMs)
	td.Flush()

	if len(got) != 3 || got[0] != 'S' || got[1] != 'O' || got[2] != 'S' {
		t.Errorf("expected SOS, got %v", string(got))
	}
}

func TestTimingDecoder_WordSpace(t *testing.T) {
	var chars []rune
	var words int
	td := NewTimingDecoder(20, func(r rune) { chars = append(chars, r) }, func() { words++ })
	ditMs := td.GetDitMs()

	// E
	td.AddSymbol(SymDit, ditMs)
	td.AddPause(7 * ditMs) // word space — flushes letter + fires onWord

	if len(chars) != 1 || chars[0] != 'E' {
		t.Errorf("expected E before word space, got %v", string(chars))
	}
	if words != 1 {
		t.Errorf("expected 1 word space, got %d", words)
	}
}

func TestTimingDecoder_AdaptiveTiming(t *testing.T) {
	td := NewTimingDecoder(20, nil, nil)
	initial := td.GetDitMs() // 60ms

	// Feed 8 dits at 40ms each — should shift ditMs toward 40
	for i := 0; i < 8; i++ {
		td.AddSymbol(SymDit, 40)
	}

	adapted := td.GetDitMs()
	if adapted >= initial {
		t.Errorf("adaptive timing should decrease: initial=%.1f adapted=%.1f", initial, adapted)
	}
	if adapted < 35 || adapted > 45 {
		t.Errorf("expected adapted ditMs ≈40ms, got %.1f", adapted)
	}
}

func TestTimingDecoder_InterElementIgnored(t *testing.T) {
	var got []rune
	td := NewTimingDecoder(20, func(r rune) { got = append(got, r) }, nil)
	ditMs := td.GetDitMs()

	td.AddSymbol(SymDit, ditMs)
	td.AddPause(ditMs) // inter-element — should NOT flush
	td.AddSymbol(SymDit, ditMs)
	td.AddPause(ditMs)
	td.AddSymbol(SymDit, ditMs)
	td.Flush()

	if len(got) != 1 || got[0] != 'S' {
		t.Errorf("inter-element pause should not flush; expected S, got %v", string(got))
	}
}
