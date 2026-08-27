package config

import (
	"fmt"
	"maps"
	"regexp"
	"sort"
	"strings"
)

// Label is how the project draws one label. The map key it is stored under is
// the label exactly as it appears in task frontmatter, which is also what the
// CLI takes; nothing here changes how a label is stored or matched.
type Label struct {
	Color       string `yaml:"color" json:"color"`
	DisplayName string `yaml:"display_name" json:"display_name"`
}

// NamedLabel is a label with its key, so an ordered list of them survives
// being written to a file or printed: a Go map has no order to offer.
type NamedLabel struct {
	Name string `json:"name"`
	Label
}

// LabelSeparator groups labels for display, the way GitLab groups scoped
// labels. It is a display rule only: storage stays one flat string, so
// `tq list --label component/backend` matches the whole key.
const LabelSeparator = "/"

// hexColor is the only colour spelling accepted. Anything else is a mistake
// worth naming: the board has no way to draw "red" or "".
var hexColor = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// ValidLabelColor reports whether a colour is one the board can draw.
func ValidLabelColor(color string) bool { return hexColor.MatchString(color) }

// baseLabels is the vocabulary a new project starts with: flat type labels
// first, then the grouped components. Colours are a starting point — a project
// edits them in its own config, which is why they are seeded rather than
// hard-coded into the board.
var baseLabels = []NamedLabel{
	{"bug", Label{"#d73a4a", "Bug"}},
	{"feature", Label{"#0e8a16", "Feature"}},
	{"chore", Label{"#8b949e", "Chore"}},
	{"docs", Label{"#0075ca", "Docs"}},
	{"refactor", Label{"#6f42c1", "Refactor"}},
	{"security", Label{"#b60205", "Security"}},
	{"performance", Label{"#fbca04", "Performance"}},
	{"tests", Label{"#5319e7", "Tests"}},
	{"component/backend", Label{"#1d76db", "Backend"}},
	{"component/frontend", Label{"#c5def5", "Frontend"}},
	{"component/cli", Label{"#0052cc", "CLI"}},
	{"component/api", Label{"#006b75", "API"}},
	{"component/store", Label{"#5319e7", "Store"}},
	{"component/ci", Label{"#bfd4f2", "CI"}},
	{"component/build", Label{"#d4c5f9", "Build"}},
	{"component/config", Label{"#c2e0c6", "Config"}},
}

// DefaultLabels is the base set, in the order it is written and listed.
func DefaultLabels() []NamedLabel {
	out := make([]NamedLabel, len(baseLabels))
	copy(out, baseLabels)
	return out
}

// LabelSet is the vocabulary this config declares, as a map keyed by the label
// as it appears in frontmatter.
//
// An absent `labels` key means "use the base set", the way an absent `path`
// means ".tasks"; an explicitly empty one (`labels: {}`) means the project
// wants no vocabulary at all. A nil *Config — no config file — is the absent
// case, so callers never have to check before asking.
//
// The set is a reference, not a restriction. A label outside it is accepted
// everywhere and simply has nothing to draw it with.
func (c *Config) LabelSet() map[string]Label {
	if c == nil || c.Labels == nil {
		set := make(map[string]Label, len(baseLabels))
		for _, label := range baseLabels {
			set[label.Name] = label.Label
		}
		return set
	}
	return maps.Clone(c.Labels)
}

// SortedLabels puts a label set in a stable order, so the same config prints
// the same way twice.
func SortedLabels(set map[string]Label) []NamedLabel {
	out := make([]NamedLabel, 0, len(set))
	for name, label := range set {
		out = append(out, NamedLabel{Name: name, Label: label})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// validateLabels rejects a label set the board could not draw, and fills in the
// display name a label did not give: the key is what the CLI takes, so it is
// the honest fallback for what the board shows.
func validateLabels(set map[string]Label) error {
	for _, name := range sortedKeys(set) {
		label := set[name]
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("a label key is empty")
		}
		switch {
		case label.Color == "":
			// `color: #d73a4a` is a YAML comment, so the value parses as null
			// and lands here. Naming the quoting is the whole point.
			return fmt.Errorf("label %q has no colour (quote it, as in: color: \"#d73a4a\")", name)
		case !ValidLabelColor(label.Color):
			return fmt.Errorf("label %q has colour %q, want a hex colour like \"#d73a4a\"", name, label.Color)
		}
		if label.DisplayName == "" {
			label.DisplayName = name
			set[name] = label
		}
	}
	return nil
}

func sortedKeys(set map[string]Label) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// labelsYAML renders a label set as the `labels:` block of a config file,
// comment and all. It is written by hand rather than marshalled so the order
// is the base set's and the comment survives.
func labelsYAML(labels []NamedLabel) string {
	var b strings.Builder
	b.WriteString(`
# The project's label vocabulary. Labels stay freeform — any label can be put on
# a task — and this set is what gives them a colour, a display name and the
# grouping the board shows. A "/" groups labels for display only; the label is
# stored and matched as the whole string, and the board draws what is before it
# as the first half of a two-tone chip, with display_name naming the second.
# Hex colours must be quoted, or YAML reads them as a comment.
labels:
`)
	for _, label := range labels {
		fmt.Fprintf(&b, "  %s:\n    color: %q\n    display_name: %s\n", label.Name, label.Color, label.DisplayName)
	}
	return b.String()
}
