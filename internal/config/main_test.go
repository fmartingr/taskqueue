package config_test

import (
	"os"
	"testing"

	"github.com/fmartingr/taskqueue/internal/tqtest"
)

func TestMain(m *testing.M) {
	tqtest.Isolate()
	os.Exit(m.Run())
}
