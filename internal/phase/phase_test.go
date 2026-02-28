package phase

import "testing"

func TestNormalizeAgent(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"", ""},
		{"claude", "claude"},
		{"Claude", "claude"},
		{"CLAUDE", "claude"},
		{"claude-code", "claude"},
		{"anthropic", "claude"},
		{"anthropic-claude", "claude"},
		{"codex", "codex"},
		{"Codex", "codex"},
		{"CODEX", "codex"},
		{"openai-codex", "codex"},
		{"codex-cli", "codex"},
		{"openai", "codex"},
		{"custom-agent", "custom-agent"},
	}
	for _, tt := range tests {
		got := NormalizeAgent(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeAgent(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWorkerDefault(t *testing.T) {
	if got := WorkerDefault(""); got != "claude" {
		t.Errorf("WorkerDefault(\"\") = %q, want %q", got, "claude")
	}
	if got := WorkerDefault("codex"); got != "codex" {
		t.Errorf("WorkerDefault(\"codex\") = %q, want %q", got, "codex")
	}
}

func TestReviewerDefault(t *testing.T) {
	if got := ReviewerDefault(""); got != "codex" {
		t.Errorf("ReviewerDefault(\"\") = %q, want %q", got, "codex")
	}
	if got := ReviewerDefault("claude"); got != "claude" {
		t.Errorf("ReviewerDefault(\"claude\") = %q, want %q", got, "claude")
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"plan", "plan"},
		{"Security Audit", "security-audit"},
		{"Test Coverage!!!", "test-coverage"},
		{"---dashes---", "dashes"},
		{"UPPER CASE", "upper-case"},
		{"a  b  c", "a-b-c"},
		{"", "phase"},
		{"  ", "phase"},
	}
	for _, tt := range tests {
		got := Slugify(tt.input)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestArtifactKey(t *testing.T) {
	tests := []struct {
		name, artifact, want string
	}{
		{"plan", "", "plan"},
		{"implement", "", "implement"},
		{"Security Audit", "", "security-audit"},
		{"plan", "custom-key", "custom-key"},
		{"anything", "Already Set", "already-set"},
	}
	for _, tt := range tests {
		got := ArtifactKey(tt.name, tt.artifact)
		if got != tt.want {
			t.Errorf("ArtifactKey(%q, %q) = %q, want %q", tt.name, tt.artifact, got, tt.want)
		}
	}
}

func TestFilePaths(t *testing.T) {
	dir := ".claude/codex-review-loop"
	if got := OutputFile(dir, "plan"); got != dir+"/plan.md" {
		t.Errorf("OutputFile = %q", got)
	}
	if got := ReviewFile(dir, "plan", 1); got != dir+"/plan-review-r1.md" {
		t.Errorf("ReviewFile = %q", got)
	}
	if got := FindingsFile(dir, "plan", 2); got != dir+"/plan-findings-r2.md" {
		t.Errorf("FindingsFile = %q", got)
	}
}
