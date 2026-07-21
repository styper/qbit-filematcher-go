package filematcher

import "errors"

var (
	// ErrParse indicates a .torrent or .fastresume file could not be parsed.
	ErrParse = errors.New("filematcher: parse error")

	// ErrStale indicates the fastresume on disk changed since it was loaded.
	ErrStale = errors.New("filematcher: fastresume changed on disk")

	// ErrNoMatches indicates no files were matched when a plan requires them.
	ErrNoMatches = errors.New("filematcher: no matches")

	// ErrIncomplete indicates not all files are matched and incomplete saves
	// were not allowed.
	ErrIncomplete = errors.New("filematcher: incomplete matches")

	// ErrDuplicateMatch indicates the same on-disk file was selected for more
	// than one torrent file.
	ErrDuplicateMatch = errors.New("filematcher: duplicate match selection")

	// ErrInvalidPlan indicates a save plan failed validation.
	ErrInvalidPlan = errors.New("filematcher: invalid save plan")

	// ErrQBitRunning indicates qBittorrent appears to be running and a save
	// was refused. Callers may skip this check if they manage the process
	// themselves.
	ErrQBitRunning = errors.New("filematcher: qBittorrent is running")
)
