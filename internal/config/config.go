package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/fmartingr/taskqueue/internal/fsx"
)

// ConfigFileName is the project marker. It sits at the root of the repository,
// not inside the task directory, which is what makes discovery unambiguous:
// there is one file to find, it says where the tasks live, and the search has a
// definite stopping point instead of guessing at directory names on the way up.
const ConfigFileName = ".taskqueue.yaml"

// ConfigVersion is the highest config version this binary understands. It only
// changes on a breaking change; everything else is additive, so a file written
// by a newer tq still reads here with the keys this version knows.
const ConfigVersion = 1

// nearMissConfigName is the spelling people reach for. Ignoring it silently
// would put the queue somewhere the author did not intend, so it is an error
// that names the file tq actually reads.
const nearMissConfigName = ".taskqueue.yml"

// ErrConfig marks a config file that exists but cannot be used. Callers
// distinguish it from a missing task directory, which is not an error at all.
var ErrConfig = errors.New("invalid config")

// Config is the project configuration. Only the keys tq understands appear
// here; unknown keys are ignored on purpose, so a file written by a newer
// version stays readable.
type Config struct {
	Version int    `yaml:"version" json:"version"`
	Path    string `yaml:"path" json:"path"`

	// Labels is the project's label vocabulary, keyed by the label as it
	// appears in task frontmatter. Read it through LabelSet, which supplies
	// the base set when the key is absent.
	Labels map[string]Label `yaml:"labels" json:"labels"`

	// Columns is the project's board, left to right: a sequence rather than a
	// mapping, because the order is the board. Read it through ColumnSet, or
	// through Board for the values the store validates and sorts by.
	Columns []BoardColumn `yaml:"columns" json:"columns"`

	// Server is where `tq serve` binds when the project pins it. Read it
	// through ServerHost and ServerPort, which answer through a nil *Config.
	Server Server `yaml:"server" json:"server"`

	// Priorities is the project's priority vocabulary, most severe first: a
	// sequence rather than a mapping, because the order is the ranking and a
	// decoded YAML mapping has none. Read it through PrioritySet, or through
	// Vocabulary for the values the store validates and sorts by.
	Priorities []Priority `yaml:"priorities" json:"priorities"`

	// File is where this config was read from, and dir is the directory
	// holding it. Path resolves against dir, never against the working
	// directory, so the same committed file means the same thing wherever a
	// command runs.
	File string `yaml:"-" json:"file"`
	dir  string
}

// TaskDir is the task directory this config declares, as an absolute path.
func (c *Config) TaskDir() string {
	if filepath.IsAbs(c.Path) {
		return filepath.Clean(c.Path)
	}
	return filepath.Join(c.dir, c.Path)
}

// FindConfig reads the nearest config at or above startDir. It returns nil
// without an error when there is none: the file is optional, and a project
// without one keeps working on the defaults.
//
// The walk stops at the first config found. It is bounded the same way task
// directory discovery is, so a stray config above a project cannot capture it.
func FindConfig(startDir string) (*Config, error) {
	path, err := ConfigPath(startDir)
	if err != nil || path == "" {
		return nil, err
	}
	return loadConfig(path)
}

// ConfigPath is the walk on its own: where the nearest config is, without
// reading it. It returns "" without an error when there is none.
//
// Separate from FindConfig because a caller that only wants to know whether the
// file moved should not need it to parse — the event stream fingerprints the
// marker twice a second, and a half-saved file, which is exactly the case that
// matters, is one that cannot be parsed at all.
func ConfigPath(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	stopAt := WalkBoundary(dir)
	for {
		path := filepath.Join(dir, ConfigFileName)
		switch _, err := os.Stat(path); {
		case err == nil:
			return path, nil
		case errors.Is(err, os.ErrPermission):
			// A config tq cannot read is not the same as one that is not
			// there: walking past it would silently use the wrong queue.
			return "", fmt.Errorf("%w: %s: %v", ErrConfig, path, err)
		}
		// Anything else — missing, or a non-directory somewhere on the way up
		// — simply means there is no config at this level.

		// A typo is not an absent config. Reported here rather than at the end
		// of the walk, so the message names the file the author actually wrote.
		nearMiss := filepath.Join(dir, nearMissConfigName)
		if _, err := os.Stat(nearMiss); err == nil {
			return "", fmt.Errorf("%w: %s: tq reads %s, rename it", ErrConfig, nearMiss, ConfigFileName)
		}

		parent := filepath.Dir(dir)
		if dir == stopAt || parent == dir {
			return "", nil
		}
		dir = parent
	}
}

// loadConfig reads one config file. Unknown keys are tolerated: the version
// field exists so that additive changes stay readable by older binaries, and
// tq never rewrites this file, so nothing is lost by ignoring what it does not
// recognise.
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrConfig, path, err)
	}

	cfg := Config{File: path, dir: filepath.Dir(path)}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrConfig, path, err)
	}
	if cfg.Version > ConfigVersion {
		return nil, fmt.Errorf("%w: %s: version %d needs a newer tq (this one understands %d)",
			ErrConfig, path, cfg.Version, ConfigVersion)
	}
	if cfg.Path == "" {
		cfg.Path = TaskDirName
	}
	if err := validateLabels(cfg.Labels); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrConfig, path, err)
	}
	if err := validatePriorities(cfg.Priorities); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrConfig, path, err)
	}
	if err := validateColumns(cfg.Columns); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrConfig, path, err)
	}
	if err := validateServer(cfg.Server); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrConfig, path, err)
	}
	return &cfg, nil
}

// configTarget is where `tq init` writes a config when a project has none: the
// root of the enclosing repository, or the working directory when there is no
// repository to anchor to.
func configTarget(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	if root, ok := RepositoryRoot(dir); ok {
		dir = root
	}
	return filepath.Join(dir, ConfigFileName), nil
}

// WriteConfigIfMissing writes the marker for a task directory that has none,
// and reports the path when it wrote one. Unlike the generated guide, this file
// is the user's: an existing one is never touched, whatever it says.
func WriteConfigIfMissing(startDir, taskDir string) (string, error) {
	existing, err := FindConfig(startDir)
	if err != nil || existing != nil {
		return "", err
	}

	path, err := configTarget(startDir)
	if err != nil {
		return "", err
	}

	// The path is written relative to the config, so the committed file means
	// the same thing on every machine.
	rel, err := filepath.Rel(filepath.Dir(path), taskDir)
	if err != nil {
		rel = taskDir
	}
	// The board and both vocabularies are seeded with the marker rather than
	// left to
	// `tq init` alone: any command can be the one that creates a queue, and a
	// config the user never sees written would silently never get one. The
	// columns come first, since the board is the first thing a project tends
	// to want its own version of.
	body := fmt.Appendf(nil, "version: %d\npath: %s\n%s%s%s",
		ConfigVersion, filepath.ToSlash(rel),
		columnsYAML(DefaultColumns()), prioritiesYAML(DefaultPriorities()), labelsYAML(DefaultLabels()))
	if err := fsx.WriteAtomic(path, body); err != nil {
		return "", err
	}
	return path, nil
}
