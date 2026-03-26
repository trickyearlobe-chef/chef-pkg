# Plan: Flatten Command Structure (Option A)

## Goal

Eliminate the stuttering `packages` subcommand from `list`, `download`, and `clean`.
The bare verb becomes the leaf command for package operations.

## Changes

### Command mapping

| Before                      | After                                  |
|-----------------------------|----------------------------------------|
| `chef-pkg list packages`    | `chef-pkg list` (default, with flags)  |
| `chef-pkg list products`    | `chef-pkg list products` (unchanged)   |
| `chef-pkg list versions`    | `chef-pkg list versions` (unchanged)   |
| `chef-pkg download packages`| `chef-pkg download` (leaf command)     |
| `chef-pkg clean packages`   | `chef-pkg clean` (leaf command)        |
| `chef-pkg upload nexus`     | `chef-pkg upload nexus` (unchanged)    |
| `chef-pkg upload artifactory`| `chef-pkg upload artifactory` (unchanged) |
| `chef-pkg configure`        | `chef-pkg configure` (unchanged)       |
| `chef-pkg raw get`          | `chef-pkg raw get` (unchanged)         |
| `chef-pkg clean nexus`      | `chef-pkg clean nexus` (unchanged)     |

### Steps

1. **`cmd/root_list.go`** — Move `RunE` from `root_list_packages.go` into `listCmd` directly. The `list` command becomes a leaf that lists packages by default when called with no subcommand. Keep `products` and `versions` as subcommands.

2. **`cmd/root_list_packages.go`** — Delete this file. Its flags and `RunE` move into `root_list.go`.

3. **`cmd/root_download.go`** — Merge `root_download_packages.go` into this file. `downloadCmd` becomes a leaf command with `RunE`, flags, and examples.

4. **`cmd/root_download_packages.go`** — Delete this file.

5. **`cmd/root_clean.go`** — Merge `root_clean_packages.go` logic into `cleanCmd` as a `RunE`. Keep `clean nexus` as a subcommand.

6. **`cmd/root_clean_packages.go`** — Delete this file.

7. **Update examples** in all affected commands — remove the `packages` noun from example strings.

8. **Update specs** — `specs/commands.md` and `specs/overview.md` to reflect the new command tree.

9. **Update `plans/todo.md`** — reflect new command names.

10. **Update tests** — any test that references the old subcommand names.

### Acceptance criteria

- `chef-pkg list` lists packages (with `--product`, `--version`, etc.)
- `chef-pkg list products` and `chef-pkg list versions` still work
- `chef-pkg download` downloads packages directly
- `chef-pkg clean` cleans packages directly
- `chef-pkg clean nexus` still works
- All existing tests pass after rename
- `go vet ./...` clean
- Specs match implementation