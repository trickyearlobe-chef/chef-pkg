# Plan: Add list_products, list_versions, list_packages MCP tools

## Goal

Add three new MCP tools to the chef-pkg server alongside the existing `raw_get` tool.
These tools wrap the chefapi.Client methods and return structured, LLM-friendly output.

## Specs to read

- `specs/mcp.md` — tool definitions, input/output types, error handling
- `specs/chefapi.md` — client methods available

## Steps

1. Add `Channel` field to `ServerConfig` (default from viper `chef.channel`).
2. Update `cmd/root_serve.go` to pass channel into `ServerConfig`.
3. Add input/output types to `internal/mcp/types.go`:
   - `ListProductsInput` / `ListProductsOutput` / `ProductInfo`
   - `ListVersionsInput` / `ListVersionsOutput`
   - `ListPackagesInput` / `ListPackagesOutput` / `PackageInfo`
4. Write unit tests in `tools_test.go` for all 3 handlers (httptest, table-driven).
5. Write integration tests in `server_test.go` (in-memory transport, call each tool).
6. Implement handlers in `tools.go`:
   - `listProductsHandler` — calls `FetchProducts`, optionally with `eol=true`
   - `listVersionsHandler` — calls `FetchVersions` with channel/product defaults
   - `listPackagesHandler` — calls `FetchPackages`, flattens, filters by platform/arch substring
7. Register all 3 tools in `registerTools` in `server.go`.
8. Run `go test -race ./...` and `go vet ./...`.
9. Update `server_test.go` `TestServerListsTools` to expect 4 tools.

## Key decisions

- URL field omitted from `PackageInfo` — it embeds `license_id` (per spec).
- `list_packages` with `version=""` defaults to `"latest"`.
- Platform/arch filters are case-insensitive substring matches (Go RE2 safe: just `strings.Contains` + `strings.ToLower`).
- API errors from handlers return as regular `error` — SDK wraps to `isError: true`.
- `list_products` with `include_eol=true` calls API twice (current + eol) and merges, tagging status.

## Acceptance criteria

- [ ] All 4 tools listed by `ListTools` in integration test
- [ ] `list_products` returns products with status field
- [ ] `list_products` with `include_eol=true` includes eol products
- [ ] `list_versions` defaults product to "chef" and channel to config value
- [ ] `list_packages` returns flat package list without URL
- [ ] `list_packages` filters by platform and arch (case-insensitive)
- [ ] All tests pass with `-race`
- [ ] `go vet ./...` clean