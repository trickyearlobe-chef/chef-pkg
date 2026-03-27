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
	"github.com/trickyearlobe-chef/chef-pkg/pkg/nexus"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/repomap"
)

var uploadNexusCmd = &cobra.Command{
	Use:   "nexus",
	Short: "Upload Chef packages to a Sonatype Nexus repository",
	Long: `Upload previously downloaded Chef packages to Sonatype Nexus Repository Manager.

By default, packages are read from the local download directory (as created by
'download packages'). Use --fetch to fetch from the Chef API, download, and
upload in a single pipeline.

Repository names are computed automatically using platform/arch normalization.
Use --create-repos to have missing repositories created before upload.`,
	Example: `  # Upload previously downloaded packages
  chef-pkg upload nexus --product chef --version 18.4.12

  # Fetch, download, and upload in one step
  chef-pkg upload nexus --product chef --version 18.4.12 --fetch

  # Fetch and upload the latest version
  chef-pkg upload nexus --product chef --version latest --fetch --create-repos

  # Fetch and upload the latest version available for Ubuntu
  chef-pkg upload nexus --product chef --version latest --fetch --create-repos --platform ubuntu

  # Fetch and upload all versions
  chef-pkg upload nexus --product chef --version all --fetch --create-repos

  # Auto-create repos and filter to Ubuntu only
  chef-pkg upload nexus --product chef --version 18.4.12 --fetch --create-repos --platform ubuntu

  # Use a custom repo name prefix
  chef-pkg upload nexus --product chef --version 18.4.12 --fetch --repo-prefix mycompany-chef

  # Fetch, upload, and clean up local downloads afterwards
  chef-pkg upload nexus --product chef --version 18.4.12 --fetch --create-repos --clean`,
	RunE: runUploadNexus,
}

func init() {
	uploadCmd.AddCommand(uploadNexusCmd)

	uploadNexusCmd.Flags().StringP("product", "p", "chef", "Chef product name (e.g. chef, chef-ice, inspec)")
	uploadNexusCmd.Flags().StringP("version", "v", "latest", "Product version: semver (e.g. 18.4.12), 'latest', or 'all' (default: latest)")
	uploadNexusCmd.Flags().String("platform", "", "Filter by platform (substring match, case-insensitive)")
	uploadNexusCmd.Flags().String("arch", "", "Filter by architecture (substring match, case-insensitive)")
	uploadNexusCmd.Flags().Bool("fetch", false, "Fetch from Chef API and download before uploading")
	uploadNexusCmd.Flags().Bool("create-repos", false, "Create missing Nexus repositories before upload")
	uploadNexusCmd.Flags().String("repo-prefix", "chef", "Prefix for generated repo names (default: chef)")
	uploadNexusCmd.Flags().String("source", "", "Source directory for downloaded packages (default: ./packages)")
	uploadNexusCmd.Flags().String("nexus-url", "", "Nexus server URL")
	uploadNexusCmd.Flags().String("nexus-username", "", "Nexus username")
	uploadNexusCmd.Flags().String("nexus-password", "", "Nexus password")
	uploadNexusCmd.Flags().String("nexus-gpg-keypair", "", "GPG keypair name in Nexus for APT repo signing (without this, APT repos fall back to raw)")
	uploadNexusCmd.Flags().String("nexus-gpg-passphrase", "", "Passphrase for the GPG keypair (if required)")
	uploadNexusCmd.Flags().IntP("concurrency", "c", 0, "Max parallel downloads when using --fetch (default: 4)")
	uploadNexusCmd.Flags().Bool("clean", false, "Remove successfully uploaded files from local disk after upload")

	viper.BindPFlag("nexus.url", uploadNexusCmd.Flags().Lookup("nexus-url"))
	viper.BindPFlag("nexus.username", uploadNexusCmd.Flags().Lookup("nexus-username"))
	viper.BindPFlag("nexus.password", uploadNexusCmd.Flags().Lookup("nexus-password"))
	viper.BindPFlag("nexus.gpg_keypair", uploadNexusCmd.Flags().Lookup("nexus-gpg-keypair"))
	viper.BindPFlag("nexus.gpg_passphrase", uploadNexusCmd.Flags().Lookup("nexus-gpg-passphrase"))
}

func runUploadNexus(cmd *cobra.Command, args []string) error {
	// ── Resolve configuration ──────────────────────────────────────────
	nexusURL := viper.GetString("nexus.url")
	nexusUser := viper.GetString("nexus.username")
	nexusPass := viper.GetString("nexus.password")

	if nexusURL == "" {
		return fmt.Errorf("nexus URL is required: set --nexus-url, config nexus.url, or CHEFPKG_NEXUS_URL env var")
	}
	if nexusUser == "" {
		return fmt.Errorf("nexus username is required: set --nexus-username, config nexus.username, or CHEFPKG_NEXUS_USERNAME env var")
	}
	if nexusPass == "" {
		return fmt.Errorf("nexus password is required: set --nexus-password, config nexus.password, or CHEFPKG_NEXUS_PASSWORD env var")
	}

	// Trim trailing slash for consistency
	nexusURL = strings.TrimRight(nexusURL, "/")

	gpgKeypair := viper.GetString("nexus.gpg_keypair")
	gpgPassphrase := viper.GetString("nexus.gpg_passphrase")

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

	fmt.Fprintf(os.Stderr, "\nUploading %d package(s) to Nexus at %s\n", len(uploadable), nexusURL)

	if gpgKeypair == "" {
		fmt.Fprintln(os.Stderr, "Note: no GPG keypair configured — APT repos will be created as raw repos instead.")
		fmt.Fprintln(os.Stderr, "      To create proper APT repos, set --nexus-gpg-keypair, config nexus.gpg_keypair,")
		fmt.Fprintln(os.Stderr, "      or CHEFPKG_NEXUS_GPG_KEYPAIR to the name of a GPG key stored in Nexus.")
	}

	// ── Upload to Nexus ────────────────────────────────────────────────
	nxClient := nexus.NewClient(nexusURL, nexusUser, nexusPass)

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

		// Fall back apt → raw when no GPG keypair is configured
		if rt == "apt" && gpgKeypair == "" {
			rt = "raw"
		}

		repoName := repomap.RepoName(repoPrefix, pkg.Platform, pkg.PlatformVersion, rt)

		// Ensure repository exists
		if !repoChecked[repoName] {
			exists, err := nxClient.RepoExists(cmd.Context(), repoName)
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
				var createOpts []nexus.CreateRepoOption
				if rt == "apt" {
					createOpts = append(createOpts, nexus.WithGPGKeypair(gpgKeypair))
					if gpgPassphrase != "" {
						createOpts = append(createOpts, nexus.WithGPGPassphrase(gpgPassphrase))
					}
				}
				if err := nxClient.CreateRepo(cmd.Context(), repoName, rt, createOpts...); err != nil {
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
		if err := nxClient.Upload(cmd.Context(), repoName, remotePath, r.Path); err != nil {
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

// scanDownloadDir walks the local download directory to find previously
// downloaded packages and reconstructs DownloadResult entries from the
// directory structure: {source}/{product}/{version}/{platform}/{platform_version}/{arch}/
func scanDownloadDir(source, product, version, platformFilter, archFilter string) ([]downloader.DownloadResult, error) {
	baseDir := filepath.Join(source, product, version)

	info, err := os.Stat(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("download directory does not exist: %s (run 'download packages' first or use --fetch)", baseDir)
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", baseDir)
	}

	var results []downloader.DownloadResult

	// Walk: {baseDir}/{platform}/{platform_version}/{arch}/{filename}
	platformEntries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", baseDir, err)
	}

	for _, platformEntry := range platformEntries {
		if !platformEntry.IsDir() {
			continue
		}
		platformName := platformEntry.Name()

		// Apply platform filter
		if platformFilter != "" && !strings.Contains(strings.ToLower(platformName), strings.ToLower(platformFilter)) {
			continue
		}

		platformDir := filepath.Join(baseDir, platformName)
		pvEntries, err := os.ReadDir(platformDir)
		if err != nil {
			continue
		}

		for _, pvEntry := range pvEntries {
			if !pvEntry.IsDir() {
				continue
			}
			pvName := pvEntry.Name()
			pvDir := filepath.Join(platformDir, pvName)

			archEntries, err := os.ReadDir(pvDir)
			if err != nil {
				continue
			}

			for _, archEntry := range archEntries {
				if !archEntry.IsDir() {
					continue
				}
				archName := archEntry.Name()

				// Apply arch filter
				if archFilter != "" && !strings.Contains(strings.ToLower(archName), strings.ToLower(archFilter)) {
					continue
				}

				archDir := filepath.Join(pvDir, archName)
				fileEntries, err := os.ReadDir(archDir)
				if err != nil {
					continue
				}

				for _, fileEntry := range fileEntries {
					if fileEntry.IsDir() {
						continue
					}
					// Skip .sha256 sidecar files and hidden files
					name := fileEntry.Name()
					if strings.HasSuffix(name, ".sha256") || strings.HasPrefix(name, ".") {
						continue
					}

					filePath := filepath.Join(archDir, name)
					results = append(results, downloader.DownloadResult{
						Path: filePath,
						Package: chefapi.FlatPackage{
							Platform:        platformName,
							PlatformVersion: pvName,
							Architecture:    archName,
							PackageDetail: chefapi.PackageDetail{
								Version: version,
							},
						},
					})
				}
			}
		}
	}

	return results, nil
}
