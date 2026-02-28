// Package app provides the application container for the codex-review-loop binary.
package app

import (
	"io"

	"github.com/boyand/codex-review-loop/internal/config"
)

// App holds the application configuration and I/O writers.
type App struct {
	Config config.Config
	Stdout io.Writer
	Stderr io.Writer
}

// New creates an App with default configuration.
func New(stdout, stderr io.Writer) *App {
	return &App{
		Config: config.Load(),
		Stdout: stdout,
		Stderr: stderr,
	}
}
