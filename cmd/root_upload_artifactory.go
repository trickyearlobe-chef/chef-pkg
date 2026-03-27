package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/artifactory"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/downloader"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/repomap"
)

var uploadArtifactoryCmd = &cobra.Command{
	Use:   "artifactory",
	Short: "Upload Chef packages to a JFrog Artifactory repository",
	Long: `Upload previously downloaded Chef packages to JFrog Artifactory.

By default, packages are read from the local download directory (as created by
'download packages'). Use --fetch to fetch from the Chef API, download, and
upload in a single pipeline.

Repository names are computed automatically using platform/arch normalization.
Use --create-repos to have missing repositories created before upload.

Authentication supports both API token (--artifactory-token) and basic auth
(--artifactory-username / --artifactory-password). Token takes precedence when
both are configured.`,
	Example: `  # Upload previously downloaded packages
  chef-pkg upload artifactory --product chef --version 18.4.12

  # Fetch, download, and upload in one step
  chef-pkg upload artifactory --product chef --version 18.4.12 --fetch

  # Fetch and upload the latest version
  chef-pkg upload artifactory --product chef --version latest --fetch --create-repos

  # Fetch and upload the latest version available for Ubuntu
  chef-pkg upload artifactory --product chef --version latest --fetch --create-repos --platform ubuntu

  # Fetch and upload all versions
  chef-pkg upload artifactory --product chef --version all --fetch --create-repos

  # Auto-create repos and filter to Ubuntu only
  chef-pkg upload artifactory --product chef --version 18.4.12 --fetch --create-repos --platform ubuntu

  # Use token auth with a custom repo prefix
  chef-pkg upload artifactory --product chef --version 18.4.12 --fetch --repo-prefix mycompany-chef \
    --artifactory-url https://artifactory.example.com --artifactory-token $ART_TOKEN

  # Fetch, upload, and clean up local downloads afterwards
  chef-pkg upload artifactory --product chef --version 18.4.12 --fetch --create-repos --clean`,
	RunE: runUploadArtifactory,
}

func init() {
	uploadCmd.AddCommand(uploadArtifactoryCmd)

	uploadArtifactoryCmd.Flags().StringP("product", "p", "chef", "Chef product name (e.g. chef, chef-ice, inspec)")
	uploadArtifactoryCmd.Flags().StringP("version", "v", "latest", "Product version: semver (e.g. 18.4.12), 'latest', or 'all' (default: latest)")
	uploadArtifactoryCmd.Flags().String("platform", "", "Filter by platform (substring match, case-insensitive)")
	uploadArtifactoryCmd.Flags().String("arch", "", "Filter by architecture (substring match, case-insensitive)")
	uploadArtifactoryCmd.Flags().Bool("fetch", false, "Fetch from Chef API and download before uploading")
	uploadArtifactoryCmd.Flags().Bool("create-repos", false, "Create missing Artifactory repositories before upload")
	uploadArtifactoryCmd.Flags().String("repo-prefix", "chef", "Prefix for generated repo names (default: chef)")
	uploadArtifactoryCmd.Flags().String("source", "", "Source directory for downloaded packages (default: ./packages)")
	uploadArtifactoryCmd.Flags().String("artifactory-url", "", "Artifactory server URL")
	uploadArtifactoryCmd.Flags().String("artifactory-token", "", "Artifactory API token (takes precedence over basic auth)")
	uploadArtifactoryCmd.Flags().String("artifactory-username", "", "Artifactory username")
	uploadArtifactoryCmd.Flags().String("artifactory-password", "", "Artifactory password")
	uploadArtifactoryCmd.Flags().IntP("concurrency", "c", 0, "Max parallel downloads when using --fetch (default: 4)")
	uploadArtifactoryCmd.Flags().Bool("clean", false, "Remove successfully uploaded files from local disk after upload")

	viper.BindPFlag("artifactory.url", uploadArtifactoryCmd.Flags().Lookup("artifactory-url"))
	viper.BindPFlag("artifactory.token", uploadArtifactoryCmd.Flags().Lookup("artifactory-token"))
	viper.BindPFlag("artifactory.username", uploadArtifactoryCmd.Flags().Lookup("artifactory-username"))
	viper.BindPFlag("artifactory.password", uploadArtifactoryCmd.Flags().Lookup("artifactory-password"))
}

func runUploadArtifactory(cmd *cobra.Command, args []string) error {
	// ── Resolve configuration ──────────────────────────────────────────
	artURL := viper.GetString("artifactory.url")
	artToken := viper.GetString("artifactory.token")
	artUser := viper.GetString("artifactory.username")
	artPass := viper.GetString("artifactory.password")

	if artURL == "" {
		return fmt.Errorf("artifactory URL is required: set --artifactory-url, config artifactory.url, or CHEFPKG_ARTIFACTORY_URL env var")
	}
	if artToken == "" && artUser == "" {
		return fmt.Errorf("artifactory auth is required: set --artifactory-token or --artifactory-username/--artifactory-password (config/env also supported)")
	}

	// Trim trailing slash for consistency
	artURL = strings.TrimRight(artURL, "/")

	product, _ := cmd.Flags().GetString("product")
	versionFlag, _ := cmd.Flags().GetString("version")
	platform, _ := cmd.Flags().GetString("platform")
	arch, _ := cmd.Flags().GetString("arch")
	fetch, _ := cmd.Flags().GetBool("fetch")
	createRepos, _ := cmd.Flags().GetBool("create-repos")
	repoPrefix, _ := cmd.Flags().GetString("repo-prefix")
	source, _ := cmd.Flags().GetString("source")
	clean, _ := cmd.Flags().GetBool("clean")

	if versionFlag == "" {
		versionFlag = "latest"
	}

	repoPrefix = resolveRepoPrefix(repoPrefix)

	if source == "" {
		source = viper.GetString("download.dest")
	}
	if source == "" {
		source = "./packages"
	}

	// ── Fetch + download pipeline (optional) ───────────────────────────
	var downloadResults []downloader.DownloadResult

	if fetch {
		licenseID := viper.GetString("chef.license_id")
		if licenseID == "" {
			return fmt.Errorf("license ID is required for --fetch: set --license-id, config chef.license_id, or CHEFPKG_CHEF_LICENSE_ID env var")
		}

		baseURL := viper.GetString("chef.base_url")
		channel := viper.GetString("chef.channel")

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

		concurrency := viper.GetInt("download.concurrency")
		if c, _ := cmd.Flags().GetInt("concurrency"); c > 0 {
			concurrency = c
		}
		if concurrency <= 0 {
			concurrency = 4
		}

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

			var mu sync.Mutex
			var dlCount, skipCount, failCount int

			d := downloader.New(source, product,
				downloader.WithConcurrency(concurrency),
				downloader.WithSkipExisting(true),
				downloader.WithProgressFunc(func(index, total int, r downloader.DownloadResult) {
					mu.Lock()
					defer mu.Unlock()
					switch {
					case r.Skipped:
						skipCount++
						fmt.Fprintf(os.Stderr, "  [%d/%d] Skipped %s/%s/%s (already exists)\n",
							dlCount+skipCount+failCount, total,
							r.Package.Platform, r.Package.PlatformVersion, r.Package.Architecture)
					case r.Err != nil:
						failCount++
						fmt.Fprintf(os.Stderr, "  [%d/%d] FAILED download %s/%s/%s: %v\n",
							dlCount+skipCount+failCount, total,
							r.Package.Platform, r.Package.PlatformVersion, r.Package.Architecture, r.Err)
					default:
						dlCount++
						fmt.Fprintf(os.Stderr, "  [%d/%d] Downloaded %s\n",
							dlCount+skipCount+failCount, total,
							filepath.Base(r.Path))
					}
				}),
			)

			versionResults, err := d.Download(cmd.Context(), packages)
			if err != nil {
				return fmt.Errorf("downloading packages for version %s: %w", version, err)
			}

			downloadResults = append(downloadResults, versionResults...)
			fmt.Fprintf(os.Stderr, "Downloads: %d completed, %d skipped, %d failed\n", dlCount, skipCount, failCount)
		}
	} else {
		// For local scan, "latest" and "all" are not supported — need a concrete version
		if strings.EqualFold(versionFlag, "latest") || strings.EqualFold(versionFlag, "all") {
			return fmt.Errorf("--version %q requires --fetch (cannot resolve from local directory)", versionFlag)
		}
		// Validate semver
		if _, _, _, ok := parseSemver(versionFlag); !ok {
			return fmt.Errorf("invalid version %q: expected semver (e.g. 18.4.12), 'latest', or 'all'", versionFlag)
		}
		var err error
		downloadResults, err = scanDownloadDir(source, product, versionFlag, platform, arch)
		if err != nil {
			return fmt.Errorf("scanning download directory: %w", err)
		}
	}

	// Filter out failed downloads
	var uploadable []downloader.DownloadResult
	for _, r := range downloadResults {
		if r.Err == nil && r.Path != "" {
			uploadable = append(uploadable, r)
		}
	}

	if len(uploadable) == 0 {
		fmt.Fprintln(os.Stderr, "No packages available for upload.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "\nUploading %d package(s) to Artifactory at %s\n", len(uploadable), artURL)

	// ── Upload to Artifactory ──────────────────────────────────────────
	var artOpts []artifactory.ClientOption
	if artToken != "" {
		artOpts = append(artOpts, artifactory.WithToken(artToken))
	}
	if artUser != "" {
		artOpts = append(artOpts, artifactory.WithBasicAuth(artUser, artPass))
	}
	artClient := artifactory.NewClient(artURL, artOpts...)

	// Track repos we've already checked/created to avoid redundant API calls
	repoChecked := map[string]bool{}

	var uploaded, skippedUpload, failedUpload int
	var uploadedPaths []string

	type uploadFailure struct {
		filename string
		repoName string
		reason   string
	}
	var failures []uploadFailure

	for i, r := range uploadable {
		pkg := r.Package
		rt := repomap.RepoTypeForPackage(pkg.Platform, pkg.Architecture)
		repoName := repomap.RepoName(repoPrefix, pkg.Platform, pkg.PlatformVersion, rt)

		// Ensure repository exists
		if !repoChecked[repoName] {
			exists, err := artClient.RepoExists(cmd.Context(), repoName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[%d/%d] ERROR checking repo %s: %v\n", i+1, len(uploadable), repoName, err)
				failures = append(failures, uploadFailure{filepath.Base(r.Path), repoName, fmt.Sprintf("checking repo: %v", err)})
				failedUpload++
				continue
			}
			if !exists {
				if !createRepos {
					fmt.Fprintf(os.Stderr, "[%d/%d] SKIPPED %s — repo %s does not exist (use --create-repos to auto-create)\n",
						i+1, len(uploadable), filepath.Base(r.Path), repoName)
					skippedUpload++
					continue
				}
				fmt.Fprintf(os.Stderr, "  Creating repo %s (type: %s)...\n", repoName, rt)
				if err := artClient.CreateRepo(cmd.Context(), repoName, rt); err != nil {
					fmt.Fprintf(os.Stderr, "[%d/%d] ERROR creating repo %s: %v\n", i+1, len(uploadable), repoName, err)
					failures = append(failures, uploadFailure{filepath.Base(r.Path), repoName, fmt.Sprintf("creating repo: %v", err)})
					failedUpload++
					continue
				}
				fmt.Fprintf(os.Stderr, "  Created repo %s\n", repoName)
			}
			repoChecked[repoName] = true
		}

		// Build remote path within the repo
		filename := filepath.Base(r.Path)
		remotePath := filename

		// Upload
		if err := artClient.Upload(cmd.Context(), repoName, remotePath, r.Path); err != nil {
			fmt.Fprintf(os.Stderr, "[%d/%d] FAILED upload %s → %s/%s: %v\n",
				i+1, len(uploadable), filename, repoName, remotePath, err)
			failures = append(failures, uploadFailure{filename, repoName, fmt.Sprintf("upload: %v", err)})
			failedUpload++
			continue
		}

		uploaded++
		uploadedPaths = append(uploadedPaths, r.Path)
		fmt.Fprintf(os.Stderr, "[%d/%d] Uploaded %s → %s/%s\n",
			i+1, len(uploadable), filename, repoName, remotePath)
	}

	fmt.Fprintf(os.Stderr, "\nDone: %d uploaded, %d skipped, %d failed\n", uploaded, skippedUpload, failedUpload)

	if len(failures) > 0 {
		fmt.Fprintf(os.Stderr, "\nFailed uploads:\n")
		for _, f := range failures {
			fmt.Fprintf(os.Stderr, "  ✗ %s → %s: %s\n", f.filename, f.repoName, f.reason)
		}
	}

	// ── Clean up successfully uploaded files (optional) ────────────────
	if clean && len(uploadedPaths) > 0 {
		cleaned, err := CleanDownloadedFiles(uploadedPaths, source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cleanup error: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Cleaned %d uploaded file(s) from local disk\n", cleaned)
		}
	}

	if failedUpload > 0 {
		return fmt.Errorf("%d upload(s) failed", failedUpload)
	}
	return nil
}
