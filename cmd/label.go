package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newLabelCmd() *cobra.Command {
	var asAgent, remove, jsonFlag bool
	cmd := &cobra.Command{
		Use:   "label [<id> <name>...]",
		Short: "List all labels, or add/remove labels on an item",
		Long: "With no arguments, list every label in use with how many items carry it.\n" +
			"With an id and one or more names, attach those labels to the item (or\n" +
			"detach them with --remove). Labels are flat tags for grouping items\n" +
			"(e.g. a sprint); filter with `list --label`.",
		// No args = list the roster; an id plus one or more labels = mutate.
		// A lone id (one arg) is neither, so it's an error.
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return fmt.Errorf("provide at least one label (e.g. `label <id> <name>`), or no arguments to list all labels")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
			if len(args) == 0 {
				return runLabelRoster(jsonFlag)
			}
			labels := make([]string, 0, len(args)-1)
			for _, a := range args[1:] {
				l, err := store.ParseLabel(a)
				if err != nil {
					return err
				}
				labels = append(labels, l)
			}
			identity, err := resolveAgentIdentity(asAgent)
			if err != nil {
				return err
			}
			var item *store.Item
			if remove {
				item, err = store.RemoveLabels(args[0], labels, identity)
			} else {
				item, err = store.AddLabels(args[0], labels, identity)
			}
			if err != nil {
				return err
			}
			if len(item.Labels) == 0 {
				fmt.Printf("Updated item labels successfully: %s (no labels)\n", store.ShortID(item.ID))
			} else {
				fmt.Printf("Updated item labels successfully: %s [%s]\n", store.ShortID(item.ID), strings.Join(item.Labels, ", "))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&remove, "remove", false, "remove the given labels instead of adding them")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "with no id/name args, output the label list as JSON")
	cmd.Flags().BoolVar(&asAgent, "as-agent", false, asAgentFlagUsage)
	return cmd
}

// runLabelRoster prints every label in use with the count of items carrying
// it, most-used first (ties broken alphabetically) — the bare `label`
// command, mirroring how `git tag`/`git branch` list with no arguments.
func runLabelRoster(jsonFlag bool) error {
	items, err := store.AllItems()
	if err != nil {
		return err
	}
	counts := make(map[string]int)
	for _, it := range items {
		for _, l := range it.Labels {
			counts[l]++
		}
	}
	labels := make([]string, 0, len(counts))
	for l := range counts {
		labels = append(labels, l)
	}
	sort.Slice(labels, func(i, j int) bool {
		if counts[labels[i]] != counts[labels[j]] {
			return counts[labels[i]] > counts[labels[j]]
		}
		return labels[i] < labels[j]
	})

	if jsonFlag {
		out := make([]jsonLabelCount, 0, len(labels))
		for _, l := range labels {
			out = append(out, jsonLabelCount{Label: l, Count: counts[l]})
		}
		return printJSON(out)
	}

	if len(labels) == 0 {
		fmt.Println("no labels in use")
		return nil
	}
	width := 0
	for _, l := range labels {
		if len(l) > width {
			width = len(l)
		}
	}
	for _, l := range labels {
		fmt.Printf("%-*s  (%d)\n", width, l, counts[l])
	}
	return nil
}
