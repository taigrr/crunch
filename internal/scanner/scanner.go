// Package scanner provides filesystem scanning for crush.db files.
package scanner

import (
	_ "embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed skipdirs.txt
var skipDirsFile string

// DefaultSkipDirs contains directories to skip during scanning.
var DefaultSkipDirs = parseSkipDirs(skipDirsFile)

func parseSkipDirs(content string) map[string]bool {
	dirs := make(map[string]bool)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			dirs[line] = true
		}
	}
	return dirs
}

// Options configures the scanner behavior.
type Options struct {
	SkipDirs map[string]bool
	OnSkip   func(path string)
	OnFound  func(path string)
}

// FindCrushDBs walks the directory tree from root and returns paths to all crush.db files found.
func FindCrushDBs(root string, opts *Options) ([]string, error) {
	if opts == nil {
		opts = &Options{}
	}
	skipDirs := opts.SkipDirs
	if skipDirs == nil {
		skipDirs = DefaultSkipDirs
	}

	var dbFiles []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root || errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
				return err
			}
			return nil
		}

		if d.IsDir() {
			base := filepath.Base(path)
			if skipDirs[base] {
				if opts.OnSkip != nil {
					opts.OnSkip(path)
				}
				return filepath.SkipDir
			}
			return nil
		}

		if d.Name() == "crush.db" {
			dbFiles = append(dbFiles, path)
			if opts.OnFound != nil {
				opts.OnFound(path)
			}
		}
		return nil
	})

	return dbFiles, err
}
