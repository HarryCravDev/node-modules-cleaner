package cleaner

import (
	"os"
	"runtime"
	"sync"

	"github.com/harrycraven/nmclean/internal/scanner"
)

// ItemResult records the deletion outcome for a single directory.
type ItemResult struct {
	Entry scanner.Entry
	Err   error
}

// Summary aggregates all deletion outcomes.
type Summary struct {
	Results    []ItemResult
	FreedBytes int64
	Deleted    int
	Failed     int
}

// DeleteAll removes all given directories concurrently and returns a Summary.
// If onProgress is non-nil, it is invoked sequentially for each completed item.
func DeleteAll(entries []scanner.Entry, workers int, onProgress func(ItemResult)) Summary {
	if len(entries) == 0 {
		return Summary{}
	}

	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > len(entries) {
		workers = len(entries)
	}

	jobs := make(chan scanner.Entry, len(entries))
	for _, e := range entries {
		jobs <- e
	}
	close(jobs)

	resultsCh := make(chan ItemResult, len(entries))

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range jobs {
				err := os.RemoveAll(entry.Path)
				resultsCh <- ItemResult{Entry: entry, Err: err}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	results := make([]ItemResult, 0, len(entries))
	for res := range resultsCh {
		results = append(results, res)
		if onProgress != nil {
			onProgress(res)
		}
	}

	return summarize(results)
}

func summarize(results []ItemResult) Summary {
	s := Summary{Results: results}
	for _, r := range results {
		if r.Err != nil {
			s.Failed++
			continue
		}
		s.Deleted++
		s.FreedBytes += r.Entry.Size
	}
	return s
}
