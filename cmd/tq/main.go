// Command tq is the task queue's command line interface. It lives here rather
// than at the repository root so `go install` names the binary tq: Go takes
// that name from the last element of the package path.
package main

import (
	"os"

	"github.com/fmartingr/taskqueue"
)

func main() {
	os.Exit(taskqueue.Main(os.Args[1:]))
}
