package taskqueue

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, ConfigFileName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFindConfigReadsTheNearestFile(t *testing.T) {
	root := testRoot(t)
	writeConfig(t, root, "version: 1\npath: queue\n")
	nested := filepath.Join(root, "src", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg, err := FindConfig(nested)
	if err != nil {
		t.Fatalf("FindConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("FindConfig() = nil, want the config from the root")
	}
	if cfg.Path != "queue" {
		t.Errorf("Path = %q, want %q", cfg.Path, "queue")
	}
	// path resolves against the config's own directory, never the caller's.
	if want := filepath.Join(root, "queue"); cfg.TaskDir() != want {
		t.Errorf("TaskDir() = %q, want %q", cfg.TaskDir(), want)
	}
}

func TestFindConfigStopsAtTheNearestFile(t *testing.T) {
	root := testRoot(t)
	writeConfig(t, root, "version: 1\npath: outer\n")
	inner := filepath.Join(root, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, inner, "version: 1\npath: nearer\n")

	cfg, err := FindConfig(inner)
	if err != nil {
		t.Fatalf("FindConfig: %v", err)
	}
	if want := filepath.Join(inner, "nearer"); cfg.TaskDir() != want {
		t.Errorf("TaskDir() = %q, want the nearer config's %q", cfg.TaskDir(), want)
	}
}

func TestFindConfigReturnsNothingWhenThereIsNone(t *testing.T) {
	cfg, err := FindConfig(testRoot(t))
	if err != nil {
		t.Fatalf("FindConfig: %v", err)
	}
	if cfg != nil {
		t.Errorf("FindConfig() = %+v, want nil when no config exists", cfg)
	}
}

func TestFindConfigDefaultsThePath(t *testing.T) {
	root := testRoot(t)
	writeConfig(t, root, "version: 1\n")

	cfg, err := FindConfig(root)
	if err != nil {
		t.Fatalf("FindConfig: %v", err)
	}
	if cfg.Path != TaskDirName {
		t.Errorf("Path = %q, want the default %q", cfg.Path, TaskDirName)
	}
}

func TestFindConfigRejectsANewerVersion(t *testing.T) {
	root := testRoot(t)
	path := writeConfig(t, root, "version: 99\n")

	_, err := FindConfig(root)
	if err == nil {
		t.Fatal("FindConfig() = nil error, want one for a future version")
	}
	if !errors.Is(err, ErrConfig) {
		t.Errorf("err = %v, want it to wrap ErrConfig", err)
	}
	for _, want := range []string{path, "99", "newer"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to mention %q", err, want)
		}
	}
}

func TestFindConfigRejectsMalformedYAML(t *testing.T) {
	root := testRoot(t)
	path := writeConfig(t, root, "version: [1,\n")

	_, err := FindConfig(root)
	if err == nil {
		t.Fatal("FindConfig() = nil error, want one for malformed YAML")
	}
	if !errors.Is(err, ErrConfig) {
		t.Errorf("err = %v, want it to wrap ErrConfig", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("err = %q, want it to name the file", err)
	}
}

// Forward compatibility is the point of the version field: an older binary
// must read a file written by a newer one, ignoring what it does not know.
func TestFindConfigToleratesUnknownKeys(t *testing.T) {
	root := testRoot(t)
	writeConfig(t, root, "version: 1\npath: queue\ncolumns: [a, b]\nseverities: {high: 1}\n")

	cfg, err := FindConfig(root)
	if err != nil {
		t.Fatalf("FindConfig: %v", err)
	}
	if cfg.Path != "queue" {
		t.Errorf("Path = %q, want the known key to still be read", cfg.Path)
	}
}

// A near-miss filename is a typo, not an absent config: silently ignoring it
// would leave the queue somewhere the user did not intend.
func TestFindConfigRejectsTheWrongExtension(t *testing.T) {
	root := testRoot(t)
	if err := os.WriteFile(filepath.Join(root, ".taskqueue.yml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := FindConfig(root)
	if err == nil {
		t.Fatal("FindConfig() = nil error, want one naming the expected file")
	}
	if !strings.Contains(err.Error(), ConfigFileName) {
		t.Errorf("err = %q, want it to name %q", err, ConfigFileName)
	}
}

func TestFindConfigPrefersTheCanonicalName(t *testing.T) {
	root := testRoot(t)
	writeConfig(t, root, "version: 1\npath: right\n")
	if err := os.WriteFile(filepath.Join(root, ".taskqueue.yml"), []byte("path: wrong\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := FindConfig(root)
	if err != nil {
		t.Fatalf("FindConfig: %v", err)
	}
	if cfg.Path != "right" {
		t.Errorf("Path = %q, want the canonical file to win", cfg.Path)
	}
}

func TestConfigAbsolutePathIsUsedAsIs(t *testing.T) {
	root := testRoot(t)
	elsewhere := filepath.Join(testRoot(t), "queue")
	writeConfig(t, root, "version: 1\npath: "+elsewhere+"\n")

	cfg, err := FindConfig(root)
	if err != nil {
		t.Fatalf("FindConfig: %v", err)
	}
	if cfg.TaskDir() != elsewhere {
		t.Errorf("TaskDir() = %q, want %q", cfg.TaskDir(), elsewhere)
	}
}
