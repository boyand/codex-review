---
allowed-tools: Bash
description: Show finding decisions for the current phase and round
user-invocable: true
---

# /codex-review:summary

Show a concise decision summary for the active phase and round.

Run:
```bash
"${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" summary
```

Rules:
- Do not manually inspect or mutate workflow files in this command.
- Present the command output as-is.
