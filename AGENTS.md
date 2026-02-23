# AGENTS.md

Fast, lightweight, AI-agent-friendly CLI for Xcode `.xc*` workflows. Built in Go.

## Best Practice References

Always refer to these repositories for implementation and CLI best practices:

- <https://github.com/jesseduffield/lazygit.git>
- <https://github.com/rudrankriyam/App-Store-Connect-CLI>
- <https://github.com/cli/cli>

## Core Principles

- Explicit flags: always prefer long-form flags (`--project`, `--output`, `--apply`) over short aliases.
- JSON-first: minified JSON by default, `--output table|markdown` for humans.
- No interactive prompts: commands must remain automation-safe and CI-safe by default.
- Deterministic output: stable ordering for machine parsing and reproducible diffs.
- Safety-first writes: destructive operations require explicit opt-in and git state checks.

## xcwrap Scope (V1)

- CLI name: `xcwrap`
- Primary feature area: `.xcassets`
- Primary commands:
  - `xcwrap assets scan`
  - `xcwrap assets unused`
  - `xcwrap assets prune`

## Discovering Commands

Do not memorize command shapes. Use help output as the source of truth.

```bash
xcwrap --help
xcwrap assets --help
xcwrap assets scan --help
```

When implementing or testing command behavior, verify current flags with `--help` first.

## Input and Configuration

- Default scan root is the current directory.
- Allow explicit path override via flags.
- Support include/exclude controls for scan scope.
- Support config + env + flags precedence:
  - `flags > env > config > defaults`
- Use `.xcwrap.yaml` for repository/local configuration.

## Asset Detection (V1)

Only high-confidence conservative matching is allowed.

Treat assets as used only when referenced from:

- `.swift`
- `.m`
- `.h`
- `.xib`
- `.storyboard`

Do not ship speculative or low-confidence heuristics in V1.

## Safety Contract

- `xcwrap assets prune` must be dry-run by default.
- Deletion requires explicit `--apply`.
- For `--apply`, require clean git working tree by default.
- Allow explicit override (`--force`) for exceptional workflows.
- Rely on git safety checks; no separate backup mechanism in V1.

## Output Contract

- Default stdout: minified JSON.
- JSON field names must use camelCase.
- Human output: `--output table` or `--output markdown`.
- Errors must use a structured JSON envelope.
- Flag validation and usage errors must return exit code `2`.
- `xcwrap assets unused` must return non-zero when unused assets are found (CI gating behavior).

## Exit Codes

- `0`: success with no blocking findings.
- `1`: command/runtime failure.
- `2`: CLI usage/flag validation errors.
- `3`: unused assets detected by `assets unused`.

## Performance

- Enable parallel scanning by default.
- Auto-size worker count based on CPU.
- Keep memory usage bounded for large repos.

## Build & Test

Before PR or merge, run:

```bash
go test ./...
golangci-lint run
govulncheck ./...
```

Also run integration tests for command behavior and filesystem/git safety paths.

## PR Guardrails

- Before opening or updating a PR, run:
  - `go test ./...`
  - `golangci-lint run`
  - `govulncheck ./...`
- CI must enforce tests + lint + vulnerability checks on PRs and `main`.
- Keep changes narrowly scoped; avoid mixing refactor + feature + bug fix in one PR.

## Testing Discipline

- Use TDD for behavior changes: write failing tests first, then implementation.
- For every new/changed flag, add:
  - one valid-path test
  - one invalid-value test asserting stderr/stdout contract and exit code
- For CLI behavior changes, assert:
  - exit code
  - stdout shape (parse JSON where applicable)
  - stderr/structured error behavior
- Test real invocation patterns (flag order, mixed global + subcommand flags, unexpected values).
- Do not skip tests with broad string matching; skips must be specific and documented.

## Debugging & Bug Fixing

- Reproduce first: confirm the failure with a focused local test before changing code.
- One change at a time: verify each fix before stacking more edits.
- Do not bypass checks: no skipping lint/tests to force progress.
- Verify before claiming done: rerun the failing case and relevant suites.

## Definition of Done (Single-Pass)

Do not mark work complete until all are true:

- Behavior is implemented and documented.
- Tests were added/updated and are passing locally.
- `go test ./...` passes.
- `golangci-lint run` passes.
- `govulncheck ./...` passes.
- Command help and generated docs reflect final flags and defaults.
- Exit code and output contract are verified for the changed command path.

## CLI Implementation Checklist

- Register new commands in the central command registry.
- Keep command parsing/IO in CLI layer; put domain logic in reusable packages.
- Validate required flags explicitly and return structured errors.
- Keep write operations behind explicit flags (`--apply` pattern).
- Keep JSON schema/output fields stable unless versioned/breaking-change noted.
- Keep command help clear and complete (`Usage`, required flags, examples).

## Documentation

- Generate command docs from code.
- Keep README examples machine-safe and copy-pasteable.
- Document CI-friendly behavior and non-interactive guarantees.
- Document destructive command safeguards clearly.

## Environment Variables

| Variable | Purpose |
| ---------- | --------- |
| `XCWRAP_CONFIG_PATH` | Absolute path override for config file |
| `XCWRAP_DEFAULT_OUTPUT` | Default output (`json`, `table`, `markdown`) |
| `XCWRAP_DEBUG` | Enable debug logging (`1`/`true`) |
| `XCWRAP_WORKERS` | Override automatic worker count for scans |

Explicit CLI flags always override environment variables.

## Release & Versioning

- Follow SemVer.
- Require changelog updates per release.
- Use signed git tags.
- Publish via:
  - Homebrew
  - `go install`
  - GitHub release binaries
- Keep release automation reproducible and CI-driven.

## Platform and Privacy

- Official V1 support: macOS.
- No telemetry in V1.

## Non-Goals (V1)

- Interactive-first workflows.
- Low-confidence asset inference.
- Cross-platform support guarantees beyond macOS.
- Any analytics/telemetry collection.
