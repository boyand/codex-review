// Binary codex-review-loop is the Go engine for the codex-review-loop
// Claude Code plugin.
package main

import (
	"fmt"
	"os"

	"github.com/boyand/codex-review-loop/internal/app"
	"github.com/boyand/codex-review-loop/internal/doctor"
	"github.com/boyand/codex-review-loop/internal/engine"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	a := app.New(os.Stdout, os.Stderr)

	if len(args) == 0 {
		fmt.Fprintln(a.Stderr, "Usage: codex-review-loop <command>")
		fmt.Fprintln(a.Stderr, "Commands: hook stop, status, completion, doctor, --version")
		return 1
	}

	switch args[0] {
	case "--version":
		fmt.Fprintf(a.Stdout, "codex-review-loop %s\n", version)
		return 0

	case "hook":
		if len(args) < 2 || args[1] != "stop" {
			fmt.Fprintln(a.Stderr, "Usage: codex-review-loop hook stop")
			return 1
		}
		code, err := engine.RunStop(a.Config, a.Stdout, a.Stderr)
		if err != nil {
			fmt.Fprintf(a.Stderr, "codex-review-loop: %v\n", err)
		}
		return code

	case "status":
		code, err := engine.RunStatus(a.Stdout)
		if err != nil {
			fmt.Fprintf(a.Stderr, "codex-review-loop: %v\n", err)
			return 1
		}
		return code

	case "completion":
		code, err := engine.RunCompletion(a.Stdout)
		if err != nil {
			fmt.Fprintf(a.Stderr, "codex-review-loop: %v\n", err)
			return 1
		}
		return code

	case "doctor":
		doctor.Run(a.Stdout, a.Config)
		return 0

	default:
		fmt.Fprintf(a.Stderr, "Unknown command: %s\n", args[0])
		fmt.Fprintln(a.Stderr, "Commands: hook stop, status, completion, doctor, --version")
		return 1
	}
}
