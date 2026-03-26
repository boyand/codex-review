package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type GCReport struct {
	RemovedWorkflows []string
	RemovedLocks     []string
	SkippedActiveIDs []string
}

func GC(retainDays int) (GCReport, error) {
	var report GCReport
	entries, err := os.ReadDir(WorkflowsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return report, nil
		}
		return report, err
	}

	cutoff := time.Now().UTC().Add(-time.Duration(retainDays) * 24 * time.Hour)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		paths := PathsFor(entry.Name())
		s, err := Load(paths.WorkflowFile)
		if err != nil || s == nil {
			continue
		}
		if s.Status == "active" {
			report.SkippedActiveIDs = append(report.SkippedActiveIDs, s.ID)
			continue
		}
		updatedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(s.UpdatedAt))
		if err != nil {
			updatedAt = time.Time{}
		}
		if !updatedAt.IsZero() && updatedAt.After(cutoff) {
			continue
		}
		if err := os.RemoveAll(paths.Dir); err == nil {
			report.RemovedWorkflows = append(report.RemovedWorkflows, s.ID)
		}
	}

	lockEntries, err := filepath.Glob(filepath.Join(WorkflowsDir, "*", ".lock"))
	if err != nil {
		return report, err
	}
	for _, lockDir := range lockEntries {
		parent := filepath.Dir(lockDir)
		wfFile := filepath.Join(parent, "workflow.json")
		s, err := Load(wfFile)
		if err == nil && s != nil && s.Status == "active" {
			continue
		}
		if err := os.RemoveAll(lockDir); err == nil {
			report.RemovedLocks = append(report.RemovedLocks, lockDir)
		}
	}

	return report, nil
}
