// Package render produces status boxes, approval summaries, completion
// summaries, and state body content for the loop.
package render

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/boyand/codex-review-loop/internal/ledger"
	"github.com/boyand/codex-review-loop/internal/phase"
	"github.com/boyand/codex-review-loop/internal/state"
	"github.com/boyand/codex-review-loop/internal/textutil"
)

const boxWidth = 52

func padLine(w io.Writer, content string) {
	maxW := boxWidth - 4 // "│ " + content + " │"
	content = textutil.Truncate(content, maxW)
	pad := maxW - len(content)
	fmt.Fprintf(w, "│ %s%*s │\n", content, pad, "")
}

// Status renders the Unicode status box.
func Status(w io.Writer, s *state.LoopState, l *ledger.Ledger) {
	taskDesc := textutil.Truncate(s.TaskDescription(), 42)
	reviewID := s.ReviewID()
	curIdx := s.CurrentPhaseIndex()
	substep := s.CurrentSubstep()
	round := s.CurrentRound()
	count := s.PipelineCount()
	phaseName := s.PipelineName(curIdx)
	phaseNum := curIdx + 1

	// Build timeline
	var timeline, labels strings.Builder
	for i := 0; i < count; i++ {
		name := s.PipelineName(i)
		status := s.PipelineStatus(i)

		var marker, label string
		if status == "completed" {
			marker = "●"
			label = "DONE"
		} else if i == curIdx {
			marker = "◐"
			label = strings.ToUpper(substep)
		} else {
			marker = "○"
			label = "pending"
		}

		if i > 0 {
			timeline.WriteString(" ── ")
			labels.WriteString("    ")
		}
		timeline.WriteString(marker + " " + name)

		nameWidth := len(name) + 2
		labels.WriteString(fmt.Sprintf("%-*s", nameWidth, label))
	}

	// Phase-cumulative counts
	c := l.CountByPhase(phaseName)

	dashes := strings.Repeat("─", boxWidth-22)
	borderBot := "╰" + strings.Repeat("─", boxWidth-2) + "╯"

	fmt.Fprintf(w, "╭─ Codex Review Loop %s╮\n", dashes)
	padLine(w, "Task: "+taskDesc)
	padLine(w, "ID:   "+reviewID)
	padLine(w, "")
	padLine(w, " "+timeline.String())
	padLine(w, " "+labels.String())
	padLine(w, "")
	padLine(w, fmt.Sprintf("Phase %d/%d: %s (round %d, %s)", phaseNum, count, phaseName, round, substep))

	total := c.Critical + c.High + c.Medium + c.Low
	if total == 0 {
		padLine(w, "Findings: none yet")
	} else {
		padLine(w, fmt.Sprintf("Findings: %d critical, %d high, %d medium, %d low", c.Critical, c.High, c.Medium, c.Low))
	}
	padLine(w, fmt.Sprintf("Decisions: %d fix, %d reject, %d open", c.Fix, c.Reject, c.Open))
	fmt.Fprintln(w, borderBot)
}

// ApprovalSummary renders the approval gate with severity table, decisions, and choices.
func ApprovalSummary(w io.Writer, phaseName string, round int, l *ledger.Ledger) {
	rc := l.CountByPhaseRound(phaseName, round)
	rows := l.RowsForPhaseRound(phaseName, round)

	fmt.Fprintf(w, "## Approval Gate — Phase: %s (Round %d)\n\n", phaseName, round)
	fmt.Fprint(w, "### Findings Summary\n\n")
	fmt.Fprintln(w, "| Severity | Count | Fixed | Rejected | Open |")
	fmt.Fprintln(w, "|----------|-------|-------|----------|------|")
	fmt.Fprintf(w, "| CRITICAL | %d | %d | %d | %d |\n", rc.CritTotal, rc.CritFix, rc.CritReject, rc.CritOpen)
	fmt.Fprintf(w, "| HIGH     | %d | %d | %d | %d |\n", rc.HighTotal, rc.HighFix, rc.HighReject, rc.HighOpen)
	fmt.Fprintf(w, "| MEDIUM   | %d | %d | %d | %d |\n", rc.MedTotal, rc.MedFix, rc.MedReject, rc.MedOpen)
	fmt.Fprintf(w, "| LOW      | %d | %d | %d | %d |\n", rc.LowTotal, rc.LowFix, rc.LowReject, rc.LowOpen)

	if len(rows) > 0 {
		fmt.Fprint(w, "\n### Decisions\n\n")
		fmt.Fprintln(w, "| # | Severity | Finding | Decision | Selected |")
		fmt.Fprintln(w, "|---|----------|---------|----------|----------|")
		for _, r := range rows {
			finding := sanitizeFinding(r.Finding)
			fmt.Fprintf(w, "| %s | %s | %s | %s | %s |\n",
				r.FindingID, r.Severity, finding, r.Decision, r.Selected)
		}
	}

	fmt.Fprintln(w, "\n### What would you like to do?")
	fmt.Fprintln(w, "1. **approve** — Accept and move to next phase")
	fmt.Fprintf(w, "2. **repeat** — Re-run review (round %d)\n", round+1)
	fmt.Fprintln(w, "3. **add-phase** — Insert a new phase after this one")
	fmt.Fprintln(w, "4. **done** — Finish the loop early")
}

var mdStripper = strings.NewReplacer("`", "", "*", "", "_", "", "~", "", "[", "", "]", "", "#", "", ">", "")

func sanitizeFinding(s string) string {
	return textutil.Truncate(mdStripper.Replace(s), 60)
}

// Completion renders the completion summary box.
func Completion(w io.Writer, s *state.LoopState, l *ledger.Ledger) {
	taskDesc := textutil.Truncate(s.TaskDescription(), 42)
	count := s.PipelineCount()

	// Build timeline and rounds
	var timeline, roundsLine strings.Builder
	totalRounds := 0
	for i := 0; i < count; i++ {
		name := s.PipelineName(i)
		status := s.PipelineStatus(i)
		rounds := s.PipelineRounds(i)
		totalRounds += rounds

		var marker string
		if status == "completed" {
			marker = "●"
		} else {
			marker = "○"
		}

		if i > 0 {
			timeline.WriteString(" ── ")
			roundsLine.WriteString("    ")
		}
		timeline.WriteString(marker + " " + name)

		rLabel := fmt.Sprintf("%d rounds", rounds)
		if rounds == 1 {
			rLabel = "1 round"
		}
		nameWidth := len(name) + 2
		roundsLine.WriteString(fmt.Sprintf("%-*s", nameWidth, rLabel))
	}

	// Loop-wide counts
	c := l.CountAll()
	totalFindings := c.Critical + c.High + c.Medium + c.Low

	artifactsDir := state.ArtifactsDir

	dashes := strings.Repeat("─", boxWidth-33)
	borderBot := "╰" + strings.Repeat("─", boxWidth-2) + "╯"

	fmt.Fprintf(w, "╭─ Codex Review Loop — Complete %s╮\n", dashes)
	padLine(w, "Task: "+taskDesc)
	padLine(w, "")
	padLine(w, " "+timeline.String())
	padLine(w, " "+roundsLine.String())
	padLine(w, "")
	padLine(w, fmt.Sprintf("Total findings: %d", totalFindings))
	padLine(w, fmt.Sprintf("  %d fix, %d reject, %d open", c.Fix, c.Reject, c.Open))
	padLine(w, "")
	padLine(w, fmt.Sprintf("Artifacts in %s/", artifactsDir))
	fmt.Fprintln(w, borderBot)
}

// StateBody renders the auto-generated body content for the state file.
func StateBody(w io.Writer, s *state.LoopState, l *ledger.Ledger, artifactsDir string) {
	taskDesc := s.TaskDescription()
	count := s.PipelineCount()
	curIdx := s.CurrentPhaseIndex()
	substep := s.CurrentSubstep()
	round := s.CurrentRound()
	phaseName := s.PipelineName(curIdx)
	phaseNum := curIdx + 1

	// Build pipeline string and roles
	var pipelineStr strings.Builder
	var rolesStr strings.Builder
	for i := 0; i < count; i++ {
		name := s.PipelineName(i)
		worker := phase.WorkerDefault(s.PipelineWorker(i))
		reviewer := phase.ReviewerDefault(s.PipelineReviewer(i))
		if i > 0 {
			pipelineStr.WriteString(" → ")
		}
		pipelineStr.WriteString(name)
		fmt.Fprintf(&rolesStr, "- %s: %s/%s\n", name, worker, reviewer)
	}

	// Completed phases table
	var completedTable strings.Builder
	hasCompleted := false
	for i := 0; i < count; i++ {
		name := s.PipelineName(i)
		status := s.PipelineStatus(i)
		if status != "completed" {
			continue
		}
		hasCompleted = true
		rounds := s.PipelineRounds(i)
		c := l.CountByPhase(name)
		totalFindings := c.Critical + c.High + c.Medium + c.Low
		fmt.Fprintf(&completedTable, "| %s | %d | %d | %d | %d |\n",
			name, rounds, totalFindings, c.Fix, c.Reject)
	}

	// List artifacts
	artifactsList := listArtifacts(artifactsDir)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "<!-- This section is auto-generated by the stop hook. Do not edit. -->")
	fmt.Fprintln(w, "# Codex Review Loop")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Task: %s\n", taskDesc)
	fmt.Fprintf(w, "Pipeline: %s\n", pipelineStr.String())
	fmt.Fprintln(w, "Roles:")
	fmt.Fprint(w, rolesStr.String())
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Current Status")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Phase %d/%d: %s (round %d, %s)\n", phaseNum, count, phaseName, round, substep)

	if hasCompleted {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "## Completed Phases")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| Phase | Rounds | Findings | Fixed | Rejected |")
		fmt.Fprintln(w, "|-------|--------|----------|-------|----------|")
		fmt.Fprint(w, completedTable.String())
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Artifacts")
	fmt.Fprintln(w)
	fmt.Fprint(w, artifactsList)
}

func listArtifacts(dir string) string {
	var b strings.Builder
	patterns := []string{"*.md", "*.tsv"}
	found := false
	for _, pat := range patterns {
		matches, _ := filepath.Glob(filepath.Join(dir, pat))
		for _, m := range matches {
			fmt.Fprintf(&b, "- `%s`\n", m)
			found = true
		}
	}
	if !found {
		b.WriteString("(none yet)\n")
	}
	return b.String()
}
