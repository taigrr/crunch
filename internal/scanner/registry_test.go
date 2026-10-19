package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestFindCrushDBs_Registries(t *testing.T) {
	for _, name := range []string{
		"projects", "workspaces", "both", "duplicates", "last record wins",
		"workspace overrides project", "malformed lines", "long line",
		"missing registries", "empty registries", "malformed registries",
		"outside root", "sibling prefix", "stale entries", "empty data dir",
		"directory not database", "data dir under root", "missing root",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			root := filepath.Join(dir, "root")
			global := filepath.Join(dir, "global")
			data := filepath.Join(dir, "external-data")
			otherData := filepath.Join(dir, "other-data")
			localData := filepath.Join(root, "local", ".crush")
			for _, path := range []string{root, global, data, otherData, localData} {
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, path := range []string{data, otherData, localData} {
				if err := os.WriteFile(filepath.Join(path, "crush.db"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("CRUSH_GLOBAL_DATA", global)
			projectPath := filepath.Join(root, "project")
			marshal := func(v any) string {
				t.Helper()
				b, err := json.Marshal(v)
				if err != nil {
					t.Fatal(err)
				}
				return string(b)
			}
			project := func(path, dataDir string) registryProject {
				return registryProject{Path: path, DataDir: dataDir}
			}
			projects := func(entries ...registryProject) string {
				return marshal(map[string]any{"projects": entries})
			}
			workspace := func(path, dataDir string) string {
				return marshal(map[string]any{"root": path, "data_dir": dataDir, "last_used": 1}) + "\n"
			}
			var projectJSON, workspaceJSON string
			want := []string{filepath.Join(data, "crush.db")}
			fallback := []string{filepath.Join(localData, "crush.db")}
			switch name {
			case "projects":
				projectJSON = projects(project(projectPath, data))
			case "workspaces":
				workspaceJSON = workspace(projectPath, data)
			case "both":
				projectJSON = projects(project(projectPath, data))
				workspaceJSON = workspace(filepath.Join(root, "other"), otherData)
				want = append(want, filepath.Join(otherData, "crush.db"))
			case "duplicates":
				projectJSON = projects(project(projectPath, data), project(projectPath, data))
				workspaceJSON = workspace(projectPath, data) + workspace(filepath.Join(root, "other"), data)
			case "last record wins":
				workspaceJSON = workspace(projectPath, otherData) + workspace(projectPath, data)
			case "workspace overrides project":
				projectJSON = projects(project(projectPath, otherData))
				workspaceJSON = workspace(projectPath, data)
			case "malformed lines":
				projectJSON = "{"
				workspaceJSON = "\ninvalid\n{}\n" + workspace(projectPath, data) + "{\"root\":\n"
			case "long line":
				workspaceJSON = marshal(map[string]any{
					"root": projectPath, "data_dir": data, "extra": strings.Repeat("x", 70*1024),
				}) + "\n"
			case "missing registries":
				want = fallback
			case "empty registries":
				projectJSON, workspaceJSON = projects(), "\n"
				want = fallback
			case "malformed registries":
				projectJSON, workspaceJSON = "{", "{\n"
				want = fallback
			case "outside root":
				projectJSON = projects(project(dir, data))
				workspaceJSON = workspace(dir, data)
				want = fallback
			case "sibling prefix":
				workspaceJSON = workspace(root+"-other", data)
				want = fallback
			case "stale entries":
				workspaceJSON = workspace(projectPath, filepath.Join(dir, "missing"))
				want = fallback
			case "empty data dir":
				workspaceJSON = workspace(projectPath, "")
				want = fallback
			case "directory not database":
				directoryData := filepath.Join(dir, "directory-data")
				if err := os.MkdirAll(filepath.Join(directoryData, "crush.db"), 0o755); err != nil {
					t.Fatal(err)
				}
				workspaceJSON = workspace(projectPath, directoryData)
				want = fallback
			case "data dir under root":
				workspaceJSON = workspace(dir, localData)
				want = fallback
			case "missing root":
				root = filepath.Join(dir, "missing-root")
				workspaceJSON = workspace(root, data)
			}
			for filename, content := range map[string]string{
				"projects.json": projectJSON, "workspaces.jsonl": workspaceJSON,
			} {
				if content != "" {
					if err := os.WriteFile(filepath.Join(global, filename), []byte(content), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			var callbacks []string
			got, err := FindCrushDBs(root, &Options{
				OnFound: func(path string) { callbacks = append(callbacks, path) },
			})
			if name == "missing root" {
				if !os.IsNotExist(err) {
					t.Fatalf("expected missing root error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			slices.Sort(got)
			slices.Sort(want)
			slices.Sort(callbacks)
			if !slices.Equal(got, want) {
				t.Errorf("databases = %v, want %v", got, want)
			}
			if !slices.Equal(callbacks, want) {
				t.Errorf("callbacks = %v, want %v", callbacks, want)
			}
		})
	}
}

func TestGlobalDataDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(dir, "override"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "xdg"))
	t.Setenv("LOCALAPPDATA", filepath.Join(dir, "local"))
	t.Setenv("USERPROFILE", filepath.Join(dir, "profile"))
	t.Setenv("HOME", filepath.Join(dir, "home"))
	check := func(want string) {
		t.Helper()
		got, err := globalDataDir()
		if err != nil || got != want {
			t.Fatalf("globalDataDir() = %q, %v; want %q", got, err, want)
		}
	}
	check(filepath.Join(dir, "override"))
	t.Setenv("CRUSH_GLOBAL_DATA", "")
	check(filepath.Join(dir, "xdg", "crush"))
	t.Setenv("XDG_DATA_HOME", "")
	if runtime.GOOS == "windows" {
		check(filepath.Join(dir, "local", "crush"))
		t.Setenv("LOCALAPPDATA", "")
		check(filepath.Join(dir, "profile", "AppData", "Local", "crush"))
	} else {
		check(filepath.Join(dir, "home", ".local", "share", "crush"))
	}
}
