# Download Orchestration

## `pkg/downloader/`

- `Downloader` struct with concurrency, dest dir, skip-existing config, dedup toggle.
- `Download(ctx context.Context, packages []FlatPackage) ([]DownloadResult, error)`
- `DownloadResult` struct: local file path, success, error, skipped flag, dedup-skipped flag.
- SHA256 verification on completion.
- Writes `.sha256` sidecar files next to the downloaded file.
- Builds platform-first directory structure (see below).

## Local Download Directory Structure

```
{dest}/{platform}/{platform_version}/{arch}/{product}/{version}/
  chef-18.10.17-1.el9.x86_64.rpm
  chef-18.10.17-1.el9.x86_64.rpm.sha256
```

Full restructuring rationale is in `specs/restructure.md`. This package is
responsible for creating the on-disk layout and writing files into it.

The `.sha256` sidecar file enables skip-if-already-downloaded logic.

## SHA256 Deduplication

The Chef API often returns identical SHA256 across multiple platform versions
(e.g. the same Windows `.msi` for Windows 10, 11, 2019, 2022).

During a download batch, track SHA256 values already downloaded in the same run.
When a package's SHA256 matches one already downloaded:

1. Skip the download.
2. Log to stderr: `Skipped {platform}/{platform_version}/{arch} — identical to {original_platform}/{original_platform_version} (SHA256: {short_hash}…)`
3. Do NOT create the file or directory for the skipped platform_version.

When `sha256` is empty (generic products), dedup is disabled for that package —
there is nothing to compare.

`--no-dedup` flag disables this behavior entirely; all platform_versions are
downloaded regardless of matching SHA256.

## Warnings

Log warnings to stderr for conditions that affect reproducibility or integrity:

- **Empty SHA256** (generic products): `Warning: {product} has empty SHA256 — integrity cannot be verified`
- **Literal "latest" version**: `Warning: {product} version "latest" cannot be pinned — artifact may change on re-download`
