#!/usr/bin/env bash
# codex-review-loop stop hook — Generic N-phase state machine engine
#
# Exit codes:
#   0 — allow stop (no active loop, or approval substep, or error)
#   2 — block stop (provide instructions to Claude)
#
# Fail-open: any error allows stop with explanation.

set -euo pipefail

STATE_FILE=".claude/codex-review-loop.local.md"
ARTIFACTS_DIR=".claude/codex-review-loop"
CODEX_MODEL="${CODEX_REVIEW_LOOP_MODEL:-gpt-5.2-codex}"
CODEX_FLAGS="${CODEX_REVIEW_LOOP_FLAGS:---sandbox=read-only}"
PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"

# Fail-open trap: on any error, allow stop
fail_open() {
    echo "codex-review-loop: hook error (fail-open) — $1" >&2
    exit 0
}
trap 'fail_open "unexpected error on line $LINENO"' ERR

# --- State file helpers ---

# Check if state file exists and loop is active
check_active() {
    [[ -f "$STATE_FILE" ]] || return 1
    local active
    active=$(get_field "active")
    [[ "$active" == "true" ]]
}

# Get a scalar field from YAML frontmatter
get_field() {
    local field="$1"
    sed -n '/^---$/,/^---$/p' "$STATE_FILE" | grep "^${field}:" | head -1 | sed "s/^${field}: *//"
}

# Set a scalar field in YAML frontmatter
set_field() {
    local field="$1" value="$2"
    if grep -q "^${field}:" "$STATE_FILE"; then
        sed -i '' "s|^${field}:.*|${field}: ${value}|" "$STATE_FILE"
    else
        # Insert before closing ---
        sed -i '' "/^---$/,/^---$/{
            /^---$/{
                N
                /^---\n---$/s|^---|${field}: ${value}\n---|
            }
        }" "$STATE_FILE"
    fi
}

# Get a pipeline entry field: get_pipeline "1" "name"
get_pipeline() {
    local index="$1" field="$2"
    get_field "pipeline_${index}_${field}"
}

# Set a pipeline entry field
set_pipeline() {
    local index="$1" field="$2" value="$3"
    set_field "pipeline_${index}_${field}" "$value"
}

# --- Codex invocation ---

run_codex_review() {
    local phase_name="$1"
    local round="$2"
    local review_file="${ARTIFACTS_DIR}/${phase_name}-review-r${round}.md"

    # Check codex is available
    if ! command -v codex &>/dev/null; then
        fail_open "codex CLI not found on PATH"
    fi

    # Build the review prompt
    local prompt
    prompt=$(build_review_prompt "$phase_name")

    # Run codex
    echo "codex-review-loop: Running Codex review for phase '${phase_name}' (round ${round})..." >&2

    local -a flags_array
    read -ra flags_array <<< "$CODEX_FLAGS"
    if ! codex exec "$prompt" "${flags_array[@]}" --output-last-message "$review_file" -m "$CODEX_MODEL" 2>&1; then
        fail_open "codex exec failed"
    fi

    # Verify review file was created
    if [[ ! -f "$review_file" ]] || [[ ! -s "$review_file" ]]; then
        fail_open "codex produced no output"
    fi

    echo "$review_file"
}

build_review_prompt() {
    local phase_name="$1"
    local task_desc
    task_desc=$(get_field "task_description")
    local phase_index
    phase_index=$(get_field "current_phase_index")

    local prompt_template=""
    local compare_to
    compare_to=$(get_pipeline "$phase_index" "compare_to")
    local custom_prompt
    custom_prompt=$(get_pipeline "$phase_index" "custom_prompt")

    if [[ "$phase_name" == "plan" ]] && [[ -f "${PLUGIN_ROOT}/prompts/plan-review.txt" ]]; then
        prompt_template=$(cat "${PLUGIN_ROOT}/prompts/plan-review.txt")
        # Substitute placeholders
        local plan_content=""
        if [[ -f "${ARTIFACTS_DIR}/plan.md" ]]; then
            plan_content=$(cat "${ARTIFACTS_DIR}/plan.md")
        fi
        prompt_template="${prompt_template//\{\{TASK_DESCRIPTION\}\}/$task_desc}"
        prompt_template="${prompt_template//\{\{PLAN_CONTENT\}\}/$plan_content}"

    elif [[ "$phase_name" == "implement" ]] && [[ -f "${PLUGIN_ROOT}/prompts/implement-review.txt" ]]; then
        prompt_template=$(cat "${PLUGIN_ROOT}/prompts/implement-review.txt")
        local plan_content=""
        if [[ -n "$compare_to" ]] && [[ -f "${ARTIFACTS_DIR}/${compare_to}.md" ]]; then
            plan_content=$(cat "${ARTIFACTS_DIR}/${compare_to}.md")
        fi
        prompt_template="${prompt_template//\{\{TASK_DESCRIPTION\}\}/$task_desc}"
        prompt_template="${prompt_template//\{\{PLAN_CONTENT\}\}/$plan_content}"

    elif [[ -n "$custom_prompt" ]]; then
        # Custom phase: wrap user prompt in standard review structure
        prompt_template="You are reviewing work for the '${phase_name}' phase of a software engineering task.

## Task Description

${task_desc}

## Review Focus

${custom_prompt}"

        # Include compare_to content if specified
        if [[ -n "$compare_to" ]] && [[ -f "${ARTIFACTS_DIR}/${compare_to}.md" ]]; then
            local ref_content
            ref_content=$(cat "${ARTIFACTS_DIR}/${compare_to}.md")
            prompt_template="${prompt_template}

## Reference: ${compare_to}

${ref_content}"
        fi

        # Include phase output if it exists
        if [[ -f "${ARTIFACTS_DIR}/${phase_name}.md" ]]; then
            local phase_output
            phase_output=$(cat "${ARTIFACTS_DIR}/${phase_name}.md")
            prompt_template="${prompt_template}

## Phase Output to Review

${phase_output}"
        fi

        prompt_template="${prompt_template}

## Output Format

Write your review as structured markdown. For each finding, use severity tags:
- \`[CRITICAL-N]\` — Must be fixed. Blocking issues.
- \`[HIGH-N]\` — Should be fixed. Significant concerns.
- \`[MEDIUM-N]\` — Worth considering. Improvement opportunity.
- \`[LOW-N]\` — Minor suggestion.

Include a summary with finding counts by severity."

    else
        # Fallback: generic review prompt
        prompt_template="Review the current state of work for the '${phase_name}' phase.
Task: ${task_desc}
Provide structured findings with severity tags [CRITICAL-N], [HIGH-N], [MEDIUM-N], [LOW-N]."
    fi

    echo "$prompt_template"
}

# --- Main state machine ---

main() {
    # Check if loop is active
    if ! check_active; then
        exit 0
    fi

    local review_id
    review_id=$(get_field "review_id")
    if [[ -z "$review_id" ]]; then
        fail_open "invalid state file: missing review_id"
    fi

    local phase_index current_substep current_round max_rounds pipeline_count
    phase_index=$(get_field "current_phase_index")
    current_substep=$(get_field "current_substep")
    current_round=$(get_field "current_round")
    max_rounds=$(get_field "max_rounds")
    pipeline_count=$(get_field "pipeline_count")

    local phase_name
    phase_name=$(get_pipeline "$phase_index" "name")

    if [[ -z "$phase_name" ]]; then
        fail_open "no phase at index ${phase_index}"
    fi

    case "$current_substep" in
        working)
            # Phase work is done. Run Codex review.

            # Check max rounds
            if [[ "$current_round" -gt "$max_rounds" ]]; then
                echo "Maximum review rounds ($max_rounds) reached for phase '${phase_name}'. Allowing stop."
                exit 0
            fi

            # Run Codex review
            local review_file
            review_file=$(run_codex_review "$phase_name" "$current_round")

            # Update state
            set_field "current_substep" "review"
            set_pipeline "$phase_index" "rounds" "$current_round"

            # Block stop with instructions for Claude
            local total_phases=$pipeline_count
            local phase_num=$((phase_index + 1))
            cat <<BLOCK_MSG
Codex has reviewed your work for phase **${phase_name}** (round ${current_round}).

Review file: \`${review_file}\`

**You must now:**
1. Read the review file completely: \`${review_file}\`
2. Respond to **every finding** — see AGENTS.md for the response format
3. Write your responses to \`.claude/codex-review-loop/${phase_name}-findings-r${current_round}.md\`
4. Stop when done

Phase ${phase_num}/${total_phases}: ${phase_name} → Addressing Codex findings (round ${current_round})
BLOCK_MSG
            exit 2
            ;;

        review)
            # Claude addressed findings. Transition to approval.

            # Guard: verify findings file was actually written
            local findings_file="${ARTIFACTS_DIR}/${phase_name}-findings-r${current_round}.md"
            if [[ ! -f "$findings_file" ]] || [[ ! -s "$findings_file" ]]; then
                cat <<BLOCK_MSG
You have not yet written your findings response file.

**You must:**
1. Read the Codex review: \`.claude/codex-review-loop/${phase_name}-review-r${current_round}.md\`
2. Respond to **every finding** — see AGENTS.md for the response format
3. Write your responses to: \`${findings_file}\`
4. Stop when done
BLOCK_MSG
                exit 2
            fi

            set_field "current_substep" "approval"

            local phase_num=$((phase_index + 1))
            local total_phases=$pipeline_count
            cat <<BLOCK_MSG
You have addressed the Codex findings for phase **${phase_name}** (round ${current_round}).

**You must now present your finding responses to the user:**

1. Read your findings file: \`.claude/codex-review-loop/${phase_name}-findings-r${current_round}.md\`
2. Show a summary table of all findings with your decisions
3. Highlight any CRITICAL findings you rejected
4. Offer the user four choices:

> **What would you like to do?**
> 1. **approve** — Accept and move to the next phase
> 2. **repeat** — Re-run Codex review (round $((current_round + 1)))
> 3. **add-phase** — Insert a new review phase after this one
> 4. **done** — Finish the loop early

Phase ${phase_num}/${total_phases}: ${phase_name} → Awaiting user decision (round ${current_round})
BLOCK_MSG
            exit 2
            ;;

        approval)
            # User interaction step — allow stop
            exit 0
            ;;

        *)
            fail_open "unknown substep: ${current_substep}"
            ;;
    esac
}

main "$@"
