# Plan: Platform-First Restructuring

## Goal

Restructure on-disk layout, artifact repo model, and CLI to put
platform+platform_version at the top of the hierarchy. Enable delivering
a complete package set per customer platform.

## Specs to read

- `specs/restructure.md` — master spec for the full restructuring
- `specs/repomap.md` — updated repo naming (no arch, darwin, pv→generic)
- `specs/downloader.md` — updated directory layout, dedup, warnings
- `specs/commands.md` — updated CLI flags and behavior

## Task breakdown

Work in dependency order. Each task is a branch, tests before code.

### Phase 1: Foundations (no breaking changes yet)

#### 1a. Platform normalization fixes — `pkg/repomap/` ✅
- Add `"darwin" → "macos"` to `platformMap`
- Add `"pv" → "generic"` rule to `NormalizePlatformVersion` (before codename lookups)
- Tests first: new table-driven cases for darwin, pv, freebsd pass-through
- Run: `go test -race ./pkg/repomap/...`

#### 1b. RepoName drops arch parameter — `pkg/repomap/` ✅
- Change `RepoName(prefix, platform, platformVersion, arch, repoType)` →
  `RepoName(prefix, platform, platformVersion, repoType)`
- Remove arch from the generated name
- Update all callers (grep for `repomap.RepoName`)
- Update all tests
- Run: `go test -race ./...`

### Phase 2: Downloader restructuring

#### 2a. Directory layout change — `pkg/downloader/` ✅
- Changed `downloadOne` path from
  `{dest}/{product}/{version}/{platform}/{platform_version}/{arch}/`
  to `{dest}/{platform}/{platform_version}/{arch}/{product}/{version}/`
- Removed `product` from `Downloader` struct; product now comes from `FlatPackage.Product`
- Added `Product` field to `chefapi.FlatPackage`
- Updated `Flatten(product string)` to accept and set product name
- Updated all callers: `downloader.New()`, `Flatten()`, `scanDownloadDir`
- Updated all downloader tests
- Run: `go test -race ./...` — all pass

#### 2b. SHA256 dedup — `pkg/downloader/` ✅
- Added `WithDedup(bool)` option (default true)
- Thread-safe `dedupTracker` (sha256→first platform/pv) in `Download()` across batch
- Skip download when sha256 matches; set `DedupSkipped` on result
- Dedup disabled for empty sha256 (generic products)
- Added `DedupSkipped` field to `DownloadResult`
- Tests: dedup hit, dedup miss, empty sha256, no-dedup flag, concurrent dedup,
  cross-product dedup, per-batch isolation
- Run: `go test -race ./pkg/downloader/...` — all pass

#### 2c. Warnings for generic products — `pkg/downloader/` ✅
- Added `WithWarningFunc(fn)` option for non-fatal warnings
- Warn when `sha256 == ""`: "has empty SHA256 — integrity cannot be verified"
- Warn when `version == "latest"`: "cannot be pinned — artifact may change"
- Thread-safe `warningTracker` deduplicates per product+warning-type
- Tests: verify each warning emitted, no warnings for normal packages,
  both warnings together, once-per-product dedup
- Run: `go test -race ./pkg/downloader/...` — all pass

### Phase 3: CLI changes

#### 3a. Add `--platform-version` flag to download, list, upload commands
- Add flag to `root_download.go`, `root_list.go`, `root_upload_nexus.go`,
  `root_upload_artifactory.go`
- Wire through to `filterPackages` (add platform_version substring filter)
- Tests: verify filtering works
- Run: `go test -race ./cmd/...`

#### 3b. Add `--no-dedup` flag to download
- Pass through to downloader `WithDedup(!noDedup)`
- Run: `go test -race ./cmd/...`

#### 3c. Make `--product` optional on download (all-products flow)
- When product is empty, fetch product list from API
- Iterate products, download latest of each
- Skip generic products (platform_version == "pv" in any package)
- Forbid `--version all` without `--product`
- Forbid `--version {semver}` without `--product`
- Tests: mock API returning product list, verify iteration and skip logic
- Run: `go test -race ./cmd/...`

#### 3d. Add `--dry-run` to download
- When set, resolve everything but skip actual downloads
- Print planned actions to stderr
- Run: `go test -race ./cmd/...`

### Phase 4: Upload restructuring

#### 4a. Update `scanDownloadDir` for new hierarchy
- Walk `{source}/{platform}/{platform_version}/{arch}/{product}/{version}/`
- Apply platform, platform_version, arch, product filters
- Tests: create temp dirs in new layout, verify scan finds correct files
- Run: `go test -race ./cmd/...`

#### 4b. Change remote path in uploads
- From flat `{filename}` to `{product}/{version}/{arch}/{filename}`
- Update both nexus and artifactory upload commands
- Run: `go test -race ./cmd/...`

#### 4c. Update RepoName calls in upload commands
- Remove arch argument from `repomap.RepoName` calls
- Run: `go test -race ./cmd/...`

#### 4d. Add `--dry-run` to upload commands
- When set, resolve repos and files but skip API calls
- Print planned creates and uploads to stderr
- Run: `go test -race ./cmd/...`

### Phase 5: Clean and list updates

#### 5a. Update `clean` command for new hierarchy
- Walk new directory structure
- Run: `go test -race ./cmd/...`

#### 5b. Add `--platform-version` filter to `list` command
- Substring filter on platform_version field in package listing
- Run: `go test -race ./cmd/...`

### Phase 6: MCP tool update

#### 6a. Add `platform_version` filter to `list_packages` MCP tool
- Add `PlatformVersion` field to `ListPackagesInput`
- Apply filter in handler (same case-insensitive substring logic)
- Update unit and integration tests
- Run: `go test -race ./internal/mcp/...`

### Phase 7: Final validation

- `go test -race ./...` — all packages pass
- `go vet ./...` — clean
- Manual smoke test: download a small product, verify on-disk layout
- Manual smoke test: `--dry-run` on download and upload
- Review all specs match implementation

## Acceptance criteria

- [x] `darwin` normalizes to `macos`
- [x] `"pv"` normalizes to `"generic"` in platform version
- [x] Repo names have no arch component
- [x] On-disk layout is `{platform}/{platform_version}/{arch}/{product}/{version}/`
- [x] SHA256 dedup skips identical platform_versions with log message
- [x] `--no-dedup` disables dedup
- [x] Empty SHA256 and literal "latest" version produce warnings
- [ ] Omitting `--product` downloads all current products (excluding generics)
- [ ] `--version all` without `--product` returns error
- [ ] `--platform-version` filter works on download, upload, list
- [ ] `--dry-run` works on download and upload
- [ ] Upload remote path is `{product}/{version}/{arch}/{filename}`
- [ ] `scanDownloadDir` walks new hierarchy
- [ ] `clean` command walks new hierarchy
- [ ] `list_packages` MCP tool accepts `platform_version` filter
- [ ] All tests pass with `-race`
- [ ] `go vet` clean