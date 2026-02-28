#!/usr/bin/env bash
# codex-review-loop stop hook — thin shim that builds and executes the Go engine.
#
# Exit codes:
#   0 — allow stop (no active loop, or approval substep)
#   2 — block stop (provide instructions to Claude)
set -euo pipefail

PLUGIN_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$PLUGIN_ROOT/.bin/codex-review-loop"
STATE_FILE=".claude/codex-review-loop.local.md"

is_loop_active() { [[ -f "$STATE_FILE" ]] && grep -q "^active: true" "$STATE_FILE"; }
fail_exit() { if is_loop_active; then exit 2; else exit 0; fi; }

# --- Ensure binary is built ---

ensure_binary() {
    if ! command -v go >/dev/null 2>&1; then
        echo "codex-review-loop: Go toolchain required but 'go' not found on PATH. Install Go from https://go.dev/dl/"
        return 1
    fi

    local needs_rebuild="false"
    if [[ ! -x "$BIN" ]]; then
        needs_rebuild="true"
    elif [[ -n $(find "$PLUGIN_ROOT/cmd" "$PLUGIN_ROOT/internal" -name '*.go' -newer "$BIN" 2>/dev/null | head -1) ]]; then
        needs_rebuild="true"
    fi

    if [[ "$needs_rebuild" == "true" ]]; then
        mkdir -p "$(dirname "$BIN")"
        if ! (cd "$PLUGIN_ROOT" && go build -o "$BIN" ./cmd/codex-review-loop); then
            echo "codex-review-loop: Go build failed. Fix compilation errors and retry."
            return 1
        fi
    fi
}

# Handle --status and --completion flags (backwards compat for commands)
case "${1:-}" in
    --status)
        if ! ensure_binary; then exit 0; fi
        exec "$BIN" status
        ;;
    --completion)
        if ! ensure_binary; then exit 0; fi
        exec "$BIN" completion
        ;;
esac

if ! ensure_binary; then
    fail_exit
fi

export CLAUDE_PLUGIN_ROOT="$PLUGIN_ROOT"
exec "$BIN" hook stop "$@"
