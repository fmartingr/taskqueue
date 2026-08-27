package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/fmartingr/taskqueue/internal/fsx"
)

// ConfigFileName is the project marker. `tq init` writes it in the directory
// it is run in, and it sits beside the task directory rather than inside it,
// which is what makes discovery unambiguous: there is one file to find, it says
// where the tasks live, and the search has a definite stopping point instead of
// guessing at directory names on the way up.
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

// ErrNoConfig reports that there is no marker to read: a walk that reached its
// boundary without finding one, which is a directory belonging to no project.
//
// It exists so that nothing in this package answers with a nil *Config and a
// nil error. That shape is what let a caller resolve a queue through a marker,
// fail to find one on a second look, and quietly conclude the project had no
// configuration — which silently replaced its board and both its vocabularies
// with the built-in sets (TQ-0087). A caller for which an absent marker really
// is fine says so, in one word, through Optional.
var ErrNoConfig = errors.New("no project marker")

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

// TaskDir is the task directory this config declares, as an absolute path. It
// is "" for a nil *Config, which is the one accessor here with no built-in
// answer to give: `path` defaults to .tasks, but with no marker there is no
// directory for it to be relative to.
func (c *Config) TaskDir() string {
	if c == nil {
		return ""
	}
	if filepath.IsAbs(c.Path) {
		return filepath.Clean(c.Path)
	}
	return filepath.Join(c.dir, c.Path)
}

// MarkerPath is the marker a command run in startDir works under: the file
// TQ_CONFIG_PATH names when it is set, and otherwise the nearest
// .taskqueue.yaml at or above startDir. It returns "" without an error only for
// the second case finding nothing.
//
// These are the only two ways to get a marker, and startDir is a directory a
// command was run in — never a task directory. The marker says where the tasks
// live; the tasks say nothing about the marker (TQ-0087).
//
// A TQ_CONFIG_PATH naming something that is not a readable file is an error
// rather than an absence. Someone who pointed tq at a marker meant that marker,
// and quietly walking somewhere else instead would put the command on a queue
// they did not ask for.
func MarkerPath(startDir string) (string, error) {
	override, err := MarkerOverride()
	if err != nil || override != "" {
		return override, err
	}
	return ConfigPath(startDir)
}

// MarkerOverride is the marker TQ_CONFIG_PATH names, absolute, or "" when the
// variable is not set. It never walks.
//
// Separate from MarkerPath because `tq init` is explicitly not discovery: it
// takes the variable when there is one and looks in the directory it was run in
// otherwise, and a marker further up belongs to another project.
func MarkerOverride() (string, error) {
	override := os.Getenv(EnvConfigPath)
	if override == "" {
		return "", nil
	}

	abs, err := filepath.Abs(override)
	if err != nil {
		return "", fmt.Errorf("%w: %s=%s: %v", ErrConfig, EnvConfigPath, override, err)
	}
	info, err := os.Stat(abs)
	switch {
	case err != nil:
		return "", fmt.Errorf("%w: %s=%s: %v", ErrConfig, EnvConfigPath, override, err)
	case info.IsDir():
		// The likeliest mistake, since the variable it replaced named one.
		return "", fmt.Errorf("%w: %s=%s is a directory; it names a %s file",
			ErrConfig, EnvConfigPath, override, ConfigFileName)
	}
	return abs, nil
}

// FindConfig reads the marker a command run in startDir works under: the one
// TQ_CONFIG_PATH names, or the nearest at or above startDir. It reports
// ErrNoConfig when the walk found none, which is how a caller learns that
// startDir belongs to no project at all.
//
// The walk stops at the first config found, and at the home directory when it
// has found none by then (see WalkBoundary).
func FindConfig(startDir string) (*Config, error) {
	path, err := MarkerPath(startDir)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, fmt.Errorf("%w: no %s at or above %s", ErrNoConfig, ConfigFileName, startDir)
	}
	return Load(path)
}

// Optional folds the one absence that is not a failure: a directory that
// belongs to no project. The accessors that have a built-in answer — the board,
// both vocabularies, the server address — give it for a nil receiver, so a
// caller that only wants the effective values, `tq --help` before there is a
// project to print, takes nil and carries on. TaskDir is the exception and says
// so.
//
// It is not for a queue, which always has a marker. Every other error still
// comes back, too: a marker that will not parse, or one tq is not allowed to
// read, is the case this must not swallow.
func Optional(cfg *Config, err error) (*Config, error) {
	if errors.Is(err, ErrNoConfig) {
		return nil, nil
	}
	return cfg, err
}

// ConfigPath is the walk on its own: the nearest marker at or above startDir,
// without reading it and without consulting the environment. It returns ""
// without an error when there is none.
//
// Callers resolving a project want MarkerPath, which is this plus the override.
// This is for the two questions that are about the tree itself: whether a
// directory has a project above it at all, and — since it does not parse —
// whether a marker is merely unreadable rather than absent.
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
		if err := reportNearMiss(dir); err != nil {
			return "", err
		}

		parent := filepath.Dir(dir)
		if dir == stopAt || parent == dir {
			return "", nil
		}
		dir = parent
	}
}

// Load reads one marker, at the path given, and nothing else.
//
// It is how a caller that already knows where the marker is — because
// discovery resolved the queue through it — reads the project's configuration:
// from the disk, on every call, without walking anywhere. A marker that has
// gone missing since is an error like any other file that cannot be read, not
// an absent one: a queue resolved through a marker has one, and answering
// otherwise is the silence TQ-0087 removed.
//
// Unknown keys are tolerated: the version field exists so that additive changes
// stay readable by older binaries, and tq never rewrites this file, so nothing
// is lost by ignoring what it does not recognise.
func Load(path string) (*Config, error) {
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

// ConfigIn reads the marker in dir itself, without walking anywhere. It
// reports ErrNoConfig when dir holds none.
//
// `tq init` is what needs this: init creates the project in the directory it
// is run in, so what a parent declares is none of its business — but a
// directory that already has a marker keeps the task directory that marker
// names, which is what makes running init twice change nothing.
func ConfigIn(dir string) (*Config, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(abs, ConfigFileName)
	if _, err := os.Stat(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s: %v", ErrConfig, path, err)
		}
		// A typo is not an absent config here either: writing the canonical
		// file beside it would leave the one the author wrote silently unread.
		if err := reportNearMiss(abs); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: no %s in %s", ErrNoConfig, ConfigFileName, abs)
	}
	return Load(path)
}

// reportNearMiss returns an error when dir holds the spelling people reach for
// instead of the one tq reads. Callers use it only where the canonical file was
// not found, which is the only case in which it could be mistaken for the
// project's marker.
func reportNearMiss(dir string) error {
	nearMiss := filepath.Join(dir, nearMissConfigName)
	if _, err := os.Stat(nearMiss); err == nil {
		return fmt.Errorf("%w: %s: tq reads %s, rename it", ErrConfig, nearMiss, ConfigFileName)
	}
	return nil
}

// WriteConfigIfMissing writes the marker into dir, and reports the path when it
// wrote one. A directory that already has one is left alone: unlike the
// generated guide, this file is the user's, whatever it says.
//
// It looks in dir and nowhere else. A marker further up belongs to another
// project, and `tq init` — the only caller — is explicitly not discovery.
func WriteConfigIfMissing(dir, taskDir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(abs, ConfigFileName)
	switch _, err := os.Stat(path); {
	case err == nil:
		return "", nil
	case !errors.Is(err, os.ErrNotExist):
		return "", fmt.Errorf("%w: %s: %v", ErrConfig, path, err)
	}
	// Checked here as well as in ConfigIn, because a caller can reach this
	// without having asked ConfigIn anything. Writing the canonical file beside
	// the typo would leave the one the author wrote unread for good.
	if err := reportNearMiss(abs); err != nil {
		return "", err
	}

	// The path is written relative to the config, so the committed file means
	// the same thing on every machine.
	rel, err := filepath.Rel(filepath.Dir(path), taskDir)
	if err != nil {
		rel = taskDir
	}
	// The board and both vocabularies are seeded here rather than left to a
	// later hand edit: a config the user never sees written would silently
	// never get one. The columns come first, since the board is the first
	// thing a project tends to want its own version of.
	body := fmt.Appendf(nil, "version: %d\npath: %s\n%s%s%s",
		ConfigVersion, filepath.ToSlash(rel),
		columnsYAML(DefaultColumns()), prioritiesYAML(DefaultPriorities()), labelsYAML(DefaultLabels()))
	if err := fsx.WriteAtomic(path, body); err != nil {
		return "", err
	}
	return path, nil
}
