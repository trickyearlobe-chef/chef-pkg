# chef-pkg Specification

## Overview

`chef-pkg` is a Go CLI utility for fetching Chef client packages from the Progress Chef commercial downloads API, downloading them locally, and uploading them to artifact repositories (Sonatype Nexus, JFrog Artifactory).

## API Details

- **Base URL**: `https://commercial-acceptance.downloads.chef.co`
- **Path pattern**: `/{channel}/{product}/packages`
- **Query params**: `v` (version), `license_id` (required)
- **Response format**: Nested JSON map — `platform → platform_version → architecture → PackageDetail`

## Command Hierarchy & File Layout

```
cmd/
  root.go                            → chef-pkg (global config via Viper)
  root_configure.go                  → chef-pkg configure (set/show config)
  root_packages.go                   → chef-pkg packages (list available packages)
  root_download.go                   → chef-pkg download (fetch to local disk)
  root_upload.go                     → chef-pkg upload (parent — no action alone)
  root_upload_nexus.go               → chef-pkg upload nexus
  root_upload_artifactory.go         → chef-pkg upload artifactory
```

## Library Packages

```
pkg/
  chefapi/                           → Chef downloads API client
  downloader/                        → Download orchestration (fetch, verify, write)
  nexus/                             → Nexus REST client (upload + repo creation)
  artifactory/                       → Artifactory REST client (upload + repo creation)
  repomap/                           → Platform/arch/repo-type normalization
```

## Configuration (Viper + TOML)

Config file: `~/.chef-pkg.toml` (or `--config path/to/file.toml`).

All config items can also be set via environment variables prefixed with `CHEFPKG_` (Viper auto-bind).

```toml
[chef]
license_id = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
base_url = "https://commercial-acceptance.downloads.chef.co"
channel = "current"

[download]
dest = "./packages"
concurrency = 4

[nexus]
url = "https://nexus.example.com"
username = "admin"
password = "secret"

[artifactory]
url = "https://artifactory.example.com"
token = "AKCp..."
username = ""
password = ""
```

## Commands

### Root Command — `chef-pkg`

Persistent flags (available to all subcommands):

| Flag | Config key | Env var | Default | Description |
|---|---|---|---|---|
| `--config` | — | — | `~/.chef-pkg.toml` | Config file path |
| `--license-id` | `chef.license_id` | `CHEFPKG_CHEF_LICENSE_ID` | — | Chef license ID (required) |
| `--base-url` | `chef.base_url` | `CHEFPKG_CHEF_BASE_URL` | `https://commercial-acceptance.downloads.chef.co` | API base URL |
| `--channel` | `chef.channel` | `CHEFPKG_CHEF_CHANNEL` | `current` | Release channel |
| `--no-progress` | — | — | `false` | Force line-by-line output even in interactive mode |

### `chef-pkg configure`

Set or display configuration values. When called with flags, updates the
config file with the specified values. When called with `--show`, displays
the current resolved configuration with secrets masked.

Config file location: `~/.chef-pkg.toml` (or the path given by `--config`).
If the file does not exist, it is created.

| Flag | Config key | Description |
|---|---|---|
| `--license-id` | `chef.license_id` | Chef license ID |
| `--base-url` | `chef.base_url` | Base URL of the Chef downloads API |
| `--channel` | `chef.channel` | Release channel |
| `--download-dest` | `download.dest` | Download destination directory |
| `--download-concurrency` | `download.concurrency` | Max parallel downloads |
| `--nexus-url` | `nexus.url` | Nexus server URL |
| `--nexus-username` | `nexus.username` | Nexus username |
| `--nexus-password` | `nexus.password` | Nexus password |
| `--artifactory-url` | `artifactory.url` | Artifactory server URL |
| `--artifactory-token` | `artifactory.token` | Artifactory API token |
| `--artifactory-username` | `artifactory.username` | Artifactory username |
| `--artifactory-password` | `artifactory.password` | Artifactory password |
| `--show` | — | Display current resolved config and exit |

Secret fields (`license_id`, `password`, `token`) are masked in `--show` output.
A masked value shows the first 4 and last 4 characters with `****` in between,
or `****` if the value is shorter than 10 characters.

Examples:

```
# Set your license ID
chef-pkg configure --license-id xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# Set multiple values at once
chef-pkg configure --nexus-url https://nexus.example.com --nexus-username admin --nexus-password secret

# Show current configuration
chef-pkg configure --show

# Use a custom config file location
chef-pkg --config /path/to/config.toml configure --license-id xxxx
```

### `chef-pkg packages`

List available packages from the Chef API.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--product` | `-p` | `chef-ice` | Product name |
| `--version` | `-v` | *(required)* | Product version |
| `--platform` | | | Filter by platform (substring, case-insensitive) |
| `--arch` | | | Filter by architecture |
| `--output` | `-o` | `table` | Output format: `table` or `json` |

### `chef-pkg download`

Download packages to local disk.

| Flag | Short | Config key | Default | Description |
|---|---|---|---|---|
| `--product` | `-p` | | `chef-ice` | Product name |
| `--version` | `-v` | | *(required)* | Product version |
| `--platform` | | | | Filter by platform |
| `--arch` | | | | Filter by architecture |
| `--dest` | `-d` | `download.dest` | `./packages` | Destination root directory |
| `--skip-existing` | | | `true` | Skip files with matching SHA256 |
| `--concurrency` | `-c` | `download.concurrency` | `4` | Max parallel downloads |

### `chef-pkg upload nexus`

Upload downloaded packages to Sonatype Nexus.

| Flag | Short | Config key | Default | Description |
|---|---|---|---|---|
| `--source` | `-s` | `download.dest` | `./packages` | Local package directory |
| `--product` | `-p` | | `chef-ice` | Product name |
| `--version` | `-v` | | *(required)* | Product version |
| `--platform` | | | | Filter by platform |
| `--arch` | | | | Filter by architecture |
| `--nexus-url` | | `nexus.url` | | Nexus server URL |
| `--nexus-username` | | `nexus.username` | | Nexus username |
| `--nexus-password` | | `nexus.password` | | Nexus password |
| `--repo-prefix` | | | `chef` | Prefix for repo names |
| `--create-repos` | | | `false` | Auto-create repos if they don't exist |
| `--fetch` | | | `false` | Fetch from Chef API → download → upload (pipeline mode) |

### `chef-pkg upload artifactory`

Upload downloaded packages to JFrog Artifactory.

| Flag | Short | Config key | Default | Description |
|---|---|---|---|---|
| `--source` | `-s` | `download.dest` | `./packages` | Local package directory |
| `--product` | `-p` | | `chef-ice` | Product name |
| `--version` | `-v` | | *(required)* | Product version |
| `--platform` | | | | Filter by platform |
| `--arch` | | | | Filter by architecture |
| `--artifactory-url` | | `artifactory.url` | | Artifactory server URL |
| `--artifactory-token` | | `artifactory.token` | | Artifactory API token (takes precedence) |
| `--artifactory-username` | | `artifactory.username` | | Artifactory username |
| `--artifactory-password` | | `artifactory.password` | | Artifactory password |
| `--repo-prefix` | | | `chef` | Prefix for repo names |
| `--create-repos` | | | `false` | Auto-create repos if they don't exist |
| `--fetch` | | | `false` | Fetch from Chef API → download → upload (pipeline mode) |

## Local Download Directory Structure

```
{dest}/{product}/{version}/{platform}/{platform_version}/{arch}/
  chef-ice-19.1.158-1.el9.x86_64.rpm
  chef-ice-19.1.158-1.el9.x86_64.rpm.sha256
```

The `.sha256` sidecar file enables skip-if-already-downloaded logic.

## Platform & Architecture Normalization (`pkg/repomap/`)

### Platform Name Normalization

| Chef API | Normalized |
|---|---|
| `el` | `el` |
| `amazon` | `amzn` |
| `sles` | `sles` |
| `opensuse` | `opensuse` |
| `ubuntu` | `ubuntu` |
| `debian` | `debian` |
| `rocky` | `rocky` |
| `alma` | `alma` |
| `fedora` | `fedora` |
| `windows` | `windows` |
| `mac_os_x` | `macos` |

### Platform Version Normalization (apt codenames)

| Distro | Version | Codename |
|---|---|---|
| Ubuntu | `20.04` | `focal` |
| Ubuntu | `22.04` | `jammy` |
| Ubuntu | `24.04` | `noble` |
| Debian | `10` | `buster` |
| Debian | `11` | `bullseye` |
| Debian | `12` | `bookworm` |
| All others | `*` | used as-is |

Unknown versions produce a warning and fall back to the raw version string.

### Architecture Normalization

| Chef API | yum | apt | raw/nuget |
|---|---|---|---|
| `x86_64` | `x86_64` | `amd64` | `x86_64` |
| `aarch64` | `aarch64` | `arm64` | `aarch64` |
| `ppc64le` | `ppc64le` | `ppc64el` | `ppc64le` |
| `ppc64` | `ppc64` | `ppc64` | `ppc64` |
| `i386` | `i386` | `i386` | `i386` |
| `s390x` | `s390x` | `s390x` | `s390x` |

### Repo Type Mapping

| Platform | File extension | Repo type |
|---|---|---|
| `el`, `amazon`, `sles`, `rocky`, `alma`, `fedora`, `opensuse` | `.rpm` | `yum` |
| `ubuntu`, `debian` | `.deb` | `apt` |
| `windows` | `.msi` | `raw` |
| `windows` | `.nupkg` | `nuget` |
| everything else | `*` | `raw` |

### Repo Naming Convention

Pattern: `{prefix}-{normalizedPlatform}{normalizedVersion}-{normalizedArch}-{repoType}`

The repo type suffix is always included.

Examples:

```
chef-el9-x86_64-yum
chef-el8-aarch64-yum
chef-amzn2023-x86_64-yum
chef-sles15-x86_64-yum
chef-ubuntu-jammy-amd64-apt
chef-ubuntu-noble-amd64-apt
chef-debian-bookworm-amd64-apt
chef-windows2019-x86_64-raw
chef-windows2019-x86_64-nuget
chef-macos13-x86_64-raw
```

### Artifact Path Within Repos

Multiple products and versions coexist in the same repo:

```
chef-ice/19.1.158/chef-ice-19.1.158-1.el9.x86_64.rpm
chef/18.4.2/chef-18.4.2-1.el9.x86_64.rpm
inspec/6.8.1/inspec-6.8.1-1.el9.x86_64.rpm
```

### Functions

- `NormalizePlatform(chefPlatform) → string`
- `NormalizePlatformVersion(platform, version) → string`
- `NormalizeArch(repoType, chefArch) → string`
- `RepoType(platform, fileExtension) → string`
- `RepoName(prefix, platform, platformVersion, arch, repoType) → string`

## Library Package Design

### `pkg/chefapi/`

- `Client` struct with functional options pattern
- `ClientOption` type: `WithBaseURL(url)`, `WithHTTPClient(c)`
- `NewClient(licenseID string, opts ...ClientOption) *Client`
- `FetchPackages(ctx context.Context, channel, product, version string) (PackagesResponse, error)`
- `PackagesResponse` = `map[string]map[string]map[string]PackageDetail`
- `PackageDetail` struct: `SHA1`, `SHA256`, `URL`, `Version`
- `FlatPackage` struct: `Platform`, `PlatformVersion`, `Architecture` + embedded `PackageDetail`
- `Flatten() → []FlatPackage` — sorted by Platform, PlatformVersion, Architecture
- `APIError` custom error type with status code and response body

### `pkg/downloader/`

- `Downloader` struct with concurrency, dest dir, skip-existing config
- `Download(ctx context.Context, packages []FlatPackage) ([]DownloadResult, error)`
- `DownloadResult` struct: local file path, success, error, skipped flag
- SHA256 verification on completion
- Writes `.sha256` sidecar files
- Builds the hierarchical directory structure: `{dest}/{product}/{version}/{platform}/{platform_version}/{arch}/`

### `pkg/nexus/`

- `Client` struct with URL, username, password
- `NewClient(url, username, password string) *Client`
- `RepoExists(ctx context.Context, name string) (bool, error)`
- `CreateRepo(ctx context.Context, name string, repoType string) error` — creates yum/apt/raw/nuget hosted repo
- `Upload(ctx context.Context, repoName, remotePath, localFilePath string) error`
- `UploadPackages(ctx context.Context, results []DownloadResult, repoPrefix string, createRepos bool) error` — orchestrates

### `pkg/artifactory/`

- `Client` struct with URL, token, username, password (token takes precedence)
- `NewClient(url string, opts ...ClientOption) *Client`
- `ClientOption`: `WithToken(token)`, `WithBasicAuth(username, password)`
- `RepoExists(ctx context.Context, name string) (bool, error)`
- `CreateRepo(ctx context.Context, name string, repoType string) error` — creates local yum/apt/generic/nuget repo
- `Upload(ctx context.Context, repoName, remotePath, localFilePath string) error` — `PUT` deploy
- `UploadPackages(ctx context.Context, results []DownloadResult, repoPrefix string, createRepos bool) error` — orchestrates

## Output Behavior

- **Interactive** (stdout is a TTY): Progress bars via `schollz/progressbar/v3`
- **Batch** (stdout is not a TTY, or `--no-progress` flag): Line-by-line logging

TTY detection via `golang.org/x/term.IsTerminal()`.

### Batch output format

```
[1/24] Downloading chef-ice-19.1.158-1.el9.x86_64.rpm... done (12.3 MB)
[2/24] Downloading chef-ice-19.1.158-1.el9.aarch64.rpm... done (11.8 MB)
[3/24] Skipping chef-ice-19.1.158-1.ubuntu2204.amd64.deb (already exists, SHA256 match)
...
[24/24] Uploading chef-ice-19.1.158-1.el9.x86_64.rpm to chef-el9-x86_64-yum... done
```

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Config files + env vars |
| `github.com/schollz/progressbar/v3` | Interactive progress bars |
| `golang.org/x/term` | TTY detection |

## Testing

- `httptest`-based tests for `pkg/chefapi/` client
- Test scenarios: success, HTTP error (403, 404, 500), invalid JSON
- Tests for `Flatten()` method correctness and sort order
- Tests for `pkg/repomap/` normalization functions
- Tests for `pkg/downloader/` with mock HTTP server
- Tests for `pkg/nexus/` and `pkg/artifactory/` with mock HTTP servers