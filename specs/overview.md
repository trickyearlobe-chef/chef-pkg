# chef-pkg Overview

## Overview

`chef-pkg` is a Go CLI utility for fetching Chef client packages from the Progress Chef commercial downloads API, downloading them locally, and uploading them to artifact repositories (Sonatype Nexus, JFrog Artifactory).

## Command Hierarchy & File Layout

```
cmd/
  root.go                            → chef-pkg (global config via Viper)
  root_configure.go                  → chef-pkg configure (set/show config)
  root_list.go                        → chef-pkg list (list packages, or use subcommands)
  root_list_products.go               → chef-pkg list products
  root_list_versions.go               → chef-pkg list versions
  root_download.go                    → chef-pkg download (download packages)
  root_upload.go                     → chef-pkg upload (parent — no action alone)
  root_upload_nexus.go               → chef-pkg upload nexus
  root_upload_artifactory.go         → chef-pkg upload artifactory
  root_clean.go                      → chef-pkg clean (remove local packages, or use subcommands)
  root_clean_nexus.go                → chef-pkg clean nexus (hidden)
  root_raw.go                        → chef-pkg raw get (raw API explorer)
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
channel = "stable"

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

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Config files + env vars |
| *(planned)* `github.com/schollz/progressbar/v3` | Interactive progress bars |
| *(planned)* `golang.org/x/term` | TTY detection |

## Output Behavior

Currently, output is line-by-line logging. TTY detection and interactive
progress bars are planned but not yet implemented.

### Batch output format

```
[1/24] Downloading chef-ice-19.1.158-1.el9.x86_64.rpm... done (12.3 MB)
[2/24] Downloading chef-ice-19.1.158-1.el9.aarch64.rpm... done (11.8 MB)
[3/24] Skipping chef-ice-19.1.158-1.ubuntu2204.amd64.deb (already exists, SHA256 match)
...
[24/24] Uploading chef-ice-19.1.158-1.el9.x86_64.rpm to chef-el9-x86_64-yum... done
```
