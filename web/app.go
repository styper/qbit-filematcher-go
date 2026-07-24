// Package web serves the HTMX/Alpine web UI.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/styper/qbit-filematcher-go/config"
	"github.com/styper/qbit-filematcher-go/filematcher"
)

// Version is set by the build via -ldflags when releasing.
var Version = "dev"

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

//go:embed icon.svg
var iconSVG []byte

// Config holds HTTP server settings.
type Config struct {
	Host       string
	Port       int
	ConfigPath string
}

// DefaultConfig returns localhost:8080.
func DefaultConfig() Config {
	return Config{Host: "localhost", Port: 8080, ConfigPath: ""}
}

// Addr returns host:port.
func (c Config) Addr() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// App is the web application.
type App struct {
	Config Config
	mux    *http.ServeMux
	tpl    *template.Template

	mu          sync.Mutex
	library     *filematcher.Library
	flash       string
	errFlash    string
	scanRunning bool
	scanStarted time.Time
	scanCancel  context.CancelFunc
}

// New constructs an App with routes registered.
func New(cfg Config) (*App, error) {
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	cfg.ConfigPath = config.Path(cfg.ConfigPath)

	hashes, err := loadAssetHashes(staticFS)
	if err != nil {
		return nil, fmt.Errorf("web: asset hashes: %w", err)
	}
	iconHash := shortContentHash(iconSVG)

	tpl, err := template.New("").Funcs(template.FuncMap{
		"join": strings.Join,
		"add":  func(a, b int) int { return a + b },
		"dict": func(values ...any) (map[string]any, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict requires an even number of arguments")
			}
			m := make(map[string]any, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				m[key] = values[i+1]
			}
			return m, nil
		},
		"json": func(v any) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("null")
			}
			// #nosec G203 -- JSON for Alpine/HTMX from our own typed structs, not raw HTML.
			return template.JS(b)
		},
		"staticURL": func(name string) string {
			return staticURL(hashes, name)
		},
		"iconURL": func() string {
			return "/icon.svg?v=" + iconHash
		},
	}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("web: templates: %w", err)
	}

	a := &App{Config: cfg, mux: http.NewServeMux(), tpl: tpl}
	if err := a.routes(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) routes() error {
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("web: static assets: %w (run `make assets` first)", err)
	}

	a.mux.Handle("GET /static/", http.StripPrefix("/static/",
		withCacheControl(http.FileServer(http.FS(static)), "public, max-age=86400")))
	a.mux.HandleFunc("GET /icon.svg", a.handleIcon)
	a.mux.HandleFunc("GET /favicon.ico", a.handleIcon)
	a.mux.HandleFunc("GET /{$}", a.handleHome)
	a.mux.HandleFunc("GET /config", a.handleConfigGet)
	a.mux.HandleFunc("POST /config", a.handleConfigPost)
	a.mux.HandleFunc("GET /match", a.handleMatchGet)
	a.mux.HandleFunc("POST /match/scan", a.handleMatchScan)
	a.mux.HandleFunc("POST /match/scan/cancel", a.handleMatchScanCancel)
	a.mux.HandleFunc("GET /match/scan-status", a.handleMatchScanStatus)
	a.mux.HandleFunc("GET /match/{hash}", a.handleMatchDetail)
	a.mux.HandleFunc("GET /match/{hash}/plan", a.handleMatchPlan)
	a.mux.HandleFunc("POST /match/{hash}/select", a.handleMatchSelect)
	a.mux.HandleFunc("POST /match/{hash}/save", a.handleMatchSave)
	a.mux.HandleFunc("GET /api/qbittorrent", a.handleQBittorrentStatus)
	a.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	return nil
}

type pageData struct {
	Title    string
	Version  string
	Flash    string
	Message  string
	Hash     string
	Settings config.Settings
	Torrents []*torrentRow
	Torrent  *torrentDetail
	FileCard *fileCardData
	Error    string
	Scanning bool
	// ScanElapsedSec is seconds since the current scan started (0 if idle).
	ScanElapsedSec int
}

type torrentRow struct {
	Hash     string `json:"hash"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Location string `json:"location"` // none | incomplete | current | changed
	Tags     string `json:"tags"`
	Files    int    `json:"files"`
}

type torrentDetail struct {
	Hash            string
	Name            string
	Status          string
	Location        string // none | incomplete | current | changed
	AlreadyCurrent  bool   // LocationCurrent — save would be a no-op
	CanSave         bool
	AllSingleMatch  bool // every real file has exactly one candidate
	Tags            []string
	Files           []fileRow
	Plan            *filematcher.SavePlan
	PlanTree        []*planNode
	PlanErr         string
	PlanHint        string
	ConflictIndexes []int
	Saved           bool
	SaveMsg         string
}

type fileRow struct {
	Index         int
	Path          string
	Size          int64
	Candidates    []string
	SelectedIndex int
	AutoIndex     int
}

type fileCardData struct {
	Hash string
	File fileRow
}

func (a *App) takeFlash() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	f := a.flash
	a.flash = ""
	return f
}

func (a *App) setFlash(msg string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.flash = msg
}

func (a *App) takeError() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	e := a.errFlash
	a.errFlash = ""
	return e
}

func (a *App) setError(msg string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.errFlash = msg
}

func (a *App) handleIcon(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(iconSVG)
}

// withCacheControl sets Cache-Control on successful responses from next.
func withCacheControl(next http.Handler, value string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", value)
		next.ServeHTTP(w, r)
	})
}

func (a *App) render(w http.ResponseWriter, name string, data pageData) {
	if data.Version == "" {
		data.Version = Version
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.tpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) renderNotFound(w http.ResponseWriter, data pageData) {
	if data.Title == "" {
		data.Title = "Not found"
	}
	if data.Version == "" {
		data.Version = Version
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if err := a.tpl.ExecuteTemplate(w, "not_found.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) handleQBittorrentStatus(w http.ResponseWriter, _ *http.Request) {
	running, err := filematcher.IsQBittorrentRunning()
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]any{"running": running}
	if err != nil {
		resp["running"] = false
		resp["error"] = err.Error()
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (a *App) handleHome(w http.ResponseWriter, _ *http.Request) {
	a.render(w, "home.html", pageData{Title: "Home", Flash: a.takeFlash()})
}

func (a *App) handleConfigGet(w http.ResponseWriter, _ *http.Request) {
	s, err := config.Load(a.Config.ConfigPath)
	data := pageData{Title: "Config", Flash: a.takeFlash(), Settings: s}
	if err != nil {
		data.Error = err.Error()
	}
	a.render(w, "config.html", data)
}

func (a *App) handleConfigPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	port, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("port")))
	s := config.Settings{
		BTBackupLocation: strings.TrimSpace(r.FormValue("bt_backup_location")),
		SearchPaths:      splitLines(r.FormValue("search_paths")),
		ExcludeDirs:      splitLines(r.FormValue("exclude_dirs")),
		Host:             strings.TrimSpace(r.FormValue("host")),
		Port:             port,
	}
	if err := s.Validate(); err != nil {
		a.render(w, "config.html", pageData{Title: "Config", Settings: s, Error: err.Error()})
		return
	}
	if err := config.Save(a.Config.ConfigPath, s); err != nil {
		a.render(w, "config.html", pageData{Title: "Config", Settings: s, Error: err.Error()})
		return
	}
	a.setFlash("Configuration saved.")
	http.Redirect(w, r, "/config", http.StatusSeeOther)
}

func (a *App) handleMatchGet(w http.ResponseWriter, _ *http.Request) {
	s, _ := config.Load(a.Config.ConfigPath)
	data := pageData{
		Title:    "Match",
		Flash:    a.takeFlash(),
		Error:    a.takeError(),
		Settings: s,
	}

	a.mu.Lock()
	data.Scanning = a.scanRunning
	if a.scanRunning && !a.scanStarted.IsZero() {
		data.ScanElapsedSec = int(time.Since(a.scanStarted).Seconds())
	}
	lib := a.library
	a.mu.Unlock()
	if lib != nil {
		data.Torrents = rowsFromLibrary(lib)
	}
	a.render(w, "match_list.html", data)
}

func (a *App) handleMatchScan(w http.ResponseWriter, r *http.Request) {
	s, err := config.Load(a.Config.ConfigPath)
	if err != nil {
		a.setError(err.Error())
		http.Redirect(w, r, "/match", http.StatusSeeOther)
		return
	}
	if err := s.ValidateMatch(); err != nil {
		a.setError(err.Error())
		http.Redirect(w, r, "/match", http.StatusSeeOther)
		return
	}

	if !a.startScan(s) {
		a.setError("A scan is already running.")
	}
	http.Redirect(w, r, "/match", http.StatusSeeOther)
}

func (a *App) handleMatchScanCancel(w http.ResponseWriter, r *http.Request) {
	a.cancelScan()
	http.Redirect(w, r, "/match", http.StatusSeeOther)
}

func (a *App) handleMatchScanStatus(w http.ResponseWriter, _ *http.Request) {
	a.mu.Lock()
	running := a.scanRunning
	elapsed := 0
	if running && !a.scanStarted.IsZero() {
		elapsed = int(time.Since(a.scanStarted).Seconds())
	}
	a.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"running":     running,
		"elapsed_sec": elapsed,
	})
}

func (a *App) startScan(s config.Settings) bool {
	a.mu.Lock()
	if a.scanRunning {
		a.mu.Unlock()
		return false
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.scanCancel = cancel
	a.scanRunning = true
	a.scanStarted = time.Now()
	a.library = nil // clear previous results so the UI empties while rescanning
	a.mu.Unlock()

	go a.runScan(ctx, s)
	return true
}

func (a *App) cancelScan() {
	a.mu.Lock()
	cancel := a.scanCancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) runScan(ctx context.Context, s config.Settings) {
	start := time.Now()
	defer func() {
		a.mu.Lock()
		a.scanRunning = false
		a.scanStarted = time.Time{}
		a.scanCancel = nil
		a.mu.Unlock()
	}()

	lib, err := filematcher.LoadLibrary(s.BTBackupLocation, filematcher.LoadOptions{})
	if err != nil {
		a.setError(err.Error())
		return
	}
	if err := ctx.Err(); err != nil {
		a.setFlash("Scan cancelled.")
		return
	}

	_, err = filematcher.Scan(ctx, lib.List(), filematcher.ScanOptions{
		SearchPaths: s.SearchPaths,
		ExcludeDirs: s.ExcludeDirs,
		SelectBest:  true,
	})
	if err != nil {
		if ctx.Err() != nil {
			a.setFlash("Scan cancelled.")
			return
		}
		a.setError(err.Error())
		return
	}

	a.mu.Lock()
	a.library = lib
	a.mu.Unlock()

	torrents, files, matches := libraryScanStats(lib)
	a.setFlash(fmt.Sprintf(
		"Loaded %s with %s and %s in %s.",
		countNoun(torrents, "torrent", "torrents"),
		countNoun(files, "file", "files"),
		countNoun(matches, "match", "matches"),
		formatElapsed(time.Since(start)),
	))
}

func (a *App) handleMatchDetail(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	tor := a.getTorrent(hash)
	if tor == nil {
		msg := "No torrent with this hash is loaded. Run a scan on the Match page first, then open a torrent from the list."
		a.mu.Lock()
		hasLib := a.library != nil
		a.mu.Unlock()
		if hasLib {
			msg = "That hash is not in the current scan results. Go back to Match and pick a torrent from the list."
		}
		a.renderNotFound(w, pageData{
			Title:   "Torrent not found",
			Message: msg,
			Hash:    hash,
		})
		return
	}
	detail := detailFromTorrent(tor)
	applyPlan(detail, tor, r.URL.Query().Get("incomplete") == "1")
	a.render(w, "match_detail.html", pageData{
		Title:   tor.Info.Name,
		Flash:   a.takeFlash(),
		Error:   a.takeError(),
		Torrent: detail,
	})
}

func (a *App) handleMatchPlan(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	tor := a.getTorrent(hash)
	if tor == nil {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()
	allowIncomplete := r.URL.Query().Get("incomplete") == "1" || r.FormValue("incomplete") == "1"
	detail := detailFromTorrent(tor)
	applyPlan(detail, tor, allowIncomplete)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.tpl.ExecuteTemplate(w, "match_plan_panel", detail); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func applyPlan(detail *torrentDetail, tor *filematcher.Torrent, allowIncomplete bool) {
	detail.Plan = nil
	detail.PlanTree = nil
	detail.PlanErr = ""
	detail.PlanHint = ""
	detail.ConflictIndexes = tor.DuplicateSelectedIndexes()
	if detail.ConflictIndexes == nil {
		detail.ConflictIndexes = []int{}
	}
	plan, err := tor.MakePlan(filematcher.PlanOptions{AllowIncomplete: allowIncomplete})
	if err != nil {
		if errors.Is(err, filematcher.ErrIncomplete) {
			detail.PlanHint = "Check Allow incomplete to preview the save plan for this partial match."
			return
		}
		detail.PlanErr = err.Error()
		return
	}
	detail.Plan = plan
	detail.PlanTree = buildPlanTree(plan.MappedFiles)
}

func (a *App) handleMatchSelect(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	tor := a.getTorrent(hash)
	if tor == nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fileIdx, _ := strconv.Atoi(r.FormValue("file"))
	candIdx, _ := strconv.Atoi(r.FormValue("candidate"))
	if m := tor.Matches[fileIdx]; m != nil && candIdx >= 0 && candIdx < len(m.Candidates) {
		m.SelectedIndex = candIdx
	}

	if r.Header.Get("HX-Request") != "true" {
		loc, ok := matchDetailPath(hash)
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, loc, http.StatusSeeOther)
		return
	}

	detail := detailFromTorrent(tor)
	applyPlan(detail, tor, r.FormValue("incomplete") == "1")

	var card *fileCardData
	for _, f := range detail.Files {
		if f.Index == fileIdx {
			card = &fileCardData{Hash: hash, File: f}
			break
		}
	}
	if card == nil {
		http.Error(w, "file not found", http.StatusBadRequest)
		return
	}

	a.render(w, "match_select_response.html", pageData{
		Torrent:  detail,
		FileCard: card,
	})
}

func (a *App) handleMatchSave(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	loc, ok := matchDetailPath(hash)
	if !ok {
		http.NotFound(w, r)
		return
	}
	tor := a.getTorrent(hash)
	if tor == nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	allowIncomplete := r.FormValue("incomplete") == "1"
	if tor.Status() == filematcher.MatchNone {
		a.setError("Cannot save: torrent has no matches.")
		http.Redirect(w, r, loc, http.StatusSeeOther)
		return
	}
	plan, err := tor.MakePlan(filematcher.PlanOptions{AllowIncomplete: allowIncomplete})
	if err != nil {
		a.setError(err.Error())
		http.Redirect(w, r, loc, http.StatusSeeOther)
		return
	}
	written, err := tor.Save(plan, filematcher.DefaultSaveOptions())
	if err != nil {
		a.setError(err.Error())
		http.Redirect(w, r, loc, http.StatusSeeOther)
		return
	}
	msg := "Already up to date."
	if written {
		msg = "Fastresume updated."
	}
	if plan.IsIncomplete {
		msg += " Incomplete — qBittorrent will recheck on next start."
	}
	a.setFlash(msg)
	http.Redirect(w, r, loc, http.StatusSeeOther)
}

func (a *App) getTorrent(hash string) *filematcher.Torrent {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.library == nil {
		return nil
	}
	return a.library.Torrents[hash]
}

// matchDetailPath returns a safe /match/{hash} location when hash looks like
// a torrent infohash (40- or 64-char hex).
func matchDetailPath(hash string) (string, bool) {
	if !isTorrentHash(hash) {
		return "", false
	}
	return "/match/" + hash, true
}

func isTorrentHash(hash string) bool {
	n := len(hash)
	if n != 40 && n != 64 {
		return false
	}
	for i := 0; i < n; i++ {
		c := hash[i]
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

func rowsFromLibrary(lib *filematcher.Library) []*torrentRow {
	list := lib.List()
	sort.Slice(list, func(i, j int) bool { return list[i].Info.Name < list[j].Info.Name })
	out := make([]*torrentRow, 0, len(list))
	for _, t := range list {
		out = append(out, &torrentRow{
			Hash:     t.Info.HashV1,
			Name:     t.Info.Name,
			Status:   t.Status().String(),
			Location: t.LocationStatus().String(),
			Tags:     strings.Join(t.Info.Tags, ", "),
			Files:    len(t.Info.RealFiles()),
		})
	}
	return out
}

// libraryScanStats counts torrents, real files, and files that have at least one match.
func libraryScanStats(lib *filematcher.Library) (torrents, files, matches int) {
	if lib == nil {
		return 0, 0, 0
	}
	torrents = len(lib.Torrents)
	for _, t := range lib.Torrents {
		for _, f := range t.Info.RealFiles() {
			files++
			if t.Matches[f.Index].HasMatch() {
				matches++
			}
		}
	}
	return torrents, files, matches
}

// formatElapsed returns a compact duration like "3m17s" (no sub-second digits).
func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Truncate(time.Second)
	if d == 0 {
		return "0s"
	}
	return d.String()
}

// countNoun formats n with singular or plural noun (0 and n≠1 use plural).
func countNoun(n int, singular, pluralForm string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, pluralForm)
}

func detailFromTorrent(t *filematcher.Torrent) *torrentDetail {
	loc := t.LocationStatus()
	d := &torrentDetail{
		Hash:           t.Info.HashV1,
		Name:           t.Info.Name,
		Status:         t.Status().String(),
		Location:       loc.String(),
		AlreadyCurrent: loc == filematcher.LocationCurrent,
		CanSave:        t.Status() != filematcher.MatchNone,
		Tags:           t.Info.Tags,
	}
	allSingle := true
	for _, f := range t.Info.RealFiles() {
		m := t.Matches[f.Index]
		row := fileRow{Index: f.Index, Path: f.Path, Size: f.Size}
		if m != nil {
			row.Candidates = m.Candidates
			row.SelectedIndex = m.SelectedIndex
			row.AutoIndex = m.AutoIndex
		}
		if len(row.Candidates) != 1 {
			allSingle = false
		}
		d.Files = append(d.Files, row)
	}
	d.AllSingleMatch = allSingle && len(d.Files) > 0
	return d
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// Handler returns the root HTTP handler (with access logging to stdout).
func (a *App) Handler() http.Handler {
	return accessLog(a.mux, os.Stdout)
}

// ListenAndServe starts the HTTP server and blocks until ctx is cancelled.
func (a *App) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Addr:              a.Config.Addr(),
		Handler:           a.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
