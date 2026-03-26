# CLI Commands

## Root Command — `chef-pkg`

Persistent flags (available to all subcommands):

| Flag | Config key | Env var | Default | Description |
|---|---|---|---|---|
| `--config` | — | — | `~/.chef-pkg.toml` | Config file path |
| `--license-id` | `chef.license_id` | `CHEFPKG_CHEF_LICENSE_ID` | — | Chef license ID (required) |
| `--base-url` | `chef.base_url` | `CHEFPKG_CHEF_BASE_URL` | `https://commercial-acceptance.downloads.chef.co` | API base URL |
| `--channel` | `chef.channel` | `CHEFPKG_CHEF_CHANNEL` | `stable` | Release channel |
| `--no-progress` | — | — | `false` | Force line-by-line output even in interactive mode |

## `chef-pkg configure`

Set or display configuration values. When called with flags, updates the
config file with the specified values. When called with `--show`, displays
the current resolved configuration with secrets masked.

Config file location: `~/.chef-pkg.toml` (or the path given by `--config`).
If the file does not exist, it is created.

| Flag | Config key | Description |
|---|---|---|
| `--license-id` | `chef.license_id` | Chef license ID |
| `--cfg-base-url` | `chef.base_url` | Base URL of the Chef downloads API |
| `--cfg-channel` | `chef.channel` | Release channel |
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
A masked value is fully redacted as `*****`.

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

## `chef-pkg list products`

List all available products from the Chef downloads API.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--output` | `-o` | `table` | Output format: `table` or `json` |

No additional flags beyond the inherited root flags. Calls `GET /products?license_id={id}`.

## `chef-pkg list versions`

List available versions for a given product and channel.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--product` | `-p` | `chef` | Product name |
| `--output` | `-o` | `table` | Output format: `table` or `json` |

Calls `GET /{channel}/{product}/versions/all?license_id={id}`.

## `chef-pkg list`

List available packages from the Chef API. This is the default behavior of
`list` when called without a subcommand.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--product` | `-p` | `chef` | Product name |
| `--version` | `-v` | `latest` | Product version (`semver`, `latest`, or `all`; major-only like `18` is allowed) |
| `--platform` | | | Filter by platform (substring, case-insensitive) |
| `--arch` | | | Filter by architecture |
| `--output` | `-o` | `table` | Output format: `table` or `json` |

Package listing output intentionally does not include the package URL, because it
can embed `license_id`.

- Table output omits the URL column entirely and prints full SHA256 values.
- JSON output redacts the `url` field to an empty string.

## `chef-pkg download`

Download packages to local disk. This is the default behavior of `download`
when called without a subcommand.

| Flag | Short | Config key | Default | Description |
|---|---|---|---|---|
| `--product` | `-p` | | `chef` | Product name |
| `--version` | `-v` | | `latest` | Product version (`semver`, `latest`, or `all`; major-only like `18` is allowed) |
| `--platform` | | | | Filter by platform |
| `--arch` | | | | Filter by architecture |
| `--dest` | `-d` | `download.dest` | `./packages` | Destination root directory |
| `--skip-existing` | | | `true` | Skip files with matching SHA256 |
| `--concurrency` | `-c` | `download.concurrency` | `4` | Max parallel downloads |

## `chef-pkg upload nexus`

Upload downloaded packages to Sonatype Nexus.

| Flag | Short | Config key | Default | Description |
|---|---|---|---|---|
| `--source` | `-s` | `download.dest` | `./packages` | Local package directory |
| `--product` | `-p` | | `chef` | Product name |
| `--version` | `-v` | | `latest` | Product version (`semver`, `latest`, or `all`; major-only like `18` is allowed) |
| `--platform` | | | | Filter by platform |
| `--arch` | | | | Filter by architecture |
| `--nexus-url` | | `nexus.url` | | Nexus server URL |
| `--nexus-username` | | `nexus.username` | | Nexus username |
| `--nexus-password` | | `nexus.password` | | Nexus password |
| `--repo-prefix` | | | `chef` | Prefix for repo names |
| `--create-repos` | | | `false` | Auto-create repos if they don't exist |
| `--fetch` | | | `false` | Fetch from Chef API → download → upload (pipeline mode) |

## `chef-pkg upload artifactory`

Upload downloaded packages to JFrog Artifactory.

| Flag | Short | Config key | Default | Description |
|---|---|---|---|---|
| `--source` | `-s` | `download.dest` | `./packages` | Local package directory |
| `--product` | `-p` | | `chef` | Product name |
| `--version` | `-v` | | `latest` | Product version (`semver`, `latest`, or `all`; major-only like `18` is allowed) |
| `--platform` | | | | Filter by platform |
| `--arch` | | | | Filter by architecture |
| `--artifactory-url` | | `artifactory.url` | | Artifactory server URL |
| `--artifactory-token` | | `artifactory.token` | | Artifactory API token (takes precedence) |
| `--artifactory-username` | | `artifactory.username` | | Artifactory username |
| `--artifactory-password` | | `artifactory.password` | | Artifactory password |
| `--repo-prefix` | | | `chef` | Prefix for repo names |
| `--create-repos` | | | `false` | Auto-create repos if they don't exist |
| `--fetch` | | | `false` | Fetch from Chef API → download → upload (pipeline mode) |

## Raw API Explorer

`chef-pkg raw get <path>` sends a raw GET request to an arbitrary API path and
prints the response body.

Query parameters must be passed via `--query key=value` (repeatable). These are
combined with the required `license_id` automatically.

Examples:

```
chef-pkg raw get /stable/chef/versions/all
chef-pkg raw get /stable/chef/packages --query v=18.9.4
chef-pkg raw get /stable/chef-ice/packages --query p=linux --query m=linux
```
