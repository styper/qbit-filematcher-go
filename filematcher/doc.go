// Package filematcher rematches on-disk files to qBittorrent torrents after
// content has been moved.
//
// It operates directly on qBittorrent's BT_backup directory (paired .torrent
// and .fastresume files). There is no Web API dependency.
//
// Typical pipeline:
//
//  1. Load a library from BT_backup ([LoadLibrary]).
//  2. Optionally filter torrents ([Library.Filter]).
//  3. Scan search paths for candidates matched by extension + size ([Scan]).
//  4. Select among candidates (caller, or [Torrent.SelectBestMatches]).
//  5. Build a save plan ([Torrent.MakePlan]).
//  6. Persist updated fastresume data ([Torrent.Save]), preferably while
//     qBittorrent is not running.
//
// This package is a standalone library. CLI and web UIs live in separate
// packages and are not required to use this API.
package filematcher
