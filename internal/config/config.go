// Package config parses environment variables for the codex-review engine.
package config

import (
	"os"
	"strconv"
)

// Config holds all environment-driven settings.
type Config struct {
	CodexModel       string
	CodexReviewFlags string
	CodexWorkerFlags string
	CallTimeoutSec   int
	PluginRoot       string
}

// Load reads configuration from environment variables with defaults.
func Load() Config {
	return Config{
		CodexModel:       envOrAny([]string{"CODEX_REVIEW_MODEL", "CODEX_REVIEW_LOOP_MODEL"}, "gpt-5.3-codex"),
		CodexReviewFlags: envOrAny([]string{"CODEX_REVIEW_FLAGS", "CODEX_REVIEW_LOOP_FLAGS"}, "--sandbox=read-only"),
		CodexWorkerFlags: envOr("CODEX_WORKER_FLAGS", "--sandbox=workspace-write"),
		CallTimeoutSec:   envIntOr("CODEX_CALL_TIMEOUT_SEC", 720),
		PluginRoot:       os.Getenv("CLAUDE_PLUGIN_ROOT"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrAny(keys []string, fallback string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
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
