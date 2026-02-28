package state

import (
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	input := `---
active: true
review_id: abc-123
current_phase_index: 0
task_description: Build JWT auth
---

# Body content here
`
	fm := ParseFrontmatter(input)

	tests := []struct {
		key, want string
	}{
		{"active", "true"},
		{"review_id", "abc-123"},
		{"current_phase_index", "0"},
		{"task_description", "Build JWT auth"},
	}
	for _, tt := range tests {
		got, ok := fm.Fields.Get(tt.key)
		if !ok {
			t.Errorf("missing key %q", tt.key)
			continue
		}
		if got != tt.want {
			t.Errorf("Fields[%q] = %q, want %q", tt.key, got, tt.want)
		}
	}

	if fm.Body == "" {
		t.Error("Body should not be empty")
	}
}

func TestParseFrontmatterNoDelimiters(t *testing.T) {
	input := "Just body content"
	fm := ParseFrontmatter(input)

	if fm.Fields.Len() != 0 {
		t.Errorf("expected 0 fields, got %d", fm.Fields.Len())
	}
	if fm.Body != input {
		t.Errorf("Body = %q, want %q", fm.Body, input)
	}
}

func TestParseFrontmatterNoClosing(t *testing.T) {
	input := "---\nkey: value\nno closing"
	fm := ParseFrontmatter(input)

	if fm.Fields.Len() != 0 {
		t.Errorf("expected 0 fields, got %d", fm.Fields.Len())
	}
	if fm.Body != input {
		t.Errorf("Body should be entire input")
	}
}

func TestRoundTrip(t *testing.T) {
	input := `---
active: true
review_id: test-123
current_phase_index: 0
current_substep: working
current_round: 1
max_rounds: 10
pipeline_count: 2
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_worker: claude
pipeline_0_reviewer: codex
pipeline_0_artifact: plan
pipeline_1_name: implement
pipeline_1_status: pending
pipeline_1_worker: claude
pipeline_1_reviewer: codex
pipeline_1_artifact: implement
task_description: Build JWT auth
unknown_future_field: preserved
---
`
	fm := ParseFrontmatter(input)

	// Modify a field
	fm.Fields.Set("current_substep", "review")

	rendered := fm.Render()
	fm2 := ParseFrontmatter(rendered)

	// Verify modified field
	v, _ := fm2.Fields.Get("current_substep")
	if v != "review" {
		t.Errorf("current_substep = %q, want %q", v, "review")
	}

	// Verify unknown field preserved
	v, ok := fm2.Fields.Get("unknown_future_field")
	if !ok || v != "preserved" {
		t.Errorf("unknown_future_field = %q (ok=%v), want %q", v, ok, "preserved")
	}

	// Verify key order preserved
	keys := fm2.Fields.Keys()
	wantKeys := fm.Fields.Keys()
	if len(keys) != len(wantKeys) {
		t.Fatalf("key count %d != %d", len(keys), len(wantKeys))
	}
	for i, k := range keys {
		if k != wantKeys[i] {
			t.Errorf("key[%d] = %q, want %q", i, k, wantKeys[i])
		}
	}
}

func TestOrderedMap(t *testing.T) {
	m := NewOrderedMap()
	m.Set("b", "2")
	m.Set("a", "1")
	m.Set("c", "3")

	keys := m.Keys()
	want := []string{"b", "a", "c"}
	if len(keys) != len(want) {
		t.Fatalf("len = %d, want %d", len(keys), len(want))
	}
	for i, k := range keys {
		if k != want[i] {
			t.Errorf("key[%d] = %q, want %q", i, k, want[i])
		}
	}

	// Update existing key should not change order
	m.Set("a", "updated")
	keys = m.Keys()
	for i, k := range keys {
		if k != want[i] {
			t.Errorf("after update key[%d] = %q, want %q", i, k, want[i])
		}
	}
	v, _ := m.Get("a")
	if v != "updated" {
		t.Errorf("a = %q, want %q", v, "updated")
	}

	// Delete
	m.Delete("a")
	if m.Len() != 2 {
		t.Errorf("len after delete = %d, want 2", m.Len())
	}
	keys = m.Keys()
	if keys[0] != "b" || keys[1] != "c" {
		t.Errorf("keys after delete = %v, want [b c]", keys)
	}
}

func TestParseFrontmatterValuesWithColons(t *testing.T) {
	input := `---
task_description: Build JWT auth: implement refresh tokens
pipeline_0_custom_prompt: Focus on: security, validation
---
`
	fm := ParseFrontmatter(input)

	desc, ok := fm.Fields.Get("task_description")
	if !ok || desc != "Build JWT auth: implement refresh tokens" {
		t.Errorf("task_description = %q, want %q", desc, "Build JWT auth: implement refresh tokens")
	}

	prompt, ok := fm.Fields.Get("pipeline_0_custom_prompt")
	if !ok || prompt != "Focus on: security, validation" {
		t.Errorf("custom_prompt = %q, want %q", prompt, "Focus on: security, validation")
	}
}

func TestRenderFrontmatterOnly(t *testing.T) {
	fm := &Frontmatter{Fields: NewOrderedMap(), Body: "some body"}
	fm.Fields.Set("key", "value")

	got := fm.RenderFrontmatterOnly()
	want := "---\nkey: value\n---\n"
	if got != want {
		t.Errorf("RenderFrontmatterOnly = %q, want %q", got, want)
	}
}
