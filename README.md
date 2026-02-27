# codex-review-loop

A Claude Code plugin that creates an N-phase review loop with OpenAI Codex. Every time Claude stops, Codex automatically reviews the work and Claude must address each finding with justification.

## Features

- **Automatic reviews**: Stop hook fires Codex review every time Claude completes a phase
- **N-phase pipeline**: Default plan + implement, but add custom phases at any approval gate
- **Finding accountability**: Claude must ACCEPT/REJECT/FIX every finding with justification
- **Dynamic phase insertion**: Add phases like "test", "refactor", "security audit" on the fly
- **Cross-phase comparison**: Implementation is reviewed against the approved plan
- **Fail-open**: Errors never trap you — the hook allows stop with an explanation

## Prerequisites

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

This starts a two-phase pipeline:

1. **Plan** — Claude analyzes the codebase and writes a plan. Codex reviews it.
2. **Implement** — Claude implements the plan. Codex reviews against the plan.

### At each approval gate

After Claude addresses Codex's findings, you choose:

| Choice | What happens |
|--------|-------------|
| **approve** | Move to the next phase |
| **repeat** | Re-run Codex review (another round) |
| **add-phase** | Insert a custom phase (e.g., "test coverage") |
| **done** | Finish the loop early |

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
Stop hook runs Codex ──→ Review file created
        │
        v
Claude responds to every finding
        │
        v
   [Claude stops]
        │
        v
Hook presents to user ──→ approve / repeat / add-phase / done
```

The stop hook is a generic state machine that operates on phase index + substep. It doesn't know phase names — so any number of phases work with zero code changes.

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `CODEX_REVIEW_LOOP_MODEL` | `gpt-5.2-codex` | Codex model for reviews |
| `CODEX_REVIEW_LOOP_FLAGS` | `--sandbox read-only` | Codex sandbox flags |

## Artifacts

During a session, artifacts are stored in `.claude/codex-review-loop/`:

```
.claude/codex-review-loop/
├── plan.md                    # The implementation plan
├── plan-review-r1.md          # Codex review of the plan
├── plan-findings-r1.md        # Claude's responses to findings
├── implement-review-r1.md     # Codex review of implementation
├── implement-findings-r1.md   # Claude's responses
└── ...                        # Any custom phase artifacts
```

Artifacts are preserved after the loop completes.

## License

MIT
