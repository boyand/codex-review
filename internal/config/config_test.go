package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Clear env vars to test defaults
	for _, k := range []string{"CODEX_REVIEW_MODEL", "CODEX_REVIEW_FLAGS", "CODEX_REVIEW_LOOP_MODEL", "CODEX_REVIEW_LOOP_FLAGS", "CODEX_WORKER_FLAGS", "CODEX_CALL_TIMEOUT_SEC", "CLAUDE_PLUGIN_ROOT"} {
		os.Unsetenv(k)
	}

	c := Load()
	if c.CodexModel != "gpt-5.3-codex" {
		t.Errorf("CodexModel = %q, want %q", c.CodexModel, "gpt-5.3-codex")
	}
	if c.CodexReviewFlags != "--sandbox=read-only" {
		t.Errorf("CodexReviewFlags = %q, want %q", c.CodexReviewFlags, "--sandbox=read-only")
	}
	if c.CodexWorkerFlags != "--sandbox=workspace-write" {
		t.Errorf("CodexWorkerFlags = %q, want %q", c.CodexWorkerFlags, "--sandbox=workspace-write")
	}
	if c.CallTimeoutSec != 720 {
		t.Errorf("CallTimeoutSec = %d, want %d", c.CallTimeoutSec, 720)
	}
	if c.PluginRoot != "" {
		t.Errorf("PluginRoot = %q, want empty", c.PluginRoot)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("CODEX_REVIEW_MODEL", "test-model")
	t.Setenv("CODEX_CALL_TIMEOUT_SEC", "60")
	t.Setenv("CLAUDE_PLUGIN_ROOT", "/tmp/plugin")

	c := Load()
	if c.CodexModel != "test-model" {
		t.Errorf("CodexModel = %q, want %q", c.CodexModel, "test-model")
	}
	if c.CallTimeoutSec != 60 {
		t.Errorf("CallTimeoutSec = %d, want %d", c.CallTimeoutSec, 60)
	}
	if c.PluginRoot != "/tmp/plugin" {
		t.Errorf("PluginRoot = %q, want %q", c.PluginRoot, "/tmp/plugin")
	}
}

func TestLoadLegacyEnvFallback(t *testing.T) {
	t.Setenv("CODEX_REVIEW_MODEL", "")
	t.Setenv("CODEX_REVIEW_FLAGS", "")
	t.Setenv("CODEX_REVIEW_LOOP_MODEL", "legacy-model")
	t.Setenv("CODEX_REVIEW_LOOP_FLAGS", "--sandbox=legacy")

	c := Load()
	if c.CodexModel != "legacy-model" {
		t.Errorf("CodexModel = %q, want %q", c.CodexModel, "legacy-model")
	}
	if c.CodexReviewFlags != "--sandbox=legacy" {
		t.Errorf("CodexReviewFlags = %q, want %q", c.CodexReviewFlags, "--sandbox=legacy")
	}
}

func TestLoadBadTimeout(t *testing.T) {
	t.Setenv("CODEX_CALL_TIMEOUT_SEC", "notanumber")
	c := Load()
	if c.CallTimeoutSec != 720 {
		t.Errorf("CallTimeoutSec = %d, want 720 for invalid input", c.CallTimeoutSec)
	}
}
