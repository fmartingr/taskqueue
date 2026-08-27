package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fmartingr/taskqueue/internal/config"
	"github.com/fmartingr/taskqueue/internal/tqtest"
)

// The walk stops at the home directory, having looked in it. Everything below
// home reaches it; the home directory itself is the last thing checked, so a
// marker at ~/.taskqueue.yaml is usable; and a path that is not under home at
// all cannot reach the bound, so there is none to report.
func TestWalkBoundary(t *testing.T) {
	home := filepath.Join(tqtest.RootWithoutMarker(t), "home")
	t.Setenv("HOME", home)

	for _, tc := range []struct {
		name string
		dir  string
		want string
	}{
		{"below the home directory", filepath.Join(home, "work", "project"), home},
		{"the home directory itself", home, home},
		{"the directory above it", filepath.Dir(home), ""},
		{"a path that is not under it", filepath.Join(string(filepath.Separator), "opt", "thing"), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := config.WalkBoundary(tc.dir); got != tc.want {
				t.Errorf("WalkBoundary(%q) = %q, want %q", tc.dir, got, tc.want)
			}
		})
	}
}

// A process with no home has nothing to stop at, so it gets the same answer a
// path outside one does. Asserted rather than left implicit: this is the only
// bound the design has, and it must not fail in a way nobody notices.
func TestWalkBoundaryWithoutAHomeDirectory(t *testing.T) {
	home := filepath.Join(tqtest.RootWithoutMarker(t), "home")
	project := filepath.Join(home, "work", "project")

	// The premise: with a home directory this path has a bound, so the empty
	// answer below can only be the missing HOME.
	t.Setenv("HOME", home)
	if got := config.WalkBoundary(project); got != home {
		t.Fatalf("WalkBoundary(%q) = %q, want %q", project, got, home)
	}

	t.Setenv("HOME", "")
	if got := config.WalkBoundary(project); got != "" {
		t.Errorf("WalkBoundary(%q) = %q, want \"\" when there is no home directory to stop at", project, got)
	}
}

// HOME is the spelling in the passwd entry; a working directory arrives from
// the kernel already resolved. The bound has to hold across that difference, or
// a machine with an automounted or symlinked home has no bound at all.
func TestWalkBoundaryFollowsASymlinkedHome(t *testing.T) {
	base := tqtest.RootWithoutMarker(t)
	real := filepath.Join(base, "export", "home", "u")
	project := filepath.Join(real, "work", "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	// The unresolved spelling: <base>/home/u -> <base>/export/home/u.
	link := filepath.Join(base, "home")
	if err := os.Symlink(filepath.Join(base, "export", "home"), link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(link, "u"))

	// The resolved directory, as os.Getwd would hand it over.
	if got := config.WalkBoundary(project); got != real {
		t.Errorf("WalkBoundary(%q) = %q, want the home directory %q", project, got, real)
	}
	// And the unresolved one, which the literal comparison already handles.
	unresolved := filepath.Join(link, "u", "work", "project")
	if want := filepath.Join(link, "u"); config.WalkBoundary(unresolved) != want {
		t.Errorf("WalkBoundary(%q) = %q, want %q", unresolved, config.WalkBoundary(unresolved), want)
	}
}

// A marker in the home directory is reachable from a project below it, and one
// above the home directory is not: the walk has looked at home and stopped.
func TestConfigPathStopsAtTheHomeDirectory(t *testing.T) {
	above := tqtest.RootWithoutMarker(t)
	home := filepath.Join(above, "home")
	project := filepath.Join(home, "work", "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	tqtest.WriteConfig(t, above, "version: 1\npath: "+config.TaskDirName+"\n")
	path, err := config.ConfigPath(project)
	if err != nil {
		t.Fatalf("config.ConfigPath: %v", err)
	}
	if path != "" {
		t.Errorf("ConfigPath = %q, want nothing: it sits above the home directory", path)
	}

	want := tqtest.WriteConfig(t, home, "version: 1\npath: "+config.TaskDirName+"\n")
	path, err = config.ConfigPath(project)
	if err != nil {
		t.Fatalf("config.ConfigPath: %v", err)
	}
	if path != want {
		t.Errorf("ConfigPath = %q, want the marker in the home directory %q", path, want)
	}
}

// The nearest marker wins, which is what makes `tq init` in a subdirectory of a
// project mean something.
func TestConfigPathTakesTheNearest(t *testing.T) {
	outer := tqtest.Root(t)
	inner := filepath.Join(outer, "service")
	nested := filepath.Join(inner, "src")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	want := tqtest.WriteConfig(t, inner, "version: 1\npath: "+config.TaskDirName+"\n")

	path, err := config.ConfigPath(nested)
	if err != nil {
		t.Fatalf("config.ConfigPath: %v", err)
	}
	if path != want {
		t.Errorf("ConfigPath = %q, want the nearest marker %q", path, want)
	}
}

// ConfigIn is the half of the story that does not walk: it answers about one
// directory, which is what lets `tq init` create a project without discovering
// the one above it.
func TestConfigIn(t *testing.T) {
	outer := tqtest.Root(t)
	nested := filepath.Join(outer, "src")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.ConfigIn(nested)
	if err != nil {
		t.Fatalf("config.ConfigIn: %v", err)
	}
	if cfg != nil {
		t.Errorf("ConfigIn = %+v, want nil: the marker above is not this directory's", cfg)
	}

	tqtest.WriteConfig(t, nested, "version: 1\npath: queue\n")
	cfg, err = config.ConfigIn(nested)
	if err != nil {
		t.Fatalf("config.ConfigIn: %v", err)
	}
	if cfg == nil {
		t.Fatal("ConfigIn = nil, want the marker in this directory")
	}
	if want := filepath.Join(nested, "queue"); cfg.TaskDir() != want {
		t.Errorf("TaskDir = %q, want %q", cfg.TaskDir(), want)
	}
}

// A marker that cannot be used is reported rather than read as an absent one:
// init would otherwise write a second config beside a broken first.
func TestConfigInReportsABrokenMarker(t *testing.T) {
	root := tqtest.RootWithoutMarker(t)
	tqtest.WriteConfig(t, root, "version: [1,\n")

	if _, err := config.ConfigIn(root); err == nil {
		t.Error("config.ConfigIn() = nil error, want one for malformed YAML")
	}
}

// The near-miss spelling is a typo, not an absent marker, for the directory
// `tq init` is about to write into as much as for the walk: writing the
// canonical file beside it would leave the one the author wrote unread.
func TestConfigInReportsTheWrongExtension(t *testing.T) {
	root := tqtest.RootWithoutMarker(t)
	nearMiss := filepath.Join(root, ".taskqueue.yml")
	if err := os.WriteFile(nearMiss, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.ConfigIn(root)
	if err == nil {
		t.Fatal("config.ConfigIn() = nil error, want one naming the file tq reads")
	}
	for _, want := range []string{nearMiss, config.ConfigFileName} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to mention %q", err, want)
		}
	}
}
