# /codex-review-loop:status

Show the current status of the active Codex review loop.

## Steps

1. Check if `.claude/codex-review-loop.local.md` exists. If not, tell the user:
   > No active Codex review loop.

2. Run the status renderer:
   ```bash
   "${CLAUDE_PLUGIN_ROOT}/.bin/codex-review-loop" status 2>/dev/null || bash "${CLAUDE_PLUGIN_ROOT}/hooks/stop-hook.sh" --status
   ```

3. Present the output to the user.

4. Optionally, read `.claude/codex-review-loop/decisions.tsv` and list any recent findings with their decisions.
