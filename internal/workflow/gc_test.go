package workflow

import (
	"os"
	"testing"
	"time"
)

func TestGCRemovesInactiveWorkflow(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	s, paths, err := New("review plan", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	s.Status = "completed"
	s.UpdatedAt = time.Now().UTC().Add(-72 * time.Hour).Format(time.RFC3339)
	if err := EnsureDirs(paths); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	if err := s.Save(paths.WorkflowFile); err != nil {
		t.Fatalf("Save: %v", err)
	}

	report, err := GC(1)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if len(report.RemovedWorkflows) != 1 {
		t.Fatalf("removed=%v", report.RemovedWorkflows)
	}
}
