---
allowed-tools: Read, Bash
description: Cancel the active Codex review loop
user-invocable: true
---

# /cancel-codex-review

The user wants to cancel the active Codex review loop.

## Steps

1. Check if `.claude/codex-review-loop.local.md` exists. If not, tell the user:
   > No active Codex review loop found. Nothing to cancel.

2. If it exists, read the state file to extract:
   - `task_description`
   - `current_phase_index` and the name of the current phase
   - `current_substep`
   - `current_round`

3. Remove the state file: `rm .claude/codex-review-loop.local.md`

4. Report to the user:
   > Codex review loop cancelled.
   > - Task: **<task_description>**
   > - Cancelled at: Phase **<phase_name>**, substep **<substep>**, round **<round>**
   >
   > Artifacts in `.claude/codex-review-loop/` have been preserved. You can review or delete them.

**Do NOT delete the artifacts directory** — the user may want to reference previous review output.
