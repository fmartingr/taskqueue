package taskqueue

import (
	"fmt"
	"os"

	"github.com/fmartingr/taskqueue/internal/cli"
)

// version is set at build time via
// -ldflags "-X github.com/fmartingr/taskqueue.version=..."
var version = "dev"

// Main runs the CLI against the current directory and returns the process exit
// code. The binary in cmd/tq is a wrapper around this.
func Main(args []string) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return cli.Run(os.Stdout, os.Stderr, cwd, version, args)
}
