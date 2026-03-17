# TODO

## Project Setup
- [x] Initialize Go module and install dependencies (cobra, viper, progressbar, x/term)
- [x] Create project directory structure (cmd/, pkg/chefapi/, pkg/downloader/, pkg/nexus/, pkg/artifactory/, pkg/repomap/)
- [x] Create example config file ~/.chef-pkg.toml

## pkg/repomap — Platform/Arch/Repo Normalization
- [ ] Implement NormalizePlatform(chefPlatform) → string
- [ ] Implement NormalizePlatformVersion(platform, version) → string with Ubuntu/Debian codename lookup tables
- [ ] Implement NormalizeArch(repoType, chefArch) → string
- [ ] Implement RepoType(platform, fileExtension) → string
- [ ] Implement RepoName(prefix, platform, platformVersion, arch, repoType) → string
- [ ] Write tests for all normalization functions
- [ ] Add fallback behavior with warnings for unknown platforms/versions

## pkg/chefapi — Chef Downloads API Client
- [x] Define types: PackageDetail, PackagesResponse (nested map), FlatPackage
- [x] Implement Flatten() method on PackagesResponse with sorting
- [x] Define APIError type with status code and response body
- [x] Implement Client struct with functional options (WithBaseURL, WithHTTPClient)
- [x] Implement NewClient(licenseID, ...ClientOption) constructor
- [x] Implement FetchPackages(ctx, channel, product, version) → (PackagesResponse, error)
- [x] Write httptest-based tests: success, API error, invalid JSON

## pkg/downloader — Download Orchestration
- [ ] Define Downloader struct with concurrency, dest dir, skip-existing config
- [ ] Define DownloadResult type (path, success, error, skipped)
- [ ] Implement hierarchical directory creation ({dest}/{product}/{version}/{platform}/{platform_version}/{arch}/)
- [ ] Implement SHA256 verification and .sha256 sidecar file writing
- [ ] Implement skip-existing logic (compare SHA256 sidecar)
- [ ] Implement concurrent download with configurable parallelism
- [ ] Implement Download(ctx, []FlatPackage) → []DownloadResult
- [ ] Write tests for download logic (httptest, temp dirs, SHA256 checks)

## pkg/nexus — Nexus REST Client
- [ ] Define Client struct with URL, username, password
- [ ] Implement RepoExists(ctx, name) → bool
- [ ] Implement CreateRepo(ctx, name, repoType) — support yum, apt, nuget, raw hosted repos
- [ ] Implement Upload(ctx, repoName, remotePath, localFilePath) for single asset
- [ ] Implement UploadPackages(ctx, []DownloadResult, repoPrefix, createRepos) orchestration
- [ ] Write tests for Nexus client

## pkg/artifactory — Artifactory REST Client
- [ ] Define Client struct with URL, token, username, password (token takes precedence)
- [ ] Implement RepoExists(ctx, name) → bool
- [ ] Implement CreateRepo(ctx, name, repoType) — support yum, apt, nuget, generic local repos
- [ ] Implement Upload(ctx, repoName, remotePath, localFilePath) via PUT deploy
- [ ] Implement UploadPackages(ctx, []DownloadResult, repoPrefix, createRepos) orchestration
- [ ] Write tests for Artifactory client

## Output & Progress
- [ ] Implement TTY detection using golang.org/x/term
- [ ] Implement interactive progress bar output (schollz/progressbar)
- [ ] Implement batch line-by-line logging output
- [ ] Respect --no-progress flag to force line-by-line even in interactive mode

## cmd/root.go — Root Command
- [x] Set up Cobra root command with description
- [x] Configure Viper: config file (~/.chef-pkg.toml), env prefix (CHEFPKG_), auto-bind
- [x] Add persistent flags: --config, --license-id, --base-url, --channel, --no-progress
- [x] Bind persistent flags to Viper keys

## cmd/root_configure.go — configure Subcommand
- [x] Add configure subcommand with flags for each config item and --show
- [x] Implement config file read/create/update logic (merge with existing values)
- [x] Implement --show with secret masking (license_id, password, token fields)
- [x] Implement mask function: show first 4 + last 4 chars with **** in between, or **** if < 10 chars
- [x] Write tests for masking logic and config write/read round-trip

## cmd/root_packages.go — packages Subcommand
- [x] Add packages subcommand with flags: --product, --version, --platform, --arch, --output
- [x] Implement RunE: create chefapi.Client, call FetchPackages, Flatten, filter, output
- [x] Implement table output via text/tabwriter
- [x] Implement JSON output with indented encoding

## cmd/root_download.go — download Subcommand
- [ ] Add download subcommand with flags: --product, --version, --platform, --arch, --dest, --skip-existing, --concurrency
- [ ] Implement RunE: create chefapi.Client, fetch package list, filter, invoke Downloader
- [ ] Display progress (interactive) or line-by-line logs (batch)

## cmd/root_upload.go — upload Parent Command
- [ ] Add upload parent command (no action, just groups nexus/artifactory)

## cmd/root_upload_nexus.go — upload nexus Subcommand
- [ ] Add upload nexus subcommand with flags: --source, --product, --version, --platform, --arch, --nexus-url, --nexus-username, --nexus-password, --repo-prefix, --create-repos, --fetch
- [ ] Bind flags to Viper keys for nexus config
- [ ] Implement RunE: optionally fetch+download, then upload to Nexus with repo creation
- [ ] Use repomap for repo type/name resolution
- [ ] Display progress or line-by-line logs

## cmd/root_upload_artifactory.go — upload artifactory Subcommand
- [ ] Add upload artifactory subcommand with flags: --source, --product, --version, --platform, --arch, --artifactory-url, --artifactory-token, --artifactory-username, --artifactory-password, --repo-prefix, --create-repos, --fetch
- [ ] Bind flags to Viper keys for artifactory config
- [ ] Implement RunE: optionally fetch+download, then upload to Artifactory with repo creation
- [ ] Use repomap for repo type/name resolution
- [ ] Display progress or line-by-line logs

## main.go — Entrypoint
- [x] Create main.go that calls cmd.Execute()

## Integration & Polish
- [ ] End-to-end manual test with real Chef API (requires valid license ID)
- [ ] End-to-end test with Nexus (requires running Nexus instance)
- [ ] End-to-end test with Artifactory (requires running Artifactory instance)
- [ ] Add README.md with usage examples
- [ ] Add Makefile or Goreleaser config for building