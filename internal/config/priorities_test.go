package config_test

import (
	"strings"
	"testing"

	"github.com/fmartingr/taskqueue/internal/tqtest"

	"github.com/fmartingr/taskqueue/internal/config"
)

const customPriorities = `version: 1
path: .tasks
priorities:
  - name: p0
    color: "#b60205"
    display_name: "P0 — drop everything"
  - name: p1
    color: "#c2410c"
  - name: p2
    color: "#4b5563"
    default: true
`

// An absent `priorities` key means "use the built-in set", the way an absent
// `labels` key means the base vocabulary.
func TestPrioritySetDefaultsWhenTheKeyIsAbsent(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\n")

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	if got, want := len(cfg.PrioritySet()), len(config.DefaultPriorities()); got != want {
		t.Fatalf("PrioritySet() has %d entries, want the %d built-in", got, want)
	}
	if got, want := strings.Join(cfg.Vocabulary().Names(), ","), "urgent,high,normal,low"; got != want {
		t.Errorf("Vocabulary() = %q, want %q", got, want)
	}
	if cfg.Vocabulary().Default() != "normal" {
		t.Errorf("Default() = %q, want normal", cfg.Vocabulary().Default())
	}
}

// No config file at all is the same case, and a nil *Config answers it.
func TestPrioritySetDefaultsWithoutAConfigFile(t *testing.T) {
	cfg, err := config.Optional(config.FindConfig(tqtest.RootWithoutMarker(t)))
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	if cfg != nil {
		t.Fatalf("config.Optional(FindConfig()) = %+v, want nil", cfg)
	}
	if got, want := len(cfg.PrioritySet()), len(config.DefaultPriorities()); got != want {
		t.Errorf("PrioritySet() on a nil config = %d entries, want the %d built-in", got, want)
	}
}

// The order in the file is the ranking, and the display name falls back to the
// name — which is what the CLI takes, so it is the honest stand-in.
func TestPrioritySetKeepsTheFileOrder(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, customPriorities)

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	set := cfg.PrioritySet()
	if got, want := len(set), 3; got != want {
		t.Fatalf("PrioritySet() has %d entries, want %d", got, want)
	}
	if got, want := strings.Join(cfg.Vocabulary().Names(), ","), "p0,p1,p2"; got != want {
		t.Errorf("Vocabulary() = %q, want %q (the file order)", got, want)
	}
	if set[0].DisplayName != "P0 — drop everything" {
		t.Errorf("p0 display name = %q", set[0].DisplayName)
	}
	if set[1].DisplayName != "p1" {
		t.Errorf("p1 display name = %q, want the name itself", set[1].DisplayName)
	}
	if cfg.Vocabulary().Default() != "p2" {
		t.Errorf("Default() = %q, want p2", cfg.Vocabulary().Default())
	}
}

func TestFindPriority(t *testing.T) {
	set := config.DefaultPriorities()
	if got, ok := config.FindPriority(set, "urgent"); !ok || got.DisplayName != "Urgent" {
		t.Errorf("FindPriority(urgent) = %+v, %v", got, ok)
	}
	if _, ok := config.FindPriority(set, "p0"); ok {
		t.Error("FindPriority(p0) reported a value the built-in set does not have")
	}
}

// Every one of these would leave tq unable to file a task, or unable to say
// under what, so the config is refused with the fix in the message.
func TestPriorityConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"no default", "priorities:\n  - {name: a, color: \"#111111\"}\n", "no priority is marked"},
		{
			"two defaults",
			"priorities:\n  - {name: a, color: \"#111111\", default: true}\n  - {name: b, color: \"#222222\", default: true}\n",
			"2 priorities are marked",
		},
		{"unquoted colour", "priorities:\n  - {name: a, default: true}\n", "has no colour"},
		{"bad colour", "priorities:\n  - {name: a, color: red, default: true}\n", "want a hex colour"},
		{
			"duplicate",
			"priorities:\n  - {name: a, color: \"#111111\", default: true}\n  - {name: a, color: \"#222222\"}\n",
			"listed twice",
		},
		{"empty name", "priorities:\n  - {name: \"  \", color: \"#111111\", default: true}\n", "has no name"},
		{"empty list", "priorities: []\n", "priorities is empty"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := tqtest.Root(t)
			tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\n"+tc.body)

			_, err := config.FindConfig(root)
			if err == nil {
				t.Fatal("config.FindConfig() = nil, want an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// The marker a first command writes carries the vocabulary, so a project that
// never runs `tq init` still has one to edit rather than an invisible default.
func TestWriteConfigSeedsThePriorities(t *testing.T) {
	// WriteConfigIfMissing only writes where there is no marker.
	root := tqtest.RootWithoutMarker(t)
	if _, err := config.WriteConfigIfMissing(root, root+"/.tasks"); err != nil {
		t.Fatalf("WriteConfigIfMissing: %v", err)
	}

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("config.FindConfig() = nil after writing a marker")
	}
	// Read back through the loader, not by matching the text: what matters is
	// that the seeded block is a config this binary accepts.
	if got, want := len(cfg.Priorities), len(config.DefaultPriorities()); got != want {
		t.Fatalf("seeded config has %d priorities, want %d", got, want)
	}
	if got, want := strings.Join(cfg.Vocabulary().Names(), ","), "urgent,high,normal,low"; got != want {
		t.Errorf("seeded vocabulary = %q, want %q", got, want)
	}
	if cfg.Vocabulary().Default() != "normal" {
		t.Errorf("seeded default = %q, want normal", cfg.Vocabulary().Default())
	}
}
