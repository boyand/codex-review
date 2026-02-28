// Package phase provides phase metadata helpers: agent normalization,
// slugification, artifact key resolution, and file path builders.
package phase

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var slugRe = regexp.MustCompile(`[^a-z0-9-]+`)
var multiDash = regexp.MustCompile(`-{2,}`)

// NormalizeAgent maps worker/reviewer aliases to canonical names.
// Returns "" for empty input (caller applies default).
func NormalizeAgent(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "":
		return ""
	case "claude", "claude-code", "anthropic", "anthropic-claude":
		return "claude"
	case "codex", "openai-codex", "codex-cli", "openai":
		return "codex"
	default:
		return s
	}
}

// WorkerDefault returns the normalized worker, defaulting to "claude".
func WorkerDefault(raw string) string {
	n := NormalizeAgent(raw)
	if n == "" {
		return "claude"
	}
	return n
}

// ReviewerDefault returns the normalized reviewer, defaulting to "codex".
func ReviewerDefault(raw string) string {
	n := NormalizeAgent(raw)
	if n == "" {
		return "codex"
	}
	return n
}

// Slugify converts a name to a safe artifact key (lowercase letters, numbers, dashes).
func Slugify(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = slugRe.ReplaceAllString(s, "-")
	s = multiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "phase"
	}
	return s
}

// ArtifactKey resolves the artifact key for a phase. If artifact is already set,
// it normalizes it. Otherwise uses the phase name (plan/implement stay as-is,
// others get slugified).
func ArtifactKey(phaseName, existingArtifact string) string {
	if existingArtifact != "" {
		return Slugify(existingArtifact)
	}
	switch phaseName {
	case "plan", "implement":
		return phaseName
	default:
		return Slugify(phaseName)
	}
}

// OutputFile returns the path for a phase's output artifact.
func OutputFile(artifactsDir, artifactKey string) string {
	return filepath.Join(artifactsDir, artifactKey+".md")
}

// ReviewFile returns the path for a phase review at a given round.
func ReviewFile(artifactsDir, artifactKey string, round int) string {
	return filepath.Join(artifactsDir, fmt.Sprintf("%s-review-r%d.md", artifactKey, round))
}

// FindingsFile returns the path for a phase findings response at a given round.
func FindingsFile(artifactsDir, artifactKey string, round int) string {
	return filepath.Join(artifactsDir, fmt.Sprintf("%s-findings-r%d.md", artifactKey, round))
}
