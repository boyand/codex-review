// Package doctor runs health checks for the codex review loop.
package doctor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/boyand/codex-review/internal/workflow"
)

type Check struct {
	Name    string
	Result  string
	Details string
}

func Run(w io.Writer) {
	session, sessionCheck := checkCurrentClaudeSession()
	paths, state, workflowCheck := checkWorkflowResolution()
	ownershipCheck := checkWorkflowOwnership()
	artifactsCheck := checkArtifacts(paths, state)

	checks := []Check{
		checkGoToolchain(),
		checkCodexCLI(),
		sessionCheck,
		workflowCheck,
		ownershipCheck,
		artifactsCheck,
	}

	overall := "PASS"
	for _, check := range checks {
		switch check.Result {
		case "FAIL":
			overall = "FAIL"
		case "WARN":
			if overall == "PASS" {
				overall = "WARN"
			}
		}
	}

	rootCause := "healthy"
	switch {
	case checks[1].Result == "FAIL":
		rootCause = "codex-missing"
	case session == nil:
		rootCause = "no-claude-session"
	case workflowCheck.Result == "WARN" && strings.Contains(workflowCheck.Details, "No active workflow"):
		rootCause = "no-session-workflow"
	case workflowCheck.Result == "FAIL":
		rootCause = "workflow-resolution"
	case ownershipCheck.Result == "WARN":
		rootCause = "workflow-ownership"
	case artifactsCheck.Result == "WARN":
		rootCause = "artifacts"
	}

	fmt.Fprintln(w, "## Codex Review Doctor")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "### Summary")
	fmt.Fprintf(w, "- Overall: %s\n", overall)
	fmt.Fprintf(w, "- Root cause: %s\n", rootCause)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "### Checks")
	fmt.Fprintln(w, "| Check | Result | Details |")
	fmt.Fprintln(w, "|-------|--------|---------|")
	for _, check := range checks {
		fmt.Fprintf(w, "| %s | %s | %s |\n", check.Name, check.Result, check.Details)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "### Recommended next action")
	switch rootCause {
	case "codex-missing":
		fmt.Fprintln(w, "1. Install Codex CLI: `npm install -g @openai/codex`")
		fmt.Fprintln(w, "2. Verify with: `codex --version`")
	case "no-claude-session":
		fmt.Fprintln(w, "1. Run doctor from inside Claude Code if you want session-scoped workflow diagnostics.")
		fmt.Fprintln(w, "2. Outside Claude, use `--workflow <id>` for explicit targeting.")
	case "no-session-workflow":
		fmt.Fprintln(w, "1. Start a workflow from this Claude thread with `/codex-review:plan ...`.")
	case "workflow-resolution":
		fmt.Fprintln(w, "1. Resolve the conflicting workflows shown above.")
		fmt.Fprintln(w, "2. Use `--workflow <id>` if you need to target a non-owned workflow explicitly.")
	case "workflow-ownership":
		fmt.Fprintln(w, "1. Resolve duplicate or stale active workflows by inspecting `/codex-review:status` and, if needed, removing stale workflow directories under `.claude/codex-review/workflows/`.")
	case "artifacts":
		fmt.Fprintln(w, "1. Re-run the current review command so canonical artifacts are regenerated.")
	default:
		fmt.Fprintln(w, "1. Everything looks healthy.")
	}
}

func checkGoToolchain() Check {
	goPath, err := exec.LookPath("go")
	if err != nil {
		return Check{
			Name:    "Go toolchain",
			Result:  "FAIL",
			Details: "'go' not found on PATH (needed by the entry wrapper to rebuild). Runtime: " + runtime.Version(),
		}
	}
	return Check{
		Name:    "Go toolchain",
		Result:  "PASS",
		Details: fmt.Sprintf("%s (%s)", runtime.Version(), goPath),
	}
}

func checkCodexCLI() Check {
	path, err := exec.LookPath("codex")
	if err != nil {
		return Check{
			Name:    "Codex CLI availability",
			Result:  "FAIL",
			Details: "codex not found on PATH. Install: npm install -g @openai/codex",
		}
	}
	out, err := exec.Command("codex", "--version").CombinedOutput()
	version := strings.TrimSpace(string(out))
	if err != nil {
		version = "(version check failed)"
	}
	return Check{
		Name:    "Codex CLI availability",
		Result:  "PASS",
		Details: fmt.Sprintf("%s, %s", path, version),
	}
}

func checkCurrentClaudeSession() (*workflow.ClaudeSession, Check) {
	session, err := workflow.ResolveCurrentClaudeSession()
	if err != nil {
		return nil, Check{
			Name:    "Current Claude session",
			Result:  "FAIL",
			Details: err.Error(),
		}
	}
	if session == nil {
		return nil, Check{
			Name:    "Current Claude session",
			Result:  "WARN",
			Details: "No Claude session could be inferred from the current process tree",
		}
	}
	return session, Check{
		Name:    "Current Claude session",
		Result:  "PASS",
		Details: fmt.Sprintf("session=%s, cwd=%s", session.SessionID, session.CWD),
	}
}

func checkWorkflowResolution() (workflow.Paths, *workflow.State, Check) {
	paths, state, err := workflow.ResolvePreview("")
	if err != nil {
		return workflow.Paths{}, nil, Check{
			Name:    "Workflow resolution",
			Result:  "FAIL",
			Details: err.Error(),
		}
	}
	if state == nil {
		return workflow.Paths{}, nil, Check{
			Name:    "Workflow resolution",
			Result:  "WARN",
			Details: "No active workflow resolved for this Claude session",
		}
	}
	step, err := workflow.DeriveStep(paths, state)
	if err != nil {
		return workflow.Paths{}, nil, Check{
			Name:    "Workflow resolution",
			Result:  "FAIL",
			Details: err.Error(),
		}
	}
	return paths, state, Check{
		Name:    "Workflow resolution",
		Result:  "PASS",
		Details: fmt.Sprintf("workflow=%s, phase=%s, round=%d, step=%s", state.ID, state.Phase, state.Round, step),
	}
}

func checkWorkflowOwnership() Check {
	ids, err := workflow.DiscoverActiveIDs()
	if err != nil {
		return Check{
			Name:    "Workflow ownership",
			Result:  "FAIL",
			Details: err.Error(),
		}
	}
	if len(ids) == 0 {
		return Check{
			Name:    "Workflow ownership",
			Result:  "PASS",
			Details: "No active workflows",
		}
	}

	owners := map[string][]string{}
	var unowned []string
	for _, id := range ids {
		paths := workflow.PathsFor(id)
		state, err := workflow.Load(paths.WorkflowFile)
		if err != nil || state == nil {
			continue
		}
		owner := strings.TrimSpace(state.OwnerSessionID)
		if owner == "" {
			unowned = append(unowned, id)
			continue
		}
		owners[owner] = append(owners[owner], id)
	}

	var duplicateOwners []string
	for owner, workflowIDs := range owners {
		if len(workflowIDs) > 1 {
			duplicateOwners = append(duplicateOwners, fmt.Sprintf("%s=%s", owner, strings.Join(workflowIDs, ",")))
		}
	}

	if len(unowned) > 0 || len(duplicateOwners) > 0 {
		var parts []string
		if len(unowned) > 0 {
			parts = append(parts, "unowned="+strings.Join(unowned, ","))
		}
		if len(duplicateOwners) > 0 {
			parts = append(parts, "duplicate-owners="+strings.Join(duplicateOwners, ";"))
		}
		return Check{
			Name:    "Workflow ownership",
			Result:  "WARN",
			Details: strings.Join(parts, " | "),
		}
	}

	return Check{
		Name:    "Workflow ownership",
		Result:  "PASS",
		Details: fmt.Sprintf("%d active workflow(s), all session-owned", len(ids)),
	}
}

func checkArtifacts(paths workflow.Paths, state *workflow.State) Check {
	if state == nil {
		return Check{
			Name:    "Artifacts health",
			Result:  "WARN",
			Details: fmt.Sprintf("No resolved workflow. Workflow artifacts live under %s/", workflow.WorkflowsDir),
		}
	}

	info, err := os.Stat(paths.ArtifactsDir)
	if err != nil || !info.IsDir() {
		return Check{
			Name:    "Artifacts health",
			Result:  "WARN",
			Details: fmt.Sprintf("Artifacts directory does not exist: %s", paths.ArtifactsDir),
		}
	}

	reviewFile := workflow.ReviewFile(paths, state)
	findingsFile := workflow.FindingsFile(paths, state)
	var warnings []string
	if _, err := os.Stat(reviewFile); err == nil {
		if info, err := os.Stat(reviewFile); err == nil && info.Size() == 0 {
			warnings = append(warnings, filepath.Base(reviewFile)+" empty")
		}
	}
	if _, err := os.Stat(findingsFile); err == nil {
		if info, err := os.Stat(findingsFile); err == nil && info.Size() == 0 {
			warnings = append(warnings, filepath.Base(findingsFile)+" empty")
		}
	}
	if len(warnings) > 0 {
		return Check{
			Name:    "Artifacts health",
			Result:  "WARN",
			Details: strings.Join(warnings, ", "),
		}
	}

	entries, _ := os.ReadDir(paths.ArtifactsDir)
	return Check{
		Name:    "Artifacts health",
		Result:  "PASS",
		Details: fmt.Sprintf("%d files in %s", len(entries), paths.ArtifactsDir),
	}
}
