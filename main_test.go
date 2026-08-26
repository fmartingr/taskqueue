package taskqueue

import (
	"os"
	"testing"

	"github.com/fmartingr/taskqueue/internal/tqtest"
)

// TestMain clears the environment the whole suite runs under. TQ_DIR is the
// documented way to point tq at a queue, so a developer may well have it
// exported; without this every test here would operate on their real one.
func TestMain(m *testing.M) {
	tqtest.Isolate()
	os.Exit(m.Run())
}

func TestTheSuiteIsIsolated(t *testing.T) { tqtest.RequireIsolated(t) }
