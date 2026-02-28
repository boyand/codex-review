package textutil

import "testing"

func TestContainsWord(t *testing.T) {
	tests := []struct {
		s, word string
		want    bool
	}{
		{"decision: fix", "fix", true},
		{"prefix-fix", "fix", false},  // preceded by letter? no, by '-'
		{"fixable", "fix", false},     // followed by letter
		{"accept it", "accept", true}, // callers must lowercase before calling
		{"no match", "fix", false},
		{"fix", "fix", true},
		{"the fix is in", "fix", true},
		{"suffix", "fix", false}, // preceded by 'f' which is not a-z wait — 'suffi' ends with 'i' then 'x'... let me think: "suffix" contains "fix" at index 3, char before is 'f' (a-z) → false
	}
	for _, tt := range tests {
		got := ContainsWord(tt.s, tt.word)
		if got != tt.want {
			t.Errorf("ContainsWord(%q, %q) = %v, want %v", tt.s, tt.word, got, tt.want)
		}
	}
}
