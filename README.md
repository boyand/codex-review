# codex-review

A Claude Code plugin for multi-round Codex reviews of plans and implementations.

![codex-review demo](docs/demo.svg)

## Why this exists

I get the best results when Claude and Codex work together. Claude plans, Codex reviews the plan, Claude implements, Codex reviews the implementation. Two models checking each other's work.

The problem is that this loop falls apart fast without structure:
- the plan drifts between review rounds
- implementation review loses the approved plan context
- findings get discussed in chat and disappear
- multiple Claude sessions in the same repo step on each other

I wanted something I could reliably run — not a framework, just a thin repeatable workflow. Five commands:

- `/codex-review:plan` — review the plan
- `/codex-review:impl` — review the implementation
- `/codex-review:summary` — see what was decided
- `/codex-review:status` — check the workflow state
- `/codex-review:doctor` — diagnose when things go wrong

## What it does

1. `plan` snapshots the current Claude plan into a canonical workflow artifact.
2. Codex reviews that plan and writes a round artifact.
3. Claude writes findings responses in the same flow.
4. `impl` reviews implementation against the approved `plan.md`.
5. Every round stays on disk with a normalized decisions ledger.

## Quickstart

### Prerequisites

- [Go](https://go.dev/dl/) 1.22+
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code)
- [OpenAI Codex CLI](https://github.com/openai/codex)

Install Codex CLI:

```bash
npm install -g @openai/codex
```

Install the plugin from a local checkout:

```bash
claude plugin add /path/to/codex-review
```

Or from GitHub:

```bash
claude plugin add github:boyand/codex-review
```

### Review a plan

```text
/codex-review:plan deeply review this plan
```

What happens:
- the current Claude thread resolves or creates its own workflow
- the current Claude plan is snapshotted into `artifacts/plan.md`
- Codex writes a plan review artifact
- Claude writes findings responses
- Claude shows the summary and asks what to do next

### Review the implementation

```text
/codex-review:impl review the implementation deeply
```

What happens:
- implementation is reviewed against the approved canonical `plan.md`
- if the workflow is still `plan/approval`, `impl` auto-advances to implementation
- Claude writes findings responses and shows the summary

### Inspect the current round

```text
/codex-review:summary
```

Use it to see:
- severity totals
- `FIX` vs `NO-CHANGE` vs `OPEN`
- what the current round actually decided

### Debug workflow/session resolution

```text
/codex-review:doctor
```

Use it when:
- the wrong workflow was picked
- the current Claude session was not resolved
- Codex or Go are missing

## The core idea

Claude plans. Codex reviews. Claude implements. Codex reviews again.

This plugin makes that loop repeatable. It does not try to be a framework, a dashboard, or an agent runtime. It is a thin workflow around two operations — review the plan, review the implementation — and that constraint is the point.

## How the workflow is stored

Workflows live under:

```text
.claude/codex-review/workflows/<workflow-id>/
```

Important files:
- `workflow.json`
- `decisions.tsv`
- `artifacts/plan.md`
- `artifacts/plan-review-rN.md`
- `artifacts/plan-findings-rN.md`
- `artifacts/implement-review-rN.md`
- `artifacts/implement-findings-rN.md`

Why this matters:
- the approved plan does not drift
- review history is preserved
- each Claude session can keep its own loop in the same repo

## Public commands

- `/codex-review:plan`
- `/codex-review:impl`
- `/codex-review:summary`
- `/codex-review:status`
- `/codex-review:doctor`

Internal transition commands still exist, but Claude runs them for you after your inline choice.

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `CODEX_REVIEW_MODEL` | `gpt-5.3-codex` | Codex model for reviewer runs |
| `CODEX_REVIEW_FLAGS` | `--sandbox=read-only` | Flags for Codex reviewer runs |
| `CODEX_WORKER_FLAGS` | `--sandbox=workspace-write` | Flags for Codex worker runs |
| `CODEX_CALL_TIMEOUT_SEC` | `720` | Per-call Codex execution timeout in seconds |

Legacy aliases still accepted:
- `CODEX_REVIEW_LOOP_MODEL`
- `CODEX_REVIEW_LOOP_FLAGS`

## More docs

- [Quickstart](docs/quickstart.md)
- [FAQ](docs/faq.md)

## Open source notes

This repo is optimized for:
- local-first use
- GitHub-first distribution
- simple static docs

The launch surface should stay lightweight. A strong README, a sharp demo, and predictable commands matter more than a large docs framework.

## License

MIT
