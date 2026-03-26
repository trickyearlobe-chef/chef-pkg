# Download Orchestration

## `pkg/downloader/`

- `Downloader` struct with concurrency, dest dir, skip-existing config
- `Download(ctx context.Context, packages []FlatPackage) ([]DownloadResult, error)`
- `DownloadResult` struct: local file path, success, error, skipped flag
- SHA256 verification on completion
- Writes `.sha256` sidecar files
- Builds the hierarchical directory structure: `{dest}/{product}/{version}/{platform}/{platform_version}/{arch}/`

## Local Download Directory Structure

```
{dest}/{product}/{version}/{platform}/{platform_version}/{arch}/
  chef-ice-19.1.158-1.el9.x86_64.rpm
  chef-ice-19.1.158-1.el9.x86_64.rpm.sha256
```

The `.sha256` sidecar file enables skip-if-already-downloaded logic.