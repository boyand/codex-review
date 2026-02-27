---
allowed-tools: Read, Write, Edit, Glob, Grep, Bash
description: Start an N-phase Codex review loop for a task
user-invocable: true
---

# /codex-review-loop

The user wants to start a Codex-reviewed development loop. Their task description is in $ARGUMENTS.

## Pre-flight checks

1. Check if `.claude/codex-review-loop.local.md` exists and has `active: true`. If so, tell the user a loop is already active and they should `/cancel-codex-review` first or finish the current loop.

2. Check if `codex` is available on PATH by running `which codex`. If not found, tell the user:
   > The `codex` CLI is required but not found on PATH. Install it with: `npm install -g @openai/codex`

3. If both checks pass, proceed.

## Setup

1. Generate a review ID: `date +%Y%m%d-%H%M%S`-`openssl rand -hex 3`

2. Create the artifacts directory: `mkdir -p .claude/codex-review-loop`

3. Create the state file `.claude/codex-review-loop.local.md` with this exact format:

```markdown
---
active: true
review_id: <generated-id>
pipeline_0_name: plan
pipeline_0_status: working
pipeline_0_rounds: 0
pipeline_1_name: implement
pipeline_1_status: pending
pipeline_1_rounds: 0
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
Status: Active — Phase 1/2: plan (working)
```

## Instructions for Claude

Now begin the **plan** phase:

1. Read the relevant parts of the codebase to understand the project structure, existing patterns, and constraints.
2. Create a thorough implementation plan addressing the task.
3. Write the plan to `.claude/codex-review-loop/plan.md`.
4. After writing the plan, **stop** (do not continue to implementation). The stop hook will automatically trigger Codex to review your plan.

Tell the user:
> Starting Codex review loop for: **<task>**
> Pipeline: plan → implement
> Phase 1/2: Creating implementation plan...
>
> I'll analyze the codebase and write a plan. Once I stop, Codex will automatically review it.
