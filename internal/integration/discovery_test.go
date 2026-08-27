//go:build integration

package integration

import (
	"fmt"
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
		// The marker is written before init, so init puts the queue where it
		// says rather than at the default.
		p := &project{dir: bareDir(t)}
		if err := writeFile(p.path(".taskqueue.yaml"), "version: 1\npath: docs/queue\n"); err != nil {
			t.Fatal(err)
		}
		p.mustRun(t, "init")
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
		p := &project{dir: bareDir(t)}
		if err := writeFile(p.path(".taskqueue.yaml"), "version: 1\npath: .tasks\n"); err != nil {
			t.Fatal(err)
		}
		elsewhere := p.path("elsewhere")
		env := []string{"TQ_DIR=" + elsewhere}
		p.runIn(t, p.dir, env, "init")
		p.runIn(t, p.dir, env, "add", "via the environment")

		entries, err := os.ReadDir(elsewhere)
		if err != nil || len(entries) != 2 {
			t.Errorf("TQ_DIR holds %d entries, %v; want the task and the guide", len(entries), err)
		}
		if _, err := os.Stat(p.path(".tasks")); !os.IsNotExist(err) {
			t.Error("the marker's path should have been ignored")
		}
	})

	t.Run("a directory named .tasks is not a queue", func(t *testing.T) {
		t.Parallel()
		// No marker at all, and a .tasks above: tq looks for the marker and
		// nothing else, so there is no project here to find.
		dir := bareDir(t)
		if err := mkdirAll(filepath.Join(dir, ".tasks")); err != nil {
			t.Fatal(err)
		}
		below := filepath.Join(dir, "project")
		if err := mkdirAll(below); err != nil {
			t.Fatal(err)
		}
		p := &project{dir: below}

		r := p.run(t, "add", "mine")
		if r.Code != 3 {
			t.Errorf("exit = %d, want 3\nstderr: %s", r.Code, r.Stderr)
		}
		if !strings.Contains(r.Stderr, "tq init") {
			t.Errorf("stderr = %q, want it to name tq init", r.Stderr)
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
			dir := bareDir(t)
			if err := os.WriteFile(filepath.Join(dir, tc.file), []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			p := &project{dir: dir}

			// init as well as list: it reads the marker in the directory it is
			// about to write into, and a file it cannot use must stop it rather
			// than have it write a second one beside the first.
			for _, command := range []string{"list", "init"} {
				r := p.run(t, command)
				if r.Code != 1 {
					t.Errorf("tq %s = exit %d, want 1\nstderr: %s", command, r.Code, r.Stderr)
				}
				if !strings.Contains(r.Stderr, tc.contains) {
					t.Errorf("tq %s stderr = %q, want it to mention %q", command, r.Stderr, tc.contains)
				}
			}
		})
	}
}

// init writes both files, and never overwrites a config a person wrote.
func TestInitWritesTheMarkerAndTheGuide(t *testing.T) {
	t.Parallel()
	dir := bareDir(t)
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

	t.Run("in a directory that is not a project yet", func(t *testing.T) {
		t.Parallel()
		dir := bareDir(t)
		p := &project{dir: dir}
		guide := realPath(t, dir, ".tasks", "AGENTS.md")

		assertPrints(t, p.mustRun(t, "init"), guide)
		assertPointer(t, p.mustRun(t, "init", "--json"), guide)
	})

	t.Run("in a subdirectory of a project", func(t *testing.T) {
		t.Parallel()
		p := newProject(t)
		deep := p.path("backend", "deep")
		if err := mkdirAll(deep); err != nil {
			t.Fatal(err)
		}
		// Init creates the queue where it is run, so the guide it names is the
		// one it just wrote here — not the project's two levels up.
		guide := realPath(t, deep, ".tasks", "AGENTS.md")

		assertPrints(t, p.runIn(t, deep, nil, "init"), guide)
		assertPointer(t, p.runIn(t, deep, nil, "init", "--json"), guide)
	})

	t.Run("with TQ_DIR outside the project", func(t *testing.T) {
		t.Parallel()
		base := bareDir(t)
		dir := filepath.Join(base, "project")
		if err := mkdirAll(dir); err != nil {
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

// The two rules TQ-0085 settled on, driven through the binary: init creates the
// queue where it is run, and every other command walks up for the marker and
// fails with exit 3 when there is none.
func TestInitCreatesTheQueueWhereItIsRun(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "the parent's work")

	deep := p.path("service")
	if err := mkdirAll(deep); err != nil {
		t.Fatal(err)
	}
	sub := &project{dir: deep}
	sub.mustRun(t, "init")

	for _, name := range []string{".taskqueue.yaml", filepath.Join(".tasks", "AGENTS.md")} {
		if _, err := os.Stat(filepath.Join(deep, name)); err != nil {
			t.Errorf("init did not write %s in the subdirectory: %v", name, err)
		}
	}

	// And commands below it use the marker init just wrote, not the parent's.
	var listed []taskJSON
	sub.mustRun(t, "list", "--json").JSON(t, &listed)
	if len(listed) != 0 {
		t.Errorf("the subdirectory listed %d tasks, want its own empty queue", len(listed))
	}
}

// Running init twice leaves everything exactly as it was.
func TestInitTwiceChangesNothing(t *testing.T) {
	t.Parallel()
	dir := bareDir(t)
	p := &project{dir: dir}
	p.mustRun(t, "init")

	before := snapshot(t, dir)
	r := p.mustRun(t, "init")
	if strings.Contains(r.Stdout, "Wrote ") {
		t.Errorf("the second init reported a write: %q", r.Stdout)
	}
	if !strings.Contains(r.Stdout, "already initialized") {
		t.Errorf("stdout = %q, want it to say the queue was already there", r.Stdout)
	}
	if after := snapshot(t, dir); after != before {
		t.Errorf("the second init changed the project:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// No marker anywhere up to the home directory: the command fails with exit 3,
// names tq init on stderr, keeps stdout clean and creates nothing.
func TestCommandsFailWithoutAProject(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"list"},
		{"list", "--json"},
		{"ready", "--json"},
		{"add", "something"},
		{"show", "TQ-0001"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			dir := bareDir(t)
			p := &project{dir: dir}

			r := p.run(t, args...)
			if r.Code != 3 {
				t.Errorf("exit = %d, want 3\nstderr: %s", r.Code, r.Stderr)
			}
			for _, want := range []string{".taskqueue.yaml", "tq init"} {
				if !strings.Contains(r.Stderr, want) {
					t.Errorf("stderr = %q, want it to mention %q", r.Stderr, want)
				}
			}
			if r.Stdout != "" {
				t.Errorf("stdout = %q, want the error on stderr alone", r.Stdout)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Errorf("the command created %d entries, want none", len(entries))
			}
		})
	}
}

// A marker whose task directory is gone is the one failure that knows where the
// queue belongs. Following a bare "run tq init" from a subdirectory would fork a
// second project, so the message names the marker's own directory instead.
func TestAMissingQueueSaysWhereToInitialiseIt(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	if err := os.Remove(p.path(".tasks")); err != nil {
		t.Fatal(err)
	}
	deep := p.path("src", "deep")
	if err := mkdirAll(deep); err != nil {
		t.Fatal(err)
	}

	r := p.runIn(t, deep, nil, "list")
	if r.Code != 3 {
		t.Fatalf("exit = %d, want 3\nstderr: %s", r.Code, r.Stderr)
	}
	if want := `run "tq init" in ` + realPath(t, p.dir); !strings.Contains(r.Stderr, want) {
		t.Errorf("stderr = %q, want it to say %q", r.Stderr, want)
	}
}

// The nearest marker wins, and a .git in the way bounds nothing: a submodule
// reads the superproject's queue rather than forking one (TQ-0059).
func TestTheNearestMarkerWins(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "the parent's work")

	submodule := p.path("vendor", "dep")
	if err := mkdirAll(filepath.Join(submodule, ".git")); err != nil {
		t.Fatal(err)
	}
	var listed []taskJSON
	p.runIn(t, submodule, nil, "list", "--json").JSON(t, &listed)
	if len(listed) != 1 {
		t.Errorf("a directory with its own .git listed %d tasks, want the superproject's 1", len(listed))
	}
	if _, err := os.Stat(filepath.Join(submodule, ".taskqueue.yaml")); !os.IsNotExist(err) {
		t.Errorf("the submodule gained a marker of its own: %v", err)
	}

	// A marker of its own is what takes it off the parent's queue.
	inner := p.path("service")
	if err := mkdirAll(inner); err != nil {
		t.Fatal(err)
	}
	sub := &project{dir: inner}
	sub.mustRun(t, "init")
	sub.mustRun(t, "add", "the service's work")

	deep := filepath.Join(inner, "src")
	if err := mkdirAll(deep); err != nil {
		t.Fatal(err)
	}
	var below []taskJSON
	p.runIn(t, deep, nil, "list", "--json").JSON(t, &below)
	if len(below) != 1 || below[0].Title != "the service's work" {
		t.Errorf("list below the nearer marker = %+v, want the service's own queue", below)
	}
}

// TQ_WALK_FOREVER is gone: nothing the binary prints may still name it, and no
// value of it may change what a command does.
func TestWalkForeverIsGone(t *testing.T) {
	t.Parallel()
	p := newProject(t)

	// init first, so the guide read below is one this binary just generated.
	for _, args := range [][]string{{"init"}, {"help"}} {
		r := p.mustRun(t, args...)
		if strings.Contains(r.Stdout+r.Stderr, "TQ_WALK_FOREVER") {
			t.Errorf("tq %s still names TQ_WALK_FOREVER:\nstdout: %s\nstderr: %s", strings.Join(args, " "), r.Stdout, r.Stderr)
		}
	}
	guide, err := os.ReadFile(p.path(".tasks", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(guide), "TQ_WALK_FOREVER") {
		t.Error("the generated guide still documents TQ_WALK_FOREVER")
	}

	// A marker above the home directory stays out of reach whatever the
	// variable is set to.
	above := bareDir(t)
	dir := filepath.Join(above, "home", "project")
	if err := mkdirAll(dir); err != nil {
		t.Fatal(err)
	}
	outer := &project{dir: above}
	outer.mustRun(t, "init")
	// Resolved, because the child process answers from the kernel's idea of its
	// working directory and an unresolved HOME would not match it.
	home := realPath(t, filepath.Join(above, "home"))

	r := (&project{dir: dir}).runIn(t, dir, []string{"HOME=" + home, "TQ_WALK_FOREVER=true"}, "list")
	if r.Code != 3 {
		t.Errorf("exit = %d, want 3: the variable must not lift a bound that no longer exists\nstderr: %s", r.Code, r.Stderr)
	}
}

// snapshot is every file under dir with its contents, so a test can say that a
// command changed nothing at all.
func snapshot(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		fmt.Fprintf(&b, "%s\n%s\n", path, body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}
