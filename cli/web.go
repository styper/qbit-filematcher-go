package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/styper/qbit-filematcher-go/config"
	"github.com/styper/qbit-filematcher-go/web"
)

func (a *App) webCmd() *cobra.Command {
	var (
		host string
		port int
	)

	cmd := &cobra.Command{
		Use:     "web",
		Aliases: []string{"serve"},
		Short:   "Start the web UI server",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := web.DefaultConfig()
			cfg.ConfigPath = config.Path(a.configPath)

			settings, err := config.Load(cfg.ConfigPath)
			if err != nil {
				fmt.Fprintf(a.Stderr, "warning: %v\n", err)
			}
			if settings.Host != "" {
				cfg.Host = settings.Host
			}
			if settings.Port != 0 {
				cfg.Port = settings.Port
			}
			if host != "" {
				cfg.Host = host
			}
			if port != 0 {
				cfg.Port = port
			}

			web.Version = Version
			app, err := web.New(cfg)
			if err != nil {
				return err
			}

			runCtx := cmd.Context()
			if runCtx == nil {
				runCtx = context.Background()
			}
			runCtx, stop := signal.NotifyContext(runCtx, os.Interrupt, syscall.SIGTERM)
			defer stop()

			fmt.Fprintf(a.Stdout, "qbit-filematcher %s listening on http://%s\n", Version, cfg.Addr())
			fmt.Fprintf(a.Stdout, "config: %s\n", cfg.ConfigPath)
			return app.ListenAndServe(runCtx)
		},
	}

	cmd.Flags().StringVarP(&host, "host", "o", "", "listen host (default: from config or localhost)")
	cmd.Flags().IntVarP(&port, "port", "p", 0, "listen port (default: from config or 8080)")

	return cmd
}
