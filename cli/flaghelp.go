package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagGroup is a titled subset of local flags for help/usage output.
type flagGroup struct {
	Title string
	Names []string
}

// setGroupedFlagUsage replaces the default usage printer so local flags appear
// under titled groups instead of a single "Flags:" block.
func setGroupedFlagUsage(cmd *cobra.Command, groups ...flagGroup) {
	cmd.SetUsageFunc(func(c *cobra.Command) error {
		return writeGroupedUsage(c.OutOrStderr(), c, groups)
	})
}

func writeGroupedUsage(w io.Writer, c *cobra.Command, groups []flagGroup) error {
	fmt.Fprintln(w, "Usage:")
	if c.Runnable() {
		fmt.Fprintf(w, "  %s\n", c.UseLine())
	}
	if c.HasExample() {
		fmt.Fprintf(w, "\nExamples:\n%s\n", c.Example)
	}

	local := c.LocalFlags()
	for _, g := range groups {
		usages := flagUsagesNamed(local, g.Names...)
		if usages == "" {
			continue
		}
		fmt.Fprintf(w, "\n%s\n%s", g.Title, usages)
	}

	if c.HasAvailableInheritedFlags() {
		fmt.Fprintf(w, "\nGlobal Flags:\n%s", strings.TrimRight(c.InheritedFlags().FlagUsages(), " \n\t"))
		fmt.Fprintln(w)
	}
	return nil
}

// flagUsagesNamed formats usage lines for the named flags, in Names order.
func flagUsagesNamed(fs *pflag.FlagSet, names ...string) string {
	subset := pflag.NewFlagSet("", pflag.ContinueOnError)
	subset.SortFlags = false
	for _, name := range names {
		f := fs.Lookup(name)
		if f == nil || f.Hidden {
			continue
		}
		subset.AddFlag(f)
	}
	if !subset.HasAvailableFlags() {
		return ""
	}
	return subset.FlagUsages()
}
