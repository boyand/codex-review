# Codex Review — Required Behavior and Architecture

This plugin is command-driven and session-scoped.

## Required product behavior

These are the non-negotiable behaviors the plugin must preserve:

- Commands must resolve the correct workflow automatically from the current Claude Code session.
- The normal user path must not require `--workflow <id>`.
- Multi-round `plan` and `impl` must work with explicit user choices to move forward, repeat, or finish.
- Running `/codex-review:impl` from `plan/approval` counts as an explicit user choice to move forward and should auto-advance to `implement`.
- The canonical plan under review is `.claude/codex-review/workflows/<id>/artifacts/plan.md`.
- Implementation review must compare against that canonical approved plan, not a mutable native Claude plan file.
- Findings/outcomes must remain inspectable through `summary` and the workflow artifacts.
- The normal flow is command-driven. Hidden hook progression is not part of the default architecture.

## High-level architecture

Workflows live under:
`.claude/codex-review/workflows/<workflow-id>/`

Key files:

- `workflow.json` — workflow state for phase, round, owner session, status, and plan source
- `decisions.tsv` — normalized findings/outcome ledger used by `summary`
- `artifacts/plan.md` — sticky canonical plan snapshot
- `artifacts/plan-review-rN.md` and `artifacts/plan-findings-rN.md`
- `artifacts/implement-review-rN.md` and `artifacts/implement-findings-rN.md`

Runtime model:

- The tracked wrapper entrypoint is `"${CLAUDE_PLUGIN_ROOT}/.bin/codex-review"`.
- The wrapper rebuilds the Go engine to `.bin/codex-review-go`, walks a small parent-process chain to find the current Claude session file under `~/.claude/sessions/`, injects hidden `--session-pid` and `--session-id` runtime flags when available, and executes the binary from the workflow root when available.
- The Go engine resolves the current Claude session and then resolves that session's workflow.
- If explicit runtime hints fail to resolve, the engine fails closed. It does not guess another workflow or fall back to another Claude session's plan.
- `plan` seeds the workflow from the current session's native Claude plan only when the canonical `artifacts/plan.md` does not already exist, or when `--plan` explicitly replaces it.
- Review rounds are forward-only:
  - `__repeat` increments the round and runs another review
  - `__approve` advances `plan -> implement` or completes the workflow
- Prior round artifacts are preserved.

Current step is inferred from artifacts:

- no review artifact -> `working`
- review exists but findings are missing or incomplete -> `review`
- review and findings are both complete -> `approval`

## Command contract

Primary commands:

- `/codex-review:plan`
- `/codex-review:impl`
- `/codex-review:status`
- `/codex-review:summary`
- `/codex-review:doctor`

When running `plan` or `impl`, Claude should:

1. run the engine command via `"${CLAUDE_PLUGIN_ROOT}/.bin/codex-review"`
   - do not pass hidden runtime flags like `--session-pid` or `--session-id`; the wrapper injects them
   - if the workflow is `plan/approval`, invoking `impl` is itself the explicit approval signal; do not require a separate approval command first
2. read the generated review artifact
3. write a findings response that addresses every finding ID
4. run `"${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" summary`
5. present that summary output to the user as the approval summary
6. in that same command response only, ask inline what the user wants to do next
   - do not re-offer the same approval choices on unrelated later turns
7. if the user chooses to move forward, repeat, or finish, run the internal transition command yourself:
   - `"${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" __approve`
   - `"${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" __repeat`
   - `"${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" __done`

These are internal transition commands, not part of the primary user-facing slash-command flow.

Decision tokens:

- Plan review: `ACCEPT`, `REJECT`, `DEFER`
- Implementation review: `FIX`, `REJECT`, `WONTFIX`

## Guardrails

- Do not manually edit `workflow.json` or `decisions.tsv` unless explicitly asked.
- Preserve all workflow artifacts and round history.
- Do not re-import the mutable native Claude plan on later rounds unless `--plan` explicitly requests replacement.
- Do not fall back to repo-global active-workflow guessing as the normal UX.
