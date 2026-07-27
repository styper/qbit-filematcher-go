package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/styper/qbit-filematcher-go/config"
)

func (a *App) configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View, edit, or remove persisted settings",
	}
	cmd.AddCommand(a.configViewCmd(), a.configEditCmd(), a.configRemoveCmd())
	return cmd
}

func (a *App) configViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Print current settings",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			s, err := config.Load(a.configPath)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "config: %s\n", config.Path(a.configPath))
			fmt.Fprintf(a.Stdout, "  bt_backup_location: %s\n", s.BTBackupLocation)
			fmt.Fprintf(a.Stdout, "  search_paths:       %s\n", strings.Join(s.SearchPaths, ", "))
			fmt.Fprintf(a.Stdout, "  exclude_dirs:       %s\n", strings.Join(s.ExcludeDirs, ", "))
			fmt.Fprintf(a.Stdout, "  host:               %s\n", s.Host)
			fmt.Fprintf(a.Stdout, "  port:               %d\n", s.Port)
			fmt.Fprintf(a.Stdout, "  log_format:         %s\n", s.EffectiveLogFormat())
			fmt.Fprintf(a.Stdout, "  log_level:          %s\n", s.EffectiveLogLevel())
			return nil
		},
	}
}

func (a *App) configEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Interactively edit settings",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.editConfig(a.configPath)
		},
	}
}

func (a *App) configRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Delete the config file",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			path := config.Path(a.configPath)
			if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
				fmt.Fprintln(a.Stdout, "config file does not exist")
				return nil
			}
			fmt.Fprintf(a.Stdout, "Remove %s? [yes/no] (no): ", path)
			reader := bufio.NewReader(a.Stdin)
			line, _ := reader.ReadString('\n')
			if strings.TrimSpace(strings.ToLower(line)) != "yes" {
				fmt.Fprintln(a.Stdout, "not removed")
				return nil
			}
			if err := config.Remove(path); err != nil {
				return err
			}
			fmt.Fprintln(a.Stdout, "removed")
			return nil
		},
	}
}

func (a *App) editConfig(cfgPath string) error {
	s, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(a.Stdin)

	s.BTBackupLocation = a.promptString(reader, "bt_backup_location", s.BTBackupLocation)
	s.SearchPaths = splitArgs(a.promptString(reader, "search_paths", quoteJoin(s.SearchPaths)))
	s.ExcludeDirs = splitArgs(a.promptString(reader, "exclude_dirs", quoteJoin(s.ExcludeDirs)))
	s.Host = a.promptString(reader, "host", s.Host)
	portStr := a.promptString(reader, "port", strconv.Itoa(s.Port))
	if p, err := strconv.Atoi(portStr); err == nil {
		s.Port = p
	}
	s.LogFormat = a.promptString(reader, "log_format (text|json)", s.EffectiveLogFormat())
	s.LogLevel = a.promptString(reader, "log_level (debug|info|warn|error)", s.EffectiveLogLevel())

	fmt.Fprintf(a.Stdout, "Save to %s? [yes/no] (no): ", config.Path(cfgPath))
	line, _ := reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(line)) != "yes" {
		fmt.Fprintln(a.Stdout, "not saved")
		return nil
	}
	if err := config.Save(cfgPath, s); err != nil {
		return err
	}
	fmt.Fprintln(a.Stdout, "saved")
	return nil
}

func (a *App) promptString(r *bufio.Reader, label, def string) string {
	fmt.Fprintf(a.Stdout, "%s (%s): ", label, def)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	if line == "null" {
		return ""
	}
	return line
}

func quoteJoin(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	out := make([]string, len(vals))
	for i, v := range vals {
		if strings.ContainsAny(v, " \t") {
			out[i] = `"` + v + `"`
		} else {
			out[i] = v
		}
	}
	return strings.Join(out, " ")
}

func splitArgs(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var out []string
	var cur strings.Builder
	inQuote := false
	for _, r := range value {
		switch {
		case r == '"':
			inQuote = !inQuote
		case unicodeIsSpace(r) && !inQuote:
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func unicodeIsSpace(r rune) bool {
	return r == ' ' || r == '\t'
}
