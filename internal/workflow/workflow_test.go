package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNewSaveLoadResolve(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	s, paths, err := New("review plan", "/tmp/plan.md")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := EnsureDirs(paths); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	if err := s.Save(paths.WorkflowFile); err != nil {
		t.Fatalf("Save: %v", err)
	}

	gotPaths, got, err := Resolve(s.ID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got == nil {
		t.Fatal("expected workflow")
	}
	if gotPaths.ID != s.ID {
		t.Fatalf("resolved id=%q want %q", gotPaths.ID, s.ID)
	}
	if got.Phase != "plan" {
		t.Fatalf("phase=%q", got.Phase)
	}
}

func TestDiscoverActiveIDsAndResolveConflict(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	for i := 0; i < 2; i++ {
		s, paths, err := New("review plan", "")
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if err := EnsureDirs(paths); err != nil {
			t.Fatalf("EnsureDirs: %v", err)
		}
		if err := s.Save(paths.WorkflowFile); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	ids, err := DiscoverActiveIDs()
	if err != nil {
		t.Fatalf("DiscoverActiveIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("ids=%v", ids)
	}

	_, _, err = Resolve("")
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(err.Error(), "multiple active workflows") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolvePrefersCurrentSessionOwner(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCurrentProcessSession(t, home, dir, "sess-a")

	first, firstPaths, err := New("review plan a", "")
	if err != nil {
		t.Fatalf("New first: %v", err)
	}
	BindOwner(first, &ClaudeSession{SessionID: "sess-a", CWD: dir, ProjectKey: encodeClaudeProjectKey(dir)})
	if err := EnsureDirs(firstPaths); err != nil {
		t.Fatalf("EnsureDirs first: %v", err)
	}
	if err := first.Save(firstPaths.WorkflowFile); err != nil {
		t.Fatalf("Save first: %v", err)
	}

	second, secondPaths, err := New("review plan b", "")
	if err != nil {
		t.Fatalf("New second: %v", err)
	}
	BindOwner(second, &ClaudeSession{SessionID: "sess-b", CWD: dir, ProjectKey: encodeClaudeProjectKey(dir)})
	if err := EnsureDirs(secondPaths); err != nil {
		t.Fatalf("EnsureDirs second: %v", err)
	}
	if err := second.Save(secondPaths.WorkflowFile); err != nil {
		t.Fatalf("Save second: %v", err)
	}

	gotPaths, got, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got == nil {
		t.Fatal("expected workflow")
	}
	if got.ID != first.ID || gotPaths.ID != first.ID {
		t.Fatalf("resolved workflow=%q want %q", got.ID, first.ID)
	}
}

func TestResolveClaimsSingleUnownedWorkflowForCurrentSession(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	home := t.TempDir()
	t.Setenv("HOME", home)
	writeCurrentProcessSession(t, home, dir, "sess-a")

	state, paths, err := New("review plan", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := EnsureDirs(paths); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	if err := state.Save(paths.WorkflowFile); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, got, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got == nil {
		t.Fatal("expected workflow")
	}
	if got.OwnerSessionID != "sess-a" {
		t.Fatalf("OwnerSessionID=%q want sess-a", got.OwnerSessionID)
	}
	if got.OwnerProjectKey != encodeClaudeProjectKey(dir) {
		t.Fatalf("OwnerProjectKey=%q want %q", got.OwnerProjectKey, encodeClaudeProjectKey(dir))
	}
}

func TestDeriveStep(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	s, paths, err := New("review plan", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := EnsureDirs(paths); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	if err := s.Save(paths.WorkflowFile); err != nil {
		t.Fatalf("Save: %v", err)
	}

	step, err := DeriveStep(paths, s)
	if err != nil {
		t.Fatalf("DeriveStep: %v", err)
	}
	if step != "working" {
		t.Fatalf("step=%q want working", step)
	}

	if err := os.WriteFile(ReviewFile(paths, s), []byte("[HIGH-1] Test finding"), 0644); err != nil {
		t.Fatalf("write review: %v", err)
	}
	step, err = DeriveStep(paths, s)
	if err != nil {
		t.Fatalf("DeriveStep: %v", err)
	}
	if step != "review" {
		t.Fatalf("step=%q want review", step)
	}

	if err := os.WriteFile(FindingsFile(paths, s), []byte("# Responses\n\nHIGH-1\nDecision: FIX\n"), 0644); err != nil {
		t.Fatalf("write findings: %v", err)
	}
	step, err = DeriveStep(paths, s)
	if err != nil {
		t.Fatalf("DeriveStep: %v", err)
	}
	if step != "approval" {
		t.Fatalf("step=%q want approval", step)
	}
}

func TestResolveCurrentClaudePlanPrefersStructuredSessionSignals(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	plansDir := filepath.Join(home, ".claude", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("mkdir plans: %v", err)
	}
	sessionsDir := filepath.Join(home, ".claude", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	projectDir := filepath.Join(home, ".claude", "projects", encodeClaudeProjectKey("/repo"))
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}

	writePlan := func(name string, modTime time.Time) string {
		t.Helper()
		path := filepath.Join(plansDir, name)
		if err := os.WriteFile(path, []byte("# Plan\n"), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("chtimes %s: %v", name, err)
		}
		return path
	}

	old := time.Now().Add(-2 * time.Hour)
	newest := time.Now()
	expected := writePlan("current.md", old)
	writePlan("wrong.md", newest)
	writePlan("current-agent-a123.md", newest)

	sessionFile := filepath.Join(sessionsDir, "200.json")
	if err := os.WriteFile(sessionFile, []byte(`{"pid":200,"sessionId":"sess-1","cwd":"/repo","startedAt":1}`), 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	transcript := filepath.Join(projectDir, "sess-1.jsonl")
	content := strings.Join([]string{
		`{"message":{"content":"please do not use ~/.claude/plans/wrong.md"},"timestamp":"2026-03-22T10:00:00Z"}`,
		fmt.Sprintf(`{"message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"%s"}}]},"timestamp":"2026-03-22T10:05:00Z"}`, expected),
		fmt.Sprintf(`{"toolUseResult":{"type":"create","filePath":"%s"},"timestamp":"2026-03-22T10:05:01Z"}`, expected),
		`{"message":{"content":"review ~/.claude/plans/wrong.md next"},"timestamp":"2026-03-22T10:10:00Z"}`,
	}, "\n")
	if err := os.WriteFile(transcript, []byte(content), 0644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	got, err := resolveCurrentClaudePlan(home, runtimeSessionHints{}, []int{100, 200})
	if err != nil {
		t.Fatalf("resolveCurrentClaudePlan: %v", err)
	}
	if got != expected {
		t.Fatalf("got %q want %q", got, expected)
	}
}

func TestResolveCurrentClaudePlanFallsBackToTextMentionsWhenNeeded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	plansDir := filepath.Join(home, ".claude", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("mkdir plans: %v", err)
	}
	sessionsDir := filepath.Join(home, ".claude", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	projectDir := filepath.Join(home, ".claude", "projects", encodeClaudeProjectKey("/repo"))
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}

	expected := filepath.Join(plansDir, "current.md")
	if err := os.WriteFile(expected, []byte("# Plan\n"), 0644); err != nil {
		t.Fatalf("write expected plan: %v", err)
	}
	agentPlan := filepath.Join(plansDir, "current-agent-a123.md")
	if err := os.WriteFile(agentPlan, []byte("# Agent Plan\n"), 0644); err != nil {
		t.Fatalf("write agent plan: %v", err)
	}

	sessionFile := filepath.Join(sessionsDir, "200.json")
	if err := os.WriteFile(sessionFile, []byte(`{"pid":200,"sessionId":"sess-1","cwd":"/repo","startedAt":1}`), 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	transcript := filepath.Join(projectDir, "sess-1.jsonl")
	content := strings.Join([]string{
		`{"message":{"content":"latest plan is ~/.claude/plans/current.md"}}`,
		`{"message":{"content":"ignore ~/.claude/plans/current-agent-a123.md"}}`,
	}, "\n")
	if err := os.WriteFile(transcript, []byte(content), 0644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	got, err := resolveCurrentClaudePlan(home, runtimeSessionHints{}, []int{100, 200})
	if err != nil {
		t.Fatalf("resolveCurrentClaudePlan: %v", err)
	}
	if got != expected {
		t.Fatalf("got %q want %q", got, expected)
	}
}

func TestResolveCurrentClaudePlanFallsBackToLatestNonAgent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	plansDir := filepath.Join(home, ".claude", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("mkdir plans: %v", err)
	}

	writePlan := func(name string, modTime time.Time) string {
		t.Helper()
		path := filepath.Join(plansDir, name)
		if err := os.WriteFile(path, []byte("# Plan\n"), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("chtimes %s: %v", name, err)
		}
		return path
	}

	old := time.Now().Add(-2 * time.Hour)
	newest := time.Now()
	writePlan("older.md", old)
	expected := writePlan("current.md", newest)
	writePlan("current-agent-a123.md", newest.Add(time.Minute))

	got, err := resolveCurrentClaudePlan(home, runtimeSessionHints{}, []int{100})
	if err != nil {
		t.Fatalf("resolveCurrentClaudePlan: %v", err)
	}
	if got != expected {
		t.Fatalf("got %q want %q", got, expected)
	}
}

func writeCurrentProcessSession(t *testing.T, home, cwd, sessionID string) {
	t.Helper()
	sessionsDir := filepath.Join(home, ".claude", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	path := filepath.Join(sessionsDir, strconv.Itoa(os.Getpid())+".json")
	body := fmt.Sprintf(`{"pid":%d,"sessionId":"%s","cwd":"%s","startedAt":1}`, os.Getpid(), sessionID, cwd)
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
}

func TestResolveCurrentClaudeSessionPrefersSessionIDHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sessionsDir := filepath.Join(home, ".claude", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}

	if err := os.WriteFile(filepath.Join(sessionsDir, "21110.json"), []byte(`{"pid":21110,"sessionId":"sess-hint","cwd":"/repo-a","startedAt":10}`), 0644); err != nil {
		t.Fatalf("write hinted session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, "21111.json"), []byte(`{"pid":21111,"sessionId":"sess-other","cwd":"/repo-b","startedAt":20}`), 0644); err != nil {
		t.Fatalf("write other session: %v", err)
	}

	session, err := resolveCurrentClaudeSession(home, runtimeSessionHints{
		SessionID:  "sess-hint",
		SessionPID: 21110,
	}, nil)
	if err != nil {
		t.Fatalf("resolveCurrentClaudeSession: %v", err)
	}
	if session == nil {
		t.Fatal("expected session")
	}
	if session.SessionID != "sess-hint" {
		t.Fatalf("SessionID=%q want sess-hint", session.SessionID)
	}
	if session.CWD != "/repo-a" {
		t.Fatalf("CWD=%q want /repo-a", session.CWD)
	}
}

func TestResolveCurrentClaudePlanUsesSessionPIDHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	plansDir := filepath.Join(home, ".claude", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("mkdir plans: %v", err)
	}
	sessionsDir := filepath.Join(home, ".claude", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	projectDir := filepath.Join(home, ".claude", "projects", encodeClaudeProjectKey("/repo-hint"))
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}

	expected := filepath.Join(plansDir, "hinted.md")
	if err := os.WriteFile(expected, []byte("# Hinted plan\n"), 0644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, "21110.json"), []byte(`{"pid":21110,"sessionId":"sess-hint","cwd":"/repo-hint","startedAt":10}`), 0644); err != nil {
		t.Fatalf("write hinted session: %v", err)
	}
	transcript := filepath.Join(projectDir, "sess-hint.jsonl")
	content := fmt.Sprintf(`{"message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"%s"}}]},"timestamp":"2026-03-23T10:05:00Z"}`, expected)
	if err := os.WriteFile(transcript, []byte(content), 0644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	got, err := resolveCurrentClaudePlan(home, runtimeSessionHints{
		SessionID:  "sess-hint",
		SessionPID: 21110,
	}, nil)
	if err != nil {
		t.Fatalf("resolveCurrentClaudePlan: %v", err)
	}
	if got != expected {
		t.Fatalf("got %q want %q", got, expected)
	}
}

func TestResolveCurrentClaudeSessionFailsClosedOnMismatchedExplicitHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sessionsDir := filepath.Join(home, ".claude", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}

	if err := os.WriteFile(filepath.Join(sessionsDir, "21110.json"), []byte(`{"pid":21110,"sessionId":"sess-a","cwd":"/repo-a","startedAt":10}`), 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, "21111.json"), []byte(`{"pid":21111,"sessionId":"sess-b","cwd":"/repo-b","startedAt":20}`), 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}

	session, err := resolveCurrentClaudeSession(home, runtimeSessionHints{
		SessionID:  "sess-b",
		SessionPID: 21110,
	}, []int{21111})
	if session != nil {
		t.Fatalf("expected nil session on mismatched explicit hint, got %+v", session)
	}
	if !errors.Is(err, ErrExplicitRuntimeSessionHints) {
		t.Fatalf("resolveCurrentClaudeSession error=%v want %v", err, ErrExplicitRuntimeSessionHints)
	}
}

func TestResolveCurrentClaudePlanFailsClosedOnMismatchedExplicitHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	plansDir := filepath.Join(home, ".claude", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("mkdir plans: %v", err)
	}
	sessionsDir := filepath.Join(home, ".claude", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}

	globalLatest := filepath.Join(plansDir, "latest.md")
	if err := os.WriteFile(globalLatest, []byte("# latest\n"), 0644); err != nil {
		t.Fatalf("write latest plan: %v", err)
	}

	if err := os.WriteFile(filepath.Join(sessionsDir, "21110.json"), []byte(`{"pid":21110,"sessionId":"sess-a","cwd":"/repo-a","startedAt":10}`), 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, "21111.json"), []byte(`{"pid":21111,"sessionId":"sess-b","cwd":"/repo-b","startedAt":20}`), 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}

	got, err := resolveCurrentClaudePlan(home, runtimeSessionHints{
		SessionID:  "sess-b",
		SessionPID: 21110,
	}, []int{21111})
	if got != "" {
		t.Fatalf("expected empty plan on mismatched explicit hint, got %q", got)
	}
	if !errors.Is(err, ErrExplicitRuntimeSessionHints) {
		t.Fatalf("resolveCurrentClaudePlan error=%v want %v", err, ErrExplicitRuntimeSessionHints)
	}
}

func TestResolveCurrentClaudePlanFailsClosedWhenHintedPIDMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	plansDir := filepath.Join(home, ".claude", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("mkdir plans: %v", err)
	}

	globalLatest := filepath.Join(plansDir, "latest.md")
	if err := os.WriteFile(globalLatest, []byte("# latest\n"), 0644); err != nil {
		t.Fatalf("write latest plan: %v", err)
	}

	got, err := resolveCurrentClaudePlan(home, runtimeSessionHints{
		SessionID:  "sess-missing",
		SessionPID: 21110,
	}, []int{21111})
	if got != "" {
		t.Fatalf("expected empty plan when hinted PID is missing, got %q", got)
	}
	if !errors.Is(err, ErrExplicitRuntimeSessionHints) {
		t.Fatalf("resolveCurrentClaudePlan error=%v want %v", err, ErrExplicitRuntimeSessionHints)
	}
}

func TestResolveCurrentClaudePlanFailsClosedWhenSessionHasNoPlanReference(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	plansDir := filepath.Join(home, ".claude", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("mkdir plans: %v", err)
	}
	sessionsDir := filepath.Join(home, ".claude", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	projectDir := filepath.Join(home, ".claude", "projects", encodeClaudeProjectKey("/repo-no-plan"))
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}

	globalLatest := filepath.Join(plansDir, "latest.md")
	if err := os.WriteFile(globalLatest, []byte("# latest\n"), 0644); err != nil {
		t.Fatalf("write latest plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, "21110.json"), []byte(`{"pid":21110,"sessionId":"sess-a","cwd":"/repo-no-plan","startedAt":10}`), 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "sess-a.jsonl"), []byte(`{"message":{"content":[{"type":"text","text":"no plan path here"}]},"timestamp":"2026-03-25T12:00:00Z"}`), 0644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	got, err := resolveCurrentClaudePlan(home, runtimeSessionHints{
		SessionID:  "sess-a",
		SessionPID: 21110,
	}, nil)
	if got != "" {
		t.Fatalf("expected empty plan when session has no plan reference, got %q", got)
	}
	if !errors.Is(err, ErrNoCurrentClaudePlan) {
		t.Fatalf("resolveCurrentClaudePlan error=%v want %v", err, ErrNoCurrentClaudePlan)
	}
}

func TestLoadClaudeRuntimeSessionRepairsNULTerminatedTruncatedJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sessionsDir := filepath.Join(home, ".claude", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}

	raw := []byte("{\"pid\":21110,\"sessionId\":\"sess-a\",\"cwd\":\"/repo-a\",\"startedAt\":10,\"kind\":\"interactive\",\x00\x00\x00")
	if err := os.WriteFile(filepath.Join(sessionsDir, "21110.json"), raw, 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}

	session, err := loadClaudeRuntimeSession(home, 21110)
	if err != nil {
		t.Fatalf("loadClaudeRuntimeSession: %v", err)
	}
	if session == nil {
		t.Fatal("expected session")
	}
	if session.SessionID != "sess-a" {
		t.Fatalf("SessionID=%q want sess-a", session.SessionID)
	}
	if session.CWD != "/repo-a" {
		t.Fatalf("CWD=%q want /repo-a", session.CWD)
	}
}
