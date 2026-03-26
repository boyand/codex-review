package workflow

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/boyand/codex-review/internal/fsx"
	"github.com/boyand/codex-review/internal/phase"
	"github.com/boyand/codex-review/internal/review"
)

const (
	RootDir      = ".claude/codex-review"
	WorkflowsDir = RootDir + "/workflows"
)

type State struct {
	Version         int    `json:"version"`
	ID              string `json:"id"`
	Task            string `json:"task"`
	Phase           string `json:"phase"`
	Round           int    `json:"round"`
	Status          string `json:"status"`
	PlanSourcePath  string `json:"plan_source_path,omitempty"`
	OwnerSessionID  string `json:"owner_session_id,omitempty"`
	OwnerCWD        string `json:"owner_cwd,omitempty"`
	OwnerProjectKey string `json:"owner_project_key,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type Paths struct {
	ID            string
	Dir           string
	WorkflowFile  string
	ArtifactsDir  string
	DecisionsFile string
	LockDir       string
}

type ActiveConflictError struct {
	IDs []string
}

type claudeRuntimeSession struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
	StartedAt int64  `json:"startedAt"`
}

var claudePlanRefPattern = regexp.MustCompile(`\.claude/plans/([^\s"'` + "`" + `]+\.md)`)

var ErrExplicitRuntimeSessionHints = errors.New("unable to resolve current Claude session from explicit runtime hints")
var ErrNoCurrentClaudePlan = errors.New("no Claude plan file found for the current session")

type transcriptEnvelope struct {
	Type          string          `json:"type"`
	Timestamp     string          `json:"timestamp"`
	Snapshot      json.RawMessage `json:"snapshot"`
	Message       json.RawMessage `json:"message"`
	ToolUseResult json.RawMessage `json:"toolUseResult"`
}

type transcriptSnapshot struct {
	Timestamp          string                     `json:"timestamp"`
	TrackedFileBackups map[string]json.RawMessage `json:"trackedFileBackups"`
}

type transcriptMessage struct {
	Content json.RawMessage `json:"content"`
}

type transcriptContentItem struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Input struct {
		FilePath string `json:"file_path"`
	} `json:"input"`
}

type transcriptToolUseResult struct {
	FilePath string `json:"filePath"`
}

type planCandidate struct {
	Path       string
	Kind       string
	At         time.Time
	Line       int
	Structured bool
}

func (e *ActiveConflictError) Error() string {
	return fmt.Sprintf("multiple active workflows found: %s", strings.Join(e.IDs, ", "))
}

func PathsFor(id string) Paths {
	dir := filepath.Join(WorkflowsDir, strings.TrimSpace(id))
	return Paths{
		ID:            strings.TrimSpace(id),
		Dir:           dir,
		WorkflowFile:  filepath.Join(dir, "workflow.json"),
		ArtifactsDir:  filepath.Join(dir, "artifacts"),
		DecisionsFile: filepath.Join(dir, "decisions.tsv"),
		LockDir:       filepath.Join(dir, ".lock"),
	}
}

func New(task, planSourcePath string) (*State, Paths, error) {
	id, err := generateID()
	if err != nil {
		return nil, Paths{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	s := &State{
		Version:        2,
		ID:             id,
		Task:           sanitize(task),
		Phase:          "plan",
		Round:          1,
		Status:         "active",
		PlanSourcePath: strings.TrimSpace(planSourcePath),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return s, PathsFor(id), nil
}

func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Version < 2 {
		s.Version = 2
	}
	if s.Round <= 0 {
		s.Round = 1
	}
	return &s, nil
}

func (s *State) Save(path string) error {
	if s == nil {
		return errors.New("nil workflow state")
	}
	if s.Version < 2 {
		s.Version = 2
	}
	if s.Round <= 0 {
		s.Round = 1
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return fsx.AtomicWrite(path, data, 0644)
}

func Resolve(explicitID string) (Paths, *State, error) {
	return resolveWorkflow(explicitID, true)
}

func ResolvePreview(explicitID string) (Paths, *State, error) {
	return resolveWorkflow(explicitID, false)
}

func DiscoverActiveIDs() ([]string, error) {
	entries, err := os.ReadDir(WorkflowsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var ids []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		p := PathsFor(entry.Name())
		s, err := Load(p.WorkflowFile)
		if err != nil || s == nil {
			continue
		}
		if s.Status == "active" {
			ids = append(ids, s.ID)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func DeriveStep(paths Paths, s *State) (string, error) {
	if s == nil {
		return "", errors.New("nil workflow state")
	}
	switch strings.TrimSpace(s.Status) {
	case "completed":
		return "completed", nil
	case "cancelled":
		return "cancelled", nil
	}

	reviewFile := ReviewFile(paths, s)
	reviewContent, err := os.ReadFile(reviewFile)
	if err != nil || len(strings.TrimSpace(string(reviewContent))) == 0 {
		return "working", nil
	}
	findingsFile := FindingsFile(paths, s)
	findingsContent, err := os.ReadFile(findingsFile)
	if err != nil || len(strings.TrimSpace(string(findingsContent))) == 0 {
		return "review", nil
	}
	if coverage := review.VerifyFindingsCoverage(string(reviewContent), string(findingsContent)); coverage != nil {
		return "review", nil
	}
	return "approval", nil
}

func ResolveCurrentClaudePlan() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return resolveCurrentClaudePlan(home, runtimeSessionHintsValue(), []int{os.Getppid(), os.Getpid()})
}

func resolveCurrentClaudePlan(home string, hints runtimeSessionHints, fallbackPIDs []int) (string, error) {
	session, hadExplicitHints, err := resolveOwningClaudeSession(home, hints, fallbackPIDs)
	if err != nil {
		return "", err
	}
	if session != nil {
		path, err := resolvePlanFromSession(home, session)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(path) != "" {
			return path, nil
		}
		return "", ErrNoCurrentClaudePlan
	}
	if hadExplicitHints {
		return "", ErrExplicitRuntimeSessionHints
	}
	return resolveLatestClaudePlan(home)
}

func resolvePlanFromSession(home string, session *claudeRuntimeSession) (string, error) {
	if session == nil {
		return "", nil
	}
	transcripts, err := sessionTranscriptCandidates(home, session)
	if err != nil {
		return "", err
	}
	for _, transcript := range transcripts {
		path, err := resolvePlanFromTranscript(home, transcript)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(path) != "" {
			return path, nil
		}
	}
	return "", nil
}

func resolveOwningClaudeSession(home string, hints runtimeSessionHints, fallbackPIDs []int) (*claudeRuntimeSession, bool, error) {
	session, explicit, err := loadHintedClaudeRuntimeSession(home, hints)
	if err != nil {
		return nil, explicit, err
	}
	if session != nil && strings.TrimSpace(session.SessionID) != "" {
		return session, explicit, nil
	}
	if explicit {
		return nil, true, nil
	}

	for _, pid := range dedupePositivePIDs(fallbackPIDs) {
		session, err := loadClaudeRuntimeSession(home, pid)
		if err != nil {
			return nil, false, err
		}
		if session != nil && strings.TrimSpace(session.SessionID) != "" {
			return session, false, nil
		}
	}
	return nil, false, nil
}

func loadHintedClaudeRuntimeSession(home string, hints runtimeSessionHints) (*claudeRuntimeSession, bool, error) {
	hints.SessionID = strings.TrimSpace(hints.SessionID)
	if hints.SessionPID <= 0 && hints.SessionID == "" {
		return nil, false, nil
	}
	if hints.SessionPID <= 0 {
		return nil, true, nil
	}

	session, err := loadClaudeRuntimeSession(home, hints.SessionPID)
	if err != nil {
		return nil, true, err
	}
	if session == nil {
		return nil, true, nil
	}
	if hints.SessionID != "" && strings.TrimSpace(session.SessionID) != hints.SessionID {
		return nil, true, nil
	}
	return session, true, nil
}

func loadClaudeRuntimeSession(home string, pid int) (*claudeRuntimeSession, error) {
	path := filepath.Join(home, ".claude", "sessions", strconv.Itoa(pid)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseClaudeRuntimeSession(data, pid)
}

func parseClaudeRuntimeSession(data []byte, pid int) (*claudeRuntimeSession, error) {
	payload := data
	if idx := bytes.IndexByte(payload, 0); idx >= 0 {
		payload = payload[:idx]
	}
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return nil, errors.New("empty Claude runtime session file")
	}

	var session claudeRuntimeSession
	if err := json.Unmarshal(payload, &session); err != nil {
		// Claude can leave session files truncated after a trailing comma and then
		// NUL-pad the rest of the file. Repair that form before giving up.
		repaired := bytes.TrimRight(payload, ", \t\r\n")
		if len(repaired) == 0 {
			return nil, err
		}
		if repaired[len(repaired)-1] != '}' {
			repaired = append(repaired, '}')
		}
		if retryErr := json.Unmarshal(repaired, &session); retryErr != nil {
			return nil, err
		}
	}
	if session.PID == 0 {
		session.PID = pid
	}
	return &session, nil
}

func dedupePositivePIDs(pids []int) []int {
	seen := make(map[int]struct{}, len(pids))
	out := make([]int, 0, len(pids))
	for _, pid := range pids {
		if pid <= 1 {
			continue
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		out = append(out, pid)
	}
	return out
}

func sessionTranscriptCandidates(home string, session *claudeRuntimeSession) ([]string, error) {
	if session == nil || strings.TrimSpace(session.SessionID) == "" {
		return nil, nil
	}

	var candidates []string
	seen := map[string]struct{}{}
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		candidates = append(candidates, path)
	}

	if cwd := strings.TrimSpace(session.CWD); cwd != "" {
		add(filepath.Join(home, ".claude", "projects", encodeClaudeProjectKey(cwd), session.SessionID+".jsonl"))
	}

	matches, err := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", session.SessionID+".jsonl"))
	if err != nil {
		return nil, err
	}
	for _, match := range matches {
		add(match)
	}
	return candidates, nil
}

func resolvePlanFromTranscript(home, transcriptPath string) (string, error) {
	file, err := os.Open(transcriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer file.Close()

	var structured []planCandidate
	var text []planCandidate
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		var envelope transcriptEnvelope
		if err := json.Unmarshal([]byte(line), &envelope); err == nil {
			eventAt := parseTranscriptTime(envelope.Timestamp)
			structured = append(structured, extractStructuredPlanCandidates(home, envelope, eventAt, lineNo)...)
		}
		text = append(text, extractTextPlanCandidates(home, line, lineNo)...)
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if best := bestPlanCandidate(structured); best != nil {
		return best.Path, nil
	}
	if best := bestPlanCandidate(text); best != nil {
		return best.Path, nil
	}
	return "", nil
}

func resolveLatestClaudePlan(home string) (string, error) {
	plansDir := filepath.Join(home, ".claude", "plans")
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	type candidate struct {
		path    string
		modTime time.Time
		name    string
	}

	var candidates []candidate
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".md" || strings.Contains(name, "-agent-") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{
			path:    filepath.Join(plansDir, name),
			modTime: info.ModTime(),
			name:    name,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].modTime.Equal(candidates[j].modTime) {
			return candidates[i].name > candidates[j].name
		}
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	if len(candidates) == 0 {
		return "", nil
	}
	return candidates[0].path, nil
}

func extractStructuredPlanCandidates(home string, envelope transcriptEnvelope, eventAt time.Time, lineNo int) []planCandidate {
	var candidates []planCandidate

	if len(envelope.Snapshot) > 0 {
		var snapshot transcriptSnapshot
		if err := json.Unmarshal(envelope.Snapshot, &snapshot); err == nil {
			snapshotAt := eventAt
			if parsed := parseTranscriptTime(snapshot.Timestamp); !parsed.IsZero() {
				snapshotAt = parsed
			}
			for trackedPath := range snapshot.TrackedFileBackups {
				if planPath := normalizeClaudePlanPath(home, trackedPath); planPath != "" {
					candidates = append(candidates, planCandidate{
						Path:       planPath,
						Kind:       "snapshot",
						At:         snapshotAt,
						Line:       lineNo,
						Structured: true,
					})
				}
			}
		}
	}

	if len(envelope.Message) > 0 {
		var message transcriptMessage
		if err := json.Unmarshal(envelope.Message, &message); err == nil && len(message.Content) > 0 && message.Content[0] == '[' {
			var items []transcriptContentItem
			if err := json.Unmarshal(message.Content, &items); err == nil {
				for _, item := range items {
					if item.Type != "tool_use" {
						continue
					}
					planPath := normalizeClaudePlanPath(home, item.Input.FilePath)
					if planPath == "" {
						continue
					}
					candidates = append(candidates, planCandidate{
						Path:       planPath,
						Kind:       "tool_use:" + strings.ToLower(strings.TrimSpace(item.Name)),
						At:         eventAt,
						Line:       lineNo,
						Structured: true,
					})
				}
			}
		}
	}

	if len(envelope.ToolUseResult) > 0 {
		var result transcriptToolUseResult
		if err := json.Unmarshal(envelope.ToolUseResult, &result); err == nil {
			if planPath := normalizeClaudePlanPath(home, result.FilePath); planPath != "" {
				candidates = append(candidates, planCandidate{
					Path:       planPath,
					Kind:       "tool_result",
					At:         eventAt,
					Line:       lineNo,
					Structured: true,
				})
			}
		}
	}

	return candidates
}

func extractTextPlanCandidates(home, line string, lineNo int) []planCandidate {
	var candidates []planCandidate
	for _, match := range claudePlanRefPattern.FindAllStringSubmatch(line, -1) {
		if len(match) < 2 {
			continue
		}
		if planPath := normalizeClaudePlanPath(home, match[0]); planPath != "" {
			candidates = append(candidates, planCandidate{
				Path:       planPath,
				Kind:       "text",
				Line:       lineNo,
				Structured: false,
			})
		}
	}
	return candidates
}

func bestPlanCandidate(candidates []planCandidate) *planCandidate {
	var best *planCandidate
	for i := range candidates {
		candidate := &candidates[i]
		if best == nil || betterPlanCandidate(*candidate, *best) {
			best = candidate
		}
	}
	return best
}

func betterPlanCandidate(left, right planCandidate) bool {
	if left.Structured != right.Structured {
		return left.Structured
	}
	if !left.At.Equal(right.At) {
		if left.At.IsZero() {
			return false
		}
		if right.At.IsZero() {
			return true
		}
		return left.At.After(right.At)
	}
	if left.Line != right.Line {
		return left.Line > right.Line
	}
	return planCandidatePriority(left.Kind) > planCandidatePriority(right.Kind)
}

func planCandidatePriority(kind string) int {
	switch kind {
	case "tool_result":
		return 4
	case "tool_use:write", "tool_use:edit", "tool_use:multiedit":
		return 3
	case "tool_use:read":
		return 2
	case "snapshot":
		return 1
	default:
		return 0
	}
}

func normalizeClaudePlanPath(home, raw string) string {
	raw = strings.TrimSpace(strings.Trim(raw, `"'`))
	if raw == "" {
		return ""
	}
	if filepath.IsAbs(raw) {
		if isValidClaudePlanPath(raw) {
			return raw
		}
	}
	if strings.Contains(raw, ".claude/plans/") {
		candidate := filepath.Join(home, ".claude", "plans", filepath.Base(raw))
		if isValidClaudePlanPath(candidate) {
			return candidate
		}
	}
	return ""
}

func parseTranscriptTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func encodeClaudeProjectKey(cwd string) string {
	clean := filepath.Clean(strings.TrimSpace(cwd))
	if clean == "." || clean == "" {
		return ""
	}
	return strings.ReplaceAll(clean, string(filepath.Separator), "-")
}

func isValidClaudePlanPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasSuffix(path, ".md") {
		return false
	}
	if strings.Contains(filepath.Base(path), "-agent-") {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return false
	}
	return true
}

func OutputFile(paths Paths, s *State) string {
	return phase.OutputFile(paths.ArtifactsDir, strings.TrimSpace(s.Phase))
}

func ReviewFile(paths Paths, s *State) string {
	return phase.ReviewFile(paths.ArtifactsDir, strings.TrimSpace(s.Phase), s.Round)
}

func FindingsFile(paths Paths, s *State) string {
	return phase.FindingsFile(paths.ArtifactsDir, strings.TrimSpace(s.Phase), s.Round)
}

func EnsureDirs(paths Paths) error {
	if err := os.MkdirAll(paths.ArtifactsDir, 0755); err != nil {
		return err
	}
	return nil
}

func Touch(s *State) {
	if s == nil {
		return
	}
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func BindOwner(s *State, session *ClaudeSession) {
	if s == nil || session == nil {
		return
	}
	s.OwnerSessionID = strings.TrimSpace(session.SessionID)
	s.OwnerCWD = strings.TrimSpace(session.CWD)
	s.OwnerProjectKey = strings.TrimSpace(session.ProjectKey)
}

func generateID() (string, error) {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate workflow id: %w", err)
	}
	return time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b[:]), nil
}

func sanitize(v string) string {
	v = strings.ReplaceAll(v, "\n", " ")
	return strings.TrimSpace(v)
}
