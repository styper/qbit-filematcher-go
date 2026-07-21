package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/styper/qbit-filematcher-go/filematcher"
	"go.yaml.in/yaml/v3"
)

const (
	// DefaultFileName is the config file basename.
	DefaultFileName = "qbit-filematcher.yaml"
	// AppConfigDirName is the per-app directory under the OS config root.
	AppConfigDirName = "qbit-filematcher"
)

// Unprivileged listen ports (non-root).
const (
	MinPort = 1024
	MaxPort = 65535
)

// Settings are values persisted across CLI and WEB sessions.
type Settings struct {
	BTBackupLocation string   `yaml:"bt_backup_location,omitempty" json:"bt_backup_location"`
	SearchPaths      []string `yaml:"search_paths,omitempty" json:"search_paths"`
	ExcludeDirs      []string `yaml:"exclude_dirs,omitempty" json:"exclude_dirs"`
	Host             string   `yaml:"host,omitempty" json:"host"`
	Port             int      `yaml:"port,omitempty" json:"port"`
}

// testExeDir overrides ExecutableDir when non-empty (tests only).
var testExeDir string

// Default returns settings with conventional defaults.
func Default() Settings {
	return Settings{
		BTBackupLocation: filematcher.DefaultBTBackup(),
		Host:             "localhost",
		Port:             8080,
	}
}

// ExecutableDir returns the directory containing the running binary.
// Symlinks are resolved so a portable config sits next to the real executable.
func ExecutableDir() (string, error) {
	if testExeDir != "" {
		return testExeDir, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// AdjacentPath returns <binary-dir>/qbit-filematcher.yaml (portable lookup).
func AdjacentPath() string {
	dir, err := ExecutableDir()
	if err != nil {
		return DefaultFileName
	}
	return filepath.Join(dir, DefaultFileName)
}

// UserPath returns the OS user config path for this app:
//   - Linux: $XDG_CONFIG_HOME/qbit-filematcher/qbit-filematcher.yaml (or ~/.config/…)
//   - macOS: ~/Library/Application Support/qbit-filematcher/qbit-filematcher.yaml
//   - Windows: %APPDATA%\qbit-filematcher\qbit-filematcher.yaml
func UserPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, AppConfigDirName, DefaultFileName), nil
}

// DefaultPath returns the preferred path when creating a new config file
// (user config dir). Falls back to binary-adjacent if UserConfigDir fails.
func DefaultPath() string {
	if p, err := UserPath(); err == nil {
		return p
	}
	return AdjacentPath()
}

// Path resolves the config file path.
//
// Search order when explicit is empty:
//  1. <binary-dir>/qbit-filematcher.yaml (if it exists)
//  2. user config dir path (if it exists)
//  3. user config dir path as the write/fallback location (even if missing)
//
// A non-empty explicit path (--config) is returned as-is.
func Path(explicit string) string {
	if explicit != "" {
		return explicit
	}
	adj := AdjacentPath()
	if fileExists(adj) {
		return adj
	}
	user, err := UserPath()
	if err != nil {
		return adj
	}
	if fileExists(user) {
		return user
	}
	return user
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// Load reads settings from path. Missing file yields defaults.
func Load(path string) (Settings, error) {
	s := Default()
	path = Path(path)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return s, fmt.Errorf("config: read %s: %w", path, err)
	}
	var raw Settings
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return s, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return merge(s, raw), nil
}

func merge(base, over Settings) Settings {
	if over.BTBackupLocation != "" {
		base.BTBackupLocation = over.BTBackupLocation
	}
	if over.SearchPaths != nil {
		base.SearchPaths = over.SearchPaths
	}
	if over.ExcludeDirs != nil {
		base.ExcludeDirs = over.ExcludeDirs
	}
	if over.Host != "" {
		base.Host = over.Host
	}
	if over.Port != 0 {
		base.Port = over.Port
	}
	return base
}

// Save writes settings to path.
func Save(path string, s Settings) error {
	path = Path(path)
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("config: encode: %w", err)
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("config: mkdir: %w", err)
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("config: write: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("config: rename: %w", err)
	}
	return nil
}

// Remove deletes the config file if it exists.
func Remove(path string) error {
	path = Path(path)
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Validate checks all persisted settings (paths + web listen address).
func (s Settings) Validate() error {
	var errs []string
	errs = append(errs, collectErrors(s.ValidatePaths())...)
	errs = append(errs, collectErrors(s.ValidateListen())...)
	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "\n"))
}

// ValidateMatch checks settings required to run a match (paths only).
func (s Settings) ValidateMatch() error {
	return s.ValidatePaths()
}

// ValidatePaths checks BT_backup and search paths.
func (s Settings) ValidatePaths() error {
	var errs []string
	if err := checkReadableDir(s.BTBackupLocation, "bt_backup_location"); err != nil {
		errs = append(errs, err.Error())
	}
	if len(s.SearchPaths) == 0 {
		errs = append(errs, "at least one search_path is required")
	}
	for _, p := range s.SearchPaths {
		if err := checkReadableDir(p, "search_path"); err != nil {
			errs = append(errs, err.Error())
		}
	}
	for _, name := range s.ExcludeDirs {
		if name == "" {
			errs = append(errs, "exclude_dirs entries must not be empty")
			continue
		}
		if strings.ContainsAny(name, `/\:`) {
			errs = append(errs, fmt.Sprintf("exclude_dirs entry %q must be a directory name, not a path", name))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "\n"))
}

// ValidateListen checks host and port for the WEB server.
func (s Settings) ValidateListen() error {
	var errs []string
	if err := validateHost(s.Host); err != nil {
		errs = append(errs, err.Error())
	}
	if s.Port < MinPort || s.Port > MaxPort {
		errs = append(errs, fmt.Sprintf("port must be between %d and %d (non-root)", MinPort, MaxPort))
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "\n"))
}

func checkReadableDir(path, label string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("%s is required", label)
	}
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s %q does not exist", label, path)
		}
		return fmt.Errorf("%s %q: %v", label, path, err)
	}
	if !st.IsDir() {
		return fmt.Errorf("%s %q is not a directory", label, path)
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%s %q is not readable", label, path)
	}
	_ = f.Close()
	return nil
}

// hostPattern allows hostnames, IPv4, and IPv6 literals (letters, digits, . - :).
var hostPattern = regexp.MustCompile(`^[0-9A-Za-z.\-:]+$`)

func validateHost(host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return errors.New("host is required")
	}
	if strings.ContainsAny(host, " \t/") {
		return errors.New("host must not contain spaces or slashes")
	}
	if !hostPattern.MatchString(host) {
		return errors.New("host may only contain letters, digits, '.', '-', and ':'")
	}
	// Reject accidental "hostname:port" (IPv6 uses multiple ':' or leading "::").
	if strings.Count(host, ":") == 1 && !strings.HasPrefix(host, ":") {
		_, portPart, ok := strings.Cut(host, ":")
		if ok {
			if _, err := strconv.Atoi(portPart); err == nil {
				return errors.New("host must not include a port; use the Port field")
			}
		}
	}
	return nil
}

func collectErrors(err error) []string {
	if err == nil {
		return nil
	}
	return strings.Split(err.Error(), "\n")
}
