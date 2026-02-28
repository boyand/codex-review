// Package config parses environment variables for the codex-review-loop engine.
package config

import (
	"os"
	"strconv"
)

// Config holds all environment-driven settings.
type Config struct {
	CodexModel      string
	CodexReviewFlags string
	CodexWorkerFlags string
	CallTimeoutSec  int
	PluginRoot      string
}

// Load reads configuration from environment variables with defaults.
func Load() Config {
	return Config{
		CodexModel:       envOr("CODEX_REVIEW_LOOP_MODEL", "gpt-5.2-codex"),
		CodexReviewFlags: envOr("CODEX_REVIEW_LOOP_FLAGS", "--sandbox=read-only"),
		CodexWorkerFlags: envOr("CODEX_WORKER_FLAGS", "--sandbox=workspace-write"),
		CallTimeoutSec:   envIntOr("CODEX_CALL_TIMEOUT_SEC", 120),
		PluginRoot:       os.Getenv("CLAUDE_PLUGIN_ROOT"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
