# Quickstart

## Install

```bash
npm install -g @openai/codex
claude plugin add /path/to/codex-review
```

Or:

```bash
claude plugin add github:boyand/codex-review
```

## First run

Create or open a Claude Code thread in your repo, then run:

```text
/codex-review:plan deeply review this plan
```

After the plan summary, choose:
- move to implementation
- run another review round
- stop and keep this review

Then run:

```text
/codex-review:impl review the implementation deeply
```

## What gets written

```text
.claude/codex-review/workflows/<workflow-id>/
```

Important outputs:
- `artifacts/plan.md`
- `artifacts/plan-review-r1.md`
- `artifacts/plan-findings-r1.md`
- `artifacts/implement-review-r1.md`
- `artifacts/implement-findings-r1.md`
- `decisions.tsv`

## Useful support commands

```text
/codex-review:summary
/codex-review:status
/codex-review:doctor
```

Use them for:
- checking what the round decided
- verifying which workflow the current Claude thread owns
- diagnosing session or toolchain resolution failures
