---
allowed-tools: Bash
description: Show workflow state for the current Claude thread
user-invocable: true
---

# /codex-review:status

Show the current workflow state for the active Claude thread.

Run:
```bash
"${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" status
```

Rules:
- Do not manually inspect or mutate workflow files in this command.
- Present the command output as-is.
