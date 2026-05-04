# Spur CLI — Development Notes for Agents

## Build & Configuration (project-specific)

- Go toolchain: this repo is pinned to `go 1.23.1` in `go.mod`.
- Entry point: CLI binary entry is `main.go` (`cmd.Execute()`), with commands under `cmd/`.
- Quick local checks:

```bash
go build ./...
go run . --help
```

- The CLI contains interactive flows (`survey/v2`) in commands like `new`/`add`; for non-interactive validation prefer direct unit tests on helper functions.

## Testing: run, add, and verify

### Current state

- There are currently no committed `*_test.go` files in this repository.
- `go test ./...` currently surfaces a pre-existing vet issue in `cmd/status_auth.go` (redundant newline in `Println` argument list).

### Running tests in this repo

- For standard test execution (when vet is clean):

```bash
go test ./...
```

- For targeted test development while avoiding unrelated pre-existing vet failures:

```bash
go test -vet=off ./cmd -run TestName -count=1
```

### Adding new tests (recommended pattern)

- Place tests next to code (e.g., `cmd/foo_test.go` for `cmd/foo.go`) and keep package as `package cmd` unless black-box behavior is intentional.
- Prefer small tests on pure helpers (e.g., string/path transforms) over tests requiring interactive prompts.
- For command-level behavior, isolate deterministic internals first, then test wrappers separately.

### Verified example (executed)

The following exact flow was validated in this repository:

1. Created temporary test file `cmd/add_test.go` with `TestSanitise` covering `sanitise` from `cmd/add.go`.
2. Ran:

   ```bash
   go test -vet=off ./cmd -run TestSanitise -count=1
   ```

3. Result: pass (`ok github.com/ranakdinesh/spur-cli/cmd ...`).
4. Removed temporary test file after verification, per cleanup requirement.

## Code style & debugging notes

- Keep command UX consistent with existing style in `cmd/*`: colorized output (`fatih/color`), step-wise sections, and actionable next-step messages.
- Preserve existing error style: wrapped errors with `%w` for returned operational failures; short `fmt.Errorf("cancelled")` for user-abort paths.
- Follow existing helper layout pattern (small private functions below command handlers).
- When touching generator/template code in `internal/scaffold/scaffold.go`, verify literal template formatting carefully (many embedded markdown/code blocks).
- If tests fail unexpectedly, check for vet-triggered failures first (not only assertion failures), since `go test` runs vet by default in many scenarios.