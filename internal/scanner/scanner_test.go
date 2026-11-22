package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanFindsAndMeasuresNodeModules(t *testing.T) {
	root := t.TempDir()

	// Create two repos with node_modules and one nested node_modules inside node_modules.
	mustWriteFile(t, filepath.Join(root, "repo-a", "node_modules", "pkg", "index.js"), make([]byte, 2000))
	mustWriteFile(t, filepath.Join(root, "repo-a", "node_modules", "nested", "node_modules", "dep.js"), make([]byte, 500))
	mustWriteFile(t, filepath.Join(root, "repo-b", "node_modules", "big.bin"), make([]byte, 5000))
	mustWriteFile(t, filepath.Join(root, "repo-c", "src", "app.ts"), make([]byte, 100))
	mustWriteFile(t, filepath.Join(root, ".git", "node_modules", "ignored.js"), make([]byte, 999))

	res, err := Scan(root, 2)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	if len(res.Entries) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(res.Entries), res.Entries)
	}

	// Sorted by size descending: repo-b (5000) first, repo-a (2500) second.
	if res.Entries[0].Size != 5000 {
		t.Errorf("entries[0].Size = %d, want 5000", res.Entries[0].Size)
	}
	if res.Entries[1].Size != 2500 {
		t.Errorf("entries[1].Size = %d, want 2500", res.Entries[1].Size)
	}
	if TotalSize(res.Entries) != 7500 {
		t.Errorf("TotalSize = %d, want 7500", TotalSize(res.Entries))
	}
}

func TestScanRejectsMissingDir(t *testing.T) {
	_, err := Scan(filepath.Join(t.TempDir(), "does-not-exist"), 1)
	if err == nil {
		t.Fatal("expected error for missing directory, got nil")
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
