package store_test

import (
	"os"
	"testing"

	"github.com/fmartingr/taskqueue/internal/tqtest"
)

// TestMain clears the environment the whole suite runs under. TQ_CONFIG_PATH is
// the documented way to point tq at a project, so a developer may well have it
// exported; without this every test would operate on their real one, and one
// of them deletes the directory it is given.
func TestMain(m *testing.M) {
	tqtest.Isolate()
	os.Exit(m.Run())
}

func TestTheSuiteIsIsolated(t *testing.T) { tqtest.RequireIsolated(t) }
