package engine

import (
	"fmt"
	"io"

	"github.com/boyand/codex-review/internal/workflow"
)

// RunGC performs cleanup of stale/inactive workflows.
func RunGC(stdout io.Writer, retainDays int) (int, error) {
	workflowReport, err := workflow.GC(retainDays)
	if err != nil {
		return 1, err
	}

	fmt.Fprintln(stdout, "Codex Review GC")
	fmt.Fprintf(stdout, "- Removed workflows: %d\n", len(workflowReport.RemovedWorkflows))
	fmt.Fprintf(stdout, "- Removed stale locks: %d\n", len(workflowReport.RemovedLocks))
	fmt.Fprintf(stdout, "- Skipped active workflows: %d\n", len(workflowReport.SkippedActiveIDs))
	return 0, nil
}
