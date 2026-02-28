package state

import (
	"os"
	"path/filepath"
	"testing"
)

func writeStateFile(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "state.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadNonExistent(t *testing.T) {
	s, err := Load("/nonexistent/state.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != nil {
		t.Error("expected nil for nonexistent file")
	}
}

func TestLoadAndGetters(t *testing.T) {
	dir := t.TempDir()
	content := `---
active: true
review_id: test-id
current_phase_index: 1
current_substep: review
current_round: 2
max_rounds: 5
pipeline_count: 2
pipeline_0_name: plan
pipeline_0_status: completed
pipeline_0_rounds: 1
pipeline_0_worker: claude
pipeline_0_reviewer: codex
pipeline_0_artifact: plan
pipeline_1_name: implement
pipeline_1_status: working
pipeline_1_worker: claude
pipeline_1_reviewer: codex
pipeline_1_artifact: implement
pipeline_1_compare_to: plan
task_description: Build auth system
---
`
	path := writeStateFile(t, dir, content)
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !s.Active() {
		t.Error("Active() = false, want true")
	}
	if s.ReviewID() != "test-id" {
		t.Errorf("ReviewID() = %q", s.ReviewID())
	}
	if s.CurrentPhaseIndex() != 1 {
		t.Errorf("CurrentPhaseIndex() = %d", s.CurrentPhaseIndex())
	}
	if s.CurrentSubstep() != "review" {
		t.Errorf("CurrentSubstep() = %q", s.CurrentSubstep())
	}
	if s.CurrentRound() != 2 {
		t.Errorf("CurrentRound() = %d", s.CurrentRound())
	}
	if s.MaxRounds() != 5 {
		t.Errorf("MaxRounds() = %d", s.MaxRounds())
	}
	if s.PipelineCount() != 2 {
		t.Errorf("PipelineCount() = %d", s.PipelineCount())
	}
	if s.TaskDescription() != "Build auth system" {
		t.Errorf("TaskDescription() = %q", s.TaskDescription())
	}

	if s.PipelineName(0) != "plan" {
		t.Errorf("PipelineName(0) = %q", s.PipelineName(0))
	}
	if s.PipelineStatus(0) != "completed" {
		t.Errorf("PipelineStatus(0) = %q", s.PipelineStatus(0))
	}
	if s.PipelineRounds(0) != 1 {
		t.Errorf("PipelineRounds(0) = %d", s.PipelineRounds(0))
	}
	if s.PipelineCompareTo(1) != "plan" {
		t.Errorf("PipelineCompareTo(1) = %q", s.PipelineCompareTo(1))
	}
}

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	content := `---
active: true
review_id: save-test
current_substep: working
---
`
	path := writeStateFile(t, dir, content)
	s, _ := Load(path)

	s.SetCurrentSubstep("review")
	s.SetCurrentRound(3)
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2, _ := Load(path)
	if s2.CurrentSubstep() != "review" {
		t.Errorf("after save CurrentSubstep = %q", s2.CurrentSubstep())
	}
	if s2.CurrentRound() != 3 {
		t.Errorf("after save CurrentRound = %d", s2.CurrentRound())
	}
	// Original field preserved
	if s2.ReviewID() != "save-test" {
		t.Errorf("after save ReviewID = %q", s2.ReviewID())
	}
}

func TestMaxRoundsDefault(t *testing.T) {
	dir := t.TempDir()
	content := `---
active: true
---
`
	path := writeStateFile(t, dir, content)
	s, _ := Load(path)

	if s.MaxRounds() != 10 {
		t.Errorf("MaxRounds() default = %d, want 10", s.MaxRounds())
	}
}
