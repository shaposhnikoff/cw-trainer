package decoder

import "testing"

func TestDecode(t *testing.T) {
	tests := []struct {
		symbols []Symbol
		want    rune
		ok      bool
	}{
		{[]Symbol{SymDit}, 'E', true},
		{[]Symbol{SymDah}, 'T', true},
		{[]Symbol{SymDit, SymDit, SymDit}, 'S', true},
		{[]Symbol{SymDah, SymDah, SymDah}, 'O', true},
		{[]Symbol{SymDit, SymDah}, 'A', true},
		{[]Symbol{SymDah, SymDit, SymDit, SymDit}, 'B', true},
		{[]Symbol{SymDah, SymDit, SymDah, SymDit}, 'C', true},
		{[]Symbol{SymDit, SymDah, SymDah, SymDah}, 'J', true},
		{[]Symbol{SymDah, SymDah, SymDah, SymDah, SymDah}, '0', true},
		{[]Symbol{SymDit, SymDah, SymDah, SymDah, SymDah}, '1', true},
		// unknown
		{[]Symbol{SymDit, SymDit, SymDit, SymDit, SymDit, SymDit}, 0, false},
		{[]Symbol{}, 0, false},
	}

	for _, tc := range tests {
		got, ok := Decode(tc.symbols)
		if ok != tc.ok {
			t.Errorf("Decode(%v): ok=%v, want %v", tc.symbols, ok, tc.ok)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("Decode(%v) = %c, want %c", tc.symbols, got, tc.want)
		}
	}
}
