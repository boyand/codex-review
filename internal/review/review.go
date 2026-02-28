// Package review validates Codex review output and findings coverage.
package review

import (
	"regexp"
	"strings"

	"github.com/boyand/codex-review-loop/internal/textutil"
)

var severityRe = regexp.MustCompile(`\[(CRITICAL|HIGH|MEDIUM|LOW)-\d+\]`)
var noFindingsCountRe = regexp.MustCompile(`(critical|high|medium|low):\s*\d+`)

// IsValid checks if review content is valid: has severity-tagged findings
// or is an explicit no-findings form with summary counts.
func IsValid(content string) bool {
	if severityRe.MatchString(content) {
		return true
	}
	return isNoFindingsForm(content)
}

func isNoFindingsForm(content string) bool {
	lower := strings.ToLower(content)
	lines := strings.Split(lower, "\n")

	var hasNoFindings, hasCritical, hasHigh, hasMedium, hasLow bool

	for _, line := range lines {
		if strings.Contains(line, "no findings") {
			hasNoFindings = true
		}
		matches := noFindingsCountRe.FindAllString(line, -1)
		for _, m := range matches {
			if strings.HasPrefix(m, "critical") {
				hasCritical = true
			} else if strings.HasPrefix(m, "high") {
				hasHigh = true
			} else if strings.HasPrefix(m, "medium") {
				hasMedium = true
			} else if strings.HasPrefix(m, "low") {
				hasLow = true
			}
		}
	}

	return hasNoFindings && hasCritical && hasHigh && hasMedium && hasLow
}

// ExtractFindingIDs returns unique finding IDs from review content.
func ExtractFindingIDs(content string) []string {
	matches := severityRe.FindAllString(content, -1)
	seen := make(map[string]bool)
	var ids []string
	for _, m := range matches {
		id := m[1 : len(m)-1] // strip [ ]
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// FindingCoverage describes which findings are missing from a response.
type FindingCoverage struct {
	MissingIDs       []string
	MissingDecisions []string
}

// VerifyFindingsCoverage checks that every finding ID from the review
// is addressed in the findings response with a valid decision.
func VerifyFindingsCoverage(reviewContent, findingsContent string) *FindingCoverage {
	ids := ExtractFindingIDs(reviewContent)
	if len(ids) == 0 {
		return nil
	}

	fc := &FindingCoverage{}
	for _, id := range ids {
		if !strings.Contains(findingsContent, id) {
			fc.MissingIDs = append(fc.MissingIDs, id)
			continue
		}
		if !idHasDecision(id, findingsContent) {
			fc.MissingDecisions = append(fc.MissingDecisions, id)
		}
	}

	if len(fc.MissingIDs) == 0 && len(fc.MissingDecisions) == 0 {
		return nil
	}
	return fc
}

func idHasDecision(id, content string) bool {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if !strings.Contains(line, id) {
			continue
		}
		return textutil.FindDecisionInLines(lines, i, i+6) != ""
	}
	return false
}

