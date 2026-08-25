package config

import (
	"os"
	"path/filepath"
)

// TaskDirName is the directory, relative to the project root, that holds one
// Markdown file per task. It is the default `path`, and it is meant to be
// committed to Git.
const TaskDirName = ".tasks"

// EnvTaskDir names the task directory outright, which is useful for automation
// and tests: TQ_DIR=/repo/.tasks tq list. It overrides the marker.
const EnvTaskDir = "TQ_DIR"

// EnvWalkForever lifts the bound on the search: set to "true", the walk goes
// past the repository root to the filesystem root, for one project marker
// deliberately kept above several repositories.
const EnvWalkForever = "TQ_WALK_FOREVER"

// RepositoryRoot returns the nearest directory at or above dir that holds .git.
func RepositoryRoot(dir string) (string, bool) {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// WalkBoundary is where a search up the tree stops: the repository root, so a
// marker above a project cannot capture it. Returns "" when there is no
// repository to anchor to, or when the walk-forever escape hatch is set, in
// which case the walk runs to the filesystem root.
func WalkBoundary(dir string) string {
	if os.Getenv(EnvWalkForever) == "true" {
		return ""
	}
	if root, ok := RepositoryRoot(dir); ok {
		return root
	}
	return ""
}
