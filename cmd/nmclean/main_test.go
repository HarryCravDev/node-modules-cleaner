package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	cfg, err := parseArgs([]string{"/tmp/repos", "--dry-run", "-w", "4"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.targetDir != "/tmp/repos" || !cfg.dryRun || cfg.workers != 4 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestDryRunAndYesMode(t *testing.T) {
	root := t.TempDir()
	nmDir := filepath.Join(root, "web-app", "node_modules")
	if err := os.MkdirAll(nmDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nmDir, "bundle.js"), make([]byte, 4096), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// 1. Dry-run should preview without deleting.
	var out, errOut bytes.Buffer
	code := run([]string{root, "--dry-run"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("dry-run exit code = %d, stderr = %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "web-app/node_modules") || !strings.Contains(out.String(), "dry run") {
		t.Errorf("unexpected dry-run output:\n%s", out.String())
	}
	if _, err := os.Stat(nmDir); err != nil {
		t.Fatalf("dry-run should not delete %s", nmDir)
	}

	// 2. --yes should delete and report freed space.
	out.Reset()
	errOut.Reset()
	code = run([]string{root, "--yes"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("--yes exit code = %d, stderr = %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Deleted 1 folder(s), freed 4.0 KB") {
		t.Errorf("unexpected delete output:\n%s", out.String())
	}
	if _, err := os.Stat(nmDir); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be deleted", nmDir)
	}
}
