package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove downloaded Chef packages from local disk",
	Long: `Remove previously downloaded Chef packages from the local download directory.

By default, removes packages for a specific product and version. Use --all to
remove the entire download directory.

Empty parent directories are pruned automatically after file removal.

Use the "nexus" subcommand to clean Nexus repositories.`,
	Example: `  # Remove downloads for a specific product and version
  chef-pkg clean --product chef --version 18.4.12

  # Remove all downloads for a product (all versions)
  chef-pkg clean --product chef

  # Remove everything in the download directory
  chef-pkg clean --all

  # Dry run to see what would be removed
  chef-pkg clean --product chef --version 18.4.12 --dry-run

  # Remove from a custom download directory
  chef-pkg clean --product chef --version 18.4.12 --source /tmp/chef-packages`,
	RunE: runCleanPackages,
}

func init() {
	rootCmd.AddCommand(cleanCmd)

	cleanCmd.Flags().StringP("product", "p", "", "Chef product name to clean (e.g. chef, chef-ice, inspec)")
	cleanCmd.Flags().StringP("version", "v", "", "Product version to clean (omit to clean all versions of a product)")
	cleanCmd.Flags().String("source", "", "Download directory to clean (default: ./packages)")
	cleanCmd.Flags().Bool("all", false, "Remove the entire download directory")
	cleanCmd.Flags().Bool("dry-run", false, "Show what would be removed without deleting anything")
}

func runCleanPackages(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")
	product, _ := cmd.Flags().GetString("product")
	version, _ := cmd.Flags().GetString("version")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	source, _ := cmd.Flags().GetString("source")
	if source == "" {
		source = viper.GetString("download.dest")
	}
	if source == "" {
		source = "./packages"
	}

	if !all && product == "" {
		return fmt.Errorf("specify --product (and optionally --version) to scope the cleanup, or --all to remove everything")
	}

	if version != "" && product == "" {
		return fmt.Errorf("--version requires --product")
	}

	// Determine the target directory to remove
	var targetDir string
	switch {
	case all:
		targetDir = source
	case version != "":
		targetDir = filepath.Join(source, product, version)
	default:
		targetDir = filepath.Join(source, product)
	}

	// Check it exists
	info, err := os.Stat(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Nothing to clean: %s does not exist\n", targetDir)
			return nil
		}
		return fmt.Errorf("checking directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", targetDir)
	}

	// Count files for reporting
	fileCount, dirCount, totalBytes, err := countContents(targetDir)
	if err != nil {
		return fmt.Errorf("scanning directory: %w", err)
	}

	if fileCount == 0 && dirCount == 0 {
		fmt.Fprintf(os.Stderr, "Nothing to clean in %s\n", targetDir)
		return nil
	}

	label := "Removing"
	if dryRun {
		label = "Would remove"
	}
	fmt.Fprintf(os.Stderr, "%s %d file(s) (%s) in %d director(ies) from %s\n",
		label, fileCount, formatBytes(totalBytes), dirCount, targetDir)

	if dryRun {
		return showDryRun(targetDir)
	}

	// Remove
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("removing %s: %w", targetDir, err)
	}

	// Prune empty parent directories up to (but not including) source root
	if !all {
		pruneEmptyParents(targetDir, source)
	}

	fmt.Fprintf(os.Stderr, "Cleaned %s\n", targetDir)
	return nil
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
