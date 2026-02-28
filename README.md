# codex-review-loop

A Claude Code plugin that creates an N-phase loop with dynamic Claude/Codex role chaining. Each phase can choose who does the work and who reviews it.

## Features

- **Dynamic role chaining**: Configure `worker` and `reviewer` per phase (`claude` or `codex`)
- **Automatic Codex execution**: Hook can run Codex as worker and/or reviewer based on phase config
- **Single decision ledger**: One canonical file tracks agreed fix/no-fix outcomes and user selection
- **N-phase pipeline**: Default plan + implement, but add custom phases at any approval gate
- **Finding accountability**: Claude must ACCEPT/REJECT/FIX every finding with justification
- **Dynamic phase insertion**: Add phases like "test", "refactor", "security audit" on the fly
- **Cross-phase comparison**: Implementation is reviewed against the approved plan
- **Fail-safe progression**: Errors block progression with clear recovery instructions (no silent phase advancement)

## Prerequisites

- [Go](https://go.dev/dl/) toolchain (1.22+)
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) CLI
- [OpenAI Codex](https://github.com/openai/codex) CLI (`npm install -g @openai/codex`)

## Install

```bash
claude plugin add /path/to/codex-review-loop
```

Or from GitHub:

```bash
claude plugin add github:boyand/codex-review-loop
```

## Usage

### Start a review loop

```
/codex-review-loop Build JWT auth with refresh tokens
```

This starts a default two-phase pipeline:

1. **Plan** — `worker=claude`, `reviewer=codex`
2. **Implement** — `worker=claude`, `reviewer=codex`

You can override roles in the loop state (or via your command flow) so phases can be chained like:
- plan: `worker=codex`, `reviewer=claude`
- implement: `worker=claude`, `reviewer=codex`

### At each approval gate

After the current review round is addressed, you choose:

| Choice | What happens |
|--------|-------------|
| **approve** | Move to the next phase |
| **repeat** | Re-run Codex review (another round) |
| **add-phase** | Insert a custom phase (e.g., "test coverage") |
| **done** | Finish the loop early |

Use `.claude/codex-review-loop/decisions.tsv` at any time to inspect and select changes:
- `outcome=FIX` means agreed to fix
- `outcome=NO-CHANGE` means agreed not to change
- Set `selected=yes` for items you want applied

### Check loop status

```
/codex-review-loop:status
```

Shows a visual status box with the current phase, substep, round, and finding/decision counts.

### Cancel a loop

```
/cancel-codex-review
```

Cancels the active loop. Preserves all artifacts.

## How it works

```
Claude works on phase
        │
        v
   [Claude stops]
        │
        v
Stop hook routes phase ──→ worker/reviewer run by role
        │
        v
Claude addresses review output
        │
        v
   [Claude stops]
        │
        v
Hook presents to user ──→ approve / repeat / add-phase / done
```

The stop hook is a generic state machine using phase index + substep + role fields. It doesn't depend on fixed phase names.

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `CODEX_REVIEW_LOOP_MODEL` | `gpt-5.2-codex` | Codex model for worker/reviewer runs |
| `CODEX_REVIEW_LOOP_FLAGS` | `--sandbox read-only` | Flags for Codex reviewer runs |
| `CODEX_WORKER_FLAGS` | `--sandbox=workspace-write` | Flags for Codex worker runs |

## Artifacts

During a session, artifacts are stored in `.claude/codex-review-loop/`:

```
.claude/codex-review-loop/
├── decisions.tsv              # Canonical issue decision ledger (fix/no-change/open + selected)
├── plan.md                    # Plan phase artifact
├── plan-review-r1.md          # Review output for plan round 1
├── plan-findings-r1.md        # Findings responses (for Codex-reviewed rounds)
├── implement.md               # Implementation phase summary artifact (if produced)
├── implement-review-r1.md     # Review output for implementation round 1
├── implement-findings-r1.md   # Findings responses (for Codex-reviewed rounds)
└── ...                        # Custom phase artifacts by safe artifact key
```

Artifacts are preserved after the loop completes.

## License

MIT
