package phase

import "testing"

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

func TestFilePaths(t *testing.T) {
	dir := ".claude/codex-review"
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
