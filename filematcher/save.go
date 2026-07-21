package filematcher

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SaveOptions controls how a fastresume update is written.
type SaveOptions struct {
	// Backup, when true, writes a timestamped .bak beside the fastresume
	// before replacing it.
	Backup bool

	// CheckQBitRunning, when true, refuses to save if a qBittorrent process
	// appears to be running.
	CheckQBitRunning bool
}

// DefaultSaveOptions returns the recommended save options.
func DefaultSaveOptions() SaveOptions {
	return SaveOptions{
		Backup:           true,
		CheckQBitRunning: true,
	}
}

// Save applies plan to the torrent's fastresume on disk.
//
// Returns written=false when the on-disk contents already match the plan.
func (t *Torrent) Save(plan *SavePlan, opts SaveOptions) (written bool, err error) {
	if err := t.ValidatePlan(plan); err != nil {
		return false, err
	}

	if opts.CheckQBitRunning {
		running, err := IsQBittorrentRunning()
		if err != nil {
			return false, err
		}
		if running {
			return false, ErrQBitRunning
		}
	}

	onDiskRaw, err := os.ReadFile(t.Paths.FastresumeFile)
	if err != nil {
		return false, fmt.Errorf("%w: cannot read %s: %v", ErrStale, filepath.Base(t.Paths.FastresumeFile), err)
	}
	onDisk, err := decodeBencodeDict(onDiskRaw)
	if err != nil {
		return false, fmt.Errorf("%w: cannot decode %s: %v", ErrStale, filepath.Base(t.Paths.FastresumeFile), err)
	}
	if !deepEqualBencode(onDisk, t.fastresumeDict) {
		return false, fmt.Errorf("%w: %s changed since it was loaded", ErrStale, filepath.Base(t.Paths.FastresumeFile))
	}

	newData := cloneDict(t.fastresumeDict)
	newData["qBt-savePath"] = filepath.ToSlash(plan.SavePath)
	newData["save_path"] = plan.SavePath

	mapped := make([]any, len(plan.MappedFiles))
	for i, m := range plan.MappedFiles {
		mapped[i] = m
	}
	newData["mapped_files"] = mapped

	if t.Info.DownloadPath != "" && t.Info.SavePath != "" {
		if rel, err := filepath.Rel(t.Info.SavePath, t.Info.DownloadPath); err == nil && !containsPathDotDot(rel) {
			newDL := filepath.Join(plan.SavePath, rel)
			newData["qBt-downloadPath"] = filepath.ToSlash(newDL)
		}
	}

	if plan.IsIncomplete {
		newData["pieces"] = ""
	}
	newData["paused"] = int64(1)

	newPayload, err := encodeBencode(newData)
	if err != nil {
		return false, fmt.Errorf("filematcher: encode fastresume: %w", err)
	}
	oldPayload, err := encodeBencode(t.fastresumeDict)
	if err != nil {
		return false, fmt.Errorf("filematcher: encode original fastresume: %w", err)
	}
	if string(newPayload) == string(oldPayload) {
		return false, nil
	}

	if opts.Backup {
		bak := fmt.Sprintf("%s.%s.bak", t.Paths.FastresumeFile, time.Now().Format("2006-01-02_15-04-05"))
		if err := copyFile(t.Paths.FastresumeFile, bak); err != nil {
			return false, fmt.Errorf("filematcher: backup fastresume: %w", err)
		}
	}

	if err := atomicWrite(t.Paths.FastresumeFile, newPayload); err != nil {
		return false, err
	}

	t.fastresumeDict = newData
	t.Info.SavePath = plan.SavePath
	if v, ok := asString(newData["qBt-downloadPath"]); ok {
		t.Info.DownloadPath = ParsePath(v)
	}
	return true, nil
}

func cloneDict(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func containsPathDotDot(rel string) bool {
	for _, p := range splitNonEmpty(filepath.ToSlash(rel), '/') {
		if p == ".." {
			return true
		}
	}
	return false
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func atomicWrite(target string, payload []byte) error {
	tmp := target + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(tmp)
		}
	}()

	if _, err := f.Write(payload); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		return err
	}
	ok = true
	return nil
}
