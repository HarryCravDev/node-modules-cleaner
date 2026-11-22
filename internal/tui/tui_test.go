package tui

import (
	"strings"
	"testing"

	"github.com/harrycraven/nmclean/internal/scanner"
)

func TestChecklistStateTransitions(t *testing.T) {
	items := []scanner.Entry{
		{Path: "/tmp/b/node_modules", RelPath: "b/node_modules", Size: 2048},
		{Path: "/tmp/a/node_modules", RelPath: "a/node_modules", Size: 1024},
	}

	s := NewState(items)

	// Toggle first item
	s = Update(s, KeySpace)
	if SelectedCount(s) != 1 || SelectedBytes(s) != 2048 {
		t.Fatalf("after toggle: count=%d bytes=%d, want 1 and 2048", SelectedCount(s), SelectedBytes(s))
	}

	// Move down and select all
	s = Update(s, KeyDown)
	if s.Cursor != 1 {
		t.Fatalf("cursor = %d, want 1", s.Cursor)
	}
	s = Update(s, KeySelectAll)
	if SelectedCount(s) != 2 || SelectedBytes(s) != 3072 {
		t.Fatalf("after select all: count=%d bytes=%d, want 2 and 3072", SelectedCount(s), SelectedBytes(s))
	}

	// Sort by path
	s = Update(s, KeySort)
	if s.SortBy != SortByPath || s.Items[0].RelPath != "a/node_modules" {
		t.Fatalf("sort by path failed: mode=%v first=%s", s.SortBy, s.Items[0].RelPath)
	}

	// Enter confirmation then cancel back with 'n'
	s = Update(s, KeyEnter)
	if !s.Confirming || s.Done {
		t.Fatalf("expected Confirming=true Done=false, got %+v", s)
	}
	s = Update(s, KeySelectNone)
	if s.Confirming || s.Done {
		t.Fatalf("expected confirmation cancelled back to list, got %+v", s)
	}

	// Confirm and accept
	s = Update(s, KeyEnter)
	s = Update(s, KeyYes)
	if !s.Done || s.Cancelled {
		t.Fatalf("expected Done=true Cancelled=false, got %+v", s)
	}
	if len(SelectedEntries(s)) != 2 {
		t.Fatalf("expected 2 selected entries, got %d", len(SelectedEntries(s)))
	}
}

func TestRenderContainsItemsAndSummary(t *testing.T) {
	items := []scanner.Entry{
		{Path: "/tmp/app/node_modules", RelPath: "app/node_modules", Size: 1024 * 1024},
	}
	s := NewState(items)
	s = Update(s, KeySpace)

	out := Render(s, 80, 24)
	if !strings.Contains(out, "app/node_modules") {
		t.Errorf("render output missing item path:\n%s", out)
	}
	if !strings.Contains(out, "1.0 MB") {
		t.Errorf("render output missing formatted size:\n%s", out)
	}
}
