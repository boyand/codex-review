---
name: codex-review
user-invocable: false
description: Guides codex-review plugin workflows. Use when the user asks to review a plan or implementation with Codex, mentions codex-review, asks about review status or findings, or needs help with plan resolution or inline plans.
---

# Codex Review — Plugin Guide

This skill provides cross-cutting knowledge for the codex-review plugin. Individual commands (`/codex-review:plan`, `/codex-review:impl`, etc.) have their own detailed instructions — this skill covers what falls between them.

## When to use which command

| User intent | Command |
|---|---|
| Review the current plan | `/codex-review:plan` |
| Review the implementation against the approved plan | `/codex-review:impl` |
| See severity counts and per-finding decisions | `/codex-review:summary` |
| Check workflow phase, round, step, file paths | `/codex-review:status` |
| Diagnose session resolution or missing tools | `/codex-review:doctor` |

## Plan resolution

The `plan` command resolves the plan file from the current Claude session automatically.

If the workflow already has a canonical `artifacts/plan.md`, that plan is used on all subsequent rounds. Only `--plan <path>` replaces it.

**Inline plans**: if the plan exists only as chat text and not as a file, do not guess from another file. Save the plan to a file first:

```
I'll save the plan to a file so it can be reviewed.
```

Then rerun with `--plan <path>`.

## Findings response tokens

Plan review: `ACCEPT`, `REJECT`, `DEFER`
Implementation review: `FIX`, `REJECT`, `WONTFIX`

Every finding ID must be addressed. High-severity rejects need concrete justification — not "this is fine" but why it's actually safe to skip.

## Transition commands are internal

After a review round, ask the user what to do next inline. Then run the transition yourself:
- `__approve` — advance to next phase or complete
- `__repeat` — run another review round
- `__done` — finish early

Do not tell the user to type these. Do not re-offer the choices on unrelated later turns.

## Guardrails

- Do not pass `--session-pid` or `--session-id` manually; the wrapper injects them.
- Do not manually edit `workflow.json` or `decisions.tsv`.
- Do not guess a plan from a different Claude session.
- Preserve all workflow artifacts and review history.
