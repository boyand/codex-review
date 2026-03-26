---
allowed-tools: Read, Write, Edit, Glob, Grep, Bash
description: Run a Codex plan review round and auto-write findings responses
user-invocable: true
---

# /codex-review:plan

Run a plan review round immediately. If the current Claude thread has no active workflow, this command creates one.

The user may pass:
- focus text
- `--plan <path>` to override the current Claude plan file
- `--workflow <id>` to target a specific workflow

Default plan resolution:
- infer the plan file from the current Claude session transcript
- snapshot that source once into workflow `artifacts/plan.md`
- after workflow creation, treat `artifacts/plan.md` as the plan under review; only `--plan` should replace it
- if the plan exists only as inline chat text and not as a file, the engine should fail and Claude should ask for `--plan <path>` rather than guessing from another plan file

## Required behavior

1. Run the engine:
```bash
"${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" plan $ARGUMENTS
```
- Do not pass hidden runtime flags like `--session-pid` or `--session-id`; the wrapper injects them automatically.

2. If the engine reports a review file, read it completely.

3. Write findings responses automatically to the reported `Findings file`:
- respond to every finding ID
- use `ACCEPT`, `REJECT`, or `DEFER`
- be concrete, not generic

4. After writing findings:
- run:
  ```bash
  "${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" summary
  ```
- present that summary output as the approval summary for the round
- in this same command response only, ask the user inline what they want to do next:
  - move to implementation
  - run another review round
  - stop and keep this review
- do not re-offer these choices on unrelated later turns
- if the user chooses to move to implementation, run:
  ```bash
  "${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" __approve
  ```
- if the user chooses another review round, run:
  ```bash
  "${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" __repeat
  ```
- if the user chooses to stop and keep this review, run:
  ```bash
  "${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" __done
  ```
- do not require the user to type separate slash commands for those transitions

Rules:
- Do not manually edit `workflow.json` or `decisions.tsv`.
- Do not hand-roll the post-review counts; use the summary command output after writing findings.
- Do not ask the user to run a separate respond command.
