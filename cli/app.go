// Package cli implements the qbit-filematcher command-line interface.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// Version is set by the build via -ldflags when releasing.
var Version = "dev"

// App is the CLI application.
type App struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

	configPath string
}

// New returns an App writing to stdout/stderr by default.
func New() *App {
	return &App{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
	}
}

// Run parses args and executes the requested command.
func (a *App) Run(ctx context.Context, args []string) error {
	if a.Stdout == nil {
		a.Stdout = os.Stdout
	}
	if a.Stderr == nil {
		a.Stderr = os.Stderr
	}
	if a.Stdin == nil {
		a.Stdin = os.Stdin
	}

	cmd := a.rootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(a.Stdout)
	cmd.SetErr(a.Stderr)
	if ctx == nil {
		ctx = context.Background()
	}
	return cmd.ExecuteContext(ctx)
}

func (a *App) rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "qbit-filematcher",
		Short:         "Rematch moved files to qBittorrent torrents",
		Long:          "qbit-filematcher rematches on-disk files to qBittorrent torrents via BT_backup (no Web API).",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cobra.EnableCommandSorting = false
	cmd.SetVersionTemplate("qbit-filematcher {{.Version}}\n")
	cmd.PersistentFlags().StringVar(&a.configPath, "config", "", "path to qbit-filematcher.yaml (default: next to binary, then user config dir)")

	cmd.AddCommand(
		a.matchCmd(),
		a.configCmd(),
		a.webCmd(),
		a.versionCmd(),
	)
	cmd.Example = `  qbit-filematcher match -b ~/.local/share/qBittorrent/BT_backup -s /data/media --auto
  qbit-filematcher match --dry-run -s /data/media
  qbit-filematcher config view
  qbit-filematcher web --host localhost --port 8080`

	return cmd
}

func (a *App) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(a.Stdout, "qbit-filematcher %s\n", Version)
			return err
		},
	}
}
