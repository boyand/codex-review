package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/boyand/codex-review/internal/config"
	"github.com/boyand/codex-review/internal/ledger"
	"github.com/boyand/codex-review/internal/lock"
	"github.com/boyand/codex-review/internal/workflow"
)

func setupFakeWorkflowCodex(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	codexPath := filepath.Join(dir, "codex")
	if err := os.WriteFile(codexPath, []byte("#!/bin/bash\n"+script), 0755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}
	t.Setenv("PATH", dir)
}

func TestRunPlanReviewCreatesWorkflowAndReview(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	planPath := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(planPath, []byte("# Plan\nDo the work"), 0644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	setupFakeWorkflowCodex(t, `
for i in "$@"; do
    if [ "$prev" = "--output-last-message" ]; then
        echo "[HIGH-1] Add one more edge case" > "$i"
        exit 0
    fi
    prev="$i"
done
exit 1
`)

	var out strings.Builder
	cfg := config.Config{CodexModel: "test-model", CallTimeoutSec: 10}
	code, err := RunPlanReview(cfg, &out, PlanReviewOptions{Prompt: "deeply review this plan"})
	if err != nil {
		t.Fatalf("RunPlanReview: %v", err)
	}
	if code != 0 {
		t.Fatalf("code=%d", code)
	}

	paths, wf, err := workflow.Resolve("")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if wf == nil {
		t.Fatal("expected workflow")
	}
	if wf.Phase != "plan" || wf.Round != 1 {
		t.Fatalf("phase=%q round=%d", wf.Phase, wf.Round)
	}
	if _, err := os.Stat(workflow.ReviewFile(paths, wf)); err != nil {
		t.Fatalf("review file missing: %v", err)
	}
	if !strings.Contains(out.String(), "Codex review completed for this round.") {
		t.Fatalf("unexpected output: %s", out.String())
	}
	if !strings.Contains(out.String(), "Project root:") {
		t.Fatalf("expected project root in output: %s", out.String())
	}
	if !strings.Contains(out.String(), "Workflow dir:") {
		t.Fatalf("expected workflow dir in output: %s", out.String())
	}
	if !strings.Contains(out.String(), "Running Codex review...") {
		t.Fatalf("expected running notice in output: %s", out.String())
	}
}

func TestRunPlanReviewUsesCurrentClaudePlanByDefault(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	home := t.TempDir()
	t.Setenv("HOME", home)
	plansDir := filepath.Join(home, ".claude", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("mkdir plans: %v", err)
	}
	planPath := filepath.Join(plansDir, "current.md")
	if err := os.WriteFile(planPath, []byte("# Native Plan\nUse native Claude plan"), 0644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	setupFakeWorkflowCodex(t, `
for i in "$@"; do
    if [ "$prev" = "--output-last-message" ]; then
        echo "[HIGH-1] Add one more edge case" > "$i"
        exit 0
    fi
    prev="$i"
done
exit 1
`)

	var out strings.Builder
	cfg := config.Config{CodexModel: "test-model", CallTimeoutSec: 10}
	code, err := RunPlanReview(cfg, &out, PlanReviewOptions{
		Prompt: "deeply review this plan",
	})
	if err != nil {
		t.Fatalf("RunPlanReview: %v", err)
	}
	if code != 0 {
		t.Fatalf("code=%d", code)
	}

	paths, wf, err := workflow.Resolve("")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if wf == nil {
		t.Fatal("expected workflow")
	}
	if wf.PlanSourcePath != planPath {
		t.Fatalf("PlanSourcePath=%q want %q", wf.PlanSourcePath, planPath)
	}
	data, err := os.ReadFile(filepath.Join(paths.ArtifactsDir, "plan.md"))
	if err != nil {
		t.Fatalf("read hydrated plan: %v", err)
	}
	if !strings.Contains(string(data), "Use native Claude plan") {
		t.Fatalf("unexpected plan artifact: %q", string(data))
	}
}

func TestRunPlanReviewFailsWhenCurrentSessionHasNoPlanFile(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	home := t.TempDir()
	t.Setenv("HOME", home)
	sessionsDir := filepath.Join(home, ".claude", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	projectKey := strings.ReplaceAll(filepath.Clean(dir), string(filepath.Separator), "-")
	projectDir := filepath.Join(home, ".claude", "projects", projectKey)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}
	sessionFile := filepath.Join(sessionsDir, strconv.Itoa(os.Getpid())+".json")
	sessionBody := fmt.Sprintf(`{"pid":%d,"sessionId":"sess-current","cwd":"%s","startedAt":1}`, os.Getpid(), dir)
	if err := os.WriteFile(sessionFile, []byte(sessionBody), 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	transcript := filepath.Join(projectDir, "sess-current.jsonl")
	if err := os.WriteFile(transcript, []byte(`{"message":{"content":[{"type":"text","text":"inline plan only"}]},"timestamp":"2026-03-25T12:00:00Z"}`), 0644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	var out strings.Builder
	cfg := config.Config{CodexModel: "test-model", CallTimeoutSec: 10}
	code, err := RunPlanReview(cfg, &out, PlanReviewOptions{Prompt: "deeply review this plan"})
	if code != 1 {
		t.Fatalf("code=%d want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "--plan <path>") {
		t.Fatalf("err=%v want guidance to use --plan <path>", err)
	}
}

func TestWorkflowApproveAdvancesToImplement(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	s, paths, err := workflow.New("review plan", "")
	if err != nil {
		t.Fatalf("workflow.New: %v", err)
	}
	if err := workflow.EnsureDirs(paths); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	if err := s.Save(paths.WorkflowFile); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.ArtifactsDir, "plan.md"), []byte("# Plan\nDo the work"), 0644); err != nil {
		t.Fatalf("write plan artifact: %v", err)
	}
	if err := os.WriteFile(workflow.ReviewFile(paths, s), []byte("[HIGH-1] Add a regression test"), 0644); err != nil {
		t.Fatalf("write review: %v", err)
	}
	if err := os.WriteFile(workflow.FindingsFile(paths, s), []byte("# Responses\n\nHIGH-1\nDecision: ACCEPT\n"), 0644); err != nil {
		t.Fatalf("write findings: %v", err)
	}

	var out strings.Builder
	handled, code, err := RunWorkflowApprove(&out, s.ID)
	if !handled {
		t.Fatal("expected workflow handler")
	}
	if err != nil {
		t.Fatalf("RunWorkflowApprove: %v", err)
	}
	if code != 0 {
		t.Fatalf("code=%d", code)
	}

	_, got, err := workflow.Resolve(s.ID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Phase != "implement" || got.Round != 1 {
		t.Fatalf("phase=%q round=%d", got.Phase, got.Round)
	}
}

func TestRunPlanReviewLockErrorIncludesWorkflowContext(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	planPath := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(planPath, []byte("# Plan\nDo the work"), 0644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	wf, paths, err := workflow.New("review plan", planPath)
	if err != nil {
		t.Fatalf("workflow.New: %v", err)
	}
	if err := workflow.EnsureDirs(paths); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	if err := ledger.EnsureFile(paths.DecisionsFile); err != nil {
		t.Fatalf("EnsureFile: %v", err)
	}
	if err := wf.Save(paths.WorkflowFile); err != nil {
		t.Fatalf("Save: %v", err)
	}
	lk := lock.New(paths.LockDir)
	if err := lk.Acquire(); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer lk.Release()

	var out strings.Builder
	cfg := config.Config{CodexModel: "test-model", CallTimeoutSec: 10}
	code, err := RunPlanReview(cfg, &out, PlanReviewOptions{
		WorkflowID: wf.ID,
		PlanPath:   planPath,
		Prompt:     "deeply review this plan",
	})
	if code != 1 {
		t.Fatalf("code=%d want 1", code)
	}
	if err == nil {
		t.Fatal("expected lock error")
	}
	msg := err.Error()
	if !strings.Contains(msg, wf.ID) {
		t.Fatalf("expected workflow id in error: %v", err)
	}
	if !strings.Contains(msg, "Project root:") {
		t.Fatalf("expected project root in error: %v", err)
	}
	if !strings.Contains(msg, "Workflow dir:") {
		t.Fatalf("expected workflow dir in error: %v", err)
	}
	if !strings.Contains(msg, "Lock dir:") {
		t.Fatalf("expected lock dir in error: %v", err)
	}
}

func TestRunImplementReviewAutoApprovesPlanApproval(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	wf, paths, err := workflow.New("review plan", filepath.Join(dir, "plan.md"))
	if err != nil {
		t.Fatalf("workflow.New: %v", err)
	}
	if err := workflow.EnsureDirs(paths); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	if err := wf.Save(paths.WorkflowFile); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.ArtifactsDir, "plan.md"), []byte("# Plan\nDo the work"), 0644); err != nil {
		t.Fatalf("write plan artifact: %v", err)
	}
	if err := os.WriteFile(workflow.ReviewFile(paths, wf), []byte("[HIGH-1] Add a regression test"), 0644); err != nil {
		t.Fatalf("write review: %v", err)
	}
	if err := os.WriteFile(workflow.FindingsFile(paths, wf), []byte("# Responses\n\nHIGH-1\nDecision: ACCEPT\n"), 0644); err != nil {
		t.Fatalf("write findings: %v", err)
	}

	setupFakeWorkflowCodex(t, `
for i in "$@"; do
    if [ "$prev" = "--output-last-message" ]; then
        echo "[HIGH-1] Verify implementation paths" > "$i"
        exit 0
    fi
    prev="$i"
done
exit 1
`)

	var out strings.Builder
	cfg := config.Config{CodexModel: "test-model", CallTimeoutSec: 10}
	code, err := RunImplementReview(cfg, &out, ImplementReviewOptions{WorkflowID: wf.ID, Prompt: "review implementation"})
	if err != nil {
		t.Fatalf("RunImplementReview: %v", err)
	}
	if code != 0 {
		t.Fatalf("code=%d", code)
	}

	_, got, err := workflow.Resolve(wf.ID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Phase != "implement" || got.Round != 1 {
		t.Fatalf("phase=%q round=%d", got.Phase, got.Round)
	}
	if _, err := os.Stat(workflow.ReviewFile(paths, got)); err != nil {
		t.Fatalf("implement review file missing: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "Approved workflow "+wf.ID+" plan round. Phase advanced to implement (round 1).") {
		t.Fatalf("missing auto-approve output:\n%s", output)
	}
	if !strings.Contains(output, "Phase: implement") {
		t.Fatalf("missing implement phase output:\n%s", output)
	}
}

func TestRunImplementReviewRejectsPlanWorkingStep(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	wf, paths, err := workflow.New("review plan", filepath.Join(dir, "plan.md"))
	if err != nil {
		t.Fatalf("workflow.New: %v", err)
	}
	if err := workflow.EnsureDirs(paths); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	if err := wf.Save(paths.WorkflowFile); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.ArtifactsDir, "plan.md"), []byte("# Plan\nDo the work"), 0644); err != nil {
		t.Fatalf("write plan artifact: %v", err)
	}

	var out strings.Builder
	cfg := config.Config{CodexModel: "test-model", CallTimeoutSec: 10}
	code, err := RunImplementReview(cfg, &out, ImplementReviewOptions{WorkflowID: wf.ID, Prompt: "review implementation"})
	if err == nil {
		t.Fatal("expected error")
	}
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(err.Error(), "phase 'plan' step 'working'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunImplementReviewRejectsPlanReviewStep(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	wf, paths, err := workflow.New("review plan", filepath.Join(dir, "plan.md"))
	if err != nil {
		t.Fatalf("workflow.New: %v", err)
	}
	if err := workflow.EnsureDirs(paths); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	if err := wf.Save(paths.WorkflowFile); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.ArtifactsDir, "plan.md"), []byte("# Plan\nDo the work"), 0644); err != nil {
		t.Fatalf("write plan artifact: %v", err)
	}
	if err := os.WriteFile(workflow.ReviewFile(paths, wf), []byte("[HIGH-1] Add a regression test"), 0644); err != nil {
		t.Fatalf("write review: %v", err)
	}

	var out strings.Builder
	cfg := config.Config{CodexModel: "test-model", CallTimeoutSec: 10}
	code, err := RunImplementReview(cfg, &out, ImplementReviewOptions{WorkflowID: wf.ID, Prompt: "review implementation"})
	if err == nil {
		t.Fatal("expected error")
	}
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(err.Error(), "phase 'plan' step 'review'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPlanReviewCreatesWorkflowWhenAnotherSessionOwnsOne(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	home := t.TempDir()
	t.Setenv("HOME", home)
	sessionsDir := filepath.Join(home, ".claude", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionFile := filepath.Join(sessionsDir, strconv.Itoa(os.Getpid())+".json")
	sessionBody := fmt.Sprintf(`{"pid":%d,"sessionId":"sess-b","cwd":"%s","startedAt":1}`, os.Getpid(), dir)
	if err := os.WriteFile(sessionFile, []byte(sessionBody), 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}

	plansDir := filepath.Join(home, ".claude", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("mkdir plans: %v", err)
	}
	planPath := filepath.Join(plansDir, "current.md")
	if err := os.WriteFile(planPath, []byte("# Native Plan\nUse native Claude plan"), 0644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	other, otherPaths, err := workflow.New("other session workflow", "")
	if err != nil {
		t.Fatalf("workflow.New: %v", err)
	}
	workflow.BindOwner(other, &workflow.ClaudeSession{
		SessionID:  "sess-a",
		CWD:        dir,
		ProjectKey: strings.ReplaceAll(dir, string(os.PathSeparator), "-"),
	})
	if err := workflow.EnsureDirs(otherPaths); err != nil {
		t.Fatalf("EnsureDirs other: %v", err)
	}
	if err := other.Save(otherPaths.WorkflowFile); err != nil {
		t.Fatalf("Save other: %v", err)
	}

	setupFakeWorkflowCodex(t, `
for i in "$@"; do
    if [ "$prev" = "--output-last-message" ]; then
        echo "[HIGH-1] Add one more edge case" > "$i"
        exit 0
    fi
    prev="$i"
done
exit 1
`)

	var out strings.Builder
	cfg := config.Config{CodexModel: "test-model", CallTimeoutSec: 10}
	code, err := RunPlanReview(cfg, &out, PlanReviewOptions{
		PlanPath: planPath,
		Prompt:   "deeply review this plan",
	})
	if err != nil {
		t.Fatalf("RunPlanReview: %v", err)
	}
	if code != 0 {
		t.Fatalf("code=%d", code)
	}

	ids, err := workflow.DiscoverActiveIDs()
	if err != nil {
		t.Fatalf("DiscoverActiveIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("active workflow ids=%v want 2", ids)
	}

	paths, state, err := workflow.Resolve("")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if state == nil {
		t.Fatal("expected current-session workflow")
	}
	if state.OwnerSessionID != "sess-b" {
		t.Fatalf("OwnerSessionID=%q want sess-b", state.OwnerSessionID)
	}
	if paths.ID == other.ID {
		t.Fatal("expected a new workflow for current session, got the other session's workflow")
	}
}

func TestRunPlanReviewKeepsCanonicalArtifactWhenSourceChanges(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	planPath := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(planPath, []byte("# Plan\nAgentPage canonical"), 0644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	wf, paths, err := workflow.New("review plan", planPath)
	if err != nil {
		t.Fatalf("workflow.New: %v", err)
	}
	if err := workflow.EnsureDirs(paths); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.ArtifactsDir, "plan.md"), []byte("# Plan\nAgentPage canonical"), 0644); err != nil {
		t.Fatalf("write canonical plan: %v", err)
	}
	if err := wf.Save(paths.WorkflowFile); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := os.WriteFile(planPath, []byte("# Plan\nGame migration overwrite"), 0644); err != nil {
		t.Fatalf("overwrite source plan: %v", err)
	}

	setupFakeWorkflowCodex(t, `
for i in "$@"; do
    if [ "$prev" = "--output-last-message" ]; then
        echo "[HIGH-1] Keep reviewing canonical artifact" > "$i"
        exit 0
    fi
    prev="$i"
done
exit 1
`)

	cfg := config.Config{CodexModel: "test-model", CallTimeoutSec: 10}
	var out strings.Builder
	code, err := RunPlanReview(cfg, &out, PlanReviewOptions{WorkflowID: wf.ID, Prompt: "review plan"})
	if err != nil {
		t.Fatalf("RunPlanReview: %v", err)
	}
	if code != 0 {
		t.Fatalf("code=%d", code)
	}

	data, err := os.ReadFile(filepath.Join(paths.ArtifactsDir, "plan.md"))
	if err != nil {
		t.Fatalf("read canonical plan: %v", err)
	}
	if strings.Contains(string(data), "Game migration overwrite") {
		t.Fatalf("canonical plan was overwritten: %q", string(data))
	}
}

func TestRunPlanReviewExplicitPlanOverridesCanonicalArtifact(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	oldPlan := filepath.Join(dir, "old-plan.md")
	newPlan := filepath.Join(dir, "new-plan.md")
	if err := os.WriteFile(oldPlan, []byte("# Plan\nOld canonical"), 0644); err != nil {
		t.Fatalf("write old plan: %v", err)
	}
	if err := os.WriteFile(newPlan, []byte("# Plan\nNew explicit override"), 0644); err != nil {
		t.Fatalf("write new plan: %v", err)
	}

	wf, paths, err := workflow.New("review plan", oldPlan)
	if err != nil {
		t.Fatalf("workflow.New: %v", err)
	}
	if err := workflow.EnsureDirs(paths); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.ArtifactsDir, "plan.md"), []byte("# Plan\nOld canonical"), 0644); err != nil {
		t.Fatalf("write canonical plan: %v", err)
	}
	if err := wf.Save(paths.WorkflowFile); err != nil {
		t.Fatalf("Save: %v", err)
	}

	setupFakeWorkflowCodex(t, `
for i in "$@"; do
    if [ "$prev" = "--output-last-message" ]; then
        echo "[HIGH-1] Explicit override works" > "$i"
        exit 0
    fi
    prev="$i"
done
exit 1
`)

	cfg := config.Config{CodexModel: "test-model", CallTimeoutSec: 10}
	var out strings.Builder
	code, err := RunPlanReview(cfg, &out, PlanReviewOptions{WorkflowID: wf.ID, PlanPath: newPlan, Prompt: "review plan"})
	if err != nil {
		t.Fatalf("RunPlanReview: %v", err)
	}
	if code != 0 {
		t.Fatalf("code=%d", code)
	}

	data, err := os.ReadFile(filepath.Join(paths.ArtifactsDir, "plan.md"))
	if err != nil {
		t.Fatalf("read canonical plan: %v", err)
	}
	if !strings.Contains(string(data), "New explicit override") {
		t.Fatalf("canonical plan not updated: %q", string(data))
	}
}

func TestRunWorkflowSummarySyncsDecisionsFromFindings(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	wf, paths, err := workflow.New("review plan", filepath.Join(dir, "plan.md"))
	if err != nil {
		t.Fatalf("workflow.New: %v", err)
	}
	if err := workflow.EnsureDirs(paths); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	if err := wf.Save(paths.WorkflowFile); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := ledger.EnsureFile(paths.DecisionsFile); err != nil {
		t.Fatalf("EnsureFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.ArtifactsDir, "plan.md"), []byte("# Plan\nShip it"), 0644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	reviewContent := strings.Join([]string{
		"[HIGH-1] Add rollout validation for prod",
		"[MEDIUM-1] Clarify local test instructions",
	}, "\n")
	if err := os.WriteFile(workflow.ReviewFile(paths, wf), []byte(reviewContent), 0644); err != nil {
		t.Fatalf("write review: %v", err)
	}
	findingsContent := strings.Join([]string{
		"# Findings",
		"",
		"## HIGH-1",
		"Decision: ACCEPT",
		"",
		"## MEDIUM-1",
		"Decision: DEFER",
	}, "\n")
	if err := os.WriteFile(workflow.FindingsFile(paths, wf), []byte(findingsContent), 0644); err != nil {
		t.Fatalf("write findings: %v", err)
	}

	var out strings.Builder
	handled, code, err := RunWorkflowSummary(&out, wf.ID)
	if !handled {
		t.Fatal("expected workflow handler")
	}
	if err != nil {
		t.Fatalf("RunWorkflowSummary: %v", err)
	}
	if code != 0 {
		t.Fatalf("code=%d", code)
	}

	output := out.String()
	if !strings.Contains(output, "High: 1 total, 1 fix, 0 reject, 0 open") {
		t.Fatalf("summary did not reflect accepted finding:\n%s", output)
	}
	if !strings.Contains(output, "Medium: 1 total, 0 fix, 1 reject, 0 open") {
		t.Fatalf("summary did not reflect deferred finding:\n%s", output)
	}
	if !strings.Contains(output, "HIGH-1 HIGH decision=ACCEPT outcome=FIX selected=yes") {
		t.Fatalf("summary row missing synced accept decision:\n%s", output)
	}
	if !strings.Contains(output, "MEDIUM-1 MEDIUM decision=DEFER outcome=NO-CHANGE selected=no") {
		t.Fatalf("summary row missing synced defer decision:\n%s", output)
	}

	ldg, err := ledger.Load(paths.DecisionsFile)
	if err != nil {
		t.Fatalf("Load ledger: %v", err)
	}
	rows := ldg.RowsForPhaseRound(wf.Phase, wf.Round)
	if len(rows) != 2 {
		t.Fatalf("rows=%d want 2", len(rows))
	}
	if rows[0].Decision == "OPEN" || rows[1].Decision == "OPEN" {
		t.Fatalf("ledger was not resynced: %+v", rows)
	}
}
