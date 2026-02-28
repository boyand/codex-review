// Package textutil provides shared text processing helpers.
package textutil

import "strings"

// Truncate shortens s to maxLen characters, appending "..." if truncated.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// DecisionTokens are the recognized response tokens in findings files.
var DecisionTokens = []string{"accept", "reject", "defer", "wontfix", "fix"}

// FindDecisionInLines searches lines[start:end] for a decision token.
// Returns the uppercased token found, or "" if none.
func FindDecisionInLines(lines []string, start, end int) string {
	if end > len(lines) {
		end = len(lines)
	}
	for j := start; j < end; j++ {
		lower := strings.ToLower(lines[j])
		for _, token := range DecisionTokens {
			if strings.Contains(lower, "decision:") && strings.Contains(lower, token) {
				return strings.ToUpper(token)
			}
		}
		for _, token := range DecisionTokens {
			if ContainsWord(lower, token) {
				return strings.ToUpper(token)
			}
		}
	}
	return ""
}

// ContainsWord checks if s contains word as a standalone word
// (not part of a larger alphabetic sequence).
func ContainsWord(s, word string) bool {
	idx := strings.Index(s, word)
	if idx == -1 {
		return false
	}
	if idx > 0 {
		c := s[idx-1]
		if c >= 'a' && c <= 'z' {
			return false
		}
	}
	end := idx + len(word)
	if end < len(s) {
		c := s[end]
		if c >= 'a' && c <= 'z' {
			return false
		}
	}
	return true
}
