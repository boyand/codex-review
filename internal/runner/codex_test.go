package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boyand/codex-review/internal/config"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"--sandbox=read-only", 1},
		{"--sandbox=read-only --flag2", 2},
		{"", 0},
		{"  ", 0},
	}
	for _, tt := range tests {
		got := ParseFlags(tt.input)
		if len(got) != tt.want {
			t.Errorf("ParseFlags(%q) = %v (len %d), want len %d", tt.input, got, len(got), tt.want)
		}
	}
}

func TestResolveReferenceFile(t *testing.T) {
	dir := t.TempDir()

	// Create a plan artifact
	os.WriteFile(filepath.Join(dir, "plan.md"), []byte("plan content"), 0644)

	pipelineNames := map[string]string{
		"plan": "plan",
	}

	// Direct match
	if got := ResolveReferenceFile(dir, "plan", pipelineNames); got == "" {
		t.Error("expected to find plan.md")
	}

	// Pipeline name match
	if got := ResolveReferenceFile(dir, "plan", pipelineNames); got == "" {
		t.Error("expected to find via pipeline")
	}

	// Not found
	if got := ResolveReferenceFile(dir, "nonexistent", pipelineNames); got != "" {
		t.Errorf("expected empty for nonexistent, got %q", got)
	}
}

func TestRequireCodexCLI(t *testing.T) {
	// Just test it doesn't panic
	_ = RequireCodexCLI()
}

func TestReadLastLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	content := "line1\nline2\nline3\nline4\nline5\n"
	os.WriteFile(path, []byte(content), 0644)

	got := readLastLines(path, 3)
	if got == "" {
		t.Error("expected non-empty result")
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.txt")
	os.WriteFile(path, []byte("x"), 0644)

	if !fileExists(path) {
		t.Error("expected true for existing file")
	}
	if fileExists(filepath.Join(dir, "nope.txt")) {
		t.Error("expected false for non-existing file")
	}
	if fileExists(dir) {
		t.Error("expected false for directory")
	}
}

// setupFakeCodex creates a fake codex binary script that writes to the output file.
func setupFakeCodex(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	codexPath := filepath.Join(dir, "codex")
	os.WriteFile(codexPath, []byte("#!/bin/bash\n"+script), 0755)
	t.Setenv("PATH", dir)
	return dir
}

func TestCodexExecSuccess(t *testing.T) {
	artifactsDir := t.TempDir()
	outputFile := filepath.Join(artifactsDir, "output.md")

	// Fake codex that writes review content to --output-last-message file
	setupFakeCodex(t, `
for i in "$@"; do
    if [ "$prev" = "--output-last-message" ]; then
        echo "[HIGH-1] Test finding" > "$i"
        break
    fi
    prev="$i"
done
exit 0
`)

	cfg := config.Config{CodexModel: "test-model", CallTimeoutSec: 10}
	result, err := CodexExec(cfg, "test prompt", outputFile, []string{"--sandbox=read-only"}, artifactsDir)
	if err != nil {
		t.Fatalf("CodexExec: %v", err)
	}
	if result.LogPath != "" {
		t.Errorf("log should be cleaned up on success, got %q", result.LogPath)
	}

	data, _ := os.ReadFile(outputFile)
	if len(data) == 0 {
		t.Error("output file should have content")
	}
}

func TestCodexExecFailure(t *testing.T) {
	artifactsDir := t.TempDir()
	outputFile := filepath.Join(artifactsDir, "output.md")

	setupFakeCodex(t, `echo "error: connection failed" >&2; exit 1`)

	cfg := config.Config{CodexModel: "test-model", CallTimeoutSec: 10}
	result, err := CodexExec(cfg, "test prompt", outputFile, nil, artifactsDir)
	if err == nil {
		t.Fatal("expected error")
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
	if result.LogPath == "" {
		t.Error("log should be preserved on failure")
	}
}

func TestRunCodexReviewRetryOnMalformed(t *testing.T) {
	artifactsDir := t.TempDir()
	reviewFile := filepath.Join(artifactsDir, "review.md")

	// Fake codex that writes malformed output first, valid output second
	counterFile := filepath.Join(artifactsDir, "counter")
	os.WriteFile(counterFile, []byte("0"), 0644)

	setupFakeCodex(t, `
COUNTER_FILE="`+counterFile+`"
COUNT=$(cat "$COUNTER_FILE")
for i in "$@"; do
    if [ "$prev" = "--output-last-message" ]; then
        OUTPUT_FILE="$i"
        break
    fi
    prev="$i"
done
if [ "$COUNT" = "0" ]; then
    echo "1" > "$COUNTER_FILE"
    echo "Just some text without findings" > "$OUTPUT_FILE"
else
    echo "[HIGH-1] Real finding" > "$OUTPUT_FILE"
fi
exit 0
`)

	validator := func(content string) bool {
		return len(content) > 0 && (content[0] == '[' || false) // simple check
	}

	cfg := config.Config{CodexModel: "test-model", CallTimeoutSec: 10}
	result, err := RunCodexReview(cfg, "review prompt", "\nStrict!", reviewFile, nil, artifactsDir, validator)
	if err != nil {
		t.Fatalf("RunCodexReview: %v", err)
	}
	_ = result

	data, _ := os.ReadFile(reviewFile)
	if string(data) != "[HIGH-1] Real finding\n" {
		t.Errorf("review file = %q, want valid finding", string(data))
	}
}

func TestRunCodexReviewExecFailureNoRetry(t *testing.T) {
	artifactsDir := t.TempDir()
	reviewFile := filepath.Join(artifactsDir, "review.md")

	setupFakeCodex(t, `exit 1`)

	cfg := config.Config{CodexModel: "test-model", CallTimeoutSec: 10}
	_, err := RunCodexReview(cfg, "prompt", "\nStrict!", reviewFile, nil, artifactsDir, nil)
	if err == nil {
		t.Fatal("expected error for exec failure")
	}
}

func TestCodexExecRetriesTrustedDirectoryErrorWithSkipGitFlag(t *testing.T) {
	artifactsDir := t.TempDir()
	outputFile := filepath.Join(artifactsDir, "output.md")
	counterFile := filepath.Join(artifactsDir, "count")
	if err := os.WriteFile(counterFile, []byte("0"), 0644); err != nil {
		t.Fatalf("write counter: %v", err)
	}

	setupFakeCodex(t, `
COUNTER_FILE="`+counterFile+`"
COUNT=$(cat "$COUNTER_FILE")
HAS_SKIP=0
for i in "$@"; do
    if [ "$i" = "--skip-git-repo-check" ]; then
        HAS_SKIP=1
    fi
    if [ "$prev" = "--output-last-message" ]; then
        OUTPUT_FILE="$i"
    fi
    prev="$i"
done

if [ "$HAS_SKIP" = "1" ]; then
    echo "[HIGH-1] Trusted dir bypassed" > "$OUTPUT_FILE"
    echo "2" > "$COUNTER_FILE"
    exit 0
fi

echo "Not inside a trusted directory and --skip-git-repo-check was not specified." >&2
echo "1" > "$COUNTER_FILE"
exit 1
`)

	cfg := config.Config{CodexModel: "test-model", CallTimeoutSec: 10}
	result, err := CodexExec(cfg, "test prompt", outputFile, []string{"--sandbox=read-only"}, artifactsDir)
	if err != nil {
		t.Fatalf("CodexExec: %v", err)
	}
	if result.LogPath != "" {
		t.Errorf("log should be cleaned up on success, got %q", result.LogPath)
	}
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != "[HIGH-1] Trusted dir bypassed\n" {
		t.Fatalf("output = %q", string(data))
	}
	count, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("read count: %v", err)
	}
	if string(count) != "2\n" {
		t.Fatalf("expected retry to run, count=%q", string(count))
	}
}
