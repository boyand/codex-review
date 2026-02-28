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

	"github.com/boyand/codex-review-loop/internal/config"
	"github.com/boyand/codex-review-loop/internal/state"
)

// Check represents a single health check result.
type Check struct {
	Name    string
	Result  string // PASS, WARN, FAIL
	Details string
}

// Run executes all health checks and writes results to w.
func Run(w io.Writer, cfg config.Config) {
	var checks []Check
	var rootCause string

	// 1. Go toolchain
	checks = append(checks, checkGoToolchain())

	// 2. Codex CLI
	codexCheck := checkCodexCLI()
	checks = append(checks, codexCheck)

	// 3. Project loop state
	stateCheck := checkLoopState()
	checks = append(checks, stateCheck)

	// 4. Artifacts health
	checks = append(checks, checkArtifacts())

	// 5. Lock health
	checks = append(checks, checkLock())

	// Classify root cause
	overall := "PASS"
	for _, c := range checks {
		if c.Result == "FAIL" {
			overall = "FAIL"
		} else if c.Result == "WARN" && overall == "PASS" {
			overall = "WARN"
		}
	}

	switch {
	case codexCheck.Result == "FAIL":
		rootCause = "codex-missing"
	case stateCheck.Result == "WARN" && strings.Contains(stateCheck.Details, "No active loop"):
		rootCause = "no-active-loop-state"
	case overall == "PASS":
		rootCause = "healthy"
	default:
		rootCause = "unknown"
	}

	// Render
	fmt.Fprintln(w, "## Codex Review Loop Doctor")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "### Summary")
	fmt.Fprintf(w, "- Overall: %s\n", overall)
	fmt.Fprintf(w, "- Root cause: %s\n", rootCause)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "### Checks")
	fmt.Fprintln(w, "| Check | Result | Details |")
	fmt.Fprintln(w, "|-------|--------|---------|")
	for _, c := range checks {
		fmt.Fprintf(w, "| %s | %s | %s |\n", c.Name, c.Result, c.Details)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "### Recommended next action")
	switch rootCause {
	case "codex-missing":
		fmt.Fprintln(w, "1. Install Codex CLI: `npm install -g @openai/codex`")
		fmt.Fprintln(w, "2. Verify: `codex --version`")
	case "no-active-loop-state":
		fmt.Fprintln(w, "1. Start a loop with `/codex-review-loop <task>`")
	case "healthy":
		fmt.Fprintln(w, "1. Everything looks good. You can start or continue a review loop.")
	default:
		fmt.Fprintln(w, "1. Review the check details above for specific issues.")
	}
}

func checkGoToolchain() Check {
	goPath, err := exec.LookPath("go")
	if err != nil {
		return Check{
			Name:    "Go toolchain",
			Result:  "FAIL",
			Details: "'go' not found on PATH (required by stop-hook shim to rebuild). Install from https://go.dev/dl/. Binary runtime: " + runtime.Version(),
		}
	}
	return Check{
		Name:    "Go toolchain",
		Result:  "PASS",
		Details: fmt.Sprintf("%s (PATH: %s)", runtime.Version(), goPath),
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

func checkLoopState() Check {
	s, err := state.Load(state.StateFile)
	if err != nil {
		return Check{
			Name:    "Project loop state",
			Result:  "FAIL",
			Details: fmt.Sprintf("Error reading state: %v", err),
		}
	}
	if s == nil {
		return Check{
			Name:    "Project loop state",
			Result:  "WARN",
			Details: "No active loop in this project",
		}
	}
	if !s.Active() {
		return Check{
			Name:    "Project loop state",
			Result:  "WARN",
			Details: "State file exists but active=false",
		}
	}

	idx := s.CurrentPhaseIndex()
	return Check{
		Name:   "Project loop state",
		Result: "PASS",
		Details: fmt.Sprintf("active=true, phase=%s, substep=%s, round=%d",
			s.PipelineName(idx), s.CurrentSubstep(), s.CurrentRound()),
	}
}

func checkArtifacts() Check {
	dir := state.ArtifactsDir
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return Check{
			Name:    "Artifacts health",
			Result:  "WARN",
			Details: "Artifacts directory does not exist",
		}
	}

	// Check for empty review files
	matches, _ := filepath.Glob(filepath.Join(dir, "*-review-r*.md"))
	var warnings []string
	for _, m := range matches {
		fi, err := os.Stat(m)
		if err == nil && fi.Size() == 0 {
			warnings = append(warnings, filepath.Base(m))
		}
	}

	if len(warnings) > 0 {
		return Check{
			Name:    "Artifacts health",
			Result:  "WARN",
			Details: fmt.Sprintf("empty review files: %s", strings.Join(warnings, ", ")),
		}
	}

	entries, _ := os.ReadDir(dir)
	return Check{
		Name:    "Artifacts health",
		Result:  "PASS",
		Details: fmt.Sprintf("%d files in artifacts directory", len(entries)),
	}
}

func checkLock() Check {
	lockDir := state.LockDir
	_, err := os.Stat(lockDir)
	if os.IsNotExist(err) {
		return Check{
			Name:    "Hook lock health",
			Result:  "PASS",
			Details: "No stale lock",
		}
	}

	pidData, _ := os.ReadFile(filepath.Join(lockDir, "pid"))
	return Check{
		Name:    "Hook lock health",
		Result:  "WARN",
		Details: fmt.Sprintf("Lock directory exists (pid: %s). May be stale.", strings.TrimSpace(string(pidData))),
	}
}
