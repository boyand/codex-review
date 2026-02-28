package state

import (
	"fmt"
	"os"
	"strconv"

	"github.com/boyand/codex-review-loop/internal/fsx"
)

const (
	StateFile     = ".claude/codex-review-loop.local.md"
	ArtifactsDir  = ".claude/codex-review-loop"
	DecisionsFile = ".claude/codex-review-loop/decisions.tsv"
	LockDir       = ".claude/codex-review-loop/.stop-hook.lock"
)

// LoopState wraps the frontmatter for typed access to loop state fields.
type LoopState struct {
	fm   *Frontmatter
	path string
}

// Load reads and parses the state file. Returns nil if the file does not exist.
func Load(path string) (*LoopState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}
	fm := ParseFrontmatter(string(data))
	return &LoopState{fm: fm, path: path}, nil
}

// Path returns the file path.
func (s *LoopState) Path() string { return s.path }

// Frontmatter returns the underlying frontmatter for direct access.
func (s *LoopState) Frontmatter() *Frontmatter { return s.fm }

// Save writes the state file atomically.
func (s *LoopState) Save() error {
	return fsx.AtomicWrite(s.path, []byte(s.fm.Render()), 0644)
}

// SaveFrontmatterWithBody writes frontmatter + a custom body atomically.
func (s *LoopState) SaveFrontmatterWithBody(body string) error {
	s.fm.Body = body
	return s.Save()
}

// --- Typed getters ---

func (s *LoopState) Active() bool {
	v, _ := s.fm.Fields.Get("active")
	return v == "true"
}

func (s *LoopState) ReviewID() string {
	return s.getStr("review_id")
}

func (s *LoopState) CurrentPhaseIndex() int {
	return s.getInt("current_phase_index")
}

func (s *LoopState) CurrentSubstep() string {
	return s.getStr("current_substep")
}

func (s *LoopState) CurrentRound() int {
	return s.getInt("current_round")
}

func (s *LoopState) MaxRounds() int {
	n := s.getInt("max_rounds")
	if n == 0 {
		return 10 // default
	}
	return n
}

func (s *LoopState) PipelineCount() int {
	return s.getInt("pipeline_count")
}

func (s *LoopState) TaskDescription() string {
	return s.getStr("task_description")
}

func (s *LoopState) StartedAt() string {
	return s.getStr("started_at")
}

// --- Pipeline getters ---

func (s *LoopState) PipelineField(index int, field string) string {
	return s.getStr(fmt.Sprintf("pipeline_%d_%s", index, field))
}

func (s *LoopState) PipelineName(index int) string {
	return s.PipelineField(index, "name")
}

func (s *LoopState) PipelineStatus(index int) string {
	return s.PipelineField(index, "status")
}

func (s *LoopState) PipelineRounds(index int) int {
	v := s.PipelineField(index, "rounds")
	n, _ := strconv.Atoi(v)
	return n
}

func (s *LoopState) PipelineWorker(index int) string {
	return s.PipelineField(index, "worker")
}

func (s *LoopState) PipelineReviewer(index int) string {
	return s.PipelineField(index, "reviewer")
}

func (s *LoopState) PipelineArtifact(index int) string {
	return s.PipelineField(index, "artifact")
}

func (s *LoopState) PipelineCompareTo(index int) string {
	return s.PipelineField(index, "compare_to")
}

func (s *LoopState) PipelineCustomPrompt(index int) string {
	return s.PipelineField(index, "custom_prompt")
}

func (s *LoopState) PipelineWorkPrompt(index int) string {
	return s.PipelineField(index, "work_prompt")
}

// --- Typed setters ---

func (s *LoopState) SetField(key, value string) {
	s.fm.Fields.Set(key, value)
}

func (s *LoopState) SetCurrentSubstep(v string) {
	s.SetField("current_substep", v)
}

func (s *LoopState) SetCurrentRound(v int) {
	s.SetField("current_round", strconv.Itoa(v))
}

func (s *LoopState) SetCurrentPhaseIndex(v int) {
	s.SetField("current_phase_index", strconv.Itoa(v))
}

func (s *LoopState) SetPipelineField(index int, field, value string) {
	s.SetField(fmt.Sprintf("pipeline_%d_%s", index, field), value)
}

func (s *LoopState) SetPipelineStatus(index int, value string) {
	s.SetPipelineField(index, "status", value)
}

func (s *LoopState) SetPipelineRounds(index int, value int) {
	s.SetPipelineField(index, "rounds", strconv.Itoa(value))
}

// Body returns the body content (below frontmatter).
func (s *LoopState) Body() string {
	return s.fm.Body
}

// --- Helpers ---

func (s *LoopState) getStr(key string) string {
	v, _ := s.fm.Fields.Get(key)
	return v
}

func (s *LoopState) getInt(key string) int {
	v := s.getStr(key)
	n, _ := strconv.Atoi(v)
	return n
}
