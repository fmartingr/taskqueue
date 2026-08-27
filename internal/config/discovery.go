package config

import (
	"os"
	"path/filepath"
	"strings"
)

// TaskDirName is the directory, relative to the project root, that holds one
// Markdown file per task. It is the default `path`, and it is meant to be
// committed to Git.
const TaskDirName = ".tasks"

// EnvConfigPath names the project marker outright, which is useful for
// automation and tests: TQ_CONFIG_PATH=/repo/.taskqueue.yaml tq list. It stands
// in for the walk, and what it names is a marker like any other — so the tasks
// are still wherever its `path` says, resolved against the directory holding it.
//
// A file, not a directory. Handing a command the marker rather than the queue
// is what makes the rule total: every command has a marker, and nothing has to
// decide what a project with no configuration would even mean (TQ-0087).
const EnvConfigPath = "TQ_CONFIG_PATH"

// WalkBoundary is where a search up the tree stops: the home directory, which
// the walk checks and then goes no further. A marker at ~/.taskqueue.yaml is
// usable, and one in a sibling user's tree is not.
//
// It returns "" when there is no home directory to stop at — a path outside it
// (/opt/thing, or the temporary directories macOS hands out under /var/folders)
// and a process with no HOME both land here. The walk then runs to the
// filesystem root, so it is bounded by the tree rather than by a directory: it
// will use a marker it meets on the way up, and only runs out of tree when
// there is none.
//
// dir must be absolute and clean; every caller comes through filepath.Abs.
func WalkBoundary(dir string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	home = filepath.Clean(home)
	if under(home, dir) {
		return home
	}

	// A literal comparison misses a home reached through a symlink: HOME is
	// usually the spelling in the passwd entry, while the working directory
	// arrives already resolved, because that is what the kernel answers with.
	// /home/u against an automounted /export/home/u is the shape. Resolving is
	// the fallback rather than the rule — it costs syscalls, and it fails on a
	// path that is not there.
	realHome, homeErr := filepath.EvalSymlinks(home)
	realDir, dirErr := filepath.EvalSymlinks(dir)
	if homeErr != nil || dirErr != nil || !under(realHome, realDir) {
		return ""
	}
	// The answer has to be a directory the walk will actually arrive at, and
	// the walk climbs dir as it was given. So climb it by as many levels as
	// separate the resolved pair, which lands on dir's own spelling of home.
	stopAt := dir
	for range depth(realHome, realDir) {
		stopAt = filepath.Dir(stopAt)
	}
	return stopAt
}

// under reports whether dir is base or sits inside it.
func under(base, dir string) bool {
	rel, err := filepath.Rel(base, dir)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// depth is how many levels dir sits below base. Callers check `under` first.
func depth(base, dir string) int {
	rel, err := filepath.Rel(base, dir)
	if err != nil || rel == "." {
		return 0
	}
	return len(strings.Split(rel, string(filepath.Separator)))
}
