package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "decisions.tsv")

	if err := EnsureFile(path); err != nil {
		t.Fatalf("EnsureFile: %v", err)
	}
	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		t.Error("file should have header content")
	}

	// Second call should be no-op
	if err := EnsureFile(path); err != nil {
		t.Fatalf("EnsureFile second call: %v", err)
	}
}

func TestUpsertAndPreserveSelected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "decisions.tsv")

	l := &Ledger{path: path}

	// Add initial row with selected=yes
	l.Upsert(Row{
		Phase: "plan", Artifact: "plan", Round: "1", Reviewer: "codex",
		FindingID: "HIGH-1", Severity: "HIGH", Finding: "issue",
		Decision: "FIX", Outcome: "FIX", Selected: "yes",
		Status: "agreed-fix", UpdatedAt: "2024-01-01T00:00:00Z",
	})

	if len(l.rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(l.rows))
	}

	// Upsert same key with selected=no — should preserve yes
	l.Upsert(Row{
		Phase: "plan", Artifact: "plan", Round: "1", Reviewer: "codex",
		FindingID: "HIGH-1", Severity: "HIGH", Finding: "updated issue",
		Decision: "FIX", Outcome: "FIX", Selected: "no",
		Status: "agreed-fix", UpdatedAt: "2024-01-02T00:00:00Z",
	})

	if len(l.rows) != 1 {
		t.Fatalf("expected 1 row after upsert, got %d", len(l.rows))
	}
	if l.rows[0].Selected != "yes" {
		t.Errorf("Selected = %q, want %q (should be preserved)", l.rows[0].Selected, "yes")
	}
	if l.rows[0].Finding != "updated issue" {
		t.Errorf("Finding = %q, want %q", l.rows[0].Finding, "updated issue")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "decisions.tsv")

	l := &Ledger{path: path}
	l.Upsert(Row{
		Phase: "plan", Artifact: "plan", Round: "1", Reviewer: "codex",
		FindingID: "HIGH-1", Severity: "HIGH", Finding: "test finding",
		Decision: "FIX", Outcome: "FIX", Selected: "yes",
		Status: "agreed-fix", UpdatedAt: "2024-01-01T00:00:00Z",
	})
	l.Upsert(Row{
		Phase: "plan", Artifact: "plan", Round: "1", Reviewer: "codex",
		FindingID: "MEDIUM-1", Severity: "MEDIUM", Finding: "minor thing",
		Decision: "REJECT", Outcome: "NO-CHANGE", Selected: "no",
		Status: "agreed-nochange", UpdatedAt: "2024-01-01T00:00:00Z",
	})

	if err := l.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	l2, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(l2.rows) != 2 {
		t.Fatalf("loaded %d rows, want 2", len(l2.rows))
	}
	if l2.rows[0].FindingID != "HIGH-1" {
		t.Errorf("row[0].FindingID = %q", l2.rows[0].FindingID)
	}
}

func TestCountByPhase(t *testing.T) {
	l := &Ledger{}
	l.rows = []Row{
		{Phase: "plan", Severity: "CRITICAL", Outcome: "FIX"},
		{Phase: "plan", Severity: "HIGH", Outcome: "NO-CHANGE"},
		{Phase: "plan", Severity: "MEDIUM", Outcome: "OPEN"},
		{Phase: "implement", Severity: "LOW", Outcome: "FIX"},
	}

	c := l.CountByPhase("plan")
	if c.Critical != 1 || c.High != 1 || c.Medium != 1 || c.Low != 0 {
		t.Errorf("severity counts: %+v", c)
	}
	if c.Fix != 1 || c.Reject != 1 || c.Open != 1 {
		t.Errorf("decision counts: %+v", c)
	}
}

func TestCountByPhaseRound(t *testing.T) {
	l := &Ledger{}
	l.rows = []Row{
		{Phase: "plan", Round: "1", Severity: "CRITICAL", Outcome: "FIX"},
		{Phase: "plan", Round: "1", Severity: "HIGH", Outcome: "NO-CHANGE"},
		{Phase: "plan", Round: "2", Severity: "MEDIUM", Outcome: "FIX"},
	}

	rc := l.CountByPhaseRound("plan", 1)
	if rc.CritTotal != 1 || rc.CritFix != 1 {
		t.Errorf("crit: total=%d fix=%d", rc.CritTotal, rc.CritFix)
	}
	if rc.HighTotal != 1 || rc.HighReject != 1 {
		t.Errorf("high: total=%d reject=%d", rc.HighTotal, rc.HighReject)
	}
	if rc.MedTotal != 0 {
		t.Errorf("med should be 0 for round 1, got %d", rc.MedTotal)
	}
}

func TestCountAll(t *testing.T) {
	l := &Ledger{}
	l.rows = []Row{
		{Phase: "plan", Severity: "HIGH", Outcome: "FIX"},
		{Phase: "implement", Severity: "LOW", Outcome: "NO-CHANGE"},
	}

	c := l.CountAll()
	if c.High != 1 || c.Low != 1 {
		t.Errorf("counts: %+v", c)
	}
	if c.Fix != 1 || c.Reject != 1 {
		t.Errorf("decision counts: %+v", c)
	}
}

func TestExtractFindings(t *testing.T) {
	content := `## Review

[CRITICAL-1] Missing validation in auth handler
[HIGH-1] No rate limiting
[HIGH-1] duplicate should be skipped
[MEDIUM-1] Consider using constants
`
	findings := ExtractFindings(content)
	if len(findings) != 3 {
		t.Fatalf("expected 3 unique findings, got %d", len(findings))
	}
	if findings[0].ID != "CRITICAL-1" || findings[0].Severity != "CRITICAL" {
		t.Errorf("finding[0] = %+v", findings[0])
	}
}

func TestExtractDecisionForID(t *testing.T) {
	content := `### HIGH-1: Missing validation
**Decision: FIX**
Added validation in handler.

### MEDIUM-1: Consider constants
**Decision: REJECT**
Not needed for this scope.
`
	tests := []struct {
		id, want string
	}{
		{"HIGH-1", "FIX"},
		{"MEDIUM-1", "REJECT"},
		{"LOW-1", "OPEN"}, // not found
	}
	for _, tt := range tests {
		got := ExtractDecisionForID(tt.id, content)
		if got != tt.want {
			t.Errorf("ExtractDecisionForID(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestDecisionToOutcome(t *testing.T) {
	tests := []struct {
		decision, want string
	}{
		{"FIX", "FIX"},
		{"ACCEPT", "FIX"},
		{"REJECT", "NO-CHANGE"},
		{"WONTFIX", "NO-CHANGE"},
		{"DEFER", "NO-CHANGE"},
		{"OPEN", "OPEN"},
		{"unknown", "OPEN"},
	}
	for _, tt := range tests {
		if got := DecisionToOutcome(tt.decision); got != tt.want {
			t.Errorf("DecisionToOutcome(%q) = %q, want %q", tt.decision, got, tt.want)
		}
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello\tworld\nnew", "hello world new"},
		{"  spaces  ", "spaces"},
		{"tab\there", "tab here"},
	}
	for _, tt := range tests {
		if got := Sanitize(tt.input); got != tt.want {
			t.Errorf("Sanitize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSyncDecisionsForRound(t *testing.T) {
	l := &Ledger{path: "test.tsv"}

	reviewContent := `## Review
[HIGH-1] Missing input validation
[MEDIUM-1] Consider error handling
`
	findingsContent := `### HIGH-1
**Decision: FIX**
Added validation.

### MEDIUM-1
**Decision: REJECT**
Not needed.
`
	l.SyncDecisionsForRound("plan", "plan", 1, "codex", reviewContent, findingsContent)

	if len(l.rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(l.rows))
	}
	if l.rows[0].Decision != "FIX" || l.rows[0].Outcome != "FIX" || l.rows[0].Selected != "yes" {
		t.Errorf("row[0] = %+v", l.rows[0])
	}
	if l.rows[1].Decision != "REJECT" || l.rows[1].Outcome != "NO-CHANGE" || l.rows[1].Selected != "no" {
		t.Errorf("row[1] = %+v", l.rows[1])
	}
}

func TestSyncDecisionsPreservesSelected(t *testing.T) {
	l := &Ledger{path: "test.tsv"}

	reviewContent := "[HIGH-1] Missing validation\n"
	findingsContent := "### HIGH-1\n**Decision: FIX**\nDone.\n"

	// First sync — sets selected=yes (default for FIX)
	l.SyncDecisionsForRound("plan", "plan", 1, "codex", reviewContent, findingsContent)
	if l.rows[0].Selected != "yes" {
		t.Fatalf("initial selected = %q, want yes", l.rows[0].Selected)
	}

	// User edits selected to yes — simulate by ensuring it's yes
	// Now re-sync with same data but where default would be "no" (e.g., REJECT)
	findingsContent2 := "### HIGH-1\n**Decision: REJECT**\nNot needed.\n"
	l.SyncDecisionsForRound("plan", "plan", 1, "codex", reviewContent, findingsContent2)

	// The selected=yes from the first sync should be preserved
	if l.rows[0].Selected != "yes" {
		t.Errorf("after re-sync selected = %q, want yes (preserved)", l.rows[0].Selected)
	}
	if l.rows[0].Decision != "REJECT" {
		t.Errorf("decision = %q, want REJECT", l.rows[0].Decision)
	}
}

func TestLoadNonExistent(t *testing.T) {
	l, err := Load("/nonexistent/file.tsv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(l.rows) != 0 {
		t.Errorf("expected 0 rows for nonexistent file")
	}
}
