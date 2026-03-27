# Plan: MCP Server and IDE Install

## Goal

Add an MCP server mode to chef-pkg and `--install`/`--uninstall` flags for
IDE registration. Build incrementally: raw_get tool first, explore the API,
then decide what other tools to add.

## Specs to read

- `specs/mcp.md` — server entry point, tools, types, error handling
- `specs/ide-install.md` — config paths, JSONC handling, install/uninstall logic
- `specs/chefapi.md` — existing API client we're wrapping

## Completed steps

1. ~~Flatten command structure (list/download/clean packages → bare verbs)~~
2. ~~Update CLAUDE.md with MCP and Go best practices~~
3. ~~Split monolith spec into per-concern files~~
4. ~~Write specs/mcp.md and specs/ide-install.md~~

## Remaining steps

### Phase 1 — MCP server with raw_get

5. Add MCP Go SDK dependency (`github.com/modelcontextprotocol/go-sdk`)
6. Create `internal/mcp/types.go` — input/output structs for raw_get
7. Create `internal/mcp/tools.go` — raw_get handler
8. Write tests for raw_get handler (`internal/mcp/tools_test.go`)
9. Create `internal/mcp/server.go` — NewServer(), tool registration
10. Write in-memory transport integration test (`internal/mcp/server_test.go`)
11. Create `cmd/root_serve.go` — `chef-pkg serve` command
12. Manual test: run `chef-pkg serve` with a client, call raw_get

### Phase 2 — Explore API and add tools

13. Use raw_get to explore edge cases across products
14. Decide which additional tools to implement (list_products, list_versions, list_packages, or others)
15. Implement decided tools with tests
16. Update in-memory integration tests

### Phase 3 — IDE install/uninstall

17. Create `internal/ideconfig/jsonc.go` — comment stripping, preamble extraction
18. Write JSONC tests (`internal/ideconfig/jsonc_test.go`)
19. Create `internal/ideconfig/ideconfig.go` — IDE definitions, Install(), Uninstall()
20. Write install/uninstall tests (`internal/ideconfig/ideconfig_test.go`)
21. Add `--install` and `--uninstall` flags to root command
22. Manual test: install into local IDE configs, verify, uninstall, verify

### Phase 4 — Polish

23. Update README.md with MCP server usage
24. Update specs if implementation revealed gaps
25. Final `go test -race ./...` and `go vet ./...`

## Acceptance criteria

- `chef-pkg serve` starts an MCP server over stdio
- `raw_get` tool works and returns parsed JSON from any API endpoint
- All decided tools return structured JSON output
- `chef-pkg --install` registers in all detected IDE configs
- `chef-pkg --uninstall` removes from all detected IDE configs
- Idempotent: install twice is safe, uninstall twice is safe
- All tests pass with `-race`
- `go vet` clean