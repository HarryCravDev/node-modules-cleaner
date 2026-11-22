package tui

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/harrycraven/nmclean/internal/format"
	"github.com/harrycraven/nmclean/internal/scanner"
	"golang.org/x/term"
)

// SortMode controls how entries are ordered in the checklist.
type SortMode int

const (
	SortBySize SortMode = iota
	SortByPath
)

func (m SortMode) String() string {
	if m == SortByPath {
		return "Path"
	}
	return "Size"
}

// Key represents a normalized user keystroke.
type Key int

const (
	KeyUnknown Key = iota
	KeyUp
	KeyDown
	KeySpace
	KeyEnter
	KeySelectAll
	KeySelectNone
	KeySort
	KeyYes
	KeyNo
	KeyQuit
)

// State holds the complete immutable-style UI state for the checklist.
type State struct {
	Items      []scanner.Entry
	Selected   map[string]bool
	Cursor     int
	Scroll     int
	SortBy     SortMode
	Confirming bool
	Done       bool
	Cancelled  bool
	Notice     string
}

// NewState initializes a checklist state with all entries unselected.
func NewState(entries []scanner.Entry) State {
	copied := make([]scanner.Entry, len(entries))
	copy(copied, entries)
	return State{
		Items:    copied,
		Selected: make(map[string]bool, len(entries)),
		SortBy:   SortBySize,
	}
}

// Update computes the next UI state from the current state and key press.
func Update(s State, k Key) State {
	s.Notice = ""

	if s.Confirming {
		switch k {
		case KeyYes, KeyEnter:
			s.Done = true
		case KeyNo, KeySelectNone, KeyQuit:
			s.Confirming = false
		}
		return s
	}

	switch k {
	case KeyQuit:
		s.Cancelled = true
		s.Done = true
	case KeyUp:
		if s.Cursor > 0 {
			s.Cursor--
		}
	case KeyDown:
		if s.Cursor < len(s.Items)-1 {
			s.Cursor++
		}
	case KeySpace:
		if len(s.Items) > 0 {
			path := s.Items[s.Cursor].Path
			if s.Selected[path] {
				delete(s.Selected, path)
			} else {
				s.Selected[path] = true
			}
		}
	case KeySelectAll:
		for _, item := range s.Items {
			s.Selected[item.Path] = true
		}
	case KeySelectNone:
		s.Selected = make(map[string]bool)
	case KeySort:
		if s.SortBy == SortBySize {
			s.SortBy = SortByPath
		} else {
			s.SortBy = SortBySize
		}
		s.Items = sortedCopy(s.Items, s.SortBy)
		s.Cursor = 0
		s.Scroll = 0
	case KeyEnter:
		if SelectedCount(s) == 0 {
			s.Notice = "Select at least one folder with Space (or press q to quit)."
		} else {
			s.Confirming = true
		}
	}

	return s
}

// SelectedEntries returns the entries currently marked for deletion.
func SelectedEntries(s State) []scanner.Entry {
	out := make([]scanner.Entry, 0, len(s.Selected))
	for _, item := range s.Items {
		if s.Selected[item.Path] {
			out = append(out, item)
		}
	}
	return out
}

// SelectedCount returns the number of selected folders.
func SelectedCount(s State) int {
	return len(s.Selected)
}

// SelectedBytes returns the combined size in bytes of all selected folders.
func SelectedBytes(s State) int64 {
	var total int64
	for _, item := range s.Items {
		if s.Selected[item.Path] {
			total += item.Size
		}
	}
	return total
}

// Render builds the full terminal frame for the given state and dimensions.
func Render(s State, width, height int) string {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	var b strings.Builder
	// Move cursor to top-left and clear screen.
	b.WriteString("\033[H\033[2J")

	// Header
	b.WriteString("\033[1;36mnmclean\033[0m — Select node_modules folders to delete\r\n")
	b.WriteString(fmt.Sprintf(
		"\033[2mSelected: %d / %d  |  Space to free: %s  |  Total found: %s  |  Sort: %s\033[0m\r\n",
		SelectedCount(s),
		len(s.Items),
		format.Bytes(SelectedBytes(s)),
		format.Bytes(scanner.TotalSize(s.Items)),
		s.SortBy.String(),
	))
	b.WriteString(strings.Repeat("─", min(width, 80)) + "\r\n")

	// Calculate visible list viewport (reserve 3 header lines + 3 footer lines).
	viewHeight := max(height-6, 1)
	scroll := adjustScroll(s.Scroll, s.Cursor, viewHeight, len(s.Items))
	end := min(scroll+viewHeight, len(s.Items))

	for i := scroll; i < end; i++ {
		item := s.Items[i]
		cursor := "  "
		if i == s.Cursor {
			cursor = "\033[1;36m> \033[0m"
		}

		check := "[ ]"
		if s.Selected[item.Path] {
			check = "\033[1;32m[x]\033[0m"
		}

		sizeStr := fmt.Sprintf("%9s", format.Bytes(item.Size))
		maxPathLen := max(width-20, 16)
		relPath := truncate(item.RelPath, maxPathLen)

		if i == s.Cursor {
			b.WriteString(fmt.Sprintf("%s%s  \033[1m%s\033[0m  \033[1m%s\033[0m\r\n", cursor, check, sizeStr, relPath))
		} else {
			b.WriteString(fmt.Sprintf("%s%s  %s  %s\r\n", cursor, check, sizeStr, relPath))
		}
	}

	// Pad remaining rows so footer stays steady.
	for i := end - scroll; i < viewHeight; i++ {
		b.WriteString("\r\n")
	}

	b.WriteString(strings.Repeat("─", min(width, 80)) + "\r\n")

	if s.Confirming {
		b.WriteString(fmt.Sprintf(
			"\033[1;33mDelete %d folder(s) and free %s? [y/N]\033[0m\r\n",
			SelectedCount(s),
			format.Bytes(SelectedBytes(s)),
		))
	} else if s.Notice != "" {
		b.WriteString(fmt.Sprintf("\033[33m%s\033[0m\r\n", s.Notice))
	} else {
		b.WriteString("\033[2m↑/↓ move • space toggle • a all • n none • s sort • enter delete • q quit\033[0m\r\n")
	}

	return b.String()
}

// Run launches the interactive checklist in raw terminal mode and returns selected entries.
func Run(entries []scanner.Entry) ([]scanner.Entry, bool, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, false, fmt.Errorf("enable raw terminal: %w", err)
	}
	defer func() {
		_ = term.Restore(fd, oldState)
	}()

	// Enter alternate screen buffer and hide cursor.
	_, _ = io.WriteString(os.Stdout, "\033[?1049h\033[?25l")
	defer func() {
		_, _ = io.WriteString(os.Stdout, "\033[?25h\033[?1049l")
	}()

	state := NewState(entries)
	buf := make([]byte, 8)

	for !state.Done {
		width, height, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil {
			width, height = 80, 24
		}

		viewHeight := max(height-6, 1)
		state.Scroll = adjustScroll(state.Scroll, state.Cursor, viewHeight, len(state.Items))

		_, _ = io.WriteString(os.Stdout, Render(state, width, height))

		n, err := os.Stdin.Read(buf)
		if err != nil {
			return nil, true, err
		}
		key := decodeKey(buf[:n])
		state = Update(state, key)
	}

	if state.Cancelled {
		return nil, true, nil
	}
	return SelectedEntries(state), false, nil
}

func decodeKey(b []byte) Key {
	if len(b) == 0 {
		return KeyUnknown
	}
	// Escape sequences (arrow keys or bare Esc)
	if b[0] == 27 {
		if len(b) >= 3 && b[1] == '[' {
			switch b[2] {
			case 'A':
				return KeyUp
			case 'B':
				return KeyDown
			}
		}
		if len(b) == 1 {
			return KeyQuit
		}
		return KeyUnknown
	}

	switch b[0] {
	case 3: // Ctrl+C
		return KeyQuit
	case 'q', 'Q':
		return KeyQuit
	case 'k', 'K':
		return KeyUp
	case 'j', 'J':
		return KeyDown
	case ' ':
		return KeySpace
	case '\r', '\n':
		return KeyEnter
	case 'a', 'A':
		return KeySelectAll
	case 'n', 'N':
		return KeySelectNone
	case 's', 'S':
		return KeySort
	case 'y', 'Y':
		return KeyYes
	default:
		return KeyUnknown
	}
}

func adjustScroll(scroll, cursor, viewHeight, total int) int {
	if cursor < scroll {
		scroll = cursor
	}
	if cursor >= scroll+viewHeight {
		scroll = cursor - viewHeight + 1
	}
	maxScroll := max(total-viewHeight, 0)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	return scroll
}

func sortedCopy(entries []scanner.Entry, mode SortMode) []scanner.Entry {
	out := make([]scanner.Entry, len(entries))
	copy(out, entries)
	sort.Slice(out, func(i, j int) bool {
		if mode == SortByPath {
			return out[i].RelPath < out[j].RelPath
		}
		if out[i].Size != out[j].Size {
			return out[i].Size > out[j].Size
		}
		return out[i].RelPath < out[j].RelPath
	})
	return out
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return "…" + string(runes[len(runes)-(maxLen-1):])
}
