package scanner

import (
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
