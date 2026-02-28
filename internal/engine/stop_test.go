package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boyand/codex-review-loop/internal/config"
	"github.com/boyand/codex-review-loop/internal/state"
)

func setupWorkDir(t *testing.T) (string, func()) {
	t.Helper()
	orig, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	os.MkdirAll(".claude/codex-review-loop", 0755)
	return dir, func() { os.Chdir(orig) }
}

func writeState(t *testing.T, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(state.StateFile), 0755)
	os.WriteFile(state.StateFile, []byte(content), 0644)
}

func TestRunStopNoActiveLoop(t *testing.T) {
	_, cleanup := setupWorkDir(t)
	defer cleanup()

	cfg := config.Config{}
	var stdout, stderr strings.Builder
	code, err := RunStop(cfg, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRunStopApprovalSubstep(t *testing.T) {
	_, cleanup := setupWorkDir(t)
	defer cleanup()

	writeState(t, `---
active: true
review_id: test-123
current_phase_index: 0
current_substep: approval
current_round: 1
max_rounds: 10
pipeline_count: 1
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_worker: claude
pipeline_0_reviewer: codex
pipeline_0_artifact: plan
task_description: Test task
---
`)

	cfg := config.Config{}
	var stdout, stderr strings.Builder
	code, err := RunStop(cfg, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0 for approval substep", code)
	}
}

func TestRunStopWorkingClaudeWorkerClaudeReviewer(t *testing.T) {
	_, cleanup := setupWorkDir(t)
	defer cleanup()

	writeState(t, `---
active: true
review_id: test-456
current_phase_index: 0
current_substep: working
current_round: 1
max_rounds: 10
pipeline_count: 1
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_worker: claude
pipeline_0_reviewer: claude
pipeline_0_artifact: plan
task_description: Build auth system
---
`)

	// Create plan artifact so it passes validation
	os.WriteFile(".claude/codex-review-loop/plan.md", []byte("# Plan\nDo stuff"), 0644)

	cfg := config.Config{}
	var stdout, stderr strings.Builder
	code, err := RunStop(cfg, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 2 {
		t.Errorf("exit code = %d, want 2 for working substep block", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "review") {
		t.Errorf("expected review instructions in output, got: %s", out)
	}

	// Verify state was updated to review
	s, _ := state.Load(state.StateFile)
	if s.CurrentSubstep() != "review" {
		t.Errorf("substep = %q, want %q", s.CurrentSubstep(), "review")
	}
}

func TestRunStopWorkingMissingPlanArtifact(t *testing.T) {
	_, cleanup := setupWorkDir(t)
	defer cleanup()

	writeState(t, `---
active: true
review_id: test-789
current_phase_index: 0
current_substep: working
current_round: 1
max_rounds: 10
pipeline_count: 1
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_worker: claude
pipeline_0_reviewer: claude
pipeline_0_artifact: plan
task_description: Build auth
---
`)

	cfg := config.Config{}
	var stdout, stderr strings.Builder
	code, _ := RunStop(cfg, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stdout.String(), "Plan phase requires") {
		t.Error("expected plan artifact missing message")
	}
}

func TestRunStopReviewClaudeReviewerMissingReview(t *testing.T) {
	_, cleanup := setupWorkDir(t)
	defer cleanup()

	writeState(t, `---
active: true
review_id: test-rev
current_phase_index: 0
current_substep: review
current_round: 1
max_rounds: 10
pipeline_count: 1
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_worker: claude
pipeline_0_reviewer: claude
pipeline_0_artifact: plan
task_description: Build auth
---
`)

	cfg := config.Config{}
	var stdout, stderr strings.Builder
	code, _ := RunStop(cfg, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stdout.String(), "review output is missing") {
		t.Errorf("expected missing review message, got: %s", stdout.String())
	}
}

func TestRunStopReviewClaudeReviewerWithReview(t *testing.T) {
	_, cleanup := setupWorkDir(t)
	defer cleanup()

	writeState(t, `---
active: true
review_id: test-rev2
current_phase_index: 0
current_substep: review
current_round: 1
max_rounds: 10
pipeline_count: 1
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_worker: claude
pipeline_0_reviewer: claude
pipeline_0_artifact: plan
task_description: Build auth
---
`)

	// Create review file
	os.WriteFile(".claude/codex-review-loop/plan-review-r1.md", []byte("[HIGH-1] Missing validation\n\n### Summary\n- Critical: 0\n- High: 1\n- Medium: 0\n- Low: 0"), 0644)

	cfg := config.Config{}
	var stdout, stderr strings.Builder
	code, _ := RunStop(cfg, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stdout.String(), "Approval Gate") {
		t.Errorf("expected approval summary in output")
	}

	// Verify state transitioned to approval
	s, _ := state.Load(state.StateFile)
	if s.CurrentSubstep() != "approval" {
		t.Errorf("substep = %q, want %q", s.CurrentSubstep(), "approval")
	}
}

func TestRunStopUnknownSubstep(t *testing.T) {
	_, cleanup := setupWorkDir(t)
	defer cleanup()

	writeState(t, `---
active: true
review_id: test-unk
current_phase_index: 0
current_substep: invalid
current_round: 1
max_rounds: 10
pipeline_count: 1
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_worker: claude
pipeline_0_reviewer: claude
pipeline_0_artifact: plan
task_description: Test
---
`)

	cfg := config.Config{}
	var stdout, stderr strings.Builder
	code, _ := RunStop(cfg, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stdout.String(), "Unknown loop substep") {
		t.Error("expected unknown substep message")
	}
}

func TestRunStopMaxRoundsExceeded(t *testing.T) {
	_, cleanup := setupWorkDir(t)
	defer cleanup()

	writeState(t, `---
active: true
review_id: test-max
current_phase_index: 0
current_substep: working
current_round: 11
max_rounds: 10
pipeline_count: 1
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_worker: claude
pipeline_0_reviewer: claude
pipeline_0_artifact: plan
task_description: Test
---
`)

	cfg := config.Config{}
	var stdout, stderr strings.Builder
	_, _ = RunStop(cfg, &stdout, &stderr)
	// Max rounds should still exit 2 (we output a message), but the message says "Allowing stop"
	if !strings.Contains(stdout.String(), "Maximum review rounds") {
		t.Errorf("expected max rounds message, got: %s", stdout.String())
	}
}

func TestRunStatus(t *testing.T) {
	_, cleanup := setupWorkDir(t)
	defer cleanup()

	// No active loop
	var buf strings.Builder
	code, _ := RunStatus(&buf)
	if code != 0 {
		t.Errorf("code = %d", code)
	}
	if !strings.Contains(buf.String(), "No active") {
		t.Error("expected no active loop message")
	}
}

func TestRunCompletion(t *testing.T) {
	_, cleanup := setupWorkDir(t)
	defer cleanup()

	// No state file
	var buf strings.Builder
	code, _ := RunCompletion(&buf)
	if code != 0 {
		t.Errorf("code = %d", code)
	}
	if !strings.Contains(buf.String(), "No state file") {
		t.Error("expected no state file message")
	}
}

func TestRunStopReviewCodexReviewerMissingFindings(t *testing.T) {
	_, cleanup := setupWorkDir(t)
	defer cleanup()

	writeState(t, `---
active: true
review_id: test-codex-rev
current_phase_index: 0
current_substep: review
current_round: 1
max_rounds: 10
pipeline_count: 1
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_worker: claude
pipeline_0_reviewer: codex
pipeline_0_artifact: plan
task_description: Build auth
---
`)

	// Create review file but no findings file
	os.WriteFile(".claude/codex-review-loop/plan-review-r1.md", []byte("[HIGH-1] Issue found"), 0644)

	cfg := config.Config{}
	var stdout, stderr strings.Builder
	code, _ := RunStop(cfg, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stdout.String(), "not yet written") {
		t.Errorf("expected missing findings message, got: %s", stdout.String())
	}
}

func TestRunStopReviewCodexReviewerWithFindings(t *testing.T) {
	_, cleanup := setupWorkDir(t)
	defer cleanup()

	writeState(t, `---
active: true
review_id: test-codex-rev2
current_phase_index: 0
current_substep: review
current_round: 1
max_rounds: 10
pipeline_count: 1
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_worker: claude
pipeline_0_reviewer: codex
pipeline_0_artifact: plan
task_description: Build auth
---
`)

	// Create review and findings files
	os.WriteFile(".claude/codex-review-loop/plan-review-r1.md",
		[]byte("[HIGH-1] Missing validation\n\n### Summary\n- Critical: 0\n- High: 1\n- Medium: 0\n- Low: 0"), 0644)
	os.WriteFile(".claude/codex-review-loop/plan-findings-r1.md",
		[]byte("### HIGH-1\n**Decision: FIX**\nAdded validation."), 0644)

	cfg := config.Config{}
	var stdout, stderr strings.Builder
	code, _ := RunStop(cfg, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "Approval Gate") {
		t.Errorf("expected approval summary, got: %s", out)
	}

	// Verify state transitioned to approval
	s, _ := state.Load(state.StateFile)
	if s.CurrentSubstep() != "approval" {
		t.Errorf("substep = %q, want %q", s.CurrentSubstep(), "approval")
	}

	// Verify decisions file was created/updated
	if _, err := os.Stat(state.DecisionsFile); err != nil {
		t.Errorf("decisions file should exist: %v", err)
	}
}

func TestRunStopReviewCodexReviewerIncompleteCoverage(t *testing.T) {
	_, cleanup := setupWorkDir(t)
	defer cleanup()

	writeState(t, `---
active: true
review_id: test-codex-cov
current_phase_index: 0
current_substep: review
current_round: 1
max_rounds: 10
pipeline_count: 1
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_worker: claude
pipeline_0_reviewer: codex
pipeline_0_artifact: plan
task_description: Build auth
---
`)

	// Review has 2 findings but findings only addresses 1
	os.WriteFile(".claude/codex-review-loop/plan-review-r1.md",
		[]byte("[HIGH-1] Issue one\n[MEDIUM-1] Issue two"), 0644)
	os.WriteFile(".claude/codex-review-loop/plan-findings-r1.md",
		[]byte("### HIGH-1\n**Decision: FIX**\nDone."), 0644)

	cfg := config.Config{}
	var stdout, stderr strings.Builder
	code, _ := RunStop(cfg, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stdout.String(), "Missing finding IDs") {
		t.Errorf("expected missing finding IDs message, got: %s", stdout.String())
	}
}

func TestRunStopWorkingCodexWorkerNotOnPath(t *testing.T) {
	_, cleanup := setupWorkDir(t)
	defer cleanup()

	writeState(t, `---
active: true
review_id: test-no-codex
current_phase_index: 0
current_substep: working
current_round: 1
max_rounds: 10
pipeline_count: 1
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_worker: codex
pipeline_0_reviewer: claude
pipeline_0_artifact: plan
task_description: Build auth
---
`)

	// Use a PATH that definitely won't have codex
	t.Setenv("PATH", t.TempDir())
	cfg := config.Config{}
	var stdout, stderr strings.Builder
	code, _ := RunStop(cfg, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stdout.String(), "codex CLI is required") {
		t.Errorf("expected codex missing message, got: %s", stdout.String())
	}
}

func TestRunStopMissingReviewID(t *testing.T) {
	_, cleanup := setupWorkDir(t)
	defer cleanup()

	writeState(t, `---
active: true
current_phase_index: 0
current_substep: working
current_round: 1
max_rounds: 10
pipeline_count: 1
pipeline_0_name: plan
pipeline_0_status: working
task_description: Test
---
`)

	cfg := config.Config{}
	var stdout, stderr strings.Builder
	code, _ := RunStop(cfg, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stdout.String(), "missing review_id") {
		t.Error("expected missing review_id message")
	}
}
