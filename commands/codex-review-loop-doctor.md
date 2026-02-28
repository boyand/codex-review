---
allowed-tools: Read, Glob, Grep, Bash
description: Diagnose Codex review loop wiring, state, and Codex connectivity
user-invocable: true
---

# /codex-review-loop-doctor

The user wants a health check for the Codex review loop.

## Goal

Produce a concise diagnostic report showing whether handoff to Codex should work in the current project.

## Quick Start

If the Go binary is built, run:
```bash
"${CLAUDE_PLUGIN_ROOT}/.bin/codex-review-loop" doctor
```

Otherwise, run the checks manually below.

## Checks

Run these checks in order and report PASS/WARN/FAIL for each item.

1. **Go toolchain**
   - Run `go version`. If missing, report FAIL with install hint: https://go.dev/dl/

2. **Project loop state**
   - Check for `.claude/codex-review-loop.local.md` in the current working directory.
   - If found, extract:
     - `active`
     - `review_id`
     - `current_phase_index`
     - `current_substep`
     - `current_round`
     - `pipeline_{N}_name`, `pipeline_{N}_worker`, `pipeline_{N}_reviewer` for current phase
   - If not found, report WARN (no active loop in this project).

3. **Artifacts health**
   - Check `.claude/codex-review-loop/` exists.
   - List files and sizes.
   - If any `*-review-r*.md` file exists but has size 0, report WARN with the file name.
   - Check `.claude/codex-review-loop/decisions.tsv` exists and has at least header lines.

4. **Codex CLI availability**
   - Run `which codex` and `codex --version`.
   - If missing, report FAIL with install hint: `npm install -g @openai/codex`.

5. **Installed plugin path + hook files**
   - Read `~/.claude/plugins/installed_plugins.json`.
   - Locate `codex-review-loop@codex-review-loop` latest `installPath`.
   - Verify these files exist and are readable:
     - `<installPath>/hooks/stop-hook.sh`
     - `<installPath>/hooks/hooks.json`
   - If not found, report FAIL and suggest reinstall:
     - `claude plugin remove codex-review-loop`
     - `claude plugin add /path/to/codex-review-loop`

6. **Codex connectivity smoke test (non-loop)**
   - Run a minimal command:
     - `codex exec "Reply with: OK" --output-last-message /tmp/codex-review-loop-doctor-<timestamp>.txt -m ${CODEX_REVIEW_LOOP_MODEL:-gpt-5.2-codex}`
   - This should NOT call the loop hook and should NOT mutate loop state.
   - If it fails with stream/network/auth errors, report FAIL and show the exact error snippet.
   - If it succeeds but output file is empty, report FAIL.

7. **Likely root cause classification**
   - Classify into one of:
     - `no-active-loop-state`
     - `hook-not-installed`
     - `codex-missing`
     - `codex-connectivity`
     - `empty-review-output`
     - `healthy`

## Output format

Return this structure:

```markdown
## Codex Review Loop Doctor

### Summary
- Overall: PASS|WARN|FAIL
- Root cause: <classification>

### Checks
| Check | Result | Details |
|------|--------|---------|
| Go toolchain | PASS | go1.22.5 |
| Project loop state | PASS | active=true, phase=plan, substep=working, round=1 |
| Artifacts health | WARN | empty file: .claude/codex-review-loop/plan-review-r1.md |
| Codex CLI availability | PASS | /opt/homebrew/bin/codex, codex-cli 0.105.0 |
| Plugin path + hook files | PASS | installPath=.../0.1.0 |
| Codex connectivity smoke | FAIL | stream disconnected before completion ... |

### Recommended next action
1. <exact command(s) user should run next>
2. <verification command(s)>
```

## Rules

- Do not run destructive commands.
- Do not edit loop state or artifacts in this doctor command.
- Do not run the stop hook automatically here.
- Keep recommendations concrete and command-ready.
