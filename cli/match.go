package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/styper/qbit-filematcher-go/config"
	"github.com/styper/qbit-filematcher-go/filematcher"
)

func (a *App) matchCmd() *cobra.Command {
	var (
		btBackup   string
		auto       bool
		incomplete bool
		dryRun     bool
		search     []string
		exclude    []string
		hashes     []string
		tags       []string
		names      []string
	)

	cmd := &cobra.Command{
		Use:   "match",
		Short: "Scan disk and update fastresume files",
		Long: `Scan disk and update fastresume files.

Path settings come from the config file; flags below override when set.
After merge, --bt-backup and at least one --search path are required.`,
		Example: `  qbit-filematcher match -b ~/.local/share/qBittorrent/BT_backup -s /data/media --auto
  qbit-filematcher match --dry-run -s /data/media
  qbit-filematcher match -s /data/media -s /mnt/nas/media
  qbit-filematcher match -s "/data/My Media" -s /mnt/nas/media`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			settings, err := config.Load(a.configPath)
			if err != nil {
				return err
			}
			if btBackup != "" {
				settings.BTBackupLocation = btBackup
			}
			if len(search) > 0 {
				settings.SearchPaths = append([]string{}, search...)
			}
			if len(exclude) > 0 {
				settings.ExcludeDirs = append([]string{}, exclude...)
			}
			if err := settings.ValidateMatch(); err != nil {
				return err
			}

			fmt.Fprintf(a.Stdout, "BT_backup:   %s\n", settings.BTBackupLocation)
			fmt.Fprintf(a.Stdout, "search:      %s\n", strings.Join(settings.SearchPaths, ", "))
			fmt.Fprintf(a.Stdout, "exclude:     %s\n", strings.Join(settings.ExcludeDirs, ", "))
			fmt.Fprintf(a.Stdout, "auto:        %v\n", auto)
			fmt.Fprintf(a.Stdout, "incomplete:  %v\n", incomplete)
			fmt.Fprintf(a.Stdout, "dry-run:     %v\n", dryRun)

			if !dryRun {
				running, err := filematcher.IsQBittorrentRunning()
				if err != nil {
					return err
				}
				if running {
					return errors.New("qBittorrent is running; close it before saving (or use --dry-run)")
				}
			}

			start := time.Now()
			lib, err := filematcher.LoadLibrary(settings.BTBackupLocation, filematcher.LoadOptions{})
			if err != nil {
				return err
			}
			for hash, e := range lib.LoadErrors {
				fmt.Fprintf(a.Stderr, "warning: skipped %s: %v\n", hash, e)
			}

			filtered := lib.Filter(filematcher.FilterOptions{
				Hashes:       hashes,
				Names:        names,
				Tags:         tags,
				MatchAllTags: true,
			})

			torrents := filtered.List()
			sort.Slice(torrents, func(i, j int) bool {
				return torrents[i].Info.Name < torrents[j].Info.Name
			})

			added, err := filematcher.Scan(ctx, torrents, filematcher.ScanOptions{
				SearchPaths: settings.SearchPaths,
				ExcludeDirs: settings.ExcludeDirs,
				SelectBest:  true,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(a.Stdout, "Loaded %d torrent(s), %d candidate pair(s) in %s\n",
				len(torrents), added, time.Since(start).Round(time.Millisecond))

			ok, failed := 0, 0
			for _, tor := range torrents {
				if err := a.processTorrent(tor, auto, incomplete, dryRun); err != nil {
					failed++
					fmt.Fprintf(a.Stderr, "error: %s: %v\n", tor.Info.Name, err)
					continue
				}
				ok++
			}
			fmt.Fprintf(a.Stdout, "Done. Updated: %d. Failed: %d.\n", ok, failed)
			if failed > 0 {
				return fmt.Errorf("%d torrent(s) failed", failed)
			}
			return nil
		},
	}

	cmd.Flags().SortFlags = false

	// Config overrides (optional if already set in YAML; required after merge).
	cmd.Flags().StringVarP(&btBackup, "bt-backup", "b", "", "BT_backup directory (overrides config; required after merge)")
	cmd.Flags().StringArrayVarP(&search, "search", "s", nil, "search path (repeatable; overrides config; required after merge)")
	cmd.Flags().StringArrayVarP(&exclude, "exclude", "e", nil, "directory name to exclude (repeatable; overrides config)")

	// CLI-only (not stored in config).
	cmd.Flags().BoolVar(&auto, "auto", false, "auto-select best candidates when multiple matches exist")
	cmd.Flags().BoolVar(&incomplete, "incomplete", false, "allow incomplete torrents")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print plans without writing fastresume files")
	cmd.Flags().StringArrayVarP(&hashes, "hash", "a", nil, "torrent hash v1 filter (repeatable)")
	cmd.Flags().StringArrayVarP(&tags, "tag", "t", nil, "tag filter (repeatable; all tags required)")
	cmd.Flags().StringArrayVarP(&names, "name", "n", nil, "name token filter (repeatable)")

	setGroupedFlagUsage(cmd,
		flagGroup{Title: "Config Overrides:", Names: []string{"bt-backup", "search", "exclude"}},
		flagGroup{Title: "Flags:", Names: []string{"auto", "incomplete", "dry-run"}},
		flagGroup{Title: "Filters:", Names: []string{"hash", "tag", "name"}},
	)

	return cmd
}

func (a *App) processTorrent(tor *filematcher.Torrent, auto, allowIncomplete, dryRun bool) error {
	fmt.Fprintf(a.Stdout, "\n== %s (%s) ==\n", tor.Info.Name, tor.Info.HashV1)
	realFiles := tor.Info.RealFiles()
	for i, f := range realFiles {
		m := tor.Matches[f.Index]
		fmt.Fprintf(a.Stdout, "[%d/%d] %s (%d bytes)\n", i+1, len(realFiles), f.Path, f.Size)
		if m == nil || !m.HasMatch() {
			fmt.Fprintln(a.Stdout, "  (no matches)")
			continue
		}
		for ci, c := range m.Candidates {
			mark := "  "
			if ci == m.SelectedIndex {
				mark = "->"
			}
			fmt.Fprintf(a.Stdout, "%s [%d] %s\n", mark, ci, c)
		}
		if !auto && len(m.Candidates) > 1 {
			choice, err := a.promptChoice(len(m.Candidates), m.SelectedIndex)
			if err != nil {
				return err
			}
			m.SelectedIndex = choice
		}
	}

	plan, err := tor.MakePlan(filematcher.PlanOptions{AllowIncomplete: allowIncomplete})
	if err != nil {
		return err
	}
	fmt.Fprintf(a.Stdout, "save_path: %s\n", plan.SavePath)
	for i, m := range plan.MappedFiles {
		if m == "" {
			continue
		}
		fmt.Fprintf(a.Stdout, "  %d: %s\n", i, m)
	}

	if dryRun {
		fmt.Fprintln(a.Stdout, "(dry-run) not writing")
		return nil
	}

	written, err := tor.Save(plan, filematcher.DefaultSaveOptions())
	if err != nil {
		return err
	}
	if written {
		fmt.Fprintln(a.Stdout, "fastresume updated")
	} else {
		fmt.Fprintln(a.Stdout, "already up to date")
	}
	if plan.IsIncomplete {
		fmt.Fprintln(a.Stdout, "note: incomplete; qBittorrent will recheck on next start")
	}
	return nil
}

func (a *App) promptChoice(n, def int) (int, error) {
	reader := bufio.NewReader(a.Stdin)
	for {
		fmt.Fprintf(a.Stdout, "Select [0-%d] (%d): ", n-1, def)
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return 0, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			return def, nil
		}
		v, err := strconv.Atoi(line)
		if err != nil || v < 0 || v >= n {
			fmt.Fprintln(a.Stderr, "invalid choice")
			continue
		}
		return v, nil
	}
}
