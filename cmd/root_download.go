package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
logic on subsequent runs.

The --version flag accepts a semver version (e.g. 18.4.12), "latest" to
resolve the most recent version, or "all" to download packages for every
available version. When using "latest" with --platform or --arch filters,
the latest version that has matching packages is selected.`,
	Example: `  # Download all packages for chef 18.4.12 on stable
  chef-pkg download --product chef --version 18.4.12 --channel stable

  # Download the latest version
  chef-pkg download --product chef --version latest

  # Download the latest version available for Ubuntu
  chef-pkg download --product chef --version latest --platform ubuntu

  # Download all versions (filtered to aarch64)
  chef-pkg download --product chef --version all --arch aarch64

  # Download only Ubuntu packages
  chef-pkg download --product chef --version 18.4.12 --channel stable --platform ubuntu

  # Download to a custom directory with higher concurrency
  chef-pkg download --product chef --version 18.4.12 --dest /tmp/chef-packages --concurrency 8`,
	RunE: runDownloadPackages,
}

func init() {
	rootCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().StringP("product", "p", "chef", "Chef product name (e.g. chef, chef-ice, inspec)")
	downloadCmd.Flags().StringP("version", "v", "latest", "Product version: semver (e.g. 18.4.12), 'latest', or 'all' (default: latest)")
	downloadCmd.Flags().String("platform", "", "Filter by platform (substring match, case-insensitive)")
	downloadCmd.Flags().String("arch", "", "Filter by architecture (substring match, case-insensitive)")
	downloadCmd.Flags().StringP("dest", "d", "", "Destination root directory (default: ./packages)")
	downloadCmd.Flags().Bool("skip-existing", true, "Skip files that already exist with matching SHA256")
	downloadCmd.Flags().IntP("concurrency", "c", 0, "Max parallel downloads (default: 4)")

	viper.BindPFlag("download.dest", downloadCmd.Flags().Lookup("dest"))
	viper.BindPFlag("download.concurrency", downloadCmd.Flags().Lookup("concurrency"))
}

func runDownloadPackages(cmd *cobra.Command, args []string) error {
	licenseID := viper.GetString("chef.license_id")
	if licenseID == "" {
		return fmt.Errorf("license ID is required: set --license-id, config chef.license_id, or CHEFPKG_CHEF_LICENSE_ID env var")
	}

	baseURL := viper.GetString("chef.base_url")
	channel := viper.GetString("chef.channel")
	product, _ := cmd.Flags().GetString("product")
	versionFlag, _ := cmd.Flags().GetString("version")
	platform, _ := cmd.Flags().GetString("platform")
	arch, _ := cmd.Flags().GetString("arch")
	skipExisting, _ := cmd.Flags().GetBool("skip-existing")

	if versionFlag == "" {
		versionFlag = "latest"
	}

	dest := viper.GetString("download.dest")
	if dest == "" {
		dest = "./packages"
	}

	concurrency := viper.GetInt("download.concurrency")
	if concurrency <= 0 {
		concurrency = 4
	}

	// Build API client
	var clientOpts []chefapi.ClientOption
	if baseURL != "" {
		clientOpts = append(clientOpts, chefapi.WithBaseURL(baseURL))
	}
	client := chefapi.NewClient(licenseID, clientOpts...)

	// Resolve version(s)
	versions, err := resolveVersions(cmd.Context(), client, channel, product, versionFlag, platform, arch)
	if err != nil {
		return err
	}

	// Download packages for each resolved version
	var totalDownloaded, totalSkipped, totalFailed int

	for vi, version := range versions {
		if len(versions) > 1 {
			fmt.Fprintf(os.Stderr, "\n── Version %s (%d/%d) ──\n", version, vi+1, len(versions))
		}

		fmt.Fprintf(os.Stderr, "Fetching package list for %s %s (%s channel)...\n", product, version, channel)
		resp, err := client.FetchPackages(cmd.Context(), channel, product, version)
		if err != nil {
			if strings.EqualFold(versionFlag, "all") {
				fmt.Fprintf(os.Stderr, "  Warning: skipping version %s: %v\n", version, err)
				continue
			}
			return fmt.Errorf("fetching packages for version %s: %w", version, err)
		}

		packages := resp.Flatten()
		packages = filterPackages(packages, platform, arch)

		if len(packages) == 0 {
			fmt.Fprintln(os.Stderr, "No packages found matching the specified criteria.")
			continue
		}

		fmt.Fprintf(os.Stderr, "Found %d package(s) to download\n", len(packages))

		// Download with real-time progress reporting
		var mu sync.Mutex
		var downloaded, skipped, failed int

		d := downloader.New(dest, product,
			downloader.WithConcurrency(concurrency),
			downloader.WithSkipExisting(skipExisting),
			downloader.WithProgressFunc(func(index, total int, r downloader.DownloadResult) {
				mu.Lock()
				defer mu.Unlock()
				switch {
				case r.Skipped:
					skipped++
					fmt.Fprintf(os.Stderr, "[%d/%d] Skipped %s/%s/%s (already exists, SHA256 match)\n",
						downloaded+skipped+failed, total,
						r.Package.Platform, r.Package.PlatformVersion, r.Package.Architecture)
				case r.Err != nil:
					failed++
					fmt.Fprintf(os.Stderr, "[%d/%d] FAILED %s/%s/%s: %v\n",
						downloaded+skipped+failed, total,
						r.Package.Platform, r.Package.PlatformVersion, r.Package.Architecture, r.Err)
				default:
					downloaded++
					fmt.Fprintf(os.Stderr, "[%d/%d] Downloaded %s\n",
						downloaded+skipped+failed, total,
						filepath.Base(r.Path))
				}
			}),
		)

		_, err = d.Download(cmd.Context(), packages)
		if err != nil {
			return fmt.Errorf("downloading packages for version %s: %w", version, err)
		}

		fmt.Fprintf(os.Stderr, "Version %s: %d downloaded, %d skipped, %d failed\n", version, downloaded, skipped, failed)

		totalDownloaded += downloaded
		totalSkipped += skipped
		totalFailed += failed
	}

	if len(versions) > 1 {
		fmt.Fprintf(os.Stderr, "\nTotal: %d downloaded, %d skipped, %d failed\n", totalDownloaded, totalSkipped, totalFailed)
	} else {
		fmt.Fprintf(os.Stderr, "\nDone: %d downloaded, %d skipped, %d failed\n", totalDownloaded, totalSkipped, totalFailed)
	}

	if totalFailed > 0 {
		return fmt.Errorf("%d download(s) failed", totalFailed)
	}
	return nil
}
