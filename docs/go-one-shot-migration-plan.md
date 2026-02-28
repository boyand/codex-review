# Codex Review Loop: One-Shot Bash -> Go Migration Plan

This document is the implementation blueprint for replacing the current Bash engine with a Go engine in one cutover (no rollout flag, no Bash fallback logic).

## 1) Scope and Non-Negotiables

## Scope
- Replace the logic in `hooks/stop-hook.sh` with Go.
- Keep plugin UX and state/artifact file contracts intact.
- Keep existing command semantics (`/codex-review-loop`, `/codex-review-loop:status`, doctor flow, approval flow).

## Non-negotiables
- No staged rollout.
- No runtime feature flag (`bash|go`) fallback.
- After merge, Go is the only engine for hook logic.

## Compatibility contract
- Exit codes remain:
  - `0` -> allow stop
  - `2` -> block stop with instruction message
- State and artifact paths remain unchanged:
  - `.claude/codex-review-loop.local.md`
  - `.claude/codex-review-loop/`
  - `decisions.tsv`, `*-review-rN.md`, `*-findings-rN.md`
- Hook matcher wiring in `hooks/hooks.json` stays valid.

## Prerequisites
- Go toolchain (1.22+) on PATH
- Claude Code CLI
- OpenAI Codex CLI (`npm install -g @openai/codex`)

## 2) Target Runtime Architecture

## Runtime components
1. `hooks/stop-hook.sh` (thin shim only)
   - Resolves plugin root via `$(cd "$(dirname "$0")/.." && pwd)`
   - Checks `go` is on PATH; if missing, prints clear error and exits (exit 2 if loop active, else 0)
   - Ensures Go binary exists (builds it if needed)
   - Passes `CLAUDE_PLUGIN_ROOT` env var to the binary
   - Executes Go command `hook stop`
2. Go binary `codex-review-loop`
   - Owns all state transitions, locking, Codex invocation, parsing, rendering

## CLI surface (single binary)
- `codex-review-loop hook stop`
- `codex-review-loop status`
- `codex-review-loop completion`
- `codex-review-loop doctor`
- `codex-review-loop --version`

## Package layout
```text
cmd/codex-review-loop/main.go
internal/app/app.go
internal/config/config.go
internal/state/state.go
internal/state/frontmatter.go
internal/phase/phase.go
internal/ledger/ledger.go
internal/review/review.go
internal/prompt/prompt.go
internal/prompt/templates/         # embedded via //go:embed
internal/prompt/templates/plan-review.txt
internal/prompt/templates/implement-review.txt
internal/runner/codex.go
internal/lock/lock.go
internal/render/render.go
internal/engine/stop.go
internal/doctor/doctor.go
internal/fsx/atomic.go
```

## Data model (Go structs)
- `LoopState`
  - raw frontmatter key-values + typed fields:
  - `Active`, `ReviewID`, `CurrentPhaseIndex`, `CurrentSubstep`, `CurrentRound`, `MaxRounds`, `PipelineCount`, `TaskDescription`, `StartedAt`
  - `Pipelines []PhaseConfig`
- `PhaseConfig`
  - `Name`, `Status`, `Rounds`, `Worker`, `Reviewer`, `Artifact`, `CompareTo`, `CustomPrompt`, `WorkPrompt`
- `DecisionRow`
  - `Phase`, `Artifact`, `Round`, `Reviewer`, `FindingID`, `Severity`, `Finding`, `Decision`, `Outcome`, `Selected`, `Status`, `UpdatedAt`
- `RunResult`
  - `Err`, `LogPath`, `ExitCode`, `LastErrorLine`

## 3) Build and Invocation Strategy (No Fallback)

Use a shim script that builds binary when needed, then always executes binary.

## Build location
- `PLUGIN_ROOT/.bin/codex-review-loop`

## Rebuild trigger
- Rebuild if binary missing
- Rebuild if any Go source file in `cmd/` or `internal/` is newer than binary

## Shim behavior
- If `go` not on PATH:
  - print: `codex-review-loop: Go toolchain required but 'go' not found on PATH. Install Go from https://go.dev/dl/`
  - `exit 2` if loop active, else `exit 0`
- If build fails:
  - print clear error with build output and block stop (`exit 2` if loop active, else `0`)
- If build succeeds:
  - `exec "$BIN" hook stop "$@"`

This keeps Claude plugin hook compatibility and avoids shipping prebuilt binaries.

## Shim pseudo-code
```bash
#!/usr/bin/env bash
set -euo pipefail

PLUGIN_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$PLUGIN_ROOT/.bin/codex-review-loop"
STATE_FILE=".claude/codex-review-loop.local.md"

is_loop_active() { [[ -f "$STATE_FILE" ]] && grep -q "^active: true" "$STATE_FILE"; }
fail_exit() { if is_loop_active; then exit 2; else exit 0; fi; }

if ! command -v go >/dev/null 2>&1; then
    echo "codex-review-loop: Go toolchain required but 'go' not found on PATH."
    fail_exit
fi

needs_rebuild="false"
if [[ ! -x "$BIN" ]]; then
    needs_rebuild="true"
elif [[ -n $(find "$PLUGIN_ROOT/cmd" "$PLUGIN_ROOT/internal" -name '*.go' -newer "$BIN" 2>/dev/null | head -1) ]]; then
    needs_rebuild="true"
fi

if [[ "$needs_rebuild" == "true" ]]; then
    mkdir -p "$(dirname "$BIN")"
    if ! (cd "$PLUGIN_ROOT" && go build -o "$BIN" ./cmd/codex-review-loop); then
        echo "codex-review-loop: Go build failed. Fix compilation errors and retry."
        fail_exit
    fi
fi

export CLAUDE_PLUGIN_ROOT="$PLUGIN_ROOT"
exec "$BIN" hook stop "$@"
```

## 4) State Machine Contract to Preserve

Replicate current behavior exactly for substeps:

1. `working`
   - Validate round <= max_rounds
   - Run worker if `worker=codex`
   - Validate required artifacts (plan phase requires output)
   - Run reviewer if `reviewer=codex`
   - Sync decisions
   - Transition `working -> review`
   - Update state file body
   - Emit block message with status box + next instructions (`exit 2`)

2. `review`
   - If reviewer is codex:
     - Require findings response file
     - Enforce every finding ID addressed with valid decision
   - Sync decisions
   - Transition `review -> approval`
   - Update state file body
   - Emit status box + approval summary + next instructions (`exit 2`)

3. `approval`
   - allow stop (`exit 0`)

4. Unknown substep
   - block with error (`exit 2`)

### Block message composition

Every `exit 2` block message must be composed as:
1. `render_status()` output (Unicode status box)
2. Blank line
3. Instruction text (phase-specific)

This is the `block_with_status()` pattern from the current Bash. The engine must compose all block responses this way.

### Error handling

When the engine encounters an unrecoverable error:
- If a loop is active: print helpful error message and `exit 2` (so Claude sees it)
- If no loop is active: print to stderr and `exit 0` (don't block Claude)

Use `defer` + panic recovery in Go to replicate the Bash `trap 'on_error' ERR` behavior.

## 5) Codex Runner Requirements

Implement `internal/runner/codex.go` with:
- Command:
  - `codex exec <prompt> <flags...> --output-last-message <file> -m <model>`
- Timeout:
  - `CODEX_CALL_TIMEOUT_SEC` default `240`
- Log behavior:
  - Capture to `.claude/codex-review-loop/codex-exec.<suffix>.log`
  - Keep log file on failure and include its path in user-facing message
  - Delete log on success
- Retries:
  - Review pass retries once with strict suffix if malformed output
  - Integration tests must cover this path (first call malformed, second call valid)
- Process cleanup:
  - Kill child on timeout or signal
  - Use `exec.CommandContext` with `context.WithTimeout`

## 6) Parser/Validation Requirements

## Frontmatter parser
- Parse between first and second `---`
- Preserve unknown keys (forward compatibility)
- Preserve key order where possible
- Atomic write via `internal/fsx/atomic.go` (write to temp file, then rename)
- Round-trip fidelity: parse → modify → write must preserve unrecognized fields verbatim

## Review validation
`review_output_is_valid` is true iff:
1. At least one severity tag exists:
   - `[CRITICAL-N]`, `[HIGH-N]`, `[MEDIUM-N]`, `[LOW-N]`
2. Or explicit no-findings form:
   - contains `No findings.`
   - contains numeric summary counts for Critical/High/Medium/Low

## Findings coverage validation
- Every finding ID from review must appear in findings response file
- Each finding ID must have a decision token:
  - `ACCEPT`, `REJECT`, `DEFER`, `FIX`, `WONTFIX`

## Ledger behavior
- Maintain current TSV format and headers
- Sanitize fields (tabs/newlines collapsed to spaces)
- Upsert by composite key:
  - `phase, artifact, round, reviewer, finding_id`
- **Preserve user-edited `selected` values**: when upserting, check if an existing row has `selected=yes` set by the user; if so, preserve it rather than overwriting with the computed default. This is how users control which fixes get applied.

## 7) Rendering Requirements

Port these renderers 1:1:
- `render_status()` — Unicode box with task, ID, phase timeline (● done, ◐ active, ○ pending), finding/decision counts
- `render_approval_summary()` — severity summary table + decisions table (finding text sanitized/truncated to 60 chars) + approval choices
- `update_state_body()` — rewrite state file body below frontmatter with auto-generated dashboard
- `render_completion()` — Unicode box with per-phase round counts and loop-wide totals

### Decision/finding count scopes
- **Status box** (`render_status`): phase-cumulative — filter decisions.tsv by `phase` column
- **Approval gate** (`render_approval_summary`): current-round — filter by `phase` AND `round`
- **Completion** (`render_completion`): loop-wide — no filter

Rendering output does not need byte-for-byte match, but must preserve:
- Same information hierarchy
- Same decision/finding scopes
- Same approval choice wording

## 8) Prompt Template Strategy

Prompt templates (`plan-review.txt`, `implement-review.txt`) are embedded in the Go binary using `//go:embed`:

```go
//go:embed templates/plan-review.txt
var planReviewTemplate string

//go:embed templates/implement-review.txt
var implementReviewTemplate string
```

Copy current `prompts/plan-review.txt` and `prompts/implement-review.txt` to `internal/prompt/templates/` during migration. The `prompts/` directory can be removed after migration.

Template placeholders (`{{TASK_DESCRIPTION}}`, `{{PLAN_CONTENT}}`) remain the same.

### Untrusted content fencing
When inserting artifact content into prompts, wrap in markdown code fences:
````
```markdown
<content>
```
````

## 9) Agent Alias Normalization

`internal/phase/phase.go` must normalize worker/reviewer values. The canonical set:

| Input aliases | Normalized to |
|--------------|---------------|
| `claude`, `claude-code`, `anthropic`, `anthropic-claude` | `claude` |
| `codex`, `openai-codex`, `codex-cli`, `openai` | `codex` |
| empty string | `claude` (worker default) or `codex` (reviewer default) |

All comparisons case-insensitive.

## 10) File-by-File Implementation Plan

1. Add module bootstrap
- Create `go.mod`
- Add `cmd/codex-review-loop/main.go`
- Add basic command routing for `hook stop`, `status`, `completion`, `doctor`, `--version`

2. Implement config layer
- `internal/config/config.go`
- Parse env vars:
  - `CODEX_REVIEW_LOOP_MODEL` (default: `gpt-5.2-codex`)
  - `CODEX_REVIEW_LOOP_FLAGS` (default: `--sandbox=read-only`)
  - `CODEX_WORKER_FLAGS` (default: `--sandbox=workspace-write`)
  - `CODEX_CALL_TIMEOUT_SEC` (default: `240`)
  - `CLAUDE_PLUGIN_ROOT`

3. Implement state/frontmatter I/O
- `internal/state/frontmatter.go`
- `internal/state/state.go`
- Functions:
  - load state
  - set field / set pipeline field
  - write updated body atomically via `internal/fsx/atomic.go`

4. Implement phase metadata helpers
- `internal/phase/phase.go`
- `worker/reviewer/artifact` normalization (see section 9 for alias table)
- slugification and artifact key resolution

5. Implement lock handling
- `internal/lock/lock.go`
- lock path `.claude/codex-review-loop/.stop-hook.lock`
- stale lock detection by PID
- cleanup on signal/exit via `defer` and signal handler

6. Implement ledger
- `internal/ledger/ledger.go`
- ensure TSV exists
- parse findings
- extract decisions
- upsert rows (with `selected` preservation — see section 6)
- count functions for status/approval/completion scopes (see section 7)

7. Implement prompt builder
- `internal/prompt/prompt.go`
- `internal/prompt/templates/` (embedded via `//go:embed`)
- Plan and implement prompt templates (copied from `prompts/`)
- compare-to reference resolution
- untrusted markdown fencing

8. Implement codex runner
- `internal/runner/codex.go`
- timeout, retry, log retention, stderr snippets
- Use `exec.CommandContext` for process lifecycle

9. Implement review validation
- `internal/review/review.go`
- severity/no-findings format checks
- finding coverage checks

10. Implement renderers
- `internal/render/render.go`
- status, approval summary, completion, state body
- All block responses composed as: status box + blank line + instruction text

11. Implement engine
- `internal/engine/stop.go`
- full state machine transition logic
- block/allow responses with correct exit codes
- `update_state_body()` called after every substep transition
- `render_approval_summary()` included in review→approval block messages
- top-level `defer` with panic recovery for error handling (replaces Bash `trap ERR`)

12. Implement doctor
- `internal/doctor/doctor.go`
- checks:
  - Go toolchain version
  - codex CLI availability
  - codex connectivity smoke test (run `codex exec` with trivial prompt)
  - hook lock health
  - state file integrity
  - last codex logs and last known error hints

13. Replace hook shell with shim only
- Rewrite `hooks/stop-hook.sh` to build-and-exec Go binary (see section 3 pseudo-code)
- Remove old Bash engine logic from this file completely
- Shim must check for `go` on PATH before attempting build

14. Update command docs
- `commands/codex-review-loop-doctor.md`
- `commands/codex-review-loop-status.md`
- `AGENTS.md`
- `README.md` — add Go toolchain to prerequisites

## 11) Testing Plan (Must Pass Before Merge)

## Unit tests
- `go test ./...`
- Required coverage areas:
  - frontmatter parse/update
  - **frontmatter round-trip fidelity** (parse → modify → write preserves unknown keys verbatim)
  - lock acquire/release/stale recovery
  - review validation
  - ledger upsert/counts
  - **ledger `selected` preservation** (user-edited values not overwritten)
  - prompt generation
  - state transitions
  - **agent alias normalization** (all aliases from section 9)

## Integration tests (table-driven fixtures)
- Build temp workspace with fixture state/artifacts
- Use fake codex binary script in PATH to simulate:
  - valid review
  - malformed review (triggers strict retry)
  - **malformed then valid** (first call malformed, second call valid — covers retry path)
  - timeout
  - codex non-zero exit
  - no findings case

## Shim tests
- Verify shim behavior when `go` is not on PATH (clear error, correct exit code)
- Verify shim rebuilds when Go source is newer than binary
- Verify shim skips rebuild when binary is up-to-date

## Manual validation checklist
1. Start loop and complete `plan` review round.
2. Complete `implement` review round.
3. Verify approval summary and decisions ledger update.
4. Verify interrupted run does not leave unrecoverable lock.
5. Verify `status`, `completion`, and `doctor` commands.
6. Verify `--version` output.

## 12) Cutover Checklist (Single Shot)

1. Merge Go engine and shim in same change.
2. Verify plugin enabled and hook executes Go path.
3. Remove old Bash engine code from repository.
4. Remove `prompts/` directory (templates now embedded in Go binary).
5. Reinstall/reload plugin.
6. Run one real end-to-end loop on a small task.

## 13) Definition of Done

Migration is done only when all are true:
- Bash engine logic removed; only shim remains.
- Go engine handles all hook scenarios.
- No stale-lock dead-end after forced interruption.
- Codex failures produce clear error with log path.
- Existing loop contracts and artifact paths unchanged.
- Unit + integration + shim tests pass.
- README prerequisites updated to include Go toolchain.

## 14) Explicitly Out of Scope

- Adding new product features unrelated to migration.
- Changing state file schema.
- Changing artifact naming conventions.
- Any rollout/fallback branch.
