package config_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fmartingr/taskqueue/internal/tqtest"

	"github.com/fmartingr/taskqueue/internal/config"
)

// An absent `labels` key means "use the default", the way every other key in
// this file does — a project that has not said anything about labels still
// gets a vocabulary.
func TestLabelSetDefaultsWhenTheKeyIsAbsent(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\n")

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	set := cfg.LabelSet()
	if len(set) != len(config.DefaultLabels()) {
		t.Fatalf("LabelSet() has %d labels, want the %d defaults", len(set), len(config.DefaultLabels()))
	}
	if got := set["component/backend"]; got.DisplayName != "Backend" || got.Color == "" {
		t.Errorf("component/backend = %+v, want the default display name and a colour", got)
	}
}

// No config file at all is the same case: the defaults apply, and a nil
// *Config is what FindConfig hands back for it.
func TestLabelSetDefaultsWithoutAConfigFile(t *testing.T) {
	cfg, err := config.FindConfig(tqtest.RootWithGit(t))
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	if cfg != nil {
		t.Fatalf("config.FindConfig() = %+v, want nil", cfg)
	}
	if len(cfg.LabelSet()) != len(config.DefaultLabels()) {
		t.Errorf("LabelSet() on a nil config = %d labels, want the defaults", len(cfg.LabelSet()))
	}
}

// An explicitly empty map is a decision, not an omission: a project that wants
// no vocabulary must be able to say so.
func TestLabelSetHonoursAnEmptyMap(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\nlabels: {}\n")

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	if got := cfg.LabelSet(); len(got) != 0 {
		t.Errorf("LabelSet() = %v, want it empty", got)
	}
}

func TestLabelSetReplacesTheDefaults(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\nlabels:\n  urgent-ish:\n    color: \"#ff0000\"\n    display_name: Hot\n")

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	set := cfg.LabelSet()
	if len(set) != 1 {
		t.Fatalf("LabelSet() = %v, want only the configured label", set)
	}
	if set["urgent-ish"].DisplayName != "Hot" || set["urgent-ish"].Color != "#ff0000" {
		t.Errorf("urgent-ish = %+v", set["urgent-ish"])
	}
}

// The key is what the CLI takes, so it is also the fallback for the board.
func TestLabelDisplayNameFallsBackToTheKey(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\nlabels:\n  bug:\n    color: \"#d73a4a\"\n")

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	if got := cfg.LabelSet()["bug"].DisplayName; got != "bug" {
		t.Errorf("DisplayName = %q, want the key %q", got, "bug")
	}
}

// `color: #ff0000` is a YAML comment, so the value parses as null. Rendering
// that as an empty colour would be a silent, invisible bug in the board.
func TestLabelRejectsAnUnquotedColour(t *testing.T) {
	root := tqtest.Root(t)
	path := tqtest.WriteConfig(t, root, "version: 1\nlabels:\n  bug:\n    color: #d73a4a\n")

	_, err := config.FindConfig(root)
	if err == nil {
		t.Fatal("config.FindConfig() = nil error, want one for an unquoted colour")
	}
	if !errors.Is(err, config.ErrConfig) {
		t.Errorf("err = %v, want it to wrap config.ErrConfig", err)
	}
	for _, want := range []string{path, "bug", "quote"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to mention %q", err, want)
		}
	}
}

func TestLabelRejectsANonHexColour(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\nlabels:\n  bug:\n    color: red\n")

	_, err := config.FindConfig(root)
	if err == nil {
		t.Fatal("config.FindConfig() = nil error, want one for a non-hex colour")
	}
	for _, want := range []string{"bug", "red"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to mention %q", err, want)
		}
	}
}

func TestLabelAcceptsBothHexLengths(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\nlabels:\n  a:\n    color: \"#FFF\"\n  b:\n    color: \"#00ff00\"\n")

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	if len(cfg.LabelSet()) != 2 {
		t.Errorf("LabelSet() = %v, want both", cfg.LabelSet())
	}
}

func TestLabelRejectsABlankKey(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\nlabels:\n  \"  \":\n    color: \"#ffffff\"\n")

	if _, err := config.FindConfig(root); err == nil {
		t.Fatal("config.FindConfig() = nil error, want one for a blank label key")
	}
}

// LabelSet hands out a copy: a caller that edits what it got must not be
// editing the config every later reader sees.
func TestLabelSetIsACopy(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\nlabels:\n  bug:\n    color: \"#d73a4a\"\n")

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	cfg.LabelSet()["bug"] = config.Label{Color: "#000000"}
	if got := cfg.LabelSet()["bug"].Color; got != "#d73a4a" {
		t.Errorf("colour = %q after editing a returned set, want it untouched", got)
	}
}

// The base set is what `tq init` seeds, so it has to satisfy the rules the
// loader enforces on a hand-written file.
func TestDefaultLabelsAreValid(t *testing.T) {
	defaults := config.DefaultLabels()
	if len(defaults) == 0 {
		t.Fatal("DefaultLabels() is empty")
	}
	seen := map[string]bool{}
	for _, label := range defaults {
		if seen[label.Name] {
			t.Errorf("%q appears twice in the base set", label.Name)
		}
		seen[label.Name] = true
		if label.DisplayName == "" {
			t.Errorf("%q has no display name", label.Name)
		}
		if !config.ValidLabelColor(label.Color) {
			t.Errorf("%q has colour %q, which the loader would reject", label.Name, label.Color)
		}
	}
	for _, want := range []string{"bug", "feature", "component/backend", "component/config"} {
		if !seen[want] {
			t.Errorf("the base set is missing %q", want)
		}
	}
}

// The order is the file's order, not a map's: the seeded config and every
// listing have to come out the same way twice.
func TestDefaultLabelsAreOrdered(t *testing.T) {
	first := config.DefaultLabels()
	second := config.DefaultLabels()
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("DefaultLabels()[%d] = %+v then %+v", i, first[i], second[i])
		}
	}
}

func TestSortedLabelsOrdersByName(t *testing.T) {
	got := config.SortedLabels(map[string]config.Label{
		"zebra": {Color: "#ffffff", DisplayName: "Zebra"},
		"apple": {Color: "#000000", DisplayName: "Apple"},
	})
	if len(got) != 2 || got[0].Name != "apple" || got[1].Name != "zebra" {
		t.Errorf("SortedLabels() = %+v, want apple before zebra", got)
	}
}

// The seeded block has to read back as what was seeded: it is written by hand
// rather than marshalled, so nothing but a test keeps the two in step.
func TestWriteConfigSeedsTheBaseSet(t *testing.T) {
	// WriteConfigIfMissing only writes where there is no marker.
	root := tqtest.RootWithGit(t)
	path, err := config.WriteConfigIfMissing(root, filepath.Join(root, config.TaskDirName))
	if err != nil {
		t.Fatalf("config.WriteConfigIfMissing: %v", err)
	}
	if path == "" {
		t.Fatal("config.WriteConfigIfMissing() wrote nothing")
	}

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	set := cfg.LabelSet()
	for _, want := range config.DefaultLabels() {
		got, ok := set[want.Name]
		if !ok {
			t.Errorf("%q was seeded but did not read back", want.Name)
			continue
		}
		if got != want.Label {
			t.Errorf("%q read back as %+v, want %+v", want.Name, got, want.Label)
		}
	}
	if len(set) != len(config.DefaultLabels()) {
		t.Errorf("read back %d labels, want %d", len(set), len(config.DefaultLabels()))
	}

	// And it is the file's own labels being read, not the defaults standing in
	// for an absent key.
	if cfg.Labels == nil {
		t.Error("the seeded config has no labels key of its own")
	}
}
