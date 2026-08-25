package main

import (
	"fmt"
	"os"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(exitError)
	}

	os.Exit(runCLI(&cli{stdout: os.Stdout, stderr: os.Stderr, dir: cwd}, os.Args[1:]))
}
