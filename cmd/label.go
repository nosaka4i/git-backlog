package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nosaka4i/git-backlog/internal/store"
	"github.com/spf13/cobra"
)

func newLabelCmd() *cobra.Command {
	var asAgent, remove bool
	cmd := &cobra.Command{
		Use:   "label <id> <label>...",
		Short: "Add or remove labels on an item",
		Long: "Add one or more labels to an item, or with --remove, take them off.\n" +
			"Labels are flat tags for grouping items (e.g. a sprint); filter with `list --label`.",
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
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
	cmd.Flags().BoolVar(&asAgent, "as-agent", false, asAgentFlagUsage)
	cmd.AddCommand(newLabelLsCmd())
	return cmd
}

func newLabelLsCmd() *cobra.Command {
	var jsonFlag bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List every label in use with how many items carry it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInit(); err != nil {
				return err
			}
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
			// Most-used first, ties broken alphabetically — the common
			// grouping labels (a busy sprint) surface at the top.
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
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "output as JSON")
	return cmd
}
