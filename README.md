# Spur CLI

Spur CLI is the project bootstrap and acceleration layer for a Go-based backend platform (Supabase-alternative direction).
It is designed to help create AI-enabled web services in hours instead of weeks by automating project creation, protocol setup, and module workflows.

## Product Intent (from your initial narrative)

- Create a new project quickly from `spur-template` using `spur new <projectname>`.
- Add reusable modules (for example `spur-identity`) from GitHub using `spur add module <name>`.
- Enable protocols/services like HTTP, gRPC, WebSocket, HLS, RTMP, Temporal, etc.
- Scaffold new modules with boilerplate + docs so they integrate with the ecosystem cleanly.

## Current Architecture (code-level map)

- Entrypoint: `main.go` → `cmd.Execute()`
- Command registry: `cmd/root.go`
- Core commands:
  - `cmd/new.go` → `spur new`
  - `cmd/add.go` → `spur add protocol`, `spur add module`
  - `cmd/make.go` → project-local scaffolding (`spur make ...`)
  - `cmd/create.go` → standalone module repo scaffolding (`spur create module ...`)
  - `cmd/status_auth.go` → `spur status`, `spur auth`
- Shared internals:
  - `internal/config/config.go` → CLI config + `.spur.json` state
  - `internal/protocol/protocols.go` → protocol catalog/metadata
  - `internal/scaffold/scaffold.go` → file/template generation
  - `internal/agent/agent.go` → `GEMINI.md` + `SPUR.md` regeneration

## Functional Analysis and Coverage

### 1) Project bootstrap: `spur new <projectname>`

Status: **Covered and implemented** in `cmd/new.go`.

What it currently does:
- Uses interactive prompts for:
  - Go module path
  - Protocol selection (`survey.MultiSelect`)
  - Final confirmation
- Clones template repo configured in CLI config (`TemplateRepo`).
- Rewrites module path references in generated project.
- Generates protocol-aware files:
  - `deployments/.env.example` (and copies to `.env`)
  - `deployments/docker-compose.yml`
- Generates RSA key (`keys/private.pem`) using `openssl`.
- Writes project state to `.spur.json`.
- Generates `GEMINI.md` and `SPUR.md`.
- Initializes git in the generated project directory.

Notes:
- Interactive UX is intact (survey prompts are central to this command).
- Protocol dependencies are auto-added during selection when required.

### 2) Add protocol: `spur add protocol [name]`

Status: **Covered and implemented** in `cmd/add.go`.

What it currently does:
- Supports interactive mode when no protocol arg is provided.
- Validates protocol IDs against `internal/protocol` catalog.
- Prevents duplicate protocol installation.
- Auto-adds required dependent protocols.
- Updates project artifacts:
  - `deployments/.env.example`
  - `deployments/docker-compose.yml` (when services are required)
- Updates `.spur.json` and protocol port map.
- Regenerates `GEMINI.md` / `SPUR.md`.

### 3) Add external module: `spur add module <name>`

Status: **Covered and implemented** in `cmd/add.go`.

What it currently does:
- Validates module is not already present in `.spur.json`.
- Loads CLI config and PAT.
- Configures private Go module access (`GOPRIVATE`, `GONOSUMCHECK`) and `.netrc` (if PAT provided).
- Executes `go get github.com/ranakdinesh/spur-<name>@latest`.
- Finds module directory via `go list -m -f {{.Dir}}`.
- Reads module `spur.json` manifest and wires code into `internal/app/app.go` using markers:
  - `// SPUR:IMPORTS:END`
  - `// SPUR:MODULES:END`
  - `// SPUR:ROUTES:END`
  - `// SPUR:APP_VALUES:END`
- Appends required/optional env keys to `deployments/.env.example`.
- Runs `go mod tidy`.
- Attempts `make migrate` automatically if `Makefile` exists; on failure shows warning and fallback instruction.
- Updates `.spur.json`, regenerates `GEMINI.md` / `SPUR.md`.

Important caveat:
- Auto-wiring requires the target template project to contain the expected SPUR markers in `internal/app/app.go`.

### 4) Create standalone module repo: `spur create module <name>`

Status: **Covered and implemented** in `cmd/create.go` + `internal/scaffold/scaffold.go`.

What it currently does:
- Interactive prompts for description and Go module path.
- Scaffolds full `spur-<name>` repository structure including:
  - module entry file
  - core domain/ports/services
  - adapters (postgres/http)
  - SQL migrations and queries
  - `spur.json` manifest
  - `MODULE.md`, `README.md`, `.gitignore`
- Initializes git and prints stepwise follow-up instructions.

### 5) Create project-local domain module: `spur make module <name>`

Status: **Covered and implemented** in `cmd/make.go` + `internal/scaffold/scaffold.go`.

What it currently does:
- Validates current directory is a Spur project via `.spur.json`.
- Scaffolds under `internal/modules/<name>/`.
- Shape of generated code is protocol-aware (ws/sse/grpc/temporal/queue flags from state).
- Updates `.spur.json` and prints wiring instructions for `internal/app/app.go`.

### 6) Auth + status flows

Status: **Covered and implemented** in `cmd/status_auth.go`.

- `spur auth`:
  - Collects PAT interactively (`survey.Password`)
  - Verifies PAT against GitHub API (`/user`)
  - Persists PAT in `~/.spur/config.json`
  - Sets Go private module env (`go env -w ...`)
- `spur status`:
  - Reads `.spur.json`
  - Displays enabled protocols and installed modules
  - Shows quick next commands

## Task Checklist for Future Changes (safe-change protocol)

Use this checklist before and after touching code in this repo.

1. Preserve interactive UX in `new`, `add`, `create`, and `auth` flows.
2. Keep `.spur.json` as the source of project capability state.
3. Keep protocol metadata centralized in `internal/protocol/protocols.go`.
4. Ensure `agent.Regenerate(...)` remains called after state-affecting operations.
5. For `add module`, preserve marker-based idempotent wiring behavior.
6. For template/scaffold edits, verify generated file formatting carefully.
7. Keep error style consistent (`%w` for operational wrapping, short cancellation messages).
8. When validating locally, prefer:
   - `go build ./...`
   - `go run . --help`
   - targeted tests (if added)

## Quick Verification Commands

```bash
go build ./...
go run . --help
```

## Known Boundaries / Risks

- This repo intentionally relies on interactive prompts for key commands.
- External module install depends on GitHub/PAT and private module access config.
- Wiring assumes template compatibility (SPUR markers in target files).
- There are currently no committed unit tests in this repository.

---

This document is meant to be the baseline reference before making future code changes, so we can move safely without breaking existing interactive behavior or module wiring.