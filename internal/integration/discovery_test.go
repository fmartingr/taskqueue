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

// What `tq init` says about the guide has to hold from the directory it was run
// in, and the only path that does is the guide's own absolute one. It used to
// print a relative include resolved against a base tq guessed, which named the
// wrong file from anywhere else (TQ-0061). Both surfaces are checked here: the
// human line on stdout and the `pointer` key agents read.
func TestInitNamesTheGuideByItsAbsolutePath(t *testing.T) {
	t.Parallel()

	// stdout carries the guide's path once, in a line a user can act on, and
	// the invitation to reference it from a file of their choosing.
	assertPrints := func(t *testing.T, r result, guide string) {
		t.Helper()
		if want := "\nThe agent guide is at:\n\n    " + guide + "\n"; !strings.Contains(r.Stdout, want) {
			t.Errorf("stdout does not name the guide:\ngot:\n%s\nwant it to contain:\n%s", r.Stdout, want)
		}
		if !strings.Contains(r.Stdout, "Include it in your preferred agent context file") {
			t.Errorf("stdout does not say what to do with it:\n%s", r.Stdout)
		}
	}

	assertPointer := func(t *testing.T, r result, guide string) {
		t.Helper()
		var out struct {
			TaskDir string `json:"task_dir"`
			Pointer string `json:"pointer"`
		}
		r.JSON(t, &out)
		if out.Pointer != guide {
			t.Errorf("pointer = %q, want %q", out.Pointer, guide)
		}
		if out.TaskDir != filepath.Dir(guide) {
			t.Errorf("task_dir = %q, want the directory holding %q", out.TaskDir, guide)
		}
	}

	t.Run("inside a repository", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		p := &project{dir: dir}
		guide := realPath(t, dir, ".tasks", "AGENTS.md")

		assertPrints(t, p.mustRun(t, "init"), guide)
		assertPointer(t, p.mustRun(t, "init", "--json"), guide)
	})

	t.Run("from a subdirectory", func(t *testing.T) {
		t.Parallel()
		p := newProject(t)
		p.mustRun(t, "init")
		deep := p.path("backend", "deep")
		if err := os.MkdirAll(deep, 0o755); err != nil {
			t.Fatal(err)
		}
		// The queue is two levels up, so a relative path was meaningless from
		// here: it named backend/deep/.tasks, which does not exist.
		guide := realPath(t, p.dir, ".tasks", "AGENTS.md")

		assertPrints(t, p.runIn(t, deep, nil, "init"), guide)
		assertPointer(t, p.runIn(t, deep, nil, "init", "--json"), guide)
	})

	t.Run("no repository root, with TQ_DIR outside the project", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		dir := filepath.Join(base, "project")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		p := &project{dir: dir}
		// Relative, the way a shell hands it over, and pointing sideways: the
		// shape the ticket reproduced, where tq answered "@queue/AGENTS.md".
		env := []string{"TQ_DIR=../elsewhere/queue"}
		guide := realPath(t, base, "elsewhere", "queue", "AGENTS.md")

		assertPrints(t, p.runIn(t, dir, env, "init"), guide)
		assertPointer(t, p.runIn(t, dir, env, "init", "--json"), guide)
	})
}

// A project above the repository is one discovery deliberately walked past, and
// init is the command whose job is saying where the queue is: it names the
// marker rather than staying silent (TQ-0062). The note is a note, so it goes to
// stderr — which is the half only a real process can show, and what the --json
// contract rests on.
func TestInitNamesTheProjectTheBoundExcluded(t *testing.T) {
	t.Parallel()

	// above holds the marker discovery will not reach; the repository below is
	// where init runs and makes its own queue.
	shadowed := func(t *testing.T) (*project, string) {
		t.Helper()
		above := t.TempDir()
		if err := writeFile(filepath.Join(above, ".taskqueue.yaml"), "version: 1\npath: .tasks\n"); err != nil {
			t.Fatal(err)
		}
		if err := mkdirAll(filepath.Join(above, ".tasks")); err != nil {
			t.Fatal(err)
		}
		repo := filepath.Join(above, "project")
		if err := mkdirAll(filepath.Join(repo, ".git")); err != nil {
			t.Fatal(err)
		}
		return &project{dir: repo}, realPath(t, above, ".taskqueue.yaml")
	}

	t.Run("the note is on stderr", func(t *testing.T) {
		t.Parallel()
		p, marker := shadowed(t)

		r := p.mustRun(t, "init")
		for _, want := range []string{marker, "TQ_WALK_FOREVER"} {
			if !strings.Contains(r.Stderr, want) {
				t.Errorf("stderr = %q, want it to mention %q", r.Stderr, want)
			}
		}
		if strings.Contains(r.Stdout, "TQ_WALK_FOREVER") {
			t.Errorf("the note reached stdout: %q", r.Stdout)
		}
		// init still says what it always said, on the stream it always said it.
		if !strings.Contains(r.Stdout, "Initialized task queue in "+realPath(t, p.dir, ".tasks")) {
			t.Errorf("stdout = %q, want init's own line", r.Stdout)
		}
	})

	// A project without Git had no bound, so the search already went as far as
	// TQ_WALK_FOREVER would have taken it and excluded nothing. The store pins
	// the guard itself; what a real process adds is the note reaching stderr,
	// answered from the working directory tq was run in — and here that
	// directory holds the project's own marker, the one file a mistake in the
	// guard would name back at the caller (TQ-0064).
	t.Run("a project without a repository excludes nothing", func(t *testing.T) {
		t.Parallel()
		p := newProject(t)
		requireNoRepositoryAbove(t, p.dir)

		r := p.mustRun(t, "init")
		if !strings.Contains(r.Stdout, "Initialized task queue in "+realPath(t, p.dir, ".tasks")) {
			t.Fatalf("stdout = %q, want init to have made this project its queue", r.Stdout)
		}
		for _, unwanted := range []string{"TQ_WALK_FOREVER", "was not used"} {
			if strings.Contains(r.Stderr, unwanted) {
				t.Errorf("stderr = %q, want no mention of %q: without a repository nothing bounded the search", r.Stderr, unwanted)
			}
		}
	})

	t.Run("--json keeps stdout machine-readable", func(t *testing.T) {
		t.Parallel()
		p, marker := shadowed(t)

		r := p.mustRun(t, "init", "--json")
		var out struct {
			TaskDir string `json:"task_dir"`
			Created bool   `json:"created"`
		}
		r.JSON(t, &out)
		if want := realPath(t, p.dir, ".tasks"); out.TaskDir != want {
			t.Errorf("task_dir = %q, want %q", out.TaskDir, want)
		}
		if !out.Created {
			t.Error("created = false, want init to have made the repository its own queue")
		}
		if !strings.Contains(r.Stderr, marker) {
			t.Errorf("stderr = %q, want the note even with --json", r.Stderr)
		}
	})
}
