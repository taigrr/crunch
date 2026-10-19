package scanner

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type registryProject struct {
	Path    string `json:"path"`
	DataDir string `json:"data_dir"`
}

func globalDataDir() (string, error) {
	if dir := os.Getenv("CRUSH_GLOBAL_DATA"); dir != "" {
		return dir, nil
	}
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "crush"), nil
	}
	if runtime.GOOS == "windows" {
		dir := os.Getenv("LOCALAPPDATA")
		if dir == "" {
			dir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(dir, "crush"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "crush"), nil
}

func readProjectRegistry(path string) ([]registryProject, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var registry struct {
		Projects []registryProject `json:"projects"`
	}
	if err := json.Unmarshal(content, &registry); err != nil {
		return nil, nil
	}
	return registry.Projects, nil
}

func readWorkspaceRegistry(path string) ([]registryProject, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var projects []registryProject
	byRoot := make(map[string]int)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var entry struct {
			Root    string `json:"root"`
			DataDir string `json:"data_dir"`
		}
		if err := json.Unmarshal(sc.Bytes(), &entry); err != nil || entry.Root == "" {
			continue
		}
		project := registryProject{Path: entry.Root, DataDir: entry.DataDir}
		if i, ok := byRoot[entry.Root]; ok {
			projects[i] = project
		} else {
			byRoot[entry.Root] = len(projects)
			projects = append(projects, project)
		}
	}
	return projects, sc.Err()
}

func findRegistryDBs(root string, opts *Options) ([]string, error) {
	dir, err := globalDataDir()
	if err != nil {
		return nil, err
	}
	projects, err := readProjectRegistry(filepath.Join(dir, "projects.json"))
	if err != nil {
		return nil, err
	}
	workspaces, err := readWorkspaceRegistry(filepath.Join(dir, "workspaces.jsonl"))
	if err != nil {
		return nil, err
	}
	workspaceRoots := make(map[string]bool, len(workspaces))
	for _, workspace := range workspaces {
		workspaceRoots[workspace.Path] = true
	}
	entries := make([]registryProject, 0, len(projects)+len(workspaces))
	for _, project := range projects {
		if !workspaceRoots[project.Path] {
			entries = append(entries, project)
		}
	}
	entries = append(entries, workspaces...)

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var dbFiles []string
	seen := make(map[string]bool)
	for _, entry := range entries {
		if entry.DataDir == "" || (!isPathUnderRoot(entry.Path, rootAbs) && !isPathUnderRoot(entry.DataDir, rootAbs)) {
			continue
		}
		dbPath := filepath.Join(entry.DataDir, "crush.db")
		if seen[dbPath] {
			continue
		}
		info, err := os.Stat(dbPath)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		seen[dbPath] = true
		dbFiles = append(dbFiles, dbPath)
		if opts.OnFound != nil {
			opts.OnFound(dbPath)
		}
	}
	return dbFiles, nil
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
