// Package engine implements the stop-hook state machine.
package engine

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/boyand/codex-review-loop/internal/config"
	"github.com/boyand/codex-review-loop/internal/ledger"
	"github.com/boyand/codex-review-loop/internal/lock"
	"github.com/boyand/codex-review-loop/internal/phase"
	"github.com/boyand/codex-review-loop/internal/prompt"
	"github.com/boyand/codex-review-loop/internal/render"
	"github.com/boyand/codex-review-loop/internal/review"
	"github.com/boyand/codex-review-loop/internal/runner"
	"github.com/boyand/codex-review-loop/internal/state"
)

// blockExit is a panic type used to implement the Bash `block_with_message` + `exit 2` pattern.
// The panic is always caught by the defer/recover in RunStop.
type blockExit struct {
	message string
}

func blockWithMessage(msg string) {
	panic(blockExit{message: msg})
}

func blockWithStatus(w io.Writer, s *state.LoopState, l *ledger.Ledger, msg string) {
	render.Status(w, s, l)
	fmt.Fprintln(w)
	panic(blockExit{message: msg})
}

// RunStop executes the stop hook state machine. Returns (exitCode, error).
// Exit 0 = allow stop, Exit 2 = block with message.
func RunStop(cfg config.Config, stdout, stderr io.Writer) (exitCode int, err error) {
	activeLoop := false

	defer func() {
		if r := recover(); r != nil {
			if be, ok := r.(blockExit); ok {
				fmt.Fprintln(stdout, be.message)
				exitCode = 2
				err = nil
				return
			}
			// Unexpected panic
			if activeLoop {
				fmt.Fprintf(stdout, "codex-review-loop encountered an internal error. The loop state was not advanced.\n\nFix the issue and stop again, or run /cancel-codex-review to exit the loop.\n")
				exitCode = 2
			} else {
				fmt.Fprintf(stderr, "codex-review-loop: hook error: %v\n", r)
				exitCode = 0
			}
		}
	}()

	// Check if loop is active
	s, loadErr := state.Load(state.StateFile)
	if loadErr != nil {
		fmt.Fprintf(stderr, "codex-review-loop: %v\n", loadErr)
		return 0, nil
	}
	if s == nil || !s.Active() {
		return 0, nil
	}
	activeLoop = true

	// Acquire lock
	artifactsDir := state.ArtifactsDir
	lockDir := state.LockDir
	lk := lock.New(lockDir)
	if lockErr := lk.Acquire(); lockErr != nil {
		blockWithMessage(lockErr.Error())
	}
	defer lk.Release()

	// Validate state
	reviewID := s.ReviewID()
	if reviewID == "" {
		blockWithMessage("Invalid loop state: missing review_id.")
	}

	phaseIndex := s.CurrentPhaseIndex()
	substep := s.CurrentSubstep()
	currentRound := s.CurrentRound()
	maxRounds := s.MaxRounds()
	phaseName := s.PipelineName(phaseIndex)

	if phaseName == "" {
		blockWithMessage(fmt.Sprintf("Invalid loop state: no phase at index %d.", phaseIndex))
	}

	rawWorker := s.PipelineWorker(phaseIndex)
	rawReviewer := s.PipelineReviewer(phaseIndex)
	phaseWorker := phase.WorkerDefault(rawWorker)
	phaseReviewer := phase.ReviewerDefault(rawReviewer)

	if phaseWorker != "claude" && phaseWorker != "codex" {
		blockWithMessage(fmt.Sprintf("Unsupported worker '%s' for phase '%s'. Use 'claude' or 'codex'.", phaseWorker, phaseName))
	}
	if phaseReviewer != "claude" && phaseReviewer != "codex" {
		blockWithMessage(fmt.Sprintf("Unsupported reviewer '%s' for phase '%s'. Use 'claude' or 'codex'.", phaseReviewer, phaseName))
	}

	// Resolve artifact key
	existingArtifact := s.PipelineArtifact(phaseIndex)
	artifactKey := phase.ArtifactKey(phaseName, existingArtifact)
	if existingArtifact != artifactKey {
		s.SetPipelineField(phaseIndex, "artifact", artifactKey)
	}

	// Load ledger
	decisionsFile := state.DecisionsFile
	if err := ledger.EnsureFile(decisionsFile); err != nil {
		blockWithMessage(fmt.Sprintf("Cannot create decisions file: %v", err))
	}
	ldg, ldgErr := ledger.Load(decisionsFile)
	if ldgErr != nil {
		blockWithMessage(fmt.Sprintf("Cannot load decisions file: %v", ldgErr))
	}

	switch substep {
	case "working":
		handleWorking(cfg, stdout, stderr, s, ldg, phaseIndex, phaseName, phaseWorker, phaseReviewer,
			artifactKey, currentRound, maxRounds, artifactsDir, decisionsFile)

	case "review":
		handleReview(stdout, s, ldg, phaseName, phaseReviewer,
			artifactKey, currentRound, artifactsDir, decisionsFile)

	case "approval":
		return 0, nil

	default:
		blockWithMessage(fmt.Sprintf("Unknown loop substep '%s'.", substep))
	}

	// Should not reach here — substep handlers block
	return 0, nil
}

func handleWorking(cfg config.Config, stdout, stderr io.Writer,
	s *state.LoopState, ldg *ledger.Ledger,
	phaseIndex int, phaseName, phaseWorker, phaseReviewer, artifactKey string,
	currentRound, maxRounds int, artifactsDir, decisionsFile string) {

	if currentRound > maxRounds {
		fmt.Fprintf(stdout, "Maximum review rounds (%d) reached for phase '%s'. Allowing stop.\n", maxRounds, phaseName)
		return // will reach end of RunStop and return 0
	}

	phaseOutput := phase.OutputFile(artifactsDir, artifactKey)
	taskDesc := s.TaskDescription()
	compareToContent := loadCompareToContent(s, phaseIndex, artifactsDir)

	// Run codex worker if needed
	if phaseWorker == "codex" {
		if err := runner.RequireCodexCLI(); err != nil {
			blockWithMessage(err.Error())
		}
		workPromptField := s.PipelineWorkPrompt(phaseIndex)
		workPrompt := prompt.BuildWorkPrompt(phaseName, taskDesc, compareToContent, workPromptField)

		flags := runner.ParseFlags(cfg.CodexWorkerFlags)
		fmt.Fprintf(stderr, "codex-review-loop: Running Codex worker for phase '%s'...\n", phaseName)

		result, err := runner.RunCodexWorker(cfg, workPrompt, phaseOutput, flags, artifactsDir)
		if err != nil {
			msg := fmt.Sprintf("Codex worker failed for phase '%s'.", phaseName)
			if result != nil && result.LastErrorLine != "" {
				msg += fmt.Sprintf("\nLast codex error: %s", result.LastErrorLine)
			}
			if result != nil && result.LogPath != "" {
				msg += fmt.Sprintf("\nCodex log: %s", result.LogPath)
			}
			msg += "\nFix the issue and stop again, or run /cancel-codex-review."
			blockWithMessage(msg)
		}

		info, statErr := os.Stat(phaseOutput)
		if statErr != nil || info.Size() == 0 {
			blockWithMessage(fmt.Sprintf("Codex worker produced no phase artifact for '%s'.", phaseName))
		}
	}

	// Validate plan artifact
	if phaseName == "plan" {
		info, statErr := os.Stat(phaseOutput)
		if statErr != nil || info.Size() == 0 {
			blockWithMessage(fmt.Sprintf("Plan phase requires an artifact at '%s'. Write it before stopping.", phaseOutput))
		}
	}

	s.SetPipelineRounds(phaseIndex, currentRound)

	if phaseReviewer == "codex" {
		if err := runner.RequireCodexCLI(); err != nil {
			blockWithMessage(err.Error())
		}

		reviewFile := phase.ReviewFile(artifactsDir, artifactKey, currentRound)
		findingsFile := phase.FindingsFile(artifactsDir, artifactKey, currentRound)

		customPrompt := s.PipelineCustomPrompt(phaseIndex)
		phaseOutputContent := readFileOr(phaseOutput, "")

		reviewPrompt := prompt.BuildReviewPrompt(phaseName, taskDesc, phaseOutputContent, compareToContent, customPrompt)

		flags := runner.ParseFlags(cfg.CodexReviewFlags)
		fmt.Fprintf(stderr, "codex-review-loop: Running Codex review for phase '%s' (round %d)...\n", phaseName, currentRound)

		result, err := runner.RunCodexReview(cfg, reviewPrompt, prompt.StrictSuffix, reviewFile, flags, artifactsDir, review.IsValid)
		if err != nil {
			msg := fmt.Sprintf("Codex review output for phase '%s' was malformed or missing.", phaseName)
			if result != nil && result.LastErrorLine != "" {
				msg += fmt.Sprintf("\nLast codex error: %s", result.LastErrorLine)
			}
			if result != nil && result.LogPath != "" {
				msg += fmt.Sprintf("\nCodex log: %s", result.LogPath)
			}
			msg += "\nStop again to retry when Codex is healthy, or run /cancel-codex-review."
			blockWithMessage(msg)
		}

		reviewContent := readFileOr(reviewFile, "")
		ldg.SyncDecisionsForRound(phaseName, artifactKey, currentRound, phaseReviewer, reviewContent, "")
		if err := ldg.Save(); err != nil {
			blockWithMessage(fmt.Sprintf("Failed to save decisions ledger: %v", err))
		}

		s.SetCurrentSubstep("review")
		updateStateBody(s, ldg, artifactsDir)

		blockWithStatus(stdout, s, ldg, fmt.Sprintf(`Codex has reviewed phase **%s** (round %d).

Worker: %s
Reviewer: %s
Review file: `+"`%s`"+`
Decisions ledger: `+"`%s`"+`

**You must now:**
1. Read the review file completely.
2. Respond to every finding with a decision and justification.
3. Write responses to: `+"`%s`"+`
4. Stop when done.`, phaseName, currentRound, phaseWorker, phaseReviewer, reviewFile, decisionsFile, findingsFile))
	}

	// Claude reviewer path
	reviewFile := phase.ReviewFile(artifactsDir, artifactKey, currentRound)
	s.SetCurrentSubstep("review")
	updateStateBody(s, ldg, artifactsDir)

	blockWithStatus(stdout, s, ldg, fmt.Sprintf(`Phase **%s** is ready for Claude review (round %d).

Worker: %s
Reviewer: %s
Work artifact: `+"`%s`"+`
Review output target: `+"`%s`"+`

**You must now:**
1. Review the phase output and relevant code changes.
2. Write a structured review with severity tags [CRITICAL-N], [HIGH-N], [MEDIUM-N], [LOW-N].
3. Save it to: `+"`%s`"+`
4. Stop when done.`, phaseName, currentRound, phaseWorker, phaseReviewer, phaseOutput, reviewFile, reviewFile))
}

func handleReview(stdout io.Writer,
	s *state.LoopState, ldg *ledger.Ledger,
	phaseName, phaseReviewer, artifactKey string,
	currentRound int, artifactsDir, decisionsFile string) {

	if phaseReviewer == "codex" {
		reviewFile := phase.ReviewFile(artifactsDir, artifactKey, currentRound)
		findingsFile := phase.FindingsFile(artifactsDir, artifactKey, currentRound)

		// Check findings response exists
		info, statErr := os.Stat(findingsFile)
		if statErr != nil || info.Size() == 0 {
			blockWithStatus(stdout, s, ldg, fmt.Sprintf(`You have not yet written your findings response file.

Review file: `+"`%s`"+`
Findings file: `+"`%s`"+`

Write responses for every finding and stop again.`, reviewFile, findingsFile))
		}

		// Verify coverage
		reviewContent := readFileOr(reviewFile, "")
		findingsContent := readFileOr(findingsFile, "")

		if reviewContent != "" {
			fc := review.VerifyFindingsCoverage(reviewContent, findingsContent)
			if fc != nil {
				var msg strings.Builder
				msg.WriteString("Your findings response file does not fully address the review findings.\n")
				msg.WriteString(fmt.Sprintf("\nReview file: `%s`\n", reviewFile))
				msg.WriteString(fmt.Sprintf("Findings file: `%s`\n", findingsFile))
				if len(fc.MissingIDs) > 0 {
					msg.WriteString("\nMissing finding IDs:\n")
					for _, id := range fc.MissingIDs {
						msg.WriteString(fmt.Sprintf("- %s\n", id))
					}
				}
				if len(fc.MissingDecisions) > 0 {
					msg.WriteString("\nFindings without a clear decision (ACCEPT/REJECT/DEFER/FIX/WONTFIX):\n")
					for _, id := range fc.MissingDecisions {
						msg.WriteString(fmt.Sprintf("- %s\n", id))
					}
				}
				msg.WriteString("\nUpdate the findings file so every finding ID is addressed with a decision, then stop again.")
				blockWithMessage(msg.String())
			}
		}

		ldg.SyncDecisionsForRound(phaseName, artifactKey, currentRound, phaseReviewer, reviewContent, findingsContent)
		if err := ldg.Save(); err != nil {
			blockWithMessage(fmt.Sprintf("Failed to save decisions ledger: %v", err))
		}

		s.SetCurrentSubstep("approval")
		updateStateBody(s, ldg, artifactsDir)

		var approvalBuf strings.Builder
		render.ApprovalSummary(&approvalBuf, phaseName, currentRound, ldg)

		blockWithStatus(stdout, s, ldg, fmt.Sprintf(`You have addressed Codex findings for phase **%s** (round %d).

Decisions ledger updated: `+"`%s`"+`

**Next step:** present the approval summary below to the user.

%s`, phaseName, currentRound, decisionsFile, approvalBuf.String()))
	}

	// Claude reviewer path
	reviewFile := phase.ReviewFile(artifactsDir, artifactKey, currentRound)
	info, statErr := os.Stat(reviewFile)
	if statErr != nil || info.Size() == 0 {
		blockWithStatus(stdout, s, ldg, fmt.Sprintf(`Claude review output is missing for phase **%s**.

Write the review to: `+"`%s`"+`
Then stop again.`, phaseName, reviewFile))
	}

	reviewContent := readFileOr(reviewFile, "")
	ldg.SyncDecisionsForRound(phaseName, artifactKey, currentRound, phaseReviewer, reviewContent, "")
	if err := ldg.Save(); err != nil {
		blockWithMessage(fmt.Sprintf("Failed to save decisions ledger: %v", err))
	}

	s.SetCurrentSubstep("approval")
	updateStateBody(s, ldg, artifactsDir)

	var approvalBuf strings.Builder
	render.ApprovalSummary(&approvalBuf, phaseName, currentRound, ldg)

	blockWithStatus(stdout, s, ldg, fmt.Sprintf(`Claude review is complete for phase **%s** (round %d).

Decisions ledger updated: `+"`%s`"+`

**Next step:** present the approval summary below to the user.

%s`, phaseName, currentRound, decisionsFile, approvalBuf.String()))
}

// RunStatus renders the status box and returns exit code 0.
func RunStatus(stdout io.Writer) (int, error) {
	s, err := state.Load(state.StateFile)
	if err != nil {
		return 0, err
	}
	if s == nil || !s.Active() {
		fmt.Fprintln(stdout, "No active codex review loop.")
		return 0, nil
	}

	ldg, _ := ledger.Load(state.DecisionsFile)
	if ldg == nil {
		ldg = ledger.NewEmpty()
	}

	render.Status(stdout, s, ldg)
	return 0, nil
}

// RunCompletion renders the completion summary.
func RunCompletion(stdout io.Writer) (int, error) {
	s, err := state.Load(state.StateFile)
	if err != nil {
		return 0, err
	}
	if s == nil {
		fmt.Fprintln(stdout, "No state file found. Cannot render completion summary.")
		return 0, nil
	}

	ldg, _ := ledger.Load(state.DecisionsFile)
	if ldg == nil {
		ldg = ledger.NewEmpty()
	}

	render.Completion(stdout, s, ldg)
	return 0, nil
}

// --- Helpers ---

func updateStateBody(s *state.LoopState, ldg *ledger.Ledger, artifactsDir string) {
	var buf strings.Builder
	render.StateBody(&buf, s, ldg, artifactsDir)
	if err := s.SaveFrontmatterWithBody(buf.String()); err != nil {
		blockWithMessage(fmt.Sprintf("Failed to write loop state: %v", err))
	}
}

func readFileOr(path, fallback string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	return string(data)
}

func loadCompareToContent(s *state.LoopState, phaseIndex int, artifactsDir string) string {
	compareTo := s.PipelineCompareTo(phaseIndex)
	if compareTo == "" {
		return ""
	}

	// Build pipeline name→artifact map
	names := make(map[string]string)
	for i := 0; i < s.PipelineCount(); i++ {
		n := s.PipelineName(i)
		a := s.PipelineArtifact(i)
		if a == "" {
			a = phase.ArtifactKey(n, "")
		}
		names[n] = a
	}

	refFile := runner.ResolveReferenceFile(artifactsDir, compareTo, names)
	if refFile == "" {
		return ""
	}
	return readFileOr(refFile, "")
}
