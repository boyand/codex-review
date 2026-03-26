package engine

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/boyand/codex-review/internal/config"
	"github.com/boyand/codex-review/internal/ledger"
	"github.com/boyand/codex-review/internal/lock"
	"github.com/boyand/codex-review/internal/prompt"
	"github.com/boyand/codex-review/internal/review"
	"github.com/boyand/codex-review/internal/runner"
	"github.com/boyand/codex-review/internal/workflow"
)

type PlanReviewOptions struct {
	WorkflowID string
	PlanPath   string
	Prompt     string
}

type ImplementReviewOptions struct {
	WorkflowID string
	Prompt     string
}

func RunPlanReview(cfg config.Config, stdout io.Writer, opts PlanReviewOptions) (int, error) {
	paths, wf, created, err := loadOrCreatePlanWorkflow(opts)
	if err != nil {
		return 1, err
	}
	return runWorkflowReview(cfg, stdout, paths, wf, created, opts.Prompt)
}

func RunImplementReview(cfg config.Config, stdout io.Writer, opts ImplementReviewOptions) (int, error) {
	paths, wf, err := resolveWorkflowStrict(opts.WorkflowID)
	if err != nil {
		return 1, err
	}
	if wf == nil {
		return 1, errors.New("no active workflow found")
	}
	if wf.Status != "active" {
		return 1, fmt.Errorf("workflow '%s' is not active", wf.ID)
	}
	if strings.TrimSpace(wf.Phase) == "plan" {
		step, err := workflow.DeriveStep(paths, wf)
		if err != nil {
			return 1, err
		}
		if step != "approval" {
			return 1, fmt.Errorf("workflow '%s' is in phase '%s' step '%s'; finish the current plan review first", wf.ID, wf.Phase, step)
		}
		wf.Phase = "implement"
		wf.Round = 1
		workflow.Touch(wf)
		if err := wf.Save(paths.WorkflowFile); err != nil {
			return 1, err
		}
		fmt.Fprintf(stdout, "Approved workflow %s plan round. Phase advanced to implement (round 1).\n", wf.ID)
	}
	if strings.TrimSpace(wf.Phase) != "implement" {
		return 1, fmt.Errorf("workflow '%s' is in phase '%s'; approve the plan first", wf.ID, wf.Phase)
	}
	return runWorkflowReview(cfg, stdout, paths, wf, false, opts.Prompt)
}

func RunWorkflowStatus(stdout io.Writer, explicitID string) (bool, int, error) {
	paths, wf, err := workflow.Resolve(explicitID)
	if err != nil {
		if isWorkflowConflict(err) {
			return true, 0, err
		}
		return false, 0, err
	}
	if wf == nil {
		return false, 0, nil
	}
	ldg, err := syncWorkflowDecisions(paths, wf)
	if err != nil {
		return true, 1, err
	}
	step, err := workflow.DeriveStep(paths, wf)
	if err != nil {
		return true, 1, err
	}
	counts := ldg.CountByPhaseRound(wf.Phase, wf.Round)
	fmt.Fprintf(stdout, "Workflow: %s\n", wf.ID)
	fmt.Fprintf(stdout, "Task: %s\n", displayOr(wf.Task, "(unspecified task)"))
	fmt.Fprintf(stdout, "Phase: %s\n", wf.Phase)
	fmt.Fprintf(stdout, "Round: %d\n", wf.Round)
	fmt.Fprintf(stdout, "Step: %s\n", step)
	fmt.Fprintf(stdout, "Status: %s\n", wf.Status)
	fmt.Fprintf(stdout, "Owner session: %s\n", displayOr(wf.OwnerSessionID, "(unowned)"))
	fmt.Fprintf(stdout, "Owner cwd: %s\n", displayOr(wf.OwnerCWD, "(unowned)"))
	fmt.Fprintf(stdout, "Plan source: %s\n", displayOr(wf.PlanSourcePath, "(none recorded)"))
	fmt.Fprintf(stdout, "Review file: %s\n", workflow.ReviewFile(paths, wf))
	fmt.Fprintf(stdout, "Findings file: %s\n", workflow.FindingsFile(paths, wf))
	fmt.Fprintf(stdout, "Decisions ledger: %s\n", paths.DecisionsFile)
	fmt.Fprintf(stdout, "Counts: critical=%d high=%d medium=%d low=%d\n", counts.CritTotal, counts.HighTotal, counts.MedTotal, counts.LowTotal)
	return true, 0, nil
}

func RunWorkflowSummary(stdout io.Writer, explicitID string) (bool, int, error) {
	paths, wf, err := workflow.Resolve(explicitID)
	if err != nil {
		if isWorkflowConflict(err) {
			return true, 0, err
		}
		return false, 0, err
	}
	if wf == nil {
		return false, 0, nil
	}
	ldg, err := syncWorkflowDecisions(paths, wf)
	if err != nil {
		return true, 1, err
	}
	rows := ldg.RowsForPhaseRound(wf.Phase, wf.Round)
	counts := ldg.CountByPhaseRound(wf.Phase, wf.Round)
	fmt.Fprintf(stdout, "Workflow: %s\n", wf.ID)
	fmt.Fprintf(stdout, "Phase: %s\n", wf.Phase)
	fmt.Fprintf(stdout, "Round: %d\n", wf.Round)
	fmt.Fprintf(stdout, "Owner session: %s\n", displayOr(wf.OwnerSessionID, "(unowned)"))
	fmt.Fprintf(stdout, "Owner cwd: %s\n", displayOr(wf.OwnerCWD, "(unowned)"))
	fmt.Fprintf(stdout, "Plan source: %s\n", displayOr(wf.PlanSourcePath, "(none recorded)"))
	fmt.Fprintf(stdout, "Critical: %d total, %d fix, %d reject, %d open\n", counts.CritTotal, counts.CritFix, counts.CritReject, counts.CritOpen)
	fmt.Fprintf(stdout, "High: %d total, %d fix, %d reject, %d open\n", counts.HighTotal, counts.HighFix, counts.HighReject, counts.HighOpen)
	fmt.Fprintf(stdout, "Medium: %d total, %d fix, %d reject, %d open\n", counts.MedTotal, counts.MedFix, counts.MedReject, counts.MedOpen)
	fmt.Fprintf(stdout, "Low: %d total, %d fix, %d reject, %d open\n", counts.LowTotal, counts.LowFix, counts.LowReject, counts.LowOpen)
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "No findings recorded for this round.")
		return true, 0, nil
	}
	fmt.Fprintln(stdout)
	for _, row := range rows {
		fmt.Fprintf(stdout, "%s %s decision=%s outcome=%s selected=%s\n", row.FindingID, row.Severity, row.Decision, row.Outcome, row.Selected)
		fmt.Fprintf(stdout, "  %s\n", row.Finding)
	}
	return true, 0, nil
}

func RunWorkflowApprove(stdout io.Writer, explicitID string) (bool, int, error) {
	paths, wf, err := workflow.Resolve(explicitID)
	if err != nil {
		if isWorkflowConflict(err) {
			return true, 0, err
		}
		return false, 0, err
	}
	if wf == nil {
		return false, 0, nil
	}
	if wf.Status != "active" {
		return true, 1, fmt.Errorf("workflow '%s' is not active", wf.ID)
	}
	step, err := workflow.DeriveStep(paths, wf)
	if err != nil {
		return true, 1, err
	}
	if step != "approval" {
		return true, 1, fmt.Errorf("workflow '%s' is in step '%s'; approve only works from approval", wf.ID, step)
	}

	if wf.Phase == "plan" {
		wf.Phase = "implement"
		wf.Round = 1
		workflow.Touch(wf)
		if err := wf.Save(paths.WorkflowFile); err != nil {
			return true, 1, err
		}
		fmt.Fprintf(stdout, "Approved workflow %s plan round. Phase advanced to implement (round 1).\n", wf.ID)
		return true, 0, nil
	}

	wf.Status = "completed"
	workflow.Touch(wf)
	if err := wf.Save(paths.WorkflowFile); err != nil {
		return true, 1, err
	}
	fmt.Fprintf(stdout, "Approved workflow %s implementation round. Workflow completed.\n", wf.ID)
	return true, 0, nil
}

func RunWorkflowRepeat(cfg config.Config, stdout io.Writer, explicitID, focus string) (bool, int, error) {
	paths, wf, err := workflow.Resolve(explicitID)
	if err != nil {
		if isWorkflowConflict(err) {
			return true, 0, err
		}
		return false, 0, err
	}
	if wf == nil {
		return false, 0, nil
	}
	if wf.Status != "active" {
		return true, 1, fmt.Errorf("workflow '%s' is not active", wf.ID)
	}
	step, err := workflow.DeriveStep(paths, wf)
	if err != nil {
		return true, 1, err
	}
	if step != "approval" {
		return true, 1, fmt.Errorf("workflow '%s' is in step '%s'; repeat only works from approval", wf.ID, step)
	}
	wf.Round++
	workflow.Touch(wf)
	if err := wf.Save(paths.WorkflowFile); err != nil {
		return true, 1, err
	}
	code, err := runWorkflowReview(cfg, stdout, paths, wf, false, focus)
	return true, code, err
}

func RunWorkflowDone(stdout io.Writer, explicitID string) (bool, int, error) {
	paths, wf, err := workflow.Resolve(explicitID)
	if err != nil {
		if isWorkflowConflict(err) {
			return true, 0, err
		}
		return false, 0, err
	}
	if wf == nil {
		return false, 0, nil
	}
	wf.Status = "completed"
	workflow.Touch(wf)
	if err := wf.Save(paths.WorkflowFile); err != nil {
		return true, 1, err
	}
	fmt.Fprintf(stdout, "Workflow %s marked completed.\n", wf.ID)
	return true, 0, nil
}

func RunWorkflowCancel(stdout io.Writer, explicitID string) (bool, int, error) {
	paths, wf, err := workflow.Resolve(explicitID)
	if err != nil {
		if isWorkflowConflict(err) {
			return true, 0, err
		}
		return false, 0, err
	}
	if wf == nil {
		return false, 0, nil
	}
	wf.Status = "cancelled"
	workflow.Touch(wf)
	if err := wf.Save(paths.WorkflowFile); err != nil {
		return true, 1, err
	}
	fmt.Fprintf(stdout, "Workflow %s cancelled. Artifacts preserved under %s.\n", wf.ID, paths.Dir)
	return true, 0, nil
}

func loadOrCreatePlanWorkflow(opts PlanReviewOptions) (workflow.Paths, *workflow.State, bool, error) {
	paths, wf, err := workflow.Resolve(opts.WorkflowID)
	if err != nil {
		return workflow.Paths{}, nil, false, err
	}
	if wf != nil {
		if wf.Status != "active" {
			return workflow.Paths{}, nil, false, fmt.Errorf("workflow '%s' is not active", wf.ID)
		}
		if wf.Phase != "plan" {
			return workflow.Paths{}, nil, false, fmt.Errorf("workflow '%s' is in phase '%s'; use impl or approve the current phase", wf.ID, wf.Phase)
		}
		if strings.TrimSpace(opts.PlanPath) != "" {
			wf.PlanSourcePath = strings.TrimSpace(opts.PlanPath)
			if err := syncPlanArtifactFromSource(paths, wf.PlanSourcePath); err != nil {
				return workflow.Paths{}, nil, false, err
			}
		} else if strings.TrimSpace(wf.PlanSourcePath) == "" && !planArtifactExists(paths) {
			wf.PlanSourcePath, err = workflow.ResolveCurrentClaudePlan()
			if err != nil {
				return workflow.Paths{}, nil, false, planResolutionError(err)
			}
			if strings.TrimSpace(wf.PlanSourcePath) == "" {
				return workflow.Paths{}, nil, false, planResolutionError(nil)
			}
		}
		workflow.Touch(wf)
		if err := wf.Save(paths.WorkflowFile); err != nil {
			return workflow.Paths{}, nil, false, err
		}
		return paths, wf, false, nil
	}

	planPath := strings.TrimSpace(opts.PlanPath)
	if planPath == "" {
		planPath, err = workflow.ResolveCurrentClaudePlan()
		if err != nil {
			return workflow.Paths{}, nil, false, planResolutionError(err)
		}
		if strings.TrimSpace(planPath) == "" {
			return workflow.Paths{}, nil, false, planResolutionError(nil)
		}
	}
	wf, paths, err = workflow.New(displayOr(opts.Prompt, "plan review"), planPath)
	if err != nil {
		return workflow.Paths{}, nil, false, err
	}
	if session, err := workflow.ResolveCurrentClaudeSession(); err != nil {
		return workflow.Paths{}, nil, false, err
	} else {
		workflow.BindOwner(wf, session)
	}
	if err := workflow.EnsureDirs(paths); err != nil {
		return workflow.Paths{}, nil, false, err
	}
	if err := ledger.EnsureFile(paths.DecisionsFile); err != nil {
		return workflow.Paths{}, nil, false, err
	}
	if err := syncPlanArtifactFromSource(paths, wf.PlanSourcePath); err != nil {
		return workflow.Paths{}, nil, false, err
	}
	if err := wf.Save(paths.WorkflowFile); err != nil {
		return workflow.Paths{}, nil, false, err
	}
	return paths, wf, true, nil
}

func planResolutionError(err error) error {
	if errors.Is(err, workflow.ErrNoCurrentClaudePlan) || err == nil {
		return errors.New("no plan file was resolved from the current Claude session; if the plan is inline or from another thread, save it to a file and rerun with --plan <path>")
	}
	return err
}

func resolveWorkflowStrict(explicitID string) (workflow.Paths, *workflow.State, error) {
	paths, wf, err := workflow.Resolve(explicitID)
	if err != nil {
		return workflow.Paths{}, nil, err
	}
	if wf == nil {
		return workflow.Paths{}, nil, nil
	}
	return paths, wf, nil
}

func runWorkflowReview(cfg config.Config, stdout io.Writer, paths workflow.Paths, wf *workflow.State, created bool, focus string) (int, error) {
	if wf == nil {
		return 1, errors.New("workflow not found")
	}
	if err := workflow.EnsureDirs(paths); err != nil {
		return 1, err
	}
	if err := ledger.EnsureFile(paths.DecisionsFile); err != nil {
		return 1, err
	}

	lk := lock.New(paths.LockDir)
	if err := lk.Acquire(); err != nil {
		return 1, describeLockConflict(err, paths, wf)
	}
	defer lk.Release()

	step, err := workflow.DeriveStep(paths, wf)
	if err != nil {
		return 1, err
	}
	if step != "working" {
		printWorkflowContext(stdout, paths, wf)
		fmt.Fprintf(stdout, "Workflow: %s\n", wf.ID)
		fmt.Fprintf(stdout, "Phase: %s\n", wf.Phase)
		fmt.Fprintf(stdout, "Round: %d\n", wf.Round)
		fmt.Fprintf(stdout, "Step: %s\n", step)
		fmt.Fprintf(stdout, "Owner session: %s\n", displayOr(wf.OwnerSessionID, "(unowned)"))
		fmt.Fprintf(stdout, "Owner cwd: %s\n", displayOr(wf.OwnerCWD, "(unowned)"))
		fmt.Fprintf(stdout, "Plan source: %s\n", displayOr(wf.PlanSourcePath, "(none recorded)"))
		fmt.Fprintf(stdout, "Review file: %s\n", workflow.ReviewFile(paths, wf))
		fmt.Fprintf(stdout, "Findings file: %s\n", workflow.FindingsFile(paths, wf))
		fmt.Fprintf(stdout, "Decisions ledger: %s\n", paths.DecisionsFile)
		return 0, nil
	}

	if err := hydratePlanArtifact(paths, wf); err != nil {
		return 1, err
	}
	if wf.Phase == "implement" {
		if _, err := os.Stat(workflow.PathsFor(wf.ID).ArtifactsDir); err != nil {
			return 1, err
		}
	}

	phaseOutput := workflow.OutputFile(paths, wf)
	phaseOutputContent := readFileOr(phaseOutput, "")
	planContent := readFileOr(workflow.PathsFor(wf.ID).ArtifactsDir+"/plan.md", "")

	var reviewPrompt string
	switch wf.Phase {
	case "plan":
		reviewPrompt = prompt.BuildReviewPrompt("plan", wf.Task, phaseOutputContent, "", strings.TrimSpace(focus))
	case "implement":
		if strings.TrimSpace(planContent) == "" {
			return 1, errors.New("approved plan artifact is missing; cannot run implementation review")
		}
		reviewPrompt = prompt.BuildReviewPrompt("implement", wf.Task, phaseOutputContent, planContent, strings.TrimSpace(focus))
	default:
		return 1, fmt.Errorf("unsupported workflow phase '%s'", wf.Phase)
	}

	if err := runner.RequireCodexCLI(); err != nil {
		return 1, err
	}
	reviewFile := workflow.ReviewFile(paths, wf)
	if created {
		fmt.Fprintf(stdout, "Created workflow %s.\n", wf.ID)
	}
	printWorkflowContext(stdout, paths, wf)
	fmt.Fprintf(stdout, "Review file: %s\n", absPathOr(reviewFile))
	fmt.Fprintf(stdout, "Findings file: %s\n", absPathOr(workflow.FindingsFile(paths, wf)))
	fmt.Fprintln(stdout, "Running Codex review...")
	flags := runner.ParseFlags(cfg.CodexReviewFlags)
	if _, err := runner.RunCodexReview(cfg, reviewPrompt, prompt.StrictSuffix, reviewFile, flags, paths.ArtifactsDir, review.IsValid); err != nil {
		return 1, err
	}

	if _, err := syncWorkflowDecisions(paths, wf); err != nil {
		return 1, err
	}
	workflow.Touch(wf)
	if err := wf.Save(paths.WorkflowFile); err != nil {
		return 1, err
	}

	fmt.Fprintf(stdout, "Workflow: %s\n", wf.ID)
	fmt.Fprintf(stdout, "Task: %s\n", displayOr(wf.Task, "(unspecified task)"))
	fmt.Fprintf(stdout, "Phase: %s\n", wf.Phase)
	fmt.Fprintf(stdout, "Round: %d\n", wf.Round)
	fmt.Fprintf(stdout, "Owner session: %s\n", displayOr(wf.OwnerSessionID, "(unowned)"))
	fmt.Fprintf(stdout, "Owner cwd: %s\n", displayOr(wf.OwnerCWD, "(unowned)"))
	fmt.Fprintf(stdout, "Plan source: %s\n", displayOr(wf.PlanSourcePath, "(none recorded)"))
	fmt.Fprintf(stdout, "Review file: %s\n", absPathOr(reviewFile))
	fmt.Fprintf(stdout, "Findings file: %s\n", absPathOr(workflow.FindingsFile(paths, wf)))
	fmt.Fprintf(stdout, "Decisions ledger: %s\n", absPathOr(paths.DecisionsFile))
	fmt.Fprintln(stdout, "Codex review completed for this round.")
	return 0, nil
}

func printWorkflowContext(stdout io.Writer, paths workflow.Paths, wf *workflow.State) {
	fmt.Fprintf(stdout, "Project root: %s\n", absPathOr("."))
	fmt.Fprintf(stdout, "Workflow dir: %s\n", absPathOr(paths.Dir))
	fmt.Fprintf(stdout, "Lock dir: %s\n", absPathOr(paths.LockDir))
}

func describeLockConflict(err error, paths workflow.Paths, wf *workflow.State) error {
	var held *lock.HeldError
	if !errors.As(err, &held) {
		return err
	}

	return fmt.Errorf(
		"workflow '%s' is already being reviewed by another codex-review process (pid %d).\nProject root: %s\nOwner cwd: %s\nWorkflow dir: %s\nLock dir: %s",
		displayOr(wf.ID, "(unknown)"),
		held.OwnerPID,
		absPathOr("."),
		displayOr(wf.OwnerCWD, "(unowned)"),
		absPathOr(paths.Dir),
		absPathOr(paths.LockDir),
	)
}

func absPathOr(path string) string {
	if strings.TrimSpace(path) == "" {
		return "(unknown)"
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func hydratePlanArtifact(paths workflow.Paths, wf *workflow.State) error {
	planArtifact := paths.ArtifactsDir + "/plan.md"
	if strings.TrimSpace(wf.Phase) == "plan" {
		if planArtifactExists(paths) {
			return nil
		}
		if strings.TrimSpace(wf.PlanSourcePath) != "" {
			return syncPlanArtifactFromSource(paths, wf.PlanSourcePath)
		}
		return errors.New("plan artifact is missing; no current Claude plan was resolved and artifacts/plan.md is empty")
	}

	info, err := os.Stat(planArtifact)
	if err != nil || info.Size() == 0 {
		return errors.New("approved plan artifact is missing; cannot run implementation review")
	}
	return nil
}

func syncPlanArtifactFromSource(paths workflow.Paths, sourcePath string) error {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return errors.New("plan source path is empty")
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read plan source: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return errors.New("plan source file is empty")
	}
	return os.WriteFile(filepath.Join(paths.ArtifactsDir, "plan.md"), data, 0644)
}

func planArtifactExists(paths workflow.Paths) bool {
	info, err := os.Stat(filepath.Join(paths.ArtifactsDir, "plan.md"))
	if err != nil {
		return false
	}
	return info.Size() > 0
}

func syncWorkflowDecisions(paths workflow.Paths, wf *workflow.State) (*ledger.Ledger, error) {
	if err := ledger.EnsureFile(paths.DecisionsFile); err != nil {
		return nil, err
	}
	ldg, err := ledger.Load(paths.DecisionsFile)
	if err != nil {
		return nil, err
	}
	if ldg == nil {
		ldg = ledger.NewEmpty()
	}

	reviewContent := readFileOr(workflow.ReviewFile(paths, wf), "")
	if strings.TrimSpace(reviewContent) == "" {
		return ldg, nil
	}

	findingsContent := readFileOr(workflow.FindingsFile(paths, wf), "")
	ldg.SyncDecisionsForRound(wf.Phase, wf.Phase, wf.Round, "codex", reviewContent, findingsContent)
	if err := ldg.Save(); err != nil {
		return nil, err
	}
	return ldg, nil
}

func isWorkflowConflict(err error) bool {
	var conflict *workflow.ActiveConflictError
	return errors.As(err, &conflict)
}

func displayOr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}

func readFileOr(path, fallback string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	return string(data)
}
