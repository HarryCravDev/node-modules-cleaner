package cleaner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/harrycraven/nmclean/internal/scanner"
)

func TestDeleteAllRemovesDirectoriesAndReportsFreedBytes(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "proj-a", "node_modules")
	dirB := filepath.Join(root, "proj-b", "node_modules")

	for _, d := range []string{dirA, dirB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
		if err := os.WriteFile(filepath.Join(d, "file.js"), make([]byte, 1024), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	entries := []scanner.Entry{
		{Path: dirA, RelPath: "proj-a/node_modules", Size: 1024},
		{Path: dirB, RelPath: "proj-b/node_modules", Size: 2048},
	}

	progressCount := 0
	summary := DeleteAll(entries, 2, func(_ ItemResult) {
		progressCount++
	})

	if summary.Deleted != 2 {
		t.Errorf("Deleted = %d, want 2", summary.Deleted)
	}
	if summary.Failed != 0 {
		t.Errorf("Failed = %d, want 0", summary.Failed)
	}
	if summary.FreedBytes != 3072 {
		t.Errorf("FreedBytes = %d, want 3072", summary.FreedBytes)
	}
	if progressCount != 2 {
		t.Errorf("progressCount = %d, want 2", progressCount)
	}

	for _, d := range []string{dirA, dirB} {
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			t.Errorf("expected %s to be deleted", d)
		}
	}
}
