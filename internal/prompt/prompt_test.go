package prompt

import (
	"strings"
	"testing"
)

func TestFenceMarkdown(t *testing.T) {
	got := FenceMarkdown("# Hello")
	want := "```markdown\n# Hello\n```"
	if got != want {
		t.Errorf("FenceMarkdown = %q, want %q", got, want)
	}
}

func TestBuildReviewPromptPlan(t *testing.T) {
	prompt := BuildReviewPrompt("plan", "Build auth", "# My Plan", "", "")
	if !strings.Contains(prompt, "Build auth") {
		t.Error("missing task description")
	}
	if !strings.Contains(prompt, "```markdown") {
		t.Error("missing fenced content")
	}
	if !strings.Contains(prompt, "# My Plan") {
		t.Error("missing plan content")
	}
}

func TestBuildReviewPromptImplement(t *testing.T) {
	prompt := BuildReviewPrompt("implement", "Build auth", "", "# Approved Plan", "")
	if !strings.Contains(prompt, "Build auth") {
		t.Error("missing task description")
	}
	if !strings.Contains(prompt, "# Approved Plan") {
		t.Error("missing compare-to content")
	}
}

func TestBuildReviewPromptCustomWithPrompt(t *testing.T) {
	prompt := BuildReviewPrompt("security-audit", "Build auth", "phase output", "ref content", "Focus on OWASP")
	if !strings.Contains(prompt, "security-audit") {
		t.Error("missing phase name")
	}
	if !strings.Contains(prompt, "Focus on OWASP") {
		t.Error("missing custom prompt")
	}
	if !strings.Contains(prompt, "phase output") {
		t.Error("missing phase output")
	}
	if !strings.Contains(prompt, "ref content") {
		t.Error("missing reference content")
	}
}

func TestBuildReviewPromptCustomNoPrompt(t *testing.T) {
	prompt := BuildReviewPrompt("test", "Build auth", "", "", "")
	if !strings.Contains(prompt, "test") {
		t.Error("missing phase name")
	}
	if !strings.Contains(prompt, "[CRITICAL-N]") {
		t.Error("missing severity tag instructions")
	}
}

func TestBuildWorkPromptPlan(t *testing.T) {
	prompt := BuildWorkPrompt("plan", "Build auth", "", "")
	if !strings.Contains(prompt, "implementation plan") {
		t.Error("missing plan instructions")
	}
}

func TestBuildWorkPromptImplement(t *testing.T) {
	prompt := BuildWorkPrompt("implement", "Build auth", "# Plan Content", "")
	if !strings.Contains(prompt, "implementing code") {
		t.Error("missing implement instructions")
	}
	if !strings.Contains(prompt, "# Plan Content") {
		t.Error("missing plan reference")
	}
}

func TestBuildWorkPromptCustom(t *testing.T) {
	prompt := BuildWorkPrompt("test", "Build auth", "ref", "Write unit tests")
	if !strings.Contains(prompt, "Write unit tests") {
		t.Error("missing work prompt")
	}
	if !strings.Contains(prompt, "ref") {
		t.Error("missing reference")
	}
}

func TestStrictSuffix(t *testing.T) {
	if !strings.Contains(StrictSuffix, "Strict Output Requirements") {
		t.Error("StrictSuffix missing expected content")
	}
}

func TestTemplatesEmbedded(t *testing.T) {
	if planReviewTemplate == "" {
		t.Error("plan review template not embedded")
	}
	if implementReviewTemplate == "" {
		t.Error("implement review template not embedded")
	}
	if !strings.Contains(planReviewTemplate, "{{TASK_DESCRIPTION}}") {
		t.Error("plan template missing placeholder")
	}
	if !strings.Contains(implementReviewTemplate, "{{PLAN_CONTENT}}") {
		t.Error("implement template missing placeholder")
	}
}
