package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/harrycraven/nmclean/internal/cleaner"
	"github.com/harrycraven/nmclean/internal/format"
	"github.com/harrycraven/nmclean/internal/scanner"
	"github.com/harrycraven/nmclean/internal/tui"
	"golang.org/x/term"
)

// Overridden at build time via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type config struct {
	targetDir   string
	dryRun      bool
	yes         bool
	workers     int
	showVersion bool
	showHelp    bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		printUsage(stderr)
		return 2
	}

	if cfg.showHelp {
		printUsage(stdout)
		return 0
	}

	if cfg.showVersion {
		fmt.Fprintf(stdout, "nmclean %s (commit %s, built %s)\n", version, commit, date)
		return 0
	}

	fmt.Fprintf(stdout, "Scanning %s ...\n", cfg.targetDir)
	start := time.Now()

	scanRes, err := scanner.Scan(cfg.targetDir, cfg.workers)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	if len(scanRes.Errors) > 0 {
		fmt.Fprintf(stderr, "Warning: skipped %d unreadable path(s) during scan.\n", len(scanRes.Errors))
	}

	if len(scanRes.Entries) == 0 {
		fmt.Fprintf(stdout, "No node_modules folders found in %s (%s).\n", scanRes.Root, time.Since(start).Round(time.Millisecond))
		return 0
	}

	totalFound := scanner.TotalSize(scanRes.Entries)

	if cfg.dryRun {
		printDryRun(stdout, scanRes, totalFound, time.Since(start))
		return 0
	}

	var toDelete []scanner.Entry
	if cfg.yes {
		toDelete = scanRes.Entries
	} else {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintf(stderr, "Error: interactive mode requires a terminal. Use --dry-run to preview or --yes to delete all.\n")
			return 2
		}

		selected, cancelled, err := tui.Run(scanRes.Entries)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		if cancelled || len(selected) == 0 {
			fmt.Fprintln(stdout, "Cancelled. No folders were deleted.")
			return 0
		}
		toDelete = selected
	}

	fmt.Fprintf(stdout, "\nDeleting %d folder(s)...\n\n", len(toDelete))
	delStart := time.Now()

	summary := cleaner.DeleteAll(toDelete, cfg.workers, func(res cleaner.ItemResult) {
		if res.Err != nil {
			fmt.Fprintf(stderr, "  [FAIL]  %9s  %s (%v)\n", format.Bytes(res.Entry.Size), res.Entry.RelPath, res.Err)
			return
		}
		fmt.Fprintf(stdout, "  [DONE]  %9s  %s\n", format.Bytes(res.Entry.Size), res.Entry.RelPath)
	})

	fmt.Fprintln(stdout)
	fmt.Fprintf(
		stdout,
		"Deleted %d folder(s), freed %s in %s.\n",
		summary.Deleted,
		format.Bytes(summary.FreedBytes),
		time.Since(delStart).Round(time.Millisecond),
	)

	if summary.Failed > 0 {
		fmt.Fprintf(stderr, "%d folder(s) could not be deleted.\n", summary.Failed)
		return 1
	}

	return 0
}

func printDryRun(w io.Writer, res scanner.Result, total int64, elapsed time.Duration) {
	fmt.Fprintf(w, "\nFound %d node_modules folder(s) in %s (%s):\n\n", len(res.Entries), res.Root, elapsed.Round(time.Millisecond))
	for _, e := range res.Entries {
		fmt.Fprintf(w, "  %9s  %s\n", format.Bytes(e.Size), e.RelPath)
	}
	fmt.Fprintf(w, "\nTotal reclaimable space: %s (dry run — nothing deleted)\n", format.Bytes(total))
}

func parseArgs(args []string) (config, error) {
	cfg := config{
		targetDir: ".",
		workers:   runtime.NumCPU(),
	}

	positionalSet := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-n" || arg == "--dry-run":
			cfg.dryRun = true
		case arg == "-y" || arg == "--yes":
			cfg.yes = true
		case arg == "-v" || arg == "--version":
			cfg.showVersion = true
		case arg == "-h" || arg == "--help":
			cfg.showHelp = true
		case arg == "-w" || arg == "--workers":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("flag %s requires a number", arg)
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				return cfg, fmt.Errorf("invalid worker count %q: must be >= 1", args[i])
			}
			cfg.workers = n
		case strings.HasPrefix(arg, "--workers="):
			val := strings.TrimPrefix(arg, "--workers=")
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 {
				return cfg, fmt.Errorf("invalid worker count %q: must be >= 1", val)
			}
			cfg.workers = n
		case strings.HasPrefix(arg, "-"):
			return cfg, fmt.Errorf("unknown flag: %s", arg)
		default:
			if positionalSet {
				return cfg, fmt.Errorf("unexpected extra argument: %s", arg)
			}
			cfg.targetDir = arg
			positionalSet = true
		}
	}

	return cfg, nil
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `nmclean — Fast CLI to find and remove node_modules folders

Usage:
  nmclean [directory] [flags]

Flags:
  -n, --dry-run       Show matching folders and sizes without deleting
  -y, --yes           Delete all found folders without interactive prompt
  -w, --workers <N>   Number of parallel workers (default: CPU count)
  -v, --version       Print version information
  -h, --help          Show this help message

Examples:
  nmclean ~/dev              Interactive checklist in ~/dev
  nmclean ~/dev --dry-run    Preview disk space used in ~/dev
  nmclean ~/dev --yes        Delete all node_modules in ~/dev immediately
`)
}
