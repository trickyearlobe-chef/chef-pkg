package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cleanPackagesCmd = &cobra.Command{
	Use:   "packages",
	Short: "Remove downloaded Chef packages from local disk",
	Long: `Remove previously downloaded Chef packages from the local download directory.

By default, removes packages for a specific product and version. Use --all to
remove the entire download directory.

Empty parent directories are pruned automatically after file removal.`,
	Example: `  # Remove downloads for a specific product and version
  chef-pkg clean packages --product chef --version 18.4.12

  # Remove all downloads for a product (all versions)
  chef-pkg clean packages --product chef

  # Remove everything in the download directory
  chef-pkg clean packages --all

  # Dry run to see what would be removed
  chef-pkg clean packages --product chef --version 18.4.12 --dry-run

  # Remove from a custom download directory
  chef-pkg clean packages --product chef --version 18.4.12 --source /tmp/chef-packages`,
	RunE: runCleanPackages,
}

func init() {
	cleanCmd.AddCommand(cleanPackagesCmd)

	cleanPackagesCmd.Flags().StringP("product", "p", "", "Chef product name to clean (e.g. chef, chef-ice, inspec)")
	cleanPackagesCmd.Flags().StringP("version", "v", "", "Product version to clean (omit to clean all versions of a product)")
	cleanPackagesCmd.Flags().String("source", "", "Download directory to clean (default: ./packages)")
	cleanPackagesCmd.Flags().Bool("all", false, "Remove the entire download directory")
	cleanPackagesCmd.Flags().Bool("dry-run", false, "Show what would be removed without deleting anything")
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
