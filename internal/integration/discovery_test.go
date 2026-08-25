//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The version is stamped by the build, and only a real binary can show that the
// stamp arrived.
func TestVersionReportsTheStampedString(t *testing.T) {
	t.Parallel()
	p := newProject(t)

	if out := p.mustRun(t, "version").Stdout; !strings.Contains(out, stampedVersion) {
		t.Errorf("tq version = %q, want the stamped %q", out, stampedVersion)
	}

	var v map[string]string
	p.mustRun(t, "version", "--json").JSON(t, &v)
	if v["version"] != stampedVersion {
		t.Errorf("version json = %q, want %q", v["version"], stampedVersion)
	}

	// The server reports the same string.
	srv := p.serve(t)
	var got map[string]string
	srv.get(t, "/api/version", &got)
	if got["version"] != stampedVersion {
		t.Errorf("api version = %q, want %q", got["version"], stampedVersion)
	}
}

// The guide's promise: any subdirectory of the project reaches the same queue.
func TestRunsFromASubdirectory(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "from the root")

	deep := p.path("src", "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	var listed []taskJSON
	p.runIn(t, deep, nil, "list", "--json").JSON(t, &listed)
	if len(listed) != 1 {
		t.Fatalf("list from a subdirectory = %d tasks, want 1", len(listed))
	}

	p.runIn(t, deep, nil, "add", "from below")
	if _, err := os.Stat(filepath.Join(deep, ".tasks")); !os.IsNotExist(err) {
		t.Error("a subdirectory must not get a queue of its own")
	}
}

// The marker decides where the tasks live, and TQ_DIR overrides even that.
func TestMarkerAndOverride(t *testing.T) {
	t.Parallel()

	t.Run("path moves the queue", func(t *testing.T) {
		t.Parallel()
		p := newProject(t)
		if err := os.WriteFile(p.path(".taskqueue.yaml"), []byte("version: 1\npath: docs/queue\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		p.mustRun(t, "add", "moved")

		if _, err := os.Stat(p.path("docs", "queue")); err != nil {
			t.Errorf("the queue should follow path: %v", err)
		}
		if _, err := os.Stat(p.path(".tasks")); !os.IsNotExist(err) {
			t.Error("the default directory should not have been created")
		}
	})

	t.Run("TQ_DIR beats the marker", func(t *testing.T) {
		t.Parallel()
		p := newProject(t)
		elsewhere := p.path("elsewhere")
		if err := os.MkdirAll(elsewhere, 0o755); err != nil {
			t.Fatal(err)
		}
		p.runIn(t, p.dir, []string{"TQ_DIR=" + elsewhere}, "add", "via the environment")

		entries, err := os.ReadDir(elsewhere)
		if err != nil || len(entries) != 1 {
			t.Errorf("TQ_DIR holds %d entries, %v; want the task", len(entries), err)
		}
		if _, err := os.Stat(p.path(".tasks")); !os.IsNotExist(err) {
			t.Error("the marker's path should have been ignored")
		}
	})

	t.Run("a directory named .tasks is not a queue", func(t *testing.T) {
		t.Parallel()
		// No marker at all, and a .tasks above: tq looks for the marker and
		// nothing else, so this project gets its own queue.
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".tasks"), 0o755); err != nil {
			t.Fatal(err)
		}
		below := filepath.Join(dir, "project")
		if err := os.MkdirAll(below, 0o755); err != nil {
			t.Fatal(err)
		}
		p := &project{dir: below}
		p.mustRun(t, "add", "mine")

		if _, err := os.Stat(filepath.Join(below, ".taskqueue.yaml")); err != nil {
			t.Errorf("the project should have gained its own marker: %v", err)
		}
		entries, _ := os.ReadDir(filepath.Join(dir, ".tasks"))
		if len(entries) != 0 {
			t.Errorf("the directory above was written into: %d entries", len(entries))
		}
	})
}

// A config tq cannot use is reported as such, and never as a missing queue.
func TestBrokenConfigsAreReported(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		file     string
		body     string
		contains string
	}{
		{"future version", ".taskqueue.yaml", "version: 99\n", "newer tq"},
		{"malformed yaml", ".taskqueue.yaml", "version: [1,\n", "invalid config"},
		{"wrong extension", ".taskqueue.yml", "version: 1\n", ".taskqueue.yaml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tc.file), []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			p := &project{dir: dir}

			r := p.run(t, "list")
			if r.Code != 1 {
				t.Errorf("exit = %d, want 1\nstderr: %s", r.Code, r.Stderr)
			}
			if !strings.Contains(r.Stderr, tc.contains) {
				t.Errorf("stderr = %q, want it to mention %q", r.Stderr, tc.contains)
			}
		})
	}
}

// init writes both files, and never overwrites a config a person wrote.
func TestInitWritesTheMarkerAndTheGuide(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := &project{dir: dir}

	p.mustRun(t, "init")
	for _, name := range []string{".taskqueue.yaml", filepath.Join(".tasks", "AGENTS.md")} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("init did not write %s: %v", name, err)
		}
	}

	mine := "version: 1\npath: .tasks\n# hand written\n"
	if err := os.WriteFile(filepath.Join(dir, ".taskqueue.yaml"), []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	p.mustRun(t, "init")
	after, err := os.ReadFile(filepath.Join(dir, ".taskqueue.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != mine {
		t.Errorf("init rewrote a config it did not author:\n%s", after)
	}
}
