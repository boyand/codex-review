package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boyand/codex-review-loop/internal/ledger"
	"github.com/boyand/codex-review-loop/internal/state"
)

func makeState(t *testing.T, content string) *state.LoopState {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.md")
	os.WriteFile(path, []byte(content), 0644)
	s, err := state.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStatus(t *testing.T) {
	s := makeState(t, `---
active: true
review_id: test-123
current_phase_index: 0
current_substep: working
current_round: 1
pipeline_count: 2
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_worker: claude
pipeline_0_reviewer: codex
pipeline_1_name: implement
pipeline_1_status: pending
task_description: Build JWT auth
---
`)
	l := &ledger.Ledger{}

	var buf strings.Builder
	Status(&buf, s, l)
	out := buf.String()

	if !strings.Contains(out, "Codex Review Loop") {
		t.Error("missing header")
	}
	if !strings.Contains(out, "Build JWT auth") {
		t.Error("missing task description")
	}
	if !strings.Contains(out, "test-123") {
		t.Error("missing review ID")
	}
	if !strings.Contains(out, "plan") {
		t.Error("missing phase name")
	}
	if !strings.Contains(out, "Findings: none yet") {
		t.Error("missing findings line")
	}
}

func TestApprovalSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "decisions.tsv")
	l := &ledger.Ledger{}
	l.Upsert(ledger.Row{
		Phase: "plan", Artifact: "plan", Round: "1", Reviewer: "codex",
		FindingID: "HIGH-1", Severity: "HIGH", Finding: "Missing validation",
		Decision: "FIX", Outcome: "FIX", Selected: "yes",
		Status: "agreed-fix", UpdatedAt: "2024-01-01T00:00:00Z",
	})
	// Force save/load to make it real
	_ = path

	var buf strings.Builder
	ApprovalSummary(&buf, "plan", 1, l)
	out := buf.String()

	if !strings.Contains(out, "Approval Gate") {
		t.Error("missing approval header")
	}
	if !strings.Contains(out, "HIGH-1") {
		t.Error("missing finding ID")
	}
	if !strings.Contains(out, "approve") {
		t.Error("missing approve choice")
	}
	if !strings.Contains(out, "round 2") {
		t.Error("missing next round number")
	}
}

func TestCompletion(t *testing.T) {
	s := makeState(t, `---
active: true
review_id: test-456
current_phase_index: 1
current_substep: approval
current_round: 1
pipeline_count: 2
pipeline_0_name: plan
pipeline_0_status: completed
pipeline_0_rounds: 2
pipeline_1_name: implement
pipeline_1_status: completed
pipeline_1_rounds: 1
task_description: Build JWT auth
---
`)
	l := &ledger.Ledger{}

	var buf strings.Builder
	Completion(&buf, s, l)
	out := buf.String()

	if !strings.Contains(out, "Complete") {
		t.Error("missing Complete header")
	}
	if !strings.Contains(out, "plan") {
		t.Error("missing plan phase")
	}
	if !strings.Contains(out, "implement") {
		t.Error("missing implement phase")
	}
}

func TestSanitizeFinding(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal text", "normal text"},
		{"`code` and **bold**", "code and bold"},
		{strings.Repeat("a", 100), strings.Repeat("a", 57) + "..."},
	}
	for _, tt := range tests {
		got := sanitizeFinding(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFinding(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStateBody(t *testing.T) {
	s := makeState(t, `---
active: true
current_phase_index: 0
current_substep: working
current_round: 1
pipeline_count: 2
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_worker: claude
pipeline_0_reviewer: codex
pipeline_1_name: implement
pipeline_1_status: pending
pipeline_1_worker: claude
pipeline_1_reviewer: codex
task_description: Build auth
---
`)
	l := &ledger.Ledger{}

	var buf strings.Builder
	StateBody(&buf, s, l, t.TempDir())
	out := buf.String()

	if !strings.Contains(out, "auto-generated") {
		t.Error("missing auto-gen comment")
	}
	if !strings.Contains(out, "Build auth") {
		t.Error("missing task description")
	}
	if !strings.Contains(out, "plan → implement") {
		t.Error("missing pipeline string")
	}
}

func TestStatusLongTask(t *testing.T) {
	s := makeState(t, `---
active: true
review_id: x
current_phase_index: 0
current_substep: working
current_round: 1
pipeline_count: 1
pipeline_0_name: plan
pipeline_0_status: working
task_description: This is a very long task description that should be truncated to fit
---
`)
	l := &ledger.Ledger{}

	var buf strings.Builder
	Status(&buf, s, l)
	out := buf.String()
	if !strings.Contains(out, "...") {
		t.Error("expected truncation of long task description")
	}
}
