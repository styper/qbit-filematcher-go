package filematcher

import (
	"crypto/sha1" // #nosec G505 -- BEP 3 infohash is defined as SHA-1.
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// LoadTorrent loads a single torrent / fastresume pair.
//
// hashV1 may be empty; when empty it is derived from the torrent info dict
// as SHA-1 of the canonical bencoded info dictionary (BEP 3).
func LoadTorrent(paths Paths, hashV1 string) (*Torrent, error) {
	torrentRaw, err := os.ReadFile(paths.TorrentFile)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot read %s: %v", ErrParse, filepath.Base(paths.TorrentFile), err)
	}
	fastresumeRaw, err := os.ReadFile(paths.FastresumeFile)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot read %s: %v", ErrParse, filepath.Base(paths.FastresumeFile), err)
	}

	torrentDict, err := decodeBencodeDict(torrentRaw)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot decode %s: %v", ErrParse, filepath.Base(paths.TorrentFile), err)
	}
	fastresumeDict, err := decodeBencodeDict(fastresumeRaw)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot decode %s: %v", ErrParse, filepath.Base(paths.FastresumeFile), err)
	}

	info, err := parseInfo(hashV1, torrentDict, fastresumeDict, filepath.Base(paths.TorrentFile), filepath.Base(paths.FastresumeFile))
	if err != nil {
		return nil, err
	}

	matches := make(map[int]*FileMatch, len(info.Files))
	for _, f := range info.Files {
		if f.IsPad {
			continue
		}
		matches[f.Index] = &FileMatch{SelectedIndex: 0}
	}

	return &Torrent{
		Info:           info,
		Paths:          paths,
		Matches:        matches,
		torrentDict:    torrentDict,
		fastresumeDict: fastresumeDict,
	}, nil
}

func parseInfo(
	hashV1 string,
	torrentData map[string]any,
	fastresumeData map[string]any,
	torrentName string,
	fastresumeName string,
) (Info, error) {
	infoRaw, ok := torrentData["info"]
	if !ok {
		return Info{}, fmt.Errorf("%w: %s: missing 'info' dict", ErrParse, torrentName)
	}
	infoDict, ok := infoRaw.(map[string]any)
	if !ok {
		return Info{}, fmt.Errorf("%w: %s: invalid 'info' dict", ErrParse, torrentName)
	}

	name, ok := asString(infoDict["name"])
	if !ok || name == "" {
		return Info{}, fmt.Errorf("%w: %s: missing or invalid 'info.name'", ErrParse, torrentName)
	}

	if hashV1 == "" {
		encoded, err := encodeBencode(infoDict)
		if err != nil {
			return Info{}, fmt.Errorf("%w: %s: cannot encode info dict: %v", ErrParse, torrentName, err)
		}
		sum := sha1.Sum(encoded) // #nosec G401 -- BEP 3 infohash is SHA-1.
		hashV1 = fmt.Sprintf("%x", sum[:])
	}

	files, err := parseFiles(infoDict, name, torrentName)
	if err != nil {
		return Info{}, err
	}

	displayName := name
	if qbtName, ok := asString(fastresumeData["qBt-name"]); ok && qbtName != "" {
		displayName = qbtName
	}

	if mapped, ok := fastresumeData["mapped_files"].([]any); ok && len(mapped) > 0 {
		for i, m := range mapped {
			if i >= len(files) {
				return Info{}, fmt.Errorf("%w: %s: mapped_files index %d out of range", ErrParse, fastresumeName, i)
			}
			if files[i].IsPad {
				continue
			}
			if s, ok := asString(m); ok && s != "" {
				files[i].Path = ParsePath(s)
			}
		}
	}

	savePath := ""
	if s, ok := asString(fastresumeData["save_path"]); ok && s != "" {
		savePath = ParsePath(s)
	}
	downloadPath := ""
	if s, ok := asString(fastresumeData["qBt-downloadPath"]); ok && s != "" {
		downloadPath = ParsePath(s)
	}

	var tags []string
	if rawTags, ok := fastresumeData["qBt-tags"].([]any); ok {
		for _, t := range rawTags {
			if s, ok := asString(t); ok && s != "" {
				tags = append(tags, s)
			}
		}
		sort.Strings(tags)
	}

	var addedTime int64
	if n, ok := asInt64(fastresumeData["added_time"]); ok && n >= 0 {
		addedTime = n
	}

	return Info{
		HashV1:       hashV1,
		Name:         displayName,
		Files:        files,
		Tags:         tags,
		AddedTime:    addedTime,
		SavePath:     savePath,
		DownloadPath: downloadPath,
	}, nil
}

func parseFiles(infoDict map[string]any, torrentName, fileLabel string) ([]File, error) {
	if rawFiles, ok := infoDict["files"]; ok {
		list, ok := rawFiles.([]any)
		if !ok {
			return nil, fmt.Errorf("%w: %s: 'info.files' is not a list", ErrParse, fileLabel)
		}
		files := make([]File, 0, len(list))
		for i, item := range list {
			fd, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%w: %s: 'info.files[%d]' is not a dict", ErrParse, fileLabel, i)
			}
			pathParts, ok := fd["path"].([]any)
			if !ok || len(pathParts) == 0 {
				return nil, fmt.Errorf("%w: %s: 'info.files[%d].path' missing/empty", ErrParse, fileLabel, i)
			}
			parts := make([]string, 0, len(pathParts))
			for _, p := range pathParts {
				s, ok := asString(p)
				if !ok || s == "" {
					return nil, fmt.Errorf("%w: %s: 'info.files[%d].path' has invalid part", ErrParse, fileLabel, i)
				}
				parts = append(parts, s)
			}
			length, ok := asInt64(fd["length"])
			if !ok {
				return nil, fmt.Errorf("%w: %s: 'info.files[%d].length' must be int", ErrParse, fileLabel, i)
			}
			attr, _ := asString(fd["attr"])
			files = append(files, File{
				Index: i,
				Path:  path.Join(parts...),
				Size:  length,
				IsPad: strings.Contains(attr, "p"),
			})
		}
		return files, nil
	}

	length, ok := asInt64(infoDict["length"])
	if !ok {
		return nil, fmt.Errorf("%w: %s: single-file torrent missing 'info.length'", ErrParse, fileLabel)
	}
	return []File{{
		Index: 0,
		Path:  torrentName,
		Size:  length,
		IsPad: false,
	}}, nil
}
