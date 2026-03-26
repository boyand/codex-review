// Package runner executes Codex CLI commands with timeout, logging, and retry.
package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/boyand/codex-review/internal/config"
	"github.com/boyand/codex-review/internal/phase"
)

// Result holds the outcome of a codex execution.
type Result struct {
	ExitCode      int
	LastErrorLine string
	LogPath       string
}

// RequireCodexCLI checks that codex is on PATH. Returns an error if not found.
func RequireCodexCLI() error {
	_, err := exec.LookPath("codex")
	if err != nil {
		return fmt.Errorf("the codex CLI is required but was not found on PATH. Install it with: npm install -g @openai/codex")
	}
	return nil
}

// CodexExec runs a codex exec command with timeout and log capture.
// Returns nil error on success, or an error with details on failure.
func CodexExec(cfg config.Config, prompt, outputFile string, flags []string, artifactsDir string) (*Result, error) {
	result, err := codexExecOnce(cfg, prompt, outputFile, flags, artifactsDir)
	if err == nil {
		return result, nil
	}

	// Codex CLI may require trusted-directory checks in some environments.
	// Retry once with --skip-git-repo-check when that is the only blocker.
	if shouldRetryWithSkipGitCheck(result, flags) {
		retryFlags := append([]string{}, flags...)
		retryFlags = append(retryFlags, "--skip-git-repo-check")
		retryResult, retryErr := codexExecOnce(cfg, prompt, outputFile, retryFlags, artifactsDir)
		if retryErr == nil {
			if result != nil && result.LogPath != "" {
				_ = os.Remove(result.LogPath)
			}
			return retryResult, nil
		}
		return retryResult, retryErr
	}

	return result, err
}

func codexExecOnce(cfg config.Config, prompt, outputFile string, flags []string, artifactsDir string) (*Result, error) {
	logFile, err := os.CreateTemp(artifactsDir, "codex-exec.*.log")
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}
	logPath := logFile.Name()

	timeout := time.Duration(cfg.CallTimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{"exec", prompt}
	args = append(args, flags...)
	args = append(args, "--output-last-message", outputFile, "-m", cfg.CodexModel)

	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	err = cmd.Run()
	logFile.Close()

	result := &Result{LogPath: logPath}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.ExitCode = 124
			result.LastErrorLine = fmt.Sprintf("timed out after %ds", cfg.CallTimeoutSec)
			return result, fmt.Errorf("codex timed out after %ds", cfg.CallTimeoutSec)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
		result.LastErrorLine = readLastLines(logPath, 8)
		return result, fmt.Errorf("codex exec failed: %w", err)
	}

	// Success — remove log
	os.Remove(logPath)
	result.LogPath = ""
	return result, nil
}

func shouldRetryWithSkipGitCheck(result *Result, flags []string) bool {
	if result == nil {
		return false
	}
	if hasFlag(flags, "--skip-git-repo-check") {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(result.LastErrorLine))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "not inside a trusted directory") &&
		strings.Contains(msg, "--skip-git-repo-check")
}

func hasFlag(flags []string, target string) bool {
	for _, f := range flags {
		if strings.TrimSpace(f) == target {
			return true
		}
	}
	return false
}

// ReviewValidator checks whether review output content is valid.
type ReviewValidator func(content string) bool

// RunCodexReview runs a codex review with retry on malformed output.
// Exec failures (timeout, crash) are returned immediately without retry.
// Only malformed/empty output triggers a retry with the strict suffix.
func RunCodexReview(cfg config.Config, prompt, strictSuffix, reviewFile string, flags []string, artifactsDir string, isValid ReviewValidator) (*Result, error) {
	var lastResult *Result

	for attempt := 1; attempt <= 2; attempt++ {
		p := prompt
		if attempt == 2 {
			p = prompt + strictSuffix
			fmt.Fprintln(os.Stderr, "codex-review: First review output was malformed; retrying with strict format enforcement...")
		}

		result, err := CodexExec(cfg, p, reviewFile, flags, artifactsDir)
		if err != nil {
			// Exec failure (timeout, crash) — do not retry, return immediately
			return result, err
		}
		lastResult = result

		// Check output exists and is non-empty
		data, readErr := os.ReadFile(reviewFile)
		if readErr != nil || len(data) == 0 {
			if attempt == 1 {
				continue
			}
			return result, fmt.Errorf("review output empty or missing")
		}

		// Validate output structure
		if isValid != nil && !isValid(string(data)) {
			if attempt == 1 {
				continue
			}
			return result, fmt.Errorf("review output malformed after retry")
		}

		return result, nil
	}
	return lastResult, fmt.Errorf("codex review failed after 2 attempts")
}

// RunCodexWorker runs a codex worker phase.
func RunCodexWorker(cfg config.Config, prompt, outputFile string, flags []string, artifactsDir string) (*Result, error) {
	fmt.Fprintf(os.Stderr, "codex-review: Running Codex worker...\n")
	return CodexExec(cfg, prompt, outputFile, flags, artifactsDir)
}

// ParseFlags splits a space-separated flag string into a slice.
func ParseFlags(flagStr string) []string {
	return strings.Fields(flagStr)
}

func readLastLines(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(strings.Fields(strings.Join(lines, " ")), " ")
}

// ResolveReferenceFile finds the artifact file for a compare_to reference.
func ResolveReferenceFile(artifactsDir, ref string, pipelineNames map[string]string) string {
	if ref == "" {
		return ""
	}

	// Direct artifact file check
	path := filepath.Join(artifactsDir, ref+".md")
	if fileExists(path) {
		return path
	}

	// Check pipeline names/artifacts
	if artifact, ok := pipelineNames[ref]; ok {
		path = filepath.Join(artifactsDir, artifact+".md")
		if fileExists(path) {
			return path
		}
	}

	// Slugify fallback
	path = filepath.Join(artifactsDir, phase.Slugify(ref)+".md")
	if fileExists(path) {
		return path
	}

	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
