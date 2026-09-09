// Package scanner provides filesystem scanning for crush.db files.
package scanner

import (
	_ "embed"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
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

// FindCrushDBs returns paths to crush.db files found under root.
func FindCrushDBs(root string, opts *Options) ([]string, error) {
	if opts == nil {
		opts = &Options{}
	}
	skipDirs := opts.SkipDirs
	if skipDirs == nil {
		skipDirs = DefaultSkipDirs
	}

	dbFiles, foundRegistry, err := findCrushDBsFromRegistry(root, opts)
	if err != nil {
		return nil, err
	}
	if foundRegistry {
		return dbFiles, nil
	}

	return findCrushDBsByWalking(root, opts, skipDirs)
}

func findCrushDBsByWalking(root string, opts *Options, skipDirs map[string]bool) ([]string, error) {
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

type projectsRegistry struct {
	Projects []projectEntry `json:"projects"`
}

type projectEntry struct {
	Path    string `json:"path"`
	DataDir string `json:"data_dir"`
}

func findCrushDBsFromRegistry(root string, opts *Options) ([]string, bool, error) {
	registryPath, ok, err := projectsRegistryPath()
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	registry, err := readProjectsRegistry(registryPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, false, err
	}

	var dbFiles []string
	seen := map[string]bool{}
	for _, project := range registry.Projects {
		dbPath := filepath.Join(project.DataDir, "crush.db")
		if project.DataDir == "" || !isProjectUnderRoot(project, rootAbs) || seen[dbPath] {
			continue
		}
		if _, err := os.Stat(dbPath); err != nil {
			if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, false, err
		}
		seen[dbPath] = true
		dbFiles = append(dbFiles, dbPath)
		if opts.OnFound != nil {
			opts.OnFound(dbPath)
		}
	}

	return dbFiles, len(dbFiles) > 0, nil
}

func projectsRegistryPath() (string, bool, error) {
	if dir := os.Getenv("CRUSH_GLOBAL_DATA"); dir != "" {
		return filepath.Join(dir, "projects.json"), true, nil
	}

	if runtime.GOOS == "windows" {
		dir := os.Getenv("LOCALAPPDATA")
		if dir == "" {
			return "", false, nil
		}
		return filepath.Join(dir, "crush", "projects.json"), true, nil
	}

	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "crush", "projects.json"), true, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", false, err
	}
	return filepath.Join(home, ".local", "share", "crush", "projects.json"), true, nil
}

func readProjectsRegistry(path string) (projectsRegistry, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return projectsRegistry{}, err
	}
	var registry projectsRegistry
	if err := json.Unmarshal(content, &registry); err != nil {
		return projectsRegistry{}, err
	}
	return registry, nil
}

func isProjectUnderRoot(project projectEntry, rootAbs string) bool {
	if isPathUnderRoot(project.Path, rootAbs) || isPathUnderRoot(project.DataDir, rootAbs) {
		return true
	}
	return false
}

func isPathUnderRoot(path, rootAbs string) bool {
	if path == "" {
		return false
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}
