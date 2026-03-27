# IDE Install / Uninstall

## Overview

`chef-pkg --install` registers the binary as an MCP server in all detected IDE
configs. `chef-pkg --uninstall` removes it. Both flags cause the binary to
perform the action, print results to stderr, and exit immediately without
starting the CLI or MCP server.

## Flags

```
--install    Register as MCP server in supported IDEs and exit
--uninstall  Remove from supported IDE MCP configs and exit
```

Mutually exclusive. If both are specified, exit with an error.

## Supported IDEs

| IDE             | Config Path (macOS)                                              | Config Path (Linux)                            | Config Path (Windows)                          | Top-Level Key    | Notes                                                    |
|-----------------|------------------------------------------------------------------|------------------------------------------------|------------------------------------------------|------------------|----------------------------------------------------------|
| Claude Desktop  | ~/Library/Application Support/Claude/claude_desktop_config.json  | ~/.config/Claude/claude_desktop_config.json     | %APPDATA%/Claude/claude_desktop_config.json     | mcpServers       | Global only. Requires restart after edit.                |
| VS Code         | ~/.vscode/mcp.json                                               | ~/.vscode/mcp.json                              | ~/.vscode/mcp.json                              | servers          | Also supports per-project .vscode/mcp.json               |
| Cursor          | ~/.cursor/mcp.json                                               | ~/.cursor/mcp.json                              | ~/.cursor/mcp.json                              | servers          | Same format as VS Code (it's a fork)                     |
| Windsurf        | ~/.codeium/windsurf/mcp_config.json                              | ~/.codeium/windsurf/mcp_config.json             | ~/.codeium/windsurf/mcp_config.json             | mcpServers       | Same format as Claude Desktop                            |
| Zed             | ~/.config/zed/settings.json                                      | ~/.config/zed/settings.json                     | N/A                                             | context_servers  | JSONC format (has comments). Shared settings file.       |

## Server Entry Format

Most IDEs use a minimal entry:

```json
{
  "<top-level-key>": {
    "chef-pkg": {
      "command": "/absolute/path/to/chef-pkg",
      "args": ["serve"]
    }
  }
}
```

### CRITICAL: Zed Requires Extra Fields

Zed's settings parser is strict. A bare `{"command": "..."}` entry causes
"Invalid user settings file — data did not match any variant of untagged enum
ContextServerSettingsContent". Zed entries MUST include `args` and `env`:

```json
{
  "context_servers": {
    "chef-pkg": {
      "command": "/absolute/path/to/chef-pkg",
      "args": ["serve"],
      "env": {}
    }
  }
}
```

In code, detect Zed by checking `topLevelKey == "context_servers"` and always
include `args` and `env`.

The `command` value must be an absolute path. Resolve with
`os.Executable()` followed by `filepath.EvalSymlinks()`.

## JSONC Handling (Zed)

Zed's `settings.json` uses JSONC (JSON with Comments):
- Line comments: `// comment`
- Block comments: `/* comment */`
- Trailing commas: `{"key": "value",}`

Standard `json.Unmarshal` will fail on JSONC. Comments must be stripped before
parsing.

### Comment Stripping Algorithm

1. Walk the string character by character.
2. When inside a quoted string (`"..."`), copy verbatim — do NOT strip `//` or
   `/*` inside strings.
3. Handle escape sequences (`\\`, `\"`) inside strings.
4. When `//` is found outside a string, skip to end of line.
5. When `/*` is found outside a string, skip to closing `*/`.
6. After stripping comments, remove trailing commas before `}` or `]`.

### Preamble Preservation

Zed's `settings.json` may have comment lines before the opening `{`. When
rewriting:
1. Extract everything before the first `{` as the "preamble".
2. Parse the stripped JSON.
3. Marshal the updated config.
4. Prepend the original preamble when writing back.

This preserves the user's `// Zed settings` header comments.

## Install Logic (--install)

1. Resolve binary path: `os.Executable()` → `filepath.EvalSymlinks()`.
2. For each IDE:
   a. Check if config directory exists (IDE installed?) — skip if not.
   b. Read config file (or start with `{}` if file missing).
   c. For Zed: strip JSONC comments, extract preamble.
   d. Parse JSON into `map[string]any`.
   e. Look for existing server entry under the top-level key.
   f. If entry exists with same command, same args, AND correct shape
      (all required keys present) → report "already up to date".
   g. If entry exists but wrong command, wrong args, or missing keys →
      update, report "updated".
   h. If entry missing → add, report "installed".
   i. Write back with `json.MarshalIndent` (2-space indent).
   j. For Zed: prepend the preserved preamble.
3. Create parent directories (`os.MkdirAll`) if config file's directory
   doesn't exist but the IDE directory does.

The shape check (step f) catches cases where a previous version wrote a minimal
entry that's missing Zed's required `args`/`env` fields. Re-running `--install`
will fix it.

## Uninstall Logic (--uninstall)

1. For each IDE:
   a. Check if config directory exists — skip if not.
   b. Check if config file exists — report "not present" if not.
   c. Read and parse config (with JSONC stripping for Zed).
   d. Look for server entry — report "not present" if missing.
   e. Delete the entry.
   f. If the servers map is now empty, remove the top-level key entirely.
   g. Write back, preserving other config keys and preamble.

## Output

All output goes to stderr. Per-IDE status lines:

```
Claude Desktop: installed
VS Code:        already up to date
Cursor:         skipped (not installed)
Windsurf:       skipped (not installed)
Zed:            updated
```

Uninstall:

```
Claude Desktop: removed
VS Code:        removed
Cursor:         not present
Windsurf:       skipped (not installed)
Zed:            removed
```

Exit code 0 if no errors, 1 if any IDE had an error.

## Key Design Decisions

- NEVER write to stdout (MCP owns it).
- Preserve all existing keys in config files — only touch the server entry.
- Idempotent: running `--install` twice changes nothing the second time.
- Skip gracefully when an IDE isn't installed (directory doesn't exist).
- The server name in config files is `"chef-pkg"`.

## Package Layout

```
internal/ideconfig/
    ideconfig.go       — IDE definitions, Install(), Uninstall()
    ideconfig_test.go  — unit tests
    jsonc.go           — JSONC comment stripping, preamble extraction
    jsonc_test.go      — JSONC tests
```

## Testing Strategy

Unit tests use `t.TempDir()` to create isolated config directories:
- New file creation (no existing config).
- Update existing config (preserves other keys and other servers).
- Idempotent re-run (same path → "already up to date").
- Different top-level keys (`mcpServers`, `servers`, `context_servers`).
- Zed entry includes `args` and `env`; non-Zed entries include `args` only.
- Re-install updates old Zed entries that are missing `args`/`env`.
- JSONC with line comments, block comments, trailing commas.
- Comments inside string values are NOT stripped.
- Preamble preservation (comments before opening `{`).
- Uninstall: removes entry, preserves others, cleans empty top-level key.
- Round-trip: install → verify → uninstall → verify → uninstall again (idempotent).
- Skipped when IDE directory doesn't exist.

No real IDE configs are touched during testing.

## Flag Handling in main.go

The `--install` and `--uninstall` flags are checked before Cobra's command
dispatch. If either is set, the action runs and the process exits. They are
root-level flags on `rootCmd`, not on a subcommand.

```go
rootCmd.Flags().Bool("install", false, "Register as MCP server in supported IDEs and exit")
rootCmd.Flags().Bool("uninstall", false, "Remove from supported IDE MCP configs and exit")
```

These are `Flags()` not `PersistentFlags()` — they only apply to the root
command itself.