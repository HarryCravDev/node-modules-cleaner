package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/harrycraven/nmclean/internal/format"
)

// Entry represents a discovered node_modules directory and its measured size.
type Entry struct {
	Path    string
	RelPath string
	Size    int64
	ModTime time.Time
}

// Result holds the outcome of scanning a root directory.
type Result struct {
	Root    string
	Entries []Entry
	Errors  []error
}

var skippedDirs = map[string]bool{
	".git": true,
	".hg":  true,
	".svn": true,
}

// Scan searches root for node_modules directories and calculates their sizes concurrently.
func Scan(root string, workers int) (Result, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Result{}, fmt.Errorf("resolve path %q: %w", root, err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return Result{}, fmt.Errorf("stat %q: %w", absRoot, err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("%q is not a directory", absRoot)
	}

	paths, walkErrs := findNodeModules(absRoot)
	if len(paths) == 0 {
		return Result{Root: absRoot, Errors: walkErrs}, nil
	}

	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	entries, sizeErrs := measureAll(absRoot, paths, workers)
	sortBySizeDesc(entries)

	return Result{
		Root:    absRoot,
		Entries: entries,
		Errors:  append(walkErrs, sizeErrs...),
	}, nil
}

// TotalSize returns the combined byte size of all entries.
func TotalSize(entries []Entry) int64 {
	var total int64
	for _, e := range entries {
		total += e.Size
	}
	return total
}

func findNodeModules(root string) ([]string, []error) {
	var paths []string
	var errs []error

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		name := d.Name()
		if name == "node_modules" {
			paths = append(paths, path)
			return filepath.SkipDir
		}
		if path != root && skippedDirs[name] {
			return filepath.SkipDir
		}
		return nil
	})

	return paths, errs
}

func measureAll(root string, paths []string, workers int) ([]Entry, []error) {
	if workers > len(paths) {
		workers = len(paths)
	}

	jobs := make(chan string, len(paths))
	for _, p := range paths {
		jobs <- p
	}
	close(jobs)

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		entries = make([]Entry, 0, len(paths))
		errs    []error
	)

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				size, modTime, err := dirSizeAndModTime(p)
				mu.Lock()
				if err != nil {
					errs = append(errs, err)
				}
				entries = append(entries, Entry{
					Path:    p,
					RelPath: format.RelativePath(root, p),
					Size:    size,
					ModTime: modTime,
				})
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return entries, errs
}

func dirSizeAndModTime(dir string) (int64, time.Time, error) {
	var size int64
	var modTime time.Time

	if info, err := os.Stat(dir); err == nil {
		modTime = info.ModTime()
	}

	var firstErr error
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return nil
		}
		size += info.Size()
		return nil
	})

	return size, modTime, firstErr
}

func sortBySizeDesc(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Size != entries[j].Size {
			return entries[i].Size > entries[j].Size
		}
		return entries[i].RelPath < entries[j].RelPath
	})
}
