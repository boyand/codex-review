---
allowed-tools: Read, Write, Edit, Glob, Grep, Bash
description: Run a Codex implementation review round and auto-write findings responses
user-invocable: true
---

# /codex-review:impl

Run an implementation review round immediately for the current Claude thread's active workflow.

If the current workflow is still in `plan` but already at the `approval` step, this command should treat the `impl` invocation itself as explicit approval, advance the workflow to `implement`, and continue without requiring a separate approval command.

The user may pass:
- focus text
- `--workflow <id>` to target a specific workflow

## Required behavior

1. Run the engine:
```bash
"${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" impl $ARGUMENTS
```
- Do not pass hidden runtime flags like `--session-pid` or `--session-id`; the wrapper injects them automatically.
- If the plan phase is already at `approval`, do not stop to ask for a separate approval command first. Running `impl` is the approval signal.

2. If the engine reports a review file, read it completely.

3. Write findings responses automatically to the reported `Findings file`:
- respond to every finding ID
- use `FIX`, `REJECT`, or `WONTFIX`
- include concrete justification for any `REJECT` or `WONTFIX`

4. After writing findings:
- run:
  ```bash
  "${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" summary
  ```
- present that summary output as the approval summary for the round
- in this same command response only, ask the user inline what they want to do next:
  - finish the workflow
  - run another implementation review round
  - stop and keep current results
- do not re-offer these choices on unrelated later turns
- if the user chooses to finish the workflow, run:
  ```bash
  "${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" __approve
  ```
- if the user chooses another implementation review round, run:
  ```bash
  "${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" __repeat
  ```
- if the user chooses to stop and keep current results, run:
  ```bash
  "${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" __done
  ```
- do not require the user to type separate slash commands for those transitions

Rules:
- Do not manually edit `workflow.json` or `decisions.tsv`.
- Do not hand-roll the post-review counts; use the summary command output after writing findings.
- Do not ask the user to run a separate respond command.
