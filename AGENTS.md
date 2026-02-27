# Codex Review Loop — Claude Instructions

You are participating in a Codex-reviewed development loop. A stop hook automatically invokes OpenAI Codex to review your work at each phase. Follow these instructions precisely.

## State file

The loop state is in `.claude/codex-review-loop.local.md` (YAML frontmatter). Check it to understand the current phase and substep before acting.

Key fields:
- `current_phase_index` — which phase you're on (0-indexed)
- `current_substep` — `working`, `review`, or `approval`
- `pipeline_{N}_name` — name of phase N
- `pipeline_{N}_compare_to` — optional prior phase to reference
- `pipeline_{N}_custom_prompt` — optional custom review prompt
- `pipeline_count` — total number of phases
- `task_description` — the original task

## Phase workflow

### Substep: working

You are doing the work for this phase. Follow the phase-specific instructions below, then **stop** when done. The stop hook will run Codex automatically.

### Substep: review

Codex has reviewed your work. The review file is at `.claude/codex-review-loop/{phase}-review-r{round}.md`.

**You MUST**:
1. Read the Codex review file completely
2. Respond to **every single finding** — no silent skipping
3. Write your responses to `.claude/codex-review-loop/{phase}-findings-r{round}.md`
4. **Stop** when done. The hook will transition you to the approval substep.

### Substep: approval

Present your finding responses to the user and offer choices (see Approval Gate below).

## Finding response format

### For plan reviews

For each finding `[SEVERITY-N]`, respond with one of:

| Decision | When to use |
|----------|-------------|
| **ACCEPT** | You agree and will incorporate the feedback |
| **REJECT** | You disagree — provide substantive justification |
| **DEFER** | Valid concern but out of scope for this task — explain why |

### For implementation reviews

| Decision | When to use |
|----------|-------------|
| **FIX** | You agree and have fixed (or will fix) the issue |
| **REJECT** | You disagree — provide substantive justification |
| **WONTFIX** | Valid concern but intentionally not addressing — explain trade-off |

### For custom phase reviews

| Decision | When to use |
|----------|-------------|
| **ACCEPT** | You agree and will incorporate |
| **REJECT** | You disagree — provide justification |

### Rules for responses

- **Every finding gets a response**. No exceptions.
- **CRITICAL findings need strong justification to reject.** If you reject a CRITICAL finding, explain thoroughly why it doesn't apply or is incorrect.
- Brief one-liners are not acceptable for REJECT/WONTFIX on HIGH+ findings. Provide real reasoning.

### Response file format

Write `.claude/codex-review-loop/{phase}-findings-r{round}.md` like this:

```markdown
# Phase: {phase} — Finding Responses (Round {round})

| # | Severity | Finding | Decision | Justification |
|---|----------|---------|----------|---------------|
| CRITICAL-1 | Critical | JWT stored in localStorage | FIX | Moved to httpOnly cookie as recommended |
| HIGH-1 | High | No rate limiting on login | REJECT | Rate limiting is handled by the API gateway (nginx config at infra/nginx.conf:42) |
| MEDIUM-1 | Medium | Consider token revocation | DEFER | Out of scope for MVP; tracked in issue #45 |

## Details

### CRITICAL-1: JWT stored in localStorage
**Decision: FIX**
Moved token storage from localStorage to httpOnly secure cookie. Updated auth.ts lines 15-30.

### HIGH-1: No rate limiting on login
**Decision: REJECT**
Rate limiting is already implemented at the infrastructure level via nginx. See infra/nginx.conf line 42:
`limit_req zone=login burst=5 nodelay;`
The application layer doesn't need duplicate rate limiting.
```

## Approval gate

When you reach the `approval` substep, present the findings to the user:

1. Show a summary table of all findings and your decisions
2. Note any CRITICALs that were rejected (highlight these)
3. Offer the user four choices:

> **What would you like to do?**
> 1. **approve** — Accept these results and move to the next phase
> 2. **repeat** — Re-run Codex review on this phase (round N+1)
> 3. **add-phase** — Insert a new review phase after this one
> 4. **done** — Finish the loop early

### Handling user choices

**approve**:
1. Update state: set current phase `pipeline_{N}_status: completed`
2. Increment `current_phase_index`
3. If more phases remain: set next phase status to `working`, set `current_substep: working`, reset `current_round: 1`
4. If no more phases: clean up state file (remove it), report completion summary
5. **Begin work on the next phase immediately — do NOT stop until the phase work is complete.** The stop hook will fire when you stop, so stopping before doing the work would trigger an empty Codex review.

**repeat**:
1. Increment `current_round` in state
2. Set `current_substep: working`
3. Incorporate accepted findings into your work
4. Continue working, then stop for re-review

**add-phase**:
1. Ask the user: "What should this phase be called?" and "What should Codex review?"
2. Insert a new pipeline entry after the current index:
   - Increment `pipeline_count`
   - Shift all entries after current index up by 1
   - Add the new entry with `status: pending`, `rounds: 0`, `custom_prompt: <user's prompt>`
3. Advance to the next phase (the newly inserted one) with `current_substep: working`

**done**:
1. Remove state file
2. Report completion summary across all completed phases:
   - For each completed phase: findings count by severity, accept/reject/fix counts
   - Total rounds across all phases
   - Artifacts preserved in `.claude/codex-review-loop/`

## Phase-specific work instructions

### Phase: plan

Write an implementation plan to `.claude/codex-review-loop/plan.md`. The plan should:
- Analyze the codebase and existing patterns
- Break the task into clear steps
- Identify risks and edge cases
- Specify files to create/modify
- Define the testing approach

### Phase: implement

Follow the approved plan in `.claude/codex-review-loop/plan.md`. The plan is the source of truth.
- Implement the solution as specified
- If you deviate from the plan, note why in your code or comments
- Write tests as specified in the plan
- Do not skip steps from the plan

### Custom phases

For any phase not named `plan` or `implement`:
- Read the phase description from the pipeline entry
- Do the work described
- Write output to `.claude/codex-review-loop/{phase-name}.md`
- Stop when done for Codex review

## Cross-phase references

When a phase has `compare_to` set, the Codex review will include the referenced phase's output. For example, the `implement` phase compares against `plan.md`. This ensures implementation fidelity to the approved plan.

## Important rules

1. **Always check the state file** before starting work to know your current phase and substep
2. **Stop after completing phase work** — don't continue to the next phase manually
3. **The plan file is source of truth** for implementation
4. **Preserve all artifacts** — never delete review files or finding responses
5. **Be honest in finding responses** — don't rubber-stamp everything as ACCEPT. Push back on findings that are genuinely wrong.
