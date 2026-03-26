# FAQ

## Why not just ask Claude to review things directly?

Because chat-only review loses state fast.

The hard parts are:
- preserving the approved plan
- keeping findings and responses on disk
- separating concurrent Claude sessions in one repo
- supporting multiple review rounds without manual bookkeeping

## Why is there a `plan.md` artifact?

It gives implementation review a stable reference point.

Without it, implementation review can drift if the native Claude plan changes after the plan review round.

## Does `impl` require a separate approval command?

No.

If the workflow is already at `plan/approval`, running `/codex-review:impl` is treated as explicit approval and the workflow auto-advances to implementation.

## What is `summary` for?

`summary` is the outcome view.

It shows:
- severity totals
- decisions for the current round
- whether findings became `FIX`, `NO-CHANGE`, or are still `OPEN`

## What is `status` for?

`status` is the state/debug view.

Use it when you need to know:
- which workflow this Claude thread resolved
- current phase
- current round
- current derived step
- artifact paths

## What is `doctor` for?

`doctor` checks:
- current Claude session resolution
- workflow ownership and conflicts
- Codex CLI availability
- Go toolchain availability

Use it when resolution feels wrong or the plugin fails unexpectedly.
