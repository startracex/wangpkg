package storage

import (
	"encoding/json/v2"
	"fmt"
	"os"

	"github.com/startracex/wangpkg/pkg/manifest"
)

// FileMap is the cached listing of an extracted package.
type FileMap struct {
	Files     map[string]File `json:"files"`
	Dist      manifest.Dist   `json:"dist"`
	Prefix    string          `json:"prefix"`
	FileCount int             `json:"fileCount"`
}

// File is one extracted file, addressed by its own integrity.
type File struct {
	Integrity manifest.Integrity `json:"integrity"`
	Mode      int64              `json:"mode"`
	Size      int64              `json:"size"`
}

// newFileMap builds a file map from the stored files, keyed by their path relative to the package prefix.
func newFileMap(pm *PathMap[File], d manifest.Dist) *FileMap {
	i := &FileMap{Dist: d}
	i.FileCount = pm.Len()
	out := make(map[string]File, len(pm.Map()))
	for fullname, v := range pm.Map() {
		out[pm.Relative(fullname)] = v
	}
	i.Files = out
	i.Prefix = pm.Prefix()
	return i
}

// writeFileMap persists a file map as JSON.
func writeFileMap(filePath string, fm *FileMap) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("create %s: %w", filePath, err)
	}
	defer func() { _ = file.Close() }()
	if err := json.MarshalWrite(file, fm); err != nil {
		return fmt.Errorf("write %s: %w", filePath, err)
	}
	return nil
}

// readFileMap loads a cached file map; a missing or unreadable file is an error, which callers treat as a cache miss.
func readFileMap(filePath string) (*FileMap, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filePath, err)
	}
	defer func() { _ = file.Close() }()

	fm := new(FileMap)
	if err := json.UnmarshalRead(file, fm); err != nil {
		return nil, fmt.Errorf("decode %s: %w", filePath, err)
	}

	return fm, nil
}
