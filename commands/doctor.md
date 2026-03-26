---
allowed-tools: Bash
description: Diagnose workflow resolution, ownership, and Codex connectivity
user-invocable: true
---

# /codex-review:doctor

Run the built-in doctor command only.

```bash
"${CLAUDE_PLUGIN_ROOT}/.bin/codex-review" doctor
```

Rules:
- Do not perform manual diagnostics in this command.
- Do not edit workflow files here.
