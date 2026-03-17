package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove downloaded Chef packages from local disk",
	Long: `Remove previously downloaded Chef packages from the local download directory.

Use a subcommand to specify what to clean.`,
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}

// countContents counts files, directories, and total bytes under a path.
func countContents(root string) (files int, dirs int, bytes int64, err error) {
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			dirs++
		} else {
			files++
			bytes += info.Size()
		}
		return nil
	})
	// Don't count the root itself as a directory
	if dirs > 0 {
		dirs--
	}
	return
}

// showDryRun lists all files that would be removed.
func showDryRun(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			fmt.Fprintf(os.Stderr, "  %s (%s)\n", path, formatBytes(info.Size()))
		}
		return nil
	})
}

// pruneEmptyParents removes empty directories from target up to (but not
// including) the stop directory.
func pruneEmptyParents(target, stop string) {
	absStop, err := filepath.Abs(stop)
	if err != nil {
		return
	}

	dir := filepath.Dir(target)
	for {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return
		}
		// Don't go above the stop directory
		if absDir == absStop || !isSubpath(absDir, absStop) {
			return
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		if len(entries) > 0 {
			return // Not empty, stop pruning
		}

		if err := os.Remove(dir); err != nil {
			return
		}
		fmt.Fprintf(os.Stderr, "Pruned empty directory %s\n", dir)
		dir = filepath.Dir(dir)
	}
}

// isSubpath returns true if child is a subdirectory of parent.
func isSubpath(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && len(rel) > 0 && rel[0] != '.'
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// CleanDownloadedFiles removes successfully uploaded files and their .sha256
// sidecars, then prunes empty parent directories. It is intended to be called
// from upload commands when --clean is specified. Returns the number of files
// removed and any error encountered.
func CleanDownloadedFiles(paths []string, source string) (int, error) {
	removed := 0
	dirsToCheck := map[string]bool{}

	for _, p := range paths {
		// Remove the package file
		if err := os.Remove(p); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return removed, fmt.Errorf("removing %s: %w", p, err)
		}
		removed++
		dirsToCheck[filepath.Dir(p)] = true

		// Remove the .sha256 sidecar if it exists
		shaPath := p + ".sha256"
		if err := os.Remove(shaPath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: could not remove sidecar %s: %v\n", shaPath, err)
		}
	}

	// Prune empty directories
	absSource, err := filepath.Abs(source)
	if err != nil {
		return removed, nil
	}

	for dir := range dirsToCheck {
		pruneEmptyAncestors(dir, absSource)
	}

	return removed, nil
}

// pruneEmptyAncestors removes empty directories from dir upward, stopping
// at (and not removing) the stop directory.
func pruneEmptyAncestors(dir, absStop string) {
	for {
		absDir, err := filepath.Abs(dir)
		if err != nil || absDir == absStop {
			return
		}

		rel, err := filepath.Rel(absStop, absDir)
		if err != nil || rel == "." || rel == ".." || (len(rel) > 0 && rel[0] == '.') {
			return
		}

		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}

		os.Remove(dir)
		dir = filepath.Dir(dir)
	}
}
