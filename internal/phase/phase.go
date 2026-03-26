// Package phase provides slugification and artifact file path helpers.
package phase

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var slugRe = regexp.MustCompile(`[^a-z0-9-]+`)
var multiDash = regexp.MustCompile(`-{2,}`)

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
