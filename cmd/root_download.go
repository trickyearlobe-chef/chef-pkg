package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/downloader"
)

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download Chef packages to local disk",
	Long: `Download Chef packages to a local directory, organised in a hierarchy
of product/version/platform/platform_version/architecture.

Files are verified against their SHA256 checksums after download. A .sha256
sidecar file is written alongside each package to enable skip-if-existing
logic on subsequent runs.`,
	Example: `  # Download all packages for chef 18.4.12 on stable
  chef-pkg download --product chef --version 18.4.12 --channel stable

  # Download only Ubuntu packages
  chef-pkg download --product chef --version 18.4.12 --channel stable --platform ubuntu

  # Download to a custom directory with higher concurrency
  chef-pkg download --product chef --version 18.4.12 --dest /tmp/chef-packages --concurrency 8`,
	RunE: runDownload,
}

func init() {
	rootCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().StringP("product", "p", "chef", "Chef product name (e.g. chef, chef-ice, inspec)")
	downloadCmd.Flags().StringP("version", "v", "", "Product version to download (required)")
	downloadCmd.Flags().String("platform", "", "Filter by platform (substring match, case-insensitive)")
	downloadCmd.Flags().String("arch", "", "Filter by architecture (substring match, case-insensitive)")
	downloadCmd.Flags().StringP("dest", "d", "", "Destination root directory (default: ./packages)")
	downloadCmd.Flags().Bool("skip-existing", true, "Skip files that already exist with matching SHA256")
	downloadCmd.Flags().IntP("concurrency", "c", 0, "Max parallel downloads (default: 4)")

	_ = downloadCmd.MarkFlagRequired("version")

	viper.BindPFlag("download.dest", downloadCmd.Flags().Lookup("dest"))
	viper.BindPFlag("download.concurrency", downloadCmd.Flags().Lookup("concurrency"))
}

func runDownload(cmd *cobra.Command, args []string) error {
	licenseID := viper.GetString("chef.license_id")
	if licenseID == "" {
		return fmt.Errorf("license ID is required: set --license-id, config chef.license_id, or CHEFPKG_CHEF_LICENSE_ID env var")
	}

	baseURL := viper.GetString("chef.base_url")
	channel := viper.GetString("chef.channel")
	product, _ := cmd.Flags().GetString("product")
	version, _ := cmd.Flags().GetString("version")
	platform, _ := cmd.Flags().GetString("platform")
	arch, _ := cmd.Flags().GetString("arch")
	skipExisting, _ := cmd.Flags().GetBool("skip-existing")

	dest := viper.GetString("download.dest")
	if dest == "" {
		dest = "./packages"
	}

	concurrency := viper.GetInt("download.concurrency")
	if concurrency <= 0 {
		concurrency = 4
	}

	// Fetch package list
	var clientOpts []chefapi.ClientOption
	if baseURL != "" {
		clientOpts = append(clientOpts, chefapi.WithBaseURL(baseURL))
	}
	client := chefapi.NewClient(licenseID, clientOpts...)

	fmt.Fprintf(os.Stderr, "Fetching package list for %s %s (%s channel)...\n", product, version, channel)
	resp, err := client.FetchPackages(cmd.Context(), channel, product, version)
	if err != nil {
		return fmt.Errorf("fetching packages: %w", err)
	}

	packages := resp.Flatten()
	packages = filterPackages(packages, platform, arch)

	if len(packages) == 0 {
		fmt.Fprintln(os.Stderr, "No packages found matching the specified criteria.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Found %d package(s) to download\n", len(packages))

	// Download
	d := downloader.New(dest, product,
		downloader.WithConcurrency(concurrency),
		downloader.WithSkipExisting(skipExisting),
	)

	results, err := d.Download(cmd.Context(), packages)
	if err != nil {
		return fmt.Errorf("downloading packages: %w", err)
	}

	// Report results
	var downloaded, skipped, failed int
	for i, r := range results {
		switch {
		case r.Skipped:
			skipped++
			fmt.Fprintf(os.Stderr, "[%d/%d] Skipped %s/%s/%s (already exists, SHA256 match)\n", i+1, len(results), r.Package.Platform, r.Package.PlatformVersion, r.Package.Architecture)
		case r.Err != nil:
			failed++
			fmt.Fprintf(os.Stderr, "[%d/%d] FAILED %s/%s/%s: %v\n", i+1, len(results), r.Package.Platform, r.Package.PlatformVersion, r.Package.Architecture, r.Err)
		default:
			downloaded++
			fmt.Fprintf(os.Stderr, "[%d/%d] Downloaded %s\n", i+1, len(results), r.Path)
		}
	}

	fmt.Fprintf(os.Stderr, "\nDone: %d downloaded, %d skipped, %d failed\n", downloaded, skipped, failed)

	if failed > 0 {
		return fmt.Errorf("%d download(s) failed", failed)
	}
	return nil
}
