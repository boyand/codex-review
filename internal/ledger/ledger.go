// Package ledger manages the decisions TSV file.
package ledger

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/boyand/codex-review/internal/fsx"
	"github.com/boyand/codex-review/internal/textutil"
)

const header = `# Codex Review Decisions Ledger
# Edit ` + "`selected`" + ` to ` + "`yes`" + ` for fixes you want applied.
# phase	artifact	round	reviewer	finding_id	severity	finding	decision	outcome	selected	status	updated_at
`

// Row represents one ledger entry.
type Row struct {
	Phase     string
	Artifact  string
	Round     string
	Reviewer  string
	FindingID string
	Severity  string
	Finding   string
	Decision  string
	Outcome   string
	Selected  string
	Status    string
	UpdatedAt string
}

// Counts holds finding/decision counts.
type Counts struct {
	Critical int
	High     int
	Medium   int
	Low      int
	Fix      int
	Reject   int
	Open     int
}

// RoundCounts holds per-severity breakdown with decision details.
type RoundCounts struct {
	CritTotal, CritFix, CritReject, CritOpen int
	HighTotal, HighFix, HighReject, HighOpen int
	MedTotal, MedFix, MedReject, MedOpen     int
	LowTotal, LowFix, LowReject, LowOpen     int
}

// Ledger manages the decisions TSV.
type Ledger struct {
	path string
	rows []Row
}

// NewEmpty returns a zero-value ledger safe for read operations.
func NewEmpty() *Ledger { return &Ledger{} }

// Load reads the ledger from path. Creates it if missing.
func Load(path string) (*Ledger, error) {
	l := &Ledger{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return l, nil
		}
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 12 {
			continue
		}
		l.rows = append(l.rows, Row{
			Phase:     fields[0],
			Artifact:  fields[1],
			Round:     fields[2],
			Reviewer:  fields[3],
			FindingID: fields[4],
			Severity:  fields[5],
			Finding:   fields[6],
			Decision:  fields[7],
			Outcome:   fields[8],
			Selected:  fields[9],
			Status:    fields[10],
			UpdatedAt: fields[11],
		})
	}

	return l, nil
}

// EnsureFile creates the ledger file with header if it doesn't exist.
func EnsureFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(header), 0644)
}

// Upsert adds or updates a row, preserving user-edited `selected` values.
func (l *Ledger) Upsert(r Row) {
	for i, existing := range l.rows {
		if existing.Phase == r.Phase &&
			existing.Artifact == r.Artifact &&
			existing.Round == r.Round &&
			existing.Reviewer == r.Reviewer &&
			existing.FindingID == r.FindingID {
			// Preserve user-edited selected
			if existing.Selected == "yes" {
				r.Selected = "yes"
			}
			l.rows[i] = r
			return
		}
	}
	l.rows = append(l.rows, r)
}

// Save writes the ledger atomically.
func (l *Ledger) Save() error {
	var b strings.Builder
	b.WriteString(header)
	for _, r := range l.rows {
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Phase, r.Artifact, r.Round, r.Reviewer, r.FindingID,
			r.Severity, r.Finding, r.Decision, r.Outcome,
			r.Selected, r.Status, r.UpdatedAt)
	}
	return fsx.AtomicWrite(l.path, []byte(b.String()), 0644)
}

// CountByPhase returns counts filtered by phase name (phase-cumulative for status box).
func (l *Ledger) CountByPhase(phase string) Counts {
	var c Counts
	for _, r := range l.rows {
		if r.Phase != phase {
			continue
		}
		countRow(&c, r)
	}
	return c
}

// CountByPhaseRound returns detailed counts filtered by phase and round (for approval).
func (l *Ledger) CountByPhaseRound(phase string, round int) RoundCounts {
	roundStr := strconv.Itoa(round)
	var rc RoundCounts
	for _, r := range l.rows {
		if r.Phase != phase || r.Round != roundStr {
			continue
		}
		sev := strings.ToUpper(r.Severity)
		out := strings.ToUpper(r.Outcome)
		switch sev {
		case "CRITICAL":
			rc.CritTotal++
			switch out {
			case "FIX":
				rc.CritFix++
			case "NO-CHANGE":
				rc.CritReject++
			default:
				rc.CritOpen++
			}
		case "HIGH":
			rc.HighTotal++
			switch out {
			case "FIX":
				rc.HighFix++
			case "NO-CHANGE":
				rc.HighReject++
			default:
				rc.HighOpen++
			}
		case "MEDIUM":
			rc.MedTotal++
			switch out {
			case "FIX":
				rc.MedFix++
			case "NO-CHANGE":
				rc.MedReject++
			default:
				rc.MedOpen++
			}
		case "LOW":
			rc.LowTotal++
			switch out {
			case "FIX":
				rc.LowFix++
			case "NO-CHANGE":
				rc.LowReject++
			default:
				rc.LowOpen++
			}
		}
	}
	return rc
}

// CountAll returns loop-wide counts (for completion summary).
func (l *Ledger) CountAll() Counts {
	var c Counts
	for _, r := range l.rows {
		countRow(&c, r)
	}
	return c
}

// RowsForPhaseRound returns rows matching a specific phase and round.
func (l *Ledger) RowsForPhaseRound(phase string, round int) []Row {
	roundStr := strconv.Itoa(round)
	var out []Row
	for _, r := range l.rows {
		if r.Phase == phase && r.Round == roundStr {
			out = append(out, r)
		}
	}
	return out
}

// RowsForPhaseRoundOutcome returns rows for a phase/round filtered by outcome token.
func (l *Ledger) RowsForPhaseRoundOutcome(phase string, round int, outcome string) []Row {
	roundStr := strconv.Itoa(round)
	outcome = strings.ToUpper(strings.TrimSpace(outcome))
	var out []Row
	for _, r := range l.rows {
		if r.Phase != phase || r.Round != roundStr {
			continue
		}
		if strings.ToUpper(strings.TrimSpace(r.Outcome)) == outcome {
			out = append(out, r)
		}
	}
	return out
}

// CountSelected returns how many rows in the slice have selected=yes.
func CountSelected(rows []Row) int {
	total := 0
	for _, r := range rows {
		if strings.EqualFold(strings.TrimSpace(r.Selected), "yes") {
			total++
		}
	}
	return total
}

func countRow(c *Counts, r Row) {
	sev := strings.ToUpper(r.Severity)
	switch sev {
	case "CRITICAL":
		c.Critical++
	case "HIGH":
		c.High++
	case "MEDIUM":
		c.Medium++
	case "LOW":
		c.Low++
	}

	out := strings.ToUpper(r.Outcome)
	switch out {
	case "FIX":
		c.Fix++
	case "NO-CHANGE":
		c.Reject++
	default:
		c.Open++
	}
}

// --- Finding extraction and decision sync ---

var findingRe = regexp.MustCompile(`\[(CRITICAL|HIGH|MEDIUM|LOW)-(\d+)\]`)

// ExtractFindings extracts unique finding IDs from review content.
// Returns slice of (id, severity, text) tuples.
func ExtractFindings(content string) []struct{ ID, Severity, Text string } {
	lines := strings.Split(content, "\n")
	seen := make(map[string]bool)
	var results []struct{ ID, Severity, Text string }

	for _, line := range lines {
		idx := findingRe.FindStringSubmatchIndex(line)
		if idx == nil {
			continue
		}
		sev := line[idx[2]:idx[3]]
		num := line[idx[4]:idx[5]]
		id := sev + "-" + num
		if seen[id] {
			continue
		}
		seen[id] = true

		text := strings.TrimSpace(line[idx[1]:])
		if text == "" {
			text = "(no inline description)"
		}
		text = Sanitize(text)

		results = append(results, struct{ ID, Severity, Text string }{
			ID:       id,
			Severity: sev,
			Text:     text,
		})
	}
	return results
}

// ExtractDecisionForID searches a findings response file for the decision
// for a given finding ID.
func ExtractDecisionForID(findingID, content string) string {
	return extractDecisionFromLines(findingID, strings.Split(content, "\n"))
}

func extractDecisionFromLines(findingID string, lines []string) string {
	for i, line := range lines {
		if !strings.Contains(line, findingID) {
			continue
		}
		if tok := textutil.FindDecisionInLines(lines, i, i+9); tok != "" {
			return tok
		}
		return "OPEN"
	}
	return "OPEN"
}

// SyncDecisionsForRound syncs findings from a review into the ledger.
func (l *Ledger) SyncDecisionsForRound(phase, artifact string, round int, reviewer string, reviewContent, findingsContent string) {
	findings := ExtractFindings(reviewContent)
	roundStr := strconv.Itoa(round)
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	var findingsLines []string
	if findingsContent != "" {
		findingsLines = strings.Split(findingsContent, "\n")
	}

	for _, f := range findings {
		decision := "OPEN"
		if findingsLines != nil {
			decision = extractDecisionFromLines(f.ID, findingsLines)
		}

		outcome := DecisionToOutcome(decision)
		status := DecisionToStatus(decision)

		l.Upsert(Row{
			Phase:     phase,
			Artifact:  artifact,
			Round:     roundStr,
			Reviewer:  reviewer,
			FindingID: f.ID,
			Severity:  Sanitize(f.Severity),
			Finding:   Sanitize(f.Text),
			Decision:  Sanitize(decision),
			Outcome:   Sanitize(outcome),
			Selected:  DefaultSelectedForOutcome(outcome),
			Status:    Sanitize(status),
			UpdatedAt: timestamp,
		})
	}
}

// DecisionToOutcome maps a decision token to an outcome.
func DecisionToOutcome(decision string) string {
	switch strings.ToUpper(decision) {
	case "FIX", "ACCEPT":
		return "FIX"
	case "REJECT", "WONTFIX", "DEFER":
		return "NO-CHANGE"
	default:
		return "OPEN"
	}
}

// DecisionToStatus maps a decision to a status label.
func DecisionToStatus(decision string) string {
	switch strings.ToUpper(decision) {
	case "FIX", "ACCEPT":
		return "agreed-fix"
	case "REJECT", "WONTFIX", "DEFER":
		return "agreed-nochange"
	default:
		return "proposed"
	}
}

// DefaultSelectedForOutcome returns the default selected value for an outcome.
func DefaultSelectedForOutcome(outcome string) string {
	if strings.ToUpper(outcome) == "FIX" {
		return "yes"
	}
	return "no"
}

var sanitizeReplacer = strings.NewReplacer("\t", " ", "\r", " ", "\n", " ")

// Sanitize collapses tabs, carriage returns, and newlines to spaces,
// then normalizes multiple spaces.
func Sanitize(s string) string {
	return strings.Join(strings.Fields(sanitizeReplacer.Replace(s)), " ")
}
