package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestFindCrushDBs(t *testing.T) {
	tmpDir := t.TempDir()

	projectDir := filepath.Join(tmpDir, "myproject", ".crush")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(projectDir, "crush.db")
	if err := os.WriteFile(dbPath, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	nodeModules := filepath.Join(tmpDir, "node_modules", ".crush")
	if err := os.MkdirAll(nodeModules, 0o755); err != nil {
		t.Fatal(err)
	}
	skippedDB := filepath.Join(nodeModules, "crush.db")
	if err := os.WriteFile(skippedDB, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := FindCrushDBs(tmpDir, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != 1 {
		t.Errorf("expected 1 db, got %d", len(found))
	}

	if len(found) > 0 && found[0] != dbPath {
		t.Errorf("expected %s, got %s", dbPath, found[0])
	}
}

func TestFindCrushDBs_CustomSkipDirs(t *testing.T) {
	tmpDir := t.TempDir()

	customSkip := filepath.Join(tmpDir, "skip_me", ".crush")
	if err := os.MkdirAll(customSkip, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customSkip, "crush.db"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := &Options{
		SkipDirs: map[string]bool{"skip_me": true},
	}

	found, err := FindCrushDBs(tmpDir, opts)
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != 0 {
		t.Errorf("expected 0 dbs (skip_me should be skipped), got %d", len(found))
	}
}

func TestFindCrushDBs_Callbacks(t *testing.T) {
	tmpDir := t.TempDir()

	projectDir := filepath.Join(tmpDir, "proj", ".crush")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "crush.db"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	nodeModules := filepath.Join(tmpDir, "node_modules")
	if err := os.MkdirAll(nodeModules, 0o755); err != nil {
		t.Fatal(err)
	}

	var skipped, foundPaths []string
	opts := &Options{
		OnSkip:  func(path string) { skipped = append(skipped, path) },
		OnFound: func(path string) { foundPaths = append(foundPaths, path) },
	}

	_, err := FindCrushDBs(tmpDir, opts)
	if err != nil {
		t.Fatal(err)
	}

	if len(skipped) != 1 {
		t.Errorf("expected 1 skipped dir, got %d", len(skipped))
	}

	if len(foundPaths) != 1 {
		t.Errorf("expected 1 found path, got %d", len(foundPaths))
	}
}

func TestFindCrushDBs_MissingRoot(t *testing.T) {
	missingRoot := filepath.Join(t.TempDir(), "does-not-exist")

	_, err := FindCrushDBs(missingRoot, nil)
	if err == nil {
		t.Fatal("expected error for missing root")
	}

	if !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got %v", err)
	}
}

func TestFindCrushDBs_MissingRootWithProjectsRegistry(t *testing.T) {
	tmpDir := t.TempDir()
	globalData := filepath.Join(tmpDir, "global")
	missingRoot := filepath.Join(tmpDir, "does-not-exist")
	dataDir := filepath.Join(tmpDir, "project-data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "crush.db"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProjectsRegistry(t, globalData, `{"projects":[{"path":%q,"data_dir":%q}]}`, missingRoot, dataDir)
	t.Setenv("CRUSH_GLOBAL_DATA", globalData)

	_, err := FindCrushDBs(missingRoot, nil)
	if err == nil {
		t.Fatal("expected error for missing root")
	}

	if !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got %v", err)
	}
}

func TestFindCrushDBs_ProjectsRegistry(t *testing.T) {
	tmpDir := t.TempDir()
	globalData := filepath.Join(tmpDir, "global")
	projectDir := filepath.Join(tmpDir, "project")
	dataDir := filepath.Join(tmpDir, "outside-project-data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dataDir, "crush.db")
	if err := os.WriteFile(dbPath, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProjectsRegistry(t, globalData, `{"projects":[{"path":%q,"data_dir":%q}]}`, projectDir, dataDir)
	t.Setenv("CRUSH_GLOBAL_DATA", globalData)

	found, err := FindCrushDBs(tmpDir, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != 1 {
		t.Fatalf("expected 1 db, got %d", len(found))
	}
	if found[0] != dbPath {
		t.Errorf("expected %s, got %s", dbPath, found[0])
	}
}

func TestFindCrushDBs_ProjectsRegistryFiltersRoot(t *testing.T) {
	tmpDir := t.TempDir()
	globalData := filepath.Join(tmpDir, "global")
	searchRoot := filepath.Join(tmpDir, "search-root")
	projectDir := filepath.Join(tmpDir, "other-project")
	dataDir := filepath.Join(tmpDir, "other-data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(searchRoot, "local", ".crush"), 0o755); err != nil {
		t.Fatal(err)
	}
	localDB := filepath.Join(searchRoot, "local", ".crush", "crush.db")
	if err := os.WriteFile(localDB, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "crush.db"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProjectsRegistry(t, globalData, `{"projects":[{"path":%q,"data_dir":%q}]}`, projectDir, dataDir)
	t.Setenv("CRUSH_GLOBAL_DATA", globalData)

	found, err := FindCrushDBs(searchRoot, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != 1 {
		t.Fatalf("expected fallback walk to find 1 db, got %d", len(found))
	}
	if found[0] != localDB {
		t.Errorf("expected %s, got %s", localDB, found[0])
	}
}

func TestFindCrushDBs_ProjectsRegistryFallsBackWhenEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	globalData := filepath.Join(tmpDir, "global")
	projectDir := filepath.Join(tmpDir, "project", ".crush")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(projectDir, "crush.db")
	if err := os.WriteFile(dbPath, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProjectsRegistry(t, globalData, `{"projects":[]}`)
	t.Setenv("CRUSH_GLOBAL_DATA", globalData)

	found, err := FindCrushDBs(tmpDir, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != 1 {
		t.Fatalf("expected fallback walk to find 1 db, got %d", len(found))
	}
	if found[0] != dbPath {
		t.Errorf("expected %s, got %s", dbPath, found[0])
	}
}

func writeProjectsRegistry(t *testing.T, globalData, format string, args ...string) {
	t.Helper()

	if err := os.MkdirAll(globalData, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte(format)
	if len(args) > 0 {
		content = []byte(formatRegistry(format, args...))
	}
	if err := os.WriteFile(filepath.Join(globalData, "projects.json"), content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func formatRegistry(format string, args ...string) string {
	quoted := make([]any, len(args))
	for i, arg := range args {
		quoted[i] = arg
	}
	return fmt.Sprintf(format, quoted...)
}
