# Rules

## CLAUDE.md
- CLAUDE.md is operating rules for the AI, not project documentation.
- Keep it concise. Every line costs context window budget.
- If we change our working practices, CLAUDE.md must be updated.
- Rules are specific and actionable. "NEVER write to stdout" not "be careful with stdout".
- Hard constraints use NEVER in caps. No ambiguity.
- Explicit permission boundaries — say what needs human approval.
- No implementation code in CLAUDE.md or specs. That's what TDD is for.
- When starting a new project, review the CLAUDE.md in Nuclia to check if best practices need to evolve.

## Knowledge
- Specs live in `specs/`. One file per concern. Read only what you need for the current task.
- When researching, find knowledge, put it into Nuclia RAG MCP so we have it tagged and cached for future use.
- Background research (MCP protocol, Go SDK, Chef API, IDE integration) is available via Nuclia RAG through MCP. Query it when specs are insufficient.
- Work plans live in `plans/`. One file per task or feature.

## Specs
- Specs are the source of truth. Code follows specs, not the other way around.
- If a spec is wrong or incomplete, update the spec first, then update the code.
- When implementation reveals a spec gap, add a `TODO:` comment in code and note it in the plan.
- NEVER silently diverge from a spec.
- Do not modify specs without asking.

## Planning
- Before starting work, create a plan in `plans/<task>.md`.
- Plans are short: goal, which specs to read, ordered steps, and acceptance criteria.
- Delete the plan when the work is done. Git is the history.

## Scoping and orientation
- Always consult `plans/todo.md` to see what the next task is.
- Always read the relevant spec in `specs/` for guidance on the specific task.
- Do not start implementation without a plan in `plans/`.

## Development process
- Always write tests before writing code.
- Always run tests after writing code.
- Run `go test -race ./...` after every change.
- Run `go vet ./...` before considering a task done.
- Always perform tasks on a branch named like `<type>/<description>` where type is one of `chore, bug, feature, doc`.
- Commits get a logical description line less than 70 chars, followed by a blank line and extra info if the description doesn't give full understanding.
- NEVER commit until tests are passing and the user has approved.
- NEVER merge until the user approves.
- NEVER push until the user approves.
- Clean up branches after they are merged.
- When diagnosing problems, its fine to hypothesise, but NEVER guess and start writing code. If you don't have evidence, discuss it with the user.

## Git and agents
- Spawned agents NEVER run git commands (add, commit, push, status, etc.). Only the main Claude commits.
- NEVER include PII (names, emails, internal hostnames, IPs, usernames, internal domain names) in code, specs, docs, plans, or commit messages. Use generic examples (`example.com`, `10.0.0.1`, `user@host`).

## Project boundaries
- Ask before changing the public interface of `internal/types/`.
- Ask before changing the public interface of `pkg/` packages that other code depends on.
- Ask before adding any dependency beyond the MCP Go SDK and existing deps in go.mod.
- Ask before deleting or renaming existing files.

## MCP server rules
- NEVER write to stdout. MCP owns it on stdio transport. Use `slog` or `fmt.Fprintf(os.Stderr, ...)`.
- NEVER panic. Return errors. The SDK wraps tool errors into `isError: true`.
- Tool names use `snake_case` verb-noun format (e.g. `list_products`, `search_packages`).
- Return tool errors as content, not exceptions. Regular `error` → tool error; `*jsonrpc.Error` → protocol error.
- Keep tool results concise. Paginate large outputs — LLMs have finite context windows.
- Tool descriptions are specific and self-contained — LLMs depend on them to select tools.
- Use `mcp.AddTool` generic API (recommended over low-level `server.AddTool`).
- Go regex is RE2. No lookaheads, lookbehinds, or backreferences.
- All tool outputs are Go structs marshaled to JSON. No freeform text.

## Conventions
- `gofmt` enforced. `snake_case` tool names. `CamelCase` Go types.
- Struct tags: `json:"field"` + `jsonschema:"description text"`.
- Errors: `fmt.Errorf("context: %w", err)`.
- Tests: table-driven subtests with `t.Run`.
- Logging: `slog.Info`/`slog.Error` with structured key-value pairs.
- Code comments explain *why*, not *what*. No obvious comments.
- Every exported type and function gets a one-line GoDoc comment.

## Security and compliance
- Our license model is Apache2.
- Check libraries to ensure they are not flagged as malware or licensed in a way incompatible with Apache2.
- NEVER commit passwords or secrets.
- NEVER write secrets to stdout or logs.
- Do not commit generated files, binaries, or `bin/`.
