# Codex Review Loop — Claude Instructions

You are participating in a Codex-reviewed development loop. A stop hook coordinates phase execution and review. Follow these instructions precisely.

## State file

The loop state is in `.claude/codex-review-loop.local.md` (YAML frontmatter). Check it before acting.

Key fields:
- `current_phase_index` — which phase you're on (0-indexed)
- `current_substep` — `working`, `review`, or `approval`
- `current_round` — current review round
- `pipeline_{N}_name` — name of phase N
- `pipeline_{N}_worker` — who executes phase work (`claude` or `codex`)
- `pipeline_{N}_reviewer` — who reviews that phase (`claude` or `codex`)
- `pipeline_{N}_artifact` — safe artifact key used in file names
- `pipeline_{N}_compare_to` — optional prior phase reference
- `pipeline_{N}_custom_prompt` — optional custom review focus
- `pipeline_{N}_work_prompt` — optional custom work instructions
- `pipeline_count` — total number of phases
- `task_description` — original task

Artifact file paths are based on `pipeline_{N}_artifact`:
- Phase output: `.claude/codex-review-loop/{artifact}.md`
- Review: `.claude/codex-review-loop/{artifact}-review-r{round}.md`
- Finding responses (for Codex reviews): `.claude/codex-review-loop/{artifact}-findings-r{round}.md`
- Canonical decision ledger: `.claude/codex-review-loop/decisions.tsv`
  - Columns: `phase artifact round reviewer finding_id severity finding decision outcome selected status updated_at`
  - `outcome` is clearly marked as `FIX`, `NO-CHANGE`, or `OPEN`
  - `selected` is user-editable (`yes`/`no`) to choose what should be implemented

## Phase workflow

### Substep: working

Do the work for the current phase based on `pipeline_{N}_worker`:
- If `worker=claude`: perform the phase work yourself and write/update the phase artifact as required.
- If `worker=codex`: do not manually run the phase output unless explicitly needed. Stop when ready; the hook will run Codex worker for this phase.

Before making edits, check `.claude/codex-review-loop/decisions.tsv` for rows relevant to the phase where:
- `outcome=FIX`
- `selected=yes`
Treat these as the user-approved fixes to implement.

Then stop. The hook will route to the phase reviewer.

### Substep: review

Review handling depends on `pipeline_{N}_reviewer`:
- If `reviewer=codex`:
  1. Read `.claude/codex-review-loop/{artifact}-review-r{round}.md`
  2. Respond to **every finding ID**
  3. Write responses to `.claude/codex-review-loop/{artifact}-findings-r{round}.md`
  4. Stop when done
- If `reviewer=claude`:
  1. Review phase output/code yourself
  2. Write review to `.claude/codex-review-loop/{artifact}-review-r{round}.md`
  3. Use severity tags `[CRITICAL-N]`, `[HIGH-N]`, `[MEDIUM-N]`, `[LOW-N]`
  4. Stop when done

### Substep: approval

Present the round result to the user:
- If reviewer was Codex: present your findings-response summary.
- If reviewer was Claude: present your Claude review summary.
- Include relevant rows from `.claude/codex-review-loop/decisions.tsv` so user can confirm/edit `selected`.

Then offer approval choices.

## Finding response format (Codex-reviewed phases)

For each finding `[SEVERITY-N]`, use one of:
- Plan/custom phases: `ACCEPT`, `REJECT`, `DEFER`
- Implementation phases: `FIX`, `REJECT`, `WONTFIX`

Rules:
- Every finding ID must be addressed.
- Rejected CRITICAL findings need strong justification.
- Do not use one-line justifications for HIGH+ rejects/wontfix.

Recommended format:

```markdown
# Phase: {phase} — Finding Responses (Round {round})

| # | Severity | Finding | Decision | Justification |
|---|----------|---------|----------|---------------|
| HIGH-1 | High | Missing input validation | FIX | Added validation in api/auth.ts:88 |

## Details

### HIGH-1: Missing input validation
**Decision: FIX**
Added schema validation and tests for malformed payloads.
```

## Approval gate

At `approval`, the hook provides a pre-formatted approval summary with severity tables and decisions. Present this summary to the user verbatim, then offer choices:

> **What would you like to do?**
> 1. **approve** — Accept and move to next phase
> 2. **repeat** — Re-run current phase review (round N+1)
> 3. **add-phase** — Insert a new phase after this one
> 4. **done** — Finish the loop early

### Handling user choices

**approve**:
1. Set current phase `pipeline_{N}_status: completed`
2. Increment `current_phase_index`
3. If more phases remain: set next phase status to `working`, set `current_substep: working`, set `current_round: 1`
4. If no phases remain: render the completion summary by running `"${CLAUDE_PLUGIN_ROOT}/.bin/codex-review-loop" completion 2>/dev/null || bash "${CLAUDE_PLUGIN_ROOT}/hooks/stop-hook.sh" --completion`, then remove the state file
5. Start work on the next phase immediately before stopping

**repeat**:
1. Increment `current_round`
2. Set `current_substep: working`
3. Incorporate accepted fixes
4. Continue working, then stop for re-review

**add-phase**:
1. Ask the user:
   - What should this phase be called?
   - Who should do the work (`claude` or `codex`)?
   - Who should review (`claude` or `codex`)?
   - What should the phase do? (work prompt)
   - What should the reviewer focus on? (review prompt)
2. Insert a new pipeline entry after current index:
   - Increment `pipeline_count`
   - Shift later `pipeline_*` entries up by 1
   - Add fields: `name`, `status: pending`, `rounds: 0`, `worker`, `reviewer`, `artifact`, optional `work_prompt`, optional `custom_prompt`
   - Set `artifact` to a safe lowercase slug (letters, numbers, dashes)
3. Advance to the inserted phase with `current_substep: working`, `current_round: 1`

**done**:
1. Render the completion summary by running `"${CLAUDE_PLUGIN_ROOT}/.bin/codex-review-loop" completion 2>/dev/null || bash "${CLAUDE_PLUGIN_ROOT}/hooks/stop-hook.sh" --completion`
2. Present the completion output to the user
3. Remove the state file
4. Artifacts are preserved in `.claude/codex-review-loop/`

## Phase-specific work instructions

### Phase: plan

Target artifact: `.claude/codex-review-loop/plan.md` (or the current phase artifact key if overridden).

If `worker=claude`, write a plan that includes:
- codebase/pattern analysis
- concrete implementation steps
- risks and edge cases
- files to modify/create
- testing approach

If `worker=codex`, stop when ready and let the hook run Codex worker.

### Phase: implement

Follow approved plan (`compare_to`, usually `plan`).
- Implement planned changes
- Note justified deviations
- Add tests from the plan

If `worker=codex`, stop when ready and let hook run Codex worker.

### Custom phases

For phases other than `plan`/`implement`:
- Use `pipeline_{N}_work_prompt` for phase work
- Write output to `.claude/codex-review-loop/{artifact}.md` when doing work directly
- Review focus comes from `pipeline_{N}_custom_prompt`

## Cross-phase references

When `pipeline_{N}_compare_to` is set, reviewer prompts include referenced phase output where available.

## Important rules

1. Always read state first.
2. Respect `worker`/`reviewer` roles per phase.
3. Stop after completing the current substep work.
4. Preserve all artifacts and review history files.
5. Be honest in findings responses; reject incorrect findings with evidence.
