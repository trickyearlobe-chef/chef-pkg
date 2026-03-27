# Platform-First Restructuring

## Overview

Restructure the on-disk download layout, artifact repository model, and CLI
to put platform+platform_version at the top of the hierarchy. This enables
delivering a complete package set per customer platform and simplifies
artifact repository management.

## On-Disk Layout

### Current

```
packages/{product}/{version}/{platform}/{platform_version}/{arch}/{filename}
```

### New

```
packages/{platform}/{platform_version}/{arch}/{product}/{version}/{filename}
```

Examples:

```
packages/el/9/x86_64/chef/18.10.17/chef-18.10.17-1.el9.x86_64.rpm
packages/el/9/x86_64/inspec/6.8.24/inspec-6.8.24-1.el9.x86_64.rpm
packages/el/9/x86_64/chef-server/15.10.91/chef-server-core-15.10.91-1.el9.x86_64.rpm
packages/el/9/aarch64/chef/18.10.17/chef-18.10.17-1.el9.aarch64.rpm
packages/ubuntu/22.04/amd64/chef/18.10.17/chef_18.10.17-1_amd64.deb
packages/windows/2022/x86_64/chef/18.10.17/chef-18.10.17-1-x64.msi
packages/sles/15/x86_64/chef/18.10.17/chef-18.10.17-1.sles15.x86_64.rpm
packages/freebsd/13/x86_64/chef/18.10.17/chef-18.10.17-1-freebsd13-amd64.pkg
packages/solaris2/5.11/sparc/chef/18.10.17/chef-18.10.17-1.sparc.p5p
```

Customer delivery is a directory copy:

```
# Everything for RHEL 9
tar czf customer-el9.tar.gz packages/el/9/

# All RHEL
tar czf customer-el.tar.gz packages/el/

# RHEL + Ubuntu
tar czf customer-linux.tar.gz packages/el/ packages/ubuntu/ packages/debian/
```

### Generic products

Products like `chef-360`, `automate`, and `habitat` return non-standard data
from the API:

| Field            | Standard product     | Generic product          |
|------------------|----------------------|--------------------------|
| platform         | `el`, `ubuntu`, etc. | `linux`, `darwin`        |
| platform_version | `9`, `22.04`         | `pv` (literal string)    |
| arch             | `x86_64`             | `amd64`                  |
| version          | `18.10.17`           | `latest` (literal) or real semver |
| sha256           | 64-char hex          | empty string             |

Handling:

- Normalize `platform_version == "pv"` → `"generic"` in the on-disk path.
- Normalize `platform == "darwin"` → same as `mac_os_x` (see platform fixes).
- When `sha256` is empty, warn to stderr but proceed. The sidecar file stores
  the computed hash from the downloaded bytes.
- When `version` is the literal string `"latest"`, warn to stderr that the
  artifact cannot be version-pinned.

On-disk layout for generic products:

```
packages/linux/generic/amd64/automate/latest/automate-latest-linux-amd64.tar.gz
packages/linux/generic/x86_64/habitat/2.0.450/hab-2.0.450-linux-x86_64.tar.gz
packages/linux-kernel2/generic/x86_64/habitat/2.0.450/hab-2.0.450-linux-kernel2-x86_64.tar.gz
```

## Artifact Repository Model

### Current

One repo per platform+platform_version+arch combination, single product:

```
chef-el9-x86_64-yum
chef-el8-aarch64-yum
chef-ubuntu-jammy-amd64-apt
```

This creates N×M×A repos (products × platform_versions × arches).

### New

One repo per platform+platform_version. All products and all arches coexist
in the same repo:

```
chef-el8-yum
chef-el9-yum
chef-amzn2-yum
chef-amzn2023-yum
chef-sles15-yum
chef-ubuntu-jammy-apt
chef-ubuntu-noble-apt
chef-debian-bookworm-apt
chef-windows10-raw
chef-windows2022-raw
chef-macos13-raw
chef-linux-generic-raw
```

This works because:

- **YUM** repos natively support multiple arches. RPM metadata includes the
  arch and `yum`/`dnf` selects the correct package for the client automatically.
- **APT** repos natively support multiple arches via the `Architectures`
  metadata field. `apt` requests only its own arch.
- **Raw/generic** repos are file stores with no package manager metadata.
  Multiple arches coexist in different paths.

### Repo naming

Pattern: `{prefix}-{normalizedPlatform}{normalizedPlatformVersion}-{repoType}`

- `prefix` defaults to `"chef"`, configurable via `--repo-prefix`.
- Platform and platform_version are normalized per the repomap rules.
- Repo type derived from platform (yum, apt, raw).
- Arch is NOT in the repo name.
- APT repos use a hyphen separator for readability:
  `chef-ubuntu-jammy-apt` not `chef-ubuntujammy-apt`.

### Remote path within repo

Files are organized inside the repo so they are browsable:

```
{product}/{version}/{arch}/{filename}
```

Examples inside `chef-el9-yum`:

```
chef/18.10.17/x86_64/chef-18.10.17-1.el9.x86_64.rpm
chef/18.10.17/aarch64/chef-18.10.17-1.el9.aarch64.rpm
inspec/6.8.24/x86_64/inspec-6.8.24-1.el9.x86_64.rpm
chef-server/15.10.91/x86_64/chef-server-core-15.10.91-1.el9.x86_64.rpm
```

## SHA256 Deduplication

The Chef API often returns identical SHA256 checksums across multiple
platform versions. For example, `inspec` for Windows 8, 10, 11, 2012,
2012r2, 2016, and 2022 are all the same binary. Similarly, `chef` for
Ubuntu 16.04/18.04/20.04 often shares the same `.deb`.

### Behavior

During a download batch, track SHA256 values already downloaded within the
same run. When a package's SHA256 matches one already downloaded:

1. Skip the download.
2. Log to stderr: `Skipped {platform}/{platform_version}/{arch} — identical
   to {original_platform}/{original_platform_version} (SHA256: {short_hash}…)`
3. Do NOT create the file or directory for the skipped platform_version.

This means only the first platform_version with a given SHA256 gets a file.
The skip-existing sidecar logic continues to work because it checks per-directory.

When `sha256` is empty (generic products), dedup is disabled for that package
since there is nothing to compare.

### Impact on uploads

Upload walks the on-disk tree, so skipped platform_versions simply have no
files and produce no upload. The artifact repo for the skipped platform_version
will not be created (unless the user has other products that do differ).

This means a customer pointed at `chef-windows10-raw` and a customer pointed
at `chef-windows2022-raw` might see different product availability if some
products deduped and others didn't. This is acceptable because the identical
binary works on both. If a user wants all platform_version repos populated
identically, they can use `--no-dedup` (see CLI changes below).

## Platform Normalization Fixes

### New platform mappings

Add to `platformMap` in `pkg/repomap/`:

| Chef API       | Normalized    |
|----------------|---------------|
| `darwin`       | `macos`       |
| `linux-kernel2`| `linux-kernel2` (pass-through) |

`freebsd` already passes through correctly (no entry needed).

### New architecture pass-throughs

These arches appear in real API data but aren't in any normalization table.
They pass through unchanged, which is correct:

- `sparc` (Solaris)
- `amd64` (generic products — this is their native arch string, not a
  normalization target)
- `i386` (Solaris)

No code changes needed for these — the existing pass-through behavior is
correct.

### Platform version normalization

The literal string `"pv"` returned by generic products is normalized to
`"generic"` by a new rule in `NormalizePlatformVersion`:

```
if version == "pv" → return "generic"
```

This applies before the existing platform-specific codename lookups.

## CLI Changes

### `chef-pkg download`

#### New: omit `--product` to download all current products

When `--product` is omitted, the tool fetches the current product list from
the API and downloads the latest version of each product for the specified
platform/arch filters.

- `--version` defaults to `"latest"` and applies per-product.
- `--version all` is forbidden when `--product` is omitted. Return an error:
  `--version all requires --product`.
- `--version {semver}` is forbidden when `--product` is omitted. Return an
  error: `--version {v} requires --product (version is product-specific)`.

Generic products (those returning `platform_version: "pv"`) are excluded
from the "all products" flow by default. They can be downloaded individually
with an explicit `--product automate`.

#### New flags

| Flag           | Default | Description                                   |
|----------------|---------|-----------------------------------------------|
| `--no-dedup`   | `false` | Download all platform_versions even when SHA256 matches |
| `--platform-version` | | Filter by platform version (substring, case-insensitive) |

`--platform-version` enables the use case "download everything for RHEL 9
specifically" vs "download everything for all RHEL versions":

```
# All RHEL versions, all products
chef-pkg download --platform el

# Just RHEL 9, all products
chef-pkg download --platform el --platform-version 9

# Just chef for RHEL 9
chef-pkg download --product chef --platform el --platform-version 9
```

#### New flag: `--dry-run`

Show what would be downloaded and where, without actually downloading:

```
chef-pkg download --platform el --dry-run
```

Output:

```
Would download 12 package(s) to ./packages:
  el/9/x86_64/chef/18.10.17  (new)
  el/9/aarch64/chef/18.10.17  (new)
  el/9/x86_64/inspec/6.8.24  (new)
  el/8/x86_64/chef/18.10.17  (skip: identical to el/9/x86_64, SHA256: 56c4ae04…)
  ...
```

### `chef-pkg upload nexus` and `chef-pkg upload artifactory`

#### New flag: `--platform-version`

Matches the download command. Filters which platform_version directories
are scanned for upload.

#### New flag: `--dry-run`

Show what repos would be created and what files would be uploaded:

```
chef-pkg upload nexus --platform el --dry-run --create-repos
```

Output:

```
Would create 2 repo(s) and upload 6 file(s):
  CREATE chef-el8-yum
    chef/18.10.17/x86_64/chef-18.10.17-1.el8.x86_64.rpm
    inspec/6.8.24/x86_64/inspec-6.8.24-1.el8.x86_64.rpm
  CREATE chef-el9-yum
    chef/18.10.17/x86_64/chef-18.10.17-1.el9.x86_64.rpm
    chef/18.10.17/aarch64/chef-18.10.17-1.el9.aarch64.rpm
    inspec/6.8.24/x86_64/inspec-6.8.24-1.el9.x86_64.rpm
    chef-server/15.10.91/x86_64/chef-server-core-15.10.91-1.el9.x86_64.rpm
```

#### Remote path change

The remote path within a repo changes from flat `{filename}` to structured
`{product}/{version}/{arch}/{filename}`. This makes repos browsable and
allows multiple products and versions to coexist without filename collisions.

### `chef-pkg list`

#### New flag: `--platform-version`

Filter package listing by platform version (substring, case-insensitive).

### `chef-pkg clean`

The `clean` command walks the on-disk tree. It must be updated to match the
new directory hierarchy:

```
{dest}/{platform}/{platform_version}/{arch}/{product}/{version}/{filename}
```

## Affected Packages and Files

### `pkg/repomap/`

- Add `"darwin"` to `platformMap`.
- Add `"pv" → "generic"` rule to `NormalizePlatformVersion`.
- Change `RepoName` to drop arch from the name: `{prefix}-{platform}{version}-{type}`.
- Update all tests.

### `pkg/downloader/`

- Change directory structure in `downloadOne` from
  `{dest}/{product}/{version}/{platform}/{platform_version}/{arch}/` to
  `{dest}/{platform}/{platform_version}/{arch}/{product}/{version}/`.
- Add SHA256 dedup tracking (map of sha256 → first download path).
- Add `--no-dedup` option.
- Warn on empty SHA256.
- Warn on literal `"latest"` version.
- Update all tests.

### `cmd/root_download.go`

- Make `--product` optional (empty means all current products).
- Add `--platform-version`, `--no-dedup`, `--dry-run` flags.
- Implement all-products flow: fetch product list, iterate, skip generics.
- Forbid `--version all` and `--version {semver}` without `--product`.

### `cmd/root_upload_nexus.go` and `cmd/root_upload_artifactory.go`

- Update `scanDownloadDir` to walk the new hierarchy.
- Change remote path from flat `{filename}` to `{product}/{version}/{arch}/{filename}`.
- Change `RepoName` call to use new signature (no arch parameter).
- Add `--platform-version` and `--dry-run` flags.

### `cmd/root_list.go` (or equivalent)

- Add `--platform-version` flag.

### `cmd/root_clean.go`

- Update directory walk to match new hierarchy.

### `internal/mcp/`

- Update `list_packages` tool to include `platform_version` filter in input.
- Already done: `PackageInfo` output omits URL.

### `specs/repomap.md`

- Update repo naming convention (drop arch from name).
- Add `darwin` mapping.
- Add `"pv" → "generic"` rule.

### `specs/downloader.md`

- Update directory structure documentation.
- Document dedup behavior.

### `specs/commands.md`

- Add `--platform-version` to download, upload, list commands.
- Add `--no-dedup` and `--dry-run` to download command.
- Add `--dry-run` to upload commands.
- Document `--product` omission behavior.

## Migration

There is no automated migration from the old layout to the new layout.
Users with existing download directories should re-download. The old layout
can be cleaned up manually or via `chef-pkg clean`.

This is acceptable because:

- The tool is pre-1.0 and the old layout was never documented as stable.
- Downloaded packages are ephemeral (they get uploaded to artifact repos).
- Re-downloading with `--skip-existing` would not help because the directory
  structure differs, so files would not be found.

## Worked Examples

### Example 1: Mirror all RHEL 9 packages to Nexus

```
chef-pkg download --platform el --platform-version 9
chef-pkg upload nexus --platform el --platform-version 9 --create-repos --fetch=false
```

Result:
- On disk: `packages/el/9/{x86_64,aarch64}/{chef,inspec,chef-server,...}/{version}/`
- In Nexus: single repo `chef-el9-yum` containing all products and arches.

### Example 2: Mirror just chef for all platforms to Artifactory

```
chef-pkg download --product chef
chef-pkg upload artifactory --create-repos
```

Result:
- On disk: `packages/{el,ubuntu,debian,windows,...}/{version}/{arch}/chef/{version}/`
- In Artifactory: one repo per platform_version (`chef-el8-yum`, `chef-el9-yum`,
  `chef-ubuntu-jammy-apt`, etc.), each containing only chef.
- Windows dedup: only one or two Windows platform_versions actually have files
  (the rest were skipped as identical SHA256).

### Example 3: Customer delivery for Ubuntu 22.04

```
chef-pkg download --platform ubuntu --platform-version 22.04
tar czf customer-ubuntu2204.tar.gz packages/ubuntu/22.04/
```

The tarball contains every product available for Ubuntu 22.04, all arches.

### Example 4: Download a specific generic product

```
chef-pkg download --product automate
```

Warns:
```
Warning: automate version "latest" cannot be pinned — artifact may change on re-download
Warning: automate has empty SHA256 — integrity cannot be verified
```

Result: `packages/linux/generic/amd64/automate/latest/{filename}`
