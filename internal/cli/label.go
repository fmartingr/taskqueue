package cli

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/fmartingr/taskqueue/internal/config"

	"github.com/fmartingr/taskqueue/internal/task"
)

// labelEntry is one row of `tq label list`. It is the JSON shape agents read,
// so the field names are part of the contract.
type labelEntry struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	DisplayName string `json:"display_name"`
	// Configured is false for a label that tasks carry but the project does
	// not declare. Such a label works everywhere; it just has no colour and no
	// display name, and is worth either adding to the set or cleaning up.
	Configured bool `json:"configured"`
	Count      int  `json:"count"`
}

func (c *cli) runLabel(args []string) int {
	if len(args) == 0 {
		return c.fail(fmt.Errorf("label needs a subcommand (list)"))
	}
	switch args[0] {
	case "list":
		return c.runLabelList(args[1:])
	default:
		return c.fail(fmt.Errorf("unknown label subcommand %q (want: list)", args[0]))
	}
}

func (c *cli) runLabelList(args []string) int {
	fs := c.flagSet("label list")
	jsonOut := fs.Bool("json", false, "print JSON output")
	if _, code, ok := c.parse(fs, args, 0); !ok {
		return code
	}

	// The store comes first, so a missing task directory is reported the same
	// way every other command reports it rather than after half an answer.
	st, err := c.st()
	if err != nil {
		return c.fail(err)
	}
	listing, err := st.List()
	if err != nil {
		return c.fail(err)
	}
	c.warnUnreadable(listing.Unreadable)
	// The vocabulary is resolved from the queue being listed, not from the
	// working directory: TQ_DIR can point at a queue in another project, and
	// reading that project's tasks against this one's config would have the CLI
	// and the board (which resolves from the store too) disagree about which
	// labels are configured.
	cfg, err := config.FindConfig(st.Dir)
	if err != nil {
		return c.fail(err)
	}

	entries := labelEntries(cfg.LabelSet(), listing.Tasks)
	if *jsonOut {
		return c.printJSON(entries)
	}

	rows := make([][]string, 0, len(entries))
	unconfigured := 0
	for _, entry := range entries {
		source := "config"
		if !entry.Configured {
			source = "unconfigured"
			unconfigured++
		}
		rows = append(rows, []string{entry.Name, entry.DisplayName, orDash(entry.Color), strconv.Itoa(entry.Count), source})
	}
	c.table([]string{"LABEL", "DISPLAY", "COLOR", "TASKS", "SOURCE"}, rows)

	// The tail is the point of the command, so it gets a sentence rather than
	// leaving the reader to spot the word "unconfigured" in a column.
	switch {
	case unconfigured == 1:
		fmt.Fprintf(c.stdout, "\n1 label is in use but not in %s: add it to the set, or relabel those tasks.\n",
			config.ConfigFileName)
	case unconfigured > 1:
		fmt.Fprintf(c.stdout, "\n%d labels are in use but not in %s: add them to the set, or relabel those tasks.\n",
			unconfigured, config.ConfigFileName)
	}
	return exitOK
}

// labelEntries is the vocabulary as it stands: every configured label, then
// every label tasks actually carry that the project does not declare. Both
// groups are sorted, so two runs read the same, and the unconfigured ones come
// last because they are the tail worth acting on.
func labelEntries(set map[string]config.Label, tasks []task.Task) []labelEntry {
	// Tasks, not occurrences: nothing stops a task from carrying the same label
	// twice, and the column says how many tasks a label is on.
	counts := map[string]int{}
	for _, t := range tasks {
		for _, label := range unique(t.Labels) {
			counts[label]++
		}
	}

	entries := make([]labelEntry, 0, len(set)+len(counts))
	for _, label := range config.SortedLabels(set) {
		entries = append(entries, labelEntry{
			Name:        label.Name,
			Color:       label.Color,
			DisplayName: label.DisplayName,
			Configured:  true,
			Count:       counts[label.Name],
		})
	}

	extra := make([]string, 0, len(counts))
	for name := range counts {
		if _, ok := set[name]; !ok {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	for _, name := range extra {
		// No colour and the label itself as the display name: exactly what the
		// board falls back to when it has nothing to draw a label with.
		entries = append(entries, labelEntry{Name: name, DisplayName: name, Count: counts[name]})
	}
	return entries
}

func unique(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
