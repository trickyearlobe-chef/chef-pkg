# MCP Server

## Overview

chef-pkg can run as an MCP server over stdio transport, exposing Chef package
discovery as tools that AI assistants can call directly.

## Server Entry Point

A new `chef-pkg serve` command starts the MCP server. It blocks until the
client disconnects. All human-readable output goes to stderr.

```
chef-pkg serve
```

The server requires a valid `license_id` (via config, env, or flag) just like
the CLI commands. If missing, the server exits with an error on stderr.

## Server Identity

```go
&mcp.Implementation{
    Name:    "chef-pkg",
    Version: version, // from build-time variable
}
```

Instructions sent to clients:
> You are a Chef package discovery assistant. Use the available tools to help
> users find Chef products, versions, and packages across platforms and
> architectures.

## Tools

All tools return Go structs marshaled to JSON. No freeform text.

### list_products

List all available Chef products.

| Field       | Type   | Required | Description                              |
|-------------|--------|----------|------------------------------------------|
| include_eol | bool   | no       | Include end-of-life products (default false) |

**Output:** `ListProductsOutput`

```go
type ListProductsOutput struct {
    Products []ProductInfo `json:"products"`
}

type ProductInfo struct {
    Name   string `json:"name"`
    Status string `json:"status"` // "current" or "eol"
}
```

When `include_eol` is false, all products have status "current" and eol
products are omitted.

### list_versions

List available versions for a product.

| Field   | Type   | Required | Description                          |
|---------|--------|----------|--------------------------------------|
| product | string | no       | Product name (default "chef")        |
| channel | string | no       | Release channel (default from config)|

**Output:** `ListVersionsOutput`

```go
type ListVersionsOutput struct {
    Product  string   `json:"product"`
    Channel  string   `json:"channel"`
    Versions []string `json:"versions"`
}
```

Versions are returned in ascending semver order.

### list_packages

List available packages for a product version.

| Field    | Type   | Required | Description                                  |
|----------|--------|----------|----------------------------------------------|
| product  | string | no       | Product name (default "chef")                |
| version  | string | no       | Version: semver, "latest", or "all" (default "latest") |
| channel  | string | no       | Release channel (default from config)        |
| platform | string | no       | Filter by platform (substring, case-insensitive) |
| arch     | string | no       | Filter by architecture (substring, case-insensitive) |

**Output:** `ListPackagesOutput`

```go
type ListPackagesOutput struct {
    Product  string        `json:"product"`
    Version  string        `json:"version"`
    Channel  string        `json:"channel"`
    Packages []PackageInfo `json:"packages"`
}

type PackageInfo struct {
    Platform        string `json:"platform"`
    PlatformVersion string `json:"platform_version"`
    Architecture    string `json:"architecture"`
    Version         string `json:"version"`
    SHA256          string `json:"sha256"`
}
```

The URL field is intentionally omitted — it embeds the license_id.

When version is "all", the tool fetches all versions. When version is "latest",
it resolves the latest version that has packages matching the filters.

### raw_get

Send a raw GET request to any Chef downloads API endpoint. Useful for exploring
the API, discovering undocumented endpoints, and debugging.

| Field  | Type              | Required | Description                                      |
|--------|-------------------|----------|--------------------------------------------------|
| path   | string            | yes      | API path (e.g. "/stable/chef/versions/all")      |
| params | map[string]string | no       | Additional query parameters (license_id is added automatically) |

**Output:** `RawGetOutput`

```go
type RawGetOutput struct {
    Path       string `json:"path"`
    StatusCode int    `json:"status_code"`
    Body       any    `json:"body"`
}
```

The `body` field contains the parsed JSON response if valid JSON, or the raw
string if not. This lets the LLM inspect any API response directly.

## Package Layout

```
internal/mcp/
    server.go          — NewServer(), tool registration, Run()
    server_test.go     — in-memory transport tests
    tools.go           — tool handler functions
    tools_test.go      — unit tests for handler logic
    types.go           — input/output structs
```

Tool handlers accept the chefapi.Client as a dependency (closure or struct
field), not as a global. This makes them testable with httptest.

## Error Handling

- Missing license_id at startup → exit with error on stderr, do not start server.
- API errors inside tools → return as tool error (regular `error` from handler).
  The SDK sets `isError: true` and puts the message in content.
- Invalid input → the SDK validates against the schema and returns `InvalidParams`.
- NEVER return `*jsonrpc.Error` from tool handlers — all errors are tool-level.

## Configuration

The MCP server reads configuration from the same sources as the CLI:
- Config file (`~/.chef-pkg.toml` or `--config`)
- Environment variables (`CHEFPKG_CHEF_LICENSE_ID`, etc.)
- Command-line flags on the `serve` command

The `serve` command inherits root persistent flags (`--license-id`, `--base-url`,
`--channel`, `--config`).

## Logging

All logging via `slog` to stderr. The server passes an `slog.Logger` writing
to stderr via `ServerOptions.Logger`. NEVER write to stdout.

## Testing

- Unit tests for each tool handler using httptest mock servers.
- In-memory transport integration test: create server, connect client via
  `mcp.NewInMemoryTransports()`, call each tool, assert output structure.
- Test missing license_id produces an error before server starts.
- Test API error responses produce tool errors with `isError: true`.