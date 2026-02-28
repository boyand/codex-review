---
allowed-tools: Read, Write, Edit, Glob, Grep, Bash
description: Start an N-phase Codex review loop for a task
user-invocable: true
---

# /codex-review-loop

The user wants to start a Codex-reviewed development loop. Their task description is in $ARGUMENTS.

This command supports dynamic role chaining per phase:
- `worker`: who executes the phase work (`claude` or `codex`)
- `reviewer`: who reviews that phase (`claude` or `codex`)

## Pre-flight checks

1. Check if `.claude/codex-review-loop.local.md` exists and has `active: true`. If so, tell the user a loop is already active and they should `/cancel-codex-review` first or finish the current loop.

2. Check if `codex` is available on PATH by running `which codex`. If not found, tell the user:
   > The `codex` CLI is required but not found on PATH. Install it with: `npm install -g @openai/codex`

3. Determine phase roles:
   - Default:
     - plan: `worker=claude`, `reviewer=codex`
     - implement: `worker=claude`, `reviewer=codex`
   - If the user explicitly asks for different roles in $ARGUMENTS, honor that request.
   - Only allow `claude` or `codex` as role values.

4. If checks pass, proceed.

## Setup

1. Generate a review ID: `date +%Y%m%d-%H%M%S`-`openssl rand -hex 3`

2. Create the artifacts directory: `mkdir -p .claude/codex-review-loop`

3. Initialize the canonical decision ledger at `.claude/codex-review-loop/decisions.tsv`:

```text
# Codex Review Loop Decisions Ledger
# Edit `selected` to `yes` for fixes you want applied.
# phase	artifact	round	reviewer	finding_id	severity	finding	decision	outcome	selected	status	updated_at
```

4. Create the state file `.claude/codex-review-loop.local.md` with this format:

```markdown
---
active: true
review_id: <generated-id>
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_rounds: 0
pipeline_0_worker: <claude|codex>
pipeline_0_reviewer: <claude|codex>
pipeline_0_artifact: plan
pipeline_1_name: implement
pipeline_1_status: pending
pipeline_1_rounds: 0
pipeline_1_worker: <claude|codex>
pipeline_1_reviewer: <claude|codex>
pipeline_1_artifact: implement
pipeline_1_compare_to: plan
pipeline_count: 2
current_phase_index: 0
current_substep: working
current_round: 1
max_rounds: 5
started_at: <ISO-8601 timestamp>
task_description: <user's task from $ARGUMENTS>
---

# Codex Review Loop

Task: <user's task>
Pipeline: plan → implement
Roles:
- plan: <worker>/<reviewer>
- implement: <worker>/<reviewer>
Status: Active — Phase 1/2: plan (working)
```

## Instructions for Claude

Now begin the **plan** phase according to `pipeline_0_worker`:

1. If `pipeline_0_worker: claude`:
   - Read the relevant parts of the codebase to understand structure and constraints.
   - Create a thorough implementation plan addressing the task.
   - Write it to `.claude/codex-review-loop/plan.md`.
2. If `pipeline_0_worker: codex`:
   - Do not manually produce the plan artifact.
   - Stop after setup. The stop hook will run Codex worker to generate `.claude/codex-review-loop/plan.md`.
3. In either case, stop when phase work is complete. Do not continue to implementation.

Tell the user:
> Starting Codex review loop for: **<task>**
> Pipeline: plan → implement
> Role chain:
> - plan: **<worker>/<reviewer>**
> - implement: **<worker>/<reviewer>**
> Phase 1/2: plan (working)...
>
> I'll execute the phase using the configured worker, then the stop hook will route review to the configured reviewer.
>
> A canonical decisions file will be maintained at `.claude/codex-review-loop/decisions.tsv` so you can inspect/select changes at any point.
