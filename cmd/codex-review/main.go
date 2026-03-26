// Binary codex-review is the Go engine for the codex-review
// Claude Code plugin.
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/boyand/codex-review/internal/config"
	"github.com/boyand/codex-review/internal/doctor"
	"github.com/boyand/codex-review/internal/engine"
	"github.com/boyand/codex-review/internal/workflow"
)

var version = "0.7.3"

const publicCommands = "plan, impl, status, summary, doctor, --version"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	cfg := config.Load()
	stdout := os.Stdout
	stderr := os.Stderr

	runtimeOpts, filteredArgs, err := parseRuntimeArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "codex-review: %v\n", err)
		return 1
	}
	workflow.SetRuntimeSessionHints(runtimeOpts.SessionID, runtimeOpts.SessionPID)
	args = filteredArgs

	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: codex-review <command>")
		fmt.Fprintf(stderr, "Commands: %s\n", publicCommands)
		return 1
	}

	switch args[0] {
	case "--version":
		fmt.Fprintf(stdout, "codex-review %s\n", version)
		return 0

	case "status":
		explicitID, parseErr := parseOptionalWorkflowID(args[1:])
		if parseErr != nil {
			fmt.Fprintf(stderr, "codex-review: %v\n", parseErr)
			return 1
		}
		handled, code, err := engine.RunWorkflowStatus(stdout, explicitID)
		if err != nil {
			fmt.Fprintf(stderr, "codex-review: %v\n", err)
			return 1
		}
		if !handled {
			fmt.Fprintln(stdout, "No active workflow found.")
			return 0
		}
		return code

	case "summary":
		explicitID, parseErr := parseOptionalWorkflowID(args[1:])
		if parseErr != nil {
			fmt.Fprintf(stderr, "codex-review: %v\n", parseErr)
			return 1
		}
		handled, code, err := engine.RunWorkflowSummary(stdout, explicitID)
		if err != nil {
			fmt.Fprintf(stderr, "codex-review: %v\n", err)
			return 1
		}
		if !handled {
			fmt.Fprintln(stdout, "No active workflow found.")
			return 0
		}
		return code

	case "plan":
		opts, err := parsePlanReviewArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "codex-review: %v\n", err)
			return 1
		}
		code, err := engine.RunPlanReview(cfg, stdout, opts)
		if err != nil {
			fmt.Fprintf(stderr, "codex-review: %v\n", err)
		}
		return code

	case "impl":
		opts, err := parseImplementReviewArgs(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "codex-review: %v\n", err)
			return 1
		}
		code, err := engine.RunImplementReview(cfg, stdout, opts)
		if err != nil {
			fmt.Fprintf(stderr, "codex-review: %v\n", err)
		}
		return code

	case "__approve":
		explicitSessionID, parseErr := parseOptionalWorkflowID(args[1:])
		if parseErr != nil {
			fmt.Fprintf(stderr, "codex-review: %v\n", parseErr)
			return 1
		}
		handled, code, err := engine.RunWorkflowApprove(stdout, explicitSessionID)
		if err != nil {
			fmt.Fprintf(stderr, "codex-review: %v\n", err)
			return 1
		}
		if !handled {
			fmt.Fprintln(stderr, "codex-review: no active workflow found")
			return 1
		}
		return code

	case "__repeat":
		explicitSessionID, focus, parseErr := parseWorkflowIDAndPrompt(args[1:])
		if parseErr != nil {
			fmt.Fprintf(stderr, "codex-review: %v\n", parseErr)
			return 1
		}
		handled, code, err := engine.RunWorkflowRepeat(cfg, stdout, explicitSessionID, focus)
		if err != nil {
			fmt.Fprintf(stderr, "codex-review: %v\n", err)
			return 1
		}
		if !handled {
			fmt.Fprintln(stderr, "codex-review: no active workflow found")
			return 1
		}
		return code

	case "__done":
		explicitSessionID, parseErr := parseOptionalWorkflowID(args[1:])
		if parseErr != nil {
			fmt.Fprintf(stderr, "codex-review: %v\n", parseErr)
			return 1
		}
		handled, code, err := engine.RunWorkflowDone(stdout, explicitSessionID)
		if err != nil {
			fmt.Fprintf(stderr, "codex-review: %v\n", err)
			return 1
		}
		if !handled {
			fmt.Fprintln(stderr, "codex-review: no active workflow found")
			return 1
		}
		return code

	case "doctor":
		doctor.Run(stdout)
		return 0

	default:
		fmt.Fprintf(stderr, "Unknown command: %s\n", args[0])
		fmt.Fprintf(stderr, "Commands: %s\n", publicCommands)
		return 1
	}
}

type runtimeOptions struct {
	SessionID  string
	SessionPID int
}

func parseRuntimeArgs(args []string) (runtimeOptions, []string, error) {
	var opts runtimeOptions
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--session-id":
			i++
			if i >= len(args) {
				return runtimeOptions{}, nil, errors.New("missing value for --session-id")
			}
			opts.SessionID = strings.TrimSpace(args[i])
		case "--session-pid":
			i++
			if i >= len(args) {
				return runtimeOptions{}, nil, errors.New("missing value for --session-pid")
			}
			pid, err := strconv.Atoi(strings.TrimSpace(args[i]))
			if err != nil || pid <= 0 {
				return runtimeOptions{}, nil, errors.New("invalid value for --session-pid")
			}
			opts.SessionPID = pid
		default:
			filtered = append(filtered, args[i])
		}
	}
	if opts.SessionID != "" && opts.SessionPID <= 0 {
		return runtimeOptions{}, nil, errors.New("--session-id requires --session-pid")
	}
	return opts, filtered, nil
}

func parsePlanReviewArgs(args []string) (engine.PlanReviewOptions, error) {
	var opts engine.PlanReviewOptions
	var prompt []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workflow":
			i++
			if i >= len(args) {
				return opts, errors.New("missing value for --workflow")
			}
			opts.WorkflowID = strings.TrimSpace(args[i])
		case "--plan":
			i++
			if i >= len(args) {
				return opts, errors.New("missing value for --plan")
			}
			opts.PlanPath = strings.TrimSpace(args[i])
		default:
			prompt = append(prompt, args[i])
		}
	}
	opts.Prompt = strings.TrimSpace(strings.Join(prompt, " "))
	return opts, nil
}

func parseImplementReviewArgs(args []string) (engine.ImplementReviewOptions, error) {
	var opts engine.ImplementReviewOptions
	var prompt []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workflow":
			i++
			if i >= len(args) {
				return opts, errors.New("missing value for --workflow")
			}
			opts.WorkflowID = strings.TrimSpace(args[i])
		default:
			prompt = append(prompt, args[i])
		}
	}
	opts.Prompt = strings.TrimSpace(strings.Join(prompt, " "))
	return opts, nil
}

func parseOptionalWorkflowID(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	if len(args) == 2 && args[0] == "--workflow" {
		return strings.TrimSpace(args[1]), nil
	}
	if len(args) == 1 {
		return strings.TrimSpace(args[0]), nil
	}
	return "", errors.New("usage: [--workflow <id>] or [<id>]")
}

func parseWorkflowIDAndPrompt(args []string) (string, string, error) {
	if len(args) == 0 {
		return "", "", nil
	}
	if args[0] == "--workflow" {
		if len(args) < 2 {
			return "", "", errors.New("missing value for --workflow")
		}
		return strings.TrimSpace(args[1]), strings.TrimSpace(strings.Join(args[2:], " ")), nil
	}
	return "", strings.TrimSpace(strings.Join(args, " ")), nil
}
