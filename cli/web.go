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
		host      string
		port      int
		logFormat string
		logLevel  string
	)

	cmd := &cobra.Command{
		Use:     "web",
		Aliases: []string{"serve"},
		Short:   "Start the web UI server",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			cfg.LogFormat = settings.EffectiveLogFormat()
			cfg.LogLevel = web.ParseLogLevel(settings.LogLevel)
			levelName := settings.EffectiveLogLevel()
			if host != "" {
				cfg.Host = host
			}
			if port != 0 {
				cfg.Port = port
			}
			if logFormat != "" {
				normalized, err := config.NormalizeLogFormat(logFormat)
				if err != nil {
					return err
				}
				cfg.LogFormat = normalized
			}
			if logLevel != "" {
				normalized, err := config.NormalizeLogLevel(logLevel)
				if err != nil {
					return err
				}
				levelName = normalized
				cfg.LogLevel = web.ParseLogLevel(normalized)
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
			fmt.Fprintf(a.Stdout, "log: format=%s level=%s\n", cfg.LogFormat, levelName)
			return app.ListenAndServe(runCtx)
		},
	}

	cmd.Flags().SortFlags = false
	cmd.Flags().StringVarP(&host, "host", "o", "", "listen host (default: from config or localhost)")
	cmd.Flags().IntVarP(&port, "port", "p", 0, "listen port (default: from config or 8080)")
	cmd.Flags().StringVar(&logFormat, "log-format", "", "log format: text or json (default: from config or text)")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "log level: debug, info, warn, or error (default: from config or info)")

	return cmd
}
