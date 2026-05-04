// Package scaffold generates module code from templates.
// It handles two completely different things:
//
//  1. Project domain modules (spur make module workforce)
//     → internal/modules/workforce/ inside your project
//     → your business logic, not shared
//
//  2. Standalone Spur library modules (spur create module notifications)
//     → a full spur-notifications/ directory
//     → ready to push to github.com/ranakdinesh/spur-notifications
//     → follows every contract: domain, ports, services, sqlc, httpx, MODULE.md, spur.json
package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// ─── Shared helpers ───────────────────────────────────────────────────────────

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func upper(s string) string { return strings.ToUpper(s) }

func write(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func render(tmpl string, data any) (string, error) {
	t, err := template.New("").Funcs(template.FuncMap{
		"title": title,
		"upper": upper,
	}).Parse(tmpl)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// PART 1: PROJECT DOMAIN MODULE
// spur make module <name>
// Creates internal/modules/<name>/ inside the current project.
// ─────────────────────────────────────────────────────────────────────────────

type ProjectModuleData struct {
	Name      string // e.g. "workforce"
	Title     string // e.g. "Workforce"
	GoModule  string // e.g. "github.com/ranakdinesh/nilex"
	HasWS     bool
	HasSSE    bool
	HasTemporal bool
	HasQueue  bool
	HasGRPC   bool
	HasI18n   bool
}

// CreateProjectModule scaffolds a domain module inside an existing Spur project.
func CreateProjectModule(base string, d ProjectModuleData) error {
	d.Title = title(d.Name)

	dirs := []string{
		"core/domain",
		"core/ports",
		"core/services",
		"adapters/postgres/sqlc",
		"adapters/httpx/handlers",
		"sql/migrations",
		"sql/queries",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(base, dir), 0755); err != nil {
			return err
		}
	}

	files := map[string]string{
		fmt.Sprintf("%s-module.go", d.Name): projectModuleGo,
		"core/domain/domain.go":             projectDomainGo,
		"core/ports/ports.go":               projectPortsGo,
		"core/services/service.go":          projectServiceGo,
		"adapters/postgres/store.go":        projectStoreGo,
		"adapters/httpx/routes.go":          projectRoutesGo,
		"adapters/httpx/handlers/handler.go": projectHandlerGo,
		"sql/migrations/embed.go":           projectEmbedGo,
		fmt.Sprintf("sql/migrations/0001_%s_init.sql", d.Name): projectMigrationSQL,
		fmt.Sprintf("sql/queries/%s.sql", d.Name):              projectQueriesSQL,
	}

	for relPath, tmpl := range files {
		content, err := render(tmpl, d)
		if err != nil {
			return fmt.Errorf("render %s: %w", relPath, err)
		}
		if err := write(filepath.Join(base, relPath), content); err != nil {
			return fmt.Errorf("write %s: %w", relPath, err)
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// PART 2: STANDALONE SPUR LIBRARY MODULE
// spur create module <name>
// Creates spur-<name>/ ready to push to github.com/ranakdinesh/spur-<name>
// ─────────────────────────────────────────────────────────────────────────────

type LibModuleData struct {
	Name        string // e.g. "notifications"
	Title       string // e.g. "Notifications"
	Description string // e.g. "Email and SMS delivery"
	Version     string // e.g. "1.0.0"
	GoPackage   string // e.g. "github.com/ranakdinesh/spur-notifications"
	Year        string // current year
}

// CreateLibModule creates a complete standalone spur-<name> module directory.
func CreateLibModule(base string, d LibModuleData) error {
	d.Title = title(d.Name)
	d.Version = "1.0.0"
	d.Year = fmt.Sprintf("%d", time.Now().Year())
	if d.GoPackage == "" {
		d.GoPackage = fmt.Sprintf("github.com/ranakdinesh/spur-%s", d.Name)
	}

	dirs := []string{
		"core/domain",
		"core/ports",
		"core/services",
		"adapters/postgres/sqlc",
		"adapters/httpx/handlers",
		"sql/migrations",
		"sql/queries",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(base, dir), 0755); err != nil {
			return err
		}
	}

	files := map[string]string{
		"go.mod":                              libGoMod,
		fmt.Sprintf("%s-module.go", d.Name):  libModuleGo,
		"core/domain/domain.go":              libDomainGo,
		"core/ports/ports.go":                libPortsGo,
		"core/services/service.go":           libServiceGo,
		"adapters/postgres/store.go":         libStoreGo,
		"adapters/httpx/routes.go":           libRoutesGo,
		"adapters/httpx/handlers/handler.go": libHandlerGo,
		"sql/migrations/embed.go":            libEmbedGo,
		fmt.Sprintf("sql/migrations/0001_%s_init.sql", d.Name): libMigrationSQL,
		fmt.Sprintf("sql/queries/%s.sql", d.Name):              libQueriesSQL,
		"spur.json":  libSpurJSON,
		"MODULE.md":  libModuleMD,
		"README.md":  libReadmeMD,
		".gitignore": libGitignore,
	}

	for relPath, tmpl := range files {
		content, err := render(tmpl, d)
		if err != nil {
			return fmt.Errorf("render %s: %w", relPath, err)
		}
		if err := write(filepath.Join(base, relPath), content); err != nil {
			return fmt.Errorf("write %s: %w", relPath, err)
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// PROJECT MODULE TEMPLATES
// ─────────────────────────────────────────────────────────────────────────────

var projectModuleGo = `package {{.Name}}

import (
	"context"
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"{{.GoModule}}/internal/modules/identity/core/domain"
	"{{.GoModule}}/internal/modules/{{.Name}}/adapters/httpx"
	"{{.GoModule}}/internal/modules/{{.Name}}/adapters/httpx/handlers"
	"{{.GoModule}}/internal/modules/{{.Name}}/adapters/postgres"
	"{{.GoModule}}/internal/modules/{{.Name}}/core/ports"
	"{{.GoModule}}/internal/modules/{{.Name}}/core/services"
	"{{.GoModule}}/internal/modules/{{.Name}}/sql/migrations"
	"{{.GoModule}}/internal/platform/logger"
{{- if .HasWS}}
	"{{.GoModule}}/internal/platform/server/websocket"
{{- end}}
{{- if .HasSSE}}
	"{{.GoModule}}/internal/platform/server/sse"
{{- end}}
{{- if .HasTemporal}}
	temporalclient "go.temporal.io/sdk/client"
{{- end}}
{{- if .HasQueue}}
	"{{.GoModule}}/internal/platform/queue"
{{- end}}
)

// Config holds {{.Title}}-specific configuration.
// Add fields here mapped from {{upper .Name}}_* env vars.
type Config struct{}

// Options is passed by app.go when wiring this module.
type Options struct {
	DB  *pgxpool.Pool
	Log *logger.Loggerx
	Cfg Config
{{- if .HasWS}}
	WS  *wsserver.Hub
{{- end}}
{{- if .HasSSE}}
	SSE *sseserver.Broker
{{- end}}
{{- if .HasTemporal}}
	Temporal *temporalclient.Client
{{- end}}
{{- if .HasQueue}}
	Queue *queue.Queue
{{- end}}
}

// Module is the {{.Title}} module entry point.
type Module struct {
	Manifest domain.Manifest
	svc      ports.{{.Title}}Service
	handlers *handlers.Handler
}

// New wires the {{.Title}} module. Returns error — never panics.
func New(ctx context.Context, opt Options) (*Module, error) {
	if opt.DB == nil {
		return nil, fmt.Errorf("{{.Name}}: DB pool is required")
	}

	// Run migrations
	// if err := opt.MigrationRunner(ctx, "{{.Name}}", migrations.FS); err != nil {
	// 	return nil, fmt.Errorf("{{.Name}}: migrations: %w", err)
	// }
	_ = migrations.FS // used by migration runner

	// Wire repos
	store := postgres.New(opt.DB)

	// Wire services
	svc := services.New{{.Title}}Service(store, opt.Log)

	// Wire handlers
	h := handlers.New(svc)

	manifest := domain.Manifest{
		Name:        "{{.Title}}",
		Code:        "{{.Name}}",
		Description: "{{.Title}} module",
		Permissions: []domain.ManifestPermission{
			{Slug: "{{.Name}}.read",   Description: "View {{.Name}} data"},
			{Slug: "{{.Name}}.write",  Description: "Create and update {{.Name}} data"},
			{Slug: "{{.Name}}.delete", Description: "Delete {{.Name}} data"},
		},
	}

	return &Module{
		Manifest: manifest,
		svc:      svc,
		handlers: h,
	}, nil
}

// RegisterRoutes mounts {{.Title}} HTTP routes on the root router.
func (m *Module) RegisterRoutes(r chi.Router) {
	httpx.RegisterRoutes(r, m.handlers)
}
`

var projectDomainGo = `package domain

import (
	"time"

	"github.com/google/uuid"
)

// {{.Title}} is the core domain entity.
// Add your business fields here.
type {{.Title}} struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time

	// TODO: add your domain fields
}

// New{{.Title}} creates a new {{.Title}} with validation.
func New{{.Title}}(tenantID uuid.UUID) (*{{.Title}}, error) {
	return &{{.Title}}{
		ID:        uuid.New(),
		TenantID:  tenantID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}
`

var projectPortsGo = `package ports

import (
	"context"

	"github.com/google/uuid"
	"{{.GoModule}}/internal/modules/{{.Name}}/core/domain"
)

// ─── Repository ───────────────────────────────────────────────────────────────

type {{.Title}}Repo interface {
	Create(ctx context.Context, e *domain.{{.Title}}) (*domain.{{.Title}}, error)
	GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.{{.Title}}, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]*domain.{{.Title}}, error)
	Delete(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error
}

// ─── Service ──────────────────────────────────────────────────────────────────

type {{.Title}}Service interface {
	Create(ctx context.Context, cmd Create{{.Title}}Cmd) (*domain.{{.Title}}, error)
	Get(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.{{.Title}}, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]*domain.{{.Title}}, error)
	Delete(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error
}

// ─── Commands ─────────────────────────────────────────────────────────────────

type Create{{.Title}}Cmd struct {
	TenantID  uuid.UUID
	CreatedBy uuid.UUID
	// TODO: add command fields
}
`

var projectServiceGo = `package services

import (
	"context"

	"github.com/google/uuid"
	"{{.GoModule}}/internal/modules/{{.Name}}/core/domain"
	"{{.GoModule}}/internal/modules/{{.Name}}/core/ports"
	"{{.GoModule}}/internal/platform/logger"
)

type {{.Title}}Service struct {
	repo ports.{{.Title}}Repo
	log  *logger.Loggerx
}

func New{{.Title}}Service(repo ports.{{.Title}}Repo, log *logger.Loggerx) *{{.Title}}Service {
	return &{{.Title}}Service{repo: repo, log: log}
}

func (s *{{.Title}}Service) Create(ctx context.Context, cmd ports.Create{{.Title}}Cmd) (*domain.{{.Title}}, error) {
	entity, err := domain.New{{.Title}}(cmd.TenantID)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, entity)
}

func (s *{{.Title}}Service) Get(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.{{.Title}}, error) {
	return s.repo.GetByID(ctx, id, tenantID)
}

func (s *{{.Title}}Service) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.{{.Title}}, error) {
	return s.repo.List(ctx, tenantID)
}

func (s *{{.Title}}Service) Delete(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error {
	return s.repo.Delete(ctx, id, tenantID)
}
`

var projectStoreGo = `package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"{{.GoModule}}/internal/modules/{{.Name}}/core/domain"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Create(ctx context.Context, e *domain.{{.Title}}) (*domain.{{.Title}}, error) {
	// TODO: implement using sqlc generated functions after running make sqlc
	// Example:
	// row, err := s.q(ctx).Create{{.Title}}(ctx, sqlc.Create{{.Title}}Params{...})
	return e, nil
}

func (s *Store) GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.{{.Title}}, error) {
	// TODO: implement
	return nil, nil
}

func (s *Store) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.{{.Title}}, error) {
	// TODO: implement
	return []*domain.{{.Title}}{}, nil
}

func (s *Store) Delete(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error {
	// TODO: implement
	return nil
}
`

var projectRoutesGo = `package httpx

import (
	"github.com/go-chi/chi/v5"
	"{{.GoModule}}/internal/modules/{{.Name}}/adapters/httpx/handlers"
)

func RegisterRoutes(r chi.Router, h *handlers.Handler) {
	r.Route("/{{.Name}}", func(r chi.Router) {
		r.Get("/",      h.List)
		r.Post("/",     h.Create)
		r.Get("/{id}",  h.Get)
		r.Delete("/{id}", h.Delete)
	})
}
`

var projectHandlerGo = `package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"{{.GoModule}}/internal/modules/{{.Name}}/core/ports"
	"{{.GoModule}}/internal/platform/httpserver"
)

type Handler struct {
	svc ports.{{.Title}}Service
}

func New(svc ports.{{.Title}}Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := uuid.MustParse(httpserver.GetTenantID(r.Context()))
	items, err := h.svc.List(r.Context(), tenantID)
	if err != nil {
		respondError(w, 500, "failed to list")
		return
	}
	respondJSON(w, 200, items)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := uuid.MustParse(httpserver.GetTenantID(r.Context()))
	userID   := uuid.MustParse(httpserver.GetUserID(r.Context()))

	// TODO: decode your specific request body fields
	cmd := ports.Create{{.Title}}Cmd{
		TenantID:  tenantID,
		CreatedBy: userID,
	}
	item, err := h.svc.Create(r.Context(), cmd)
	if err != nil {
		respondError(w, 500, "failed to create")
		return
	}
	respondJSON(w, 201, item)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID := uuid.MustParse(httpserver.GetTenantID(r.Context()))
	id       := uuid.MustParse(chi.URLParam(r, "id"))

	item, err := h.svc.Get(r.Context(), id, tenantID)
	if err != nil || item == nil {
		respondError(w, 404, "not found")
		return
	}
	respondJSON(w, 200, item)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID := uuid.MustParse(httpserver.GetTenantID(r.Context()))
	id       := uuid.MustParse(chi.URLParam(r, "id"))

	if err := h.svc.Delete(r.Context(), id, tenantID); err != nil {
		respondError(w, 500, "failed to delete")
		return
	}
	w.WriteHeader(204)
}

func respondJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
`

var projectEmbedGo = `package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
`

var projectMigrationSQL = `-- {{.Title}} module — initial migration
-- Schema: public (never identity.*)
-- Convention: id UUID, tenant_id UUID, created_at TIMESTAMPTZ

CREATE TABLE IF NOT EXISTS {{.Name}}s (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()

    -- TODO: add your domain columns here
);

CREATE INDEX IF NOT EXISTS idx_{{.Name}}s_tenant ON {{.Name}}s(tenant_id);

-- Auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END; $$;

CREATE TRIGGER {{.Name}}s_updated_at
    BEFORE UPDATE ON {{.Name}}s
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
`

var projectQueriesSQL = `-- {{.Title}} queries
-- Run: make sqlc  after adding queries here

-- name: Create{{.Title}} :one
INSERT INTO {{.Name}}s (id, tenant_id)
VALUES ($1, $2)
RETURNING *;

-- name: Get{{.Title}}ByID :one
SELECT * FROM {{.Name}}s
WHERE id = $1 AND tenant_id = $2;

-- name: List{{.Title}}sByTenant :many
SELECT * FROM {{.Name}}s
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: Delete{{.Title}} :exec
DELETE FROM {{.Name}}s
WHERE id = $1 AND tenant_id = $2;
`

// ─────────────────────────────────────────────────────────────────────────────
// STANDALONE LIBRARY MODULE TEMPLATES
// ─────────────────────────────────────────────────────────────────────────────

var libGoMod = `module {{.GoPackage}}

go 1.23.1

require (
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.5.5
	github.com/go-chi/chi/v5 v5.0.12
	github.com/ranakdinesh/spur-platform v1.0.0
)
`

var libModuleGo = `// Package {{.Name}} provides the {{.Title}} Spur module.
//
// Install:
//   spur add module {{.Name}}
//
// Or manually:
//   go get {{.GoPackage}}@latest
//
// Wire in app.go:
//   {{.Name}}Module, err := {{.Name}}.New(ctx, {{.Name}}.Options{DB: dbPool, Log: log, Cfg: cfg.{{.Title}}})
//   {{.Name}}Module.RegisterRoutes(r)
package {{.Name}}

import (
	"context"
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ranakdinesh/spur-platform/logger"
	"{{.GoPackage}}/adapters/httpx"
	"{{.GoPackage}}/adapters/httpx/handlers"
	"{{.GoPackage}}/adapters/postgres"
	"{{.GoPackage}}/core/ports"
	"{{.GoPackage}}/core/services"
	"{{.GoPackage}}/sql/migrations"
)

// Config holds all {{.Title}} configuration from environment variables.
type Config struct {
	// TODO: add your config fields
	// Example:
	// APIKey string ` + "`" + `env:"{{upper .Name}}_API_KEY"` + "`" + `
}

// Options is passed by app.go when constructing this module.
type Options struct {
	DB  *pgxpool.Pool
	Log *logger.Loggerx
	Cfg Config

	// MigrationRunner runs this module's SQL migrations.
	// Provided by the platform: infra.Migrations.Run
	MigrationRunner func(ctx context.Context, moduleName string, fs interface{}) error
}

// Module is the {{.Title}} module entry point.
type Module struct {
	// Services exposes this module's service interfaces to other modules.
	Services *Services

	handler *handlers.Handler
}

// Services bundles the public service interfaces.
type Services struct {
	{{.Title}} ports.{{.Title}}Service
}

// New wires the {{.Title}} module. Returns error — never panics.
func New(ctx context.Context, opt Options) (*Module, error) {
	if opt.DB == nil {
		return nil, fmt.Errorf("{{.Name}}: DB pool is required")
	}

	// Run migrations
	if opt.MigrationRunner != nil {
		if err := opt.MigrationRunner(ctx, "{{.Name}}", migrations.FS); err != nil {
			return nil, fmt.Errorf("{{.Name}}: migrations: %w", err)
		}
	}

	// Wire repo → service → handler
	store := postgres.New(opt.DB)
	svc   := services.New{{.Title}}Service(store, opt.Log)
	h     := handlers.New(svc)

	opt.Log.Info(ctx).Str("module", "{{.Name}}").Msg("{{.Title}} module initialised")

	return &Module{
		Services: &Services{{{.Title}}: svc},
		handler:  h,
	}, nil
}

// RegisterRoutes mounts {{.Title}} HTTP routes on the root router.
func (m *Module) RegisterRoutes(r chi.Router) {
	httpx.RegisterRoutes(r, m.handler)
}
`

var libDomainGo = `package domain

import (
	"time"

	"github.com/google/uuid"
)

// {{.Title}} is the core domain entity for the {{.Name}} module.
// Add your business fields below.
type {{.Title}} struct {
	ID        uuid.UUID ` + "`" + `json:"id"` + "`" + `
	TenantID  uuid.UUID ` + "`" + `json:"tenant_id"` + "`" + `
	CreatedAt time.Time ` + "`" + `json:"created_at"` + "`" + `
	UpdatedAt time.Time ` + "`" + `json:"updated_at"` + "`" + `

	// TODO: add your domain fields
}

// New{{.Title}} is the factory function. Validates inputs and returns a ready entity.
func New{{.Title}}(tenantID uuid.UUID) (*{{.Title}}, error) {
	now := time.Now().UTC()
	return &{{.Title}}{
		ID:        uuid.New(),
		TenantID:  tenantID,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
`

var libPortsGo = `package ports

import (
	"context"

	"github.com/google/uuid"
	"{{.GoPackage}}/core/domain"
)

// ─── Repository ───────────────────────────────────────────────────────────────

type {{.Title}}Repo interface {
	Create(ctx context.Context, e *domain.{{.Title}}) (*domain.{{.Title}}, error)
	GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.{{.Title}}, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]*domain.{{.Title}}, error)
	Delete(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error
}

// ─── Service ──────────────────────────────────────────────────────────────────

type {{.Title}}Service interface {
	Create(ctx context.Context, cmd Create{{.Title}}Cmd) (*domain.{{.Title}}, error)
	Get(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.{{.Title}}, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]*domain.{{.Title}}, error)
	Delete(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error
}

// ─── Commands ─────────────────────────────────────────────────────────────────

type Create{{.Title}}Cmd struct {
	TenantID  uuid.UUID ` + "`" + `json:"tenant_id"` + "`" + `
	CreatedBy uuid.UUID ` + "`" + `json:"created_by"` + "`" + `
	// TODO: add your command fields
}
`

var libServiceGo = `package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/ranakdinesh/spur-platform/logger"
	"{{.GoPackage}}/core/domain"
	"{{.GoPackage}}/core/ports"
)

type {{.Title}}Service struct {
	repo ports.{{.Title}}Repo
	log  *logger.Loggerx
}

func New{{.Title}}Service(repo ports.{{.Title}}Repo, log *logger.Loggerx) *{{.Title}}Service {
	return &{{.Title}}Service{repo: repo, log: log}
}

func (s *{{.Title}}Service) Create(ctx context.Context, cmd ports.Create{{.Title}}Cmd) (*domain.{{.Title}}, error) {
	entity, err := domain.New{{.Title}}(cmd.TenantID)
	if err != nil {
		return nil, err
	}
	result, err := s.repo.Create(ctx, entity)
	if err != nil {
		return nil, err
	}
	s.log.Info(ctx).Str("id", result.ID.String()).Msg("{{.Name}}: created")
	return result, nil
}

func (s *{{.Title}}Service) Get(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.{{.Title}}, error) {
	return s.repo.GetByID(ctx, id, tenantID)
}

func (s *{{.Title}}Service) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.{{.Title}}, error) {
	return s.repo.List(ctx, tenantID)
}

func (s *{{.Title}}Service) Delete(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error {
	if err := s.repo.Delete(ctx, id, tenantID); err != nil {
		return err
	}
	s.log.Info(ctx).Str("id", id.String()).Msg("{{.Name}}: deleted")
	return nil
}
`

var libStoreGo = `package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"{{.GoPackage}}/core/domain"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Create(ctx context.Context, e *domain.{{.Title}}) (*domain.{{.Title}}, error) {
	// TODO: use sqlc generated function after running: sqlc generate
	// Example:
	// q := sqlc.New(s.pool)
	// row, err := q.Create{{.Title}}(ctx, sqlc.Create{{.Title}}Params{
	//     ID:       e.ID,
	//     TenantID: e.TenantID,
	// })
	// if err != nil { return nil, fmt.Errorf("{{.Name}}: create: %w", err) }
	// return mapRow(row), nil
	_ = fmt.Sprintf // suppress import error
	return e, nil
}

func (s *Store) GetByID(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) (*domain.{{.Title}}, error) {
	// TODO: implement
	return nil, nil
}

func (s *Store) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.{{.Title}}, error) {
	// TODO: implement
	return []*domain.{{.Title}}{}, nil
}

func (s *Store) Delete(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error {
	// TODO: implement
	return nil
}
`

var libRoutesGo = `package httpx

import (
	"github.com/go-chi/chi/v5"
	"{{.GoPackage}}/adapters/httpx/handlers"
)

// RegisterRoutes mounts {{.Title}} HTTP routes.
// Route prefix ("/{{.Name}}") is set by the application in app.go.
func RegisterRoutes(r chi.Router, h *handlers.Handler) {
	r.Route("/{{.Name}}", func(r chi.Router) {
		// Public routes (if any) go before the auth group
		// r.Get("/public", h.PublicEndpoint)

		// Protected routes (JWT required — applied by platform middleware)
		r.Get("/",        h.List)
		r.Post("/",       h.Create)
		r.Get("/{id}",   h.Get)
		r.Delete("/{id}", h.Delete)
	})
}
`

var libHandlerGo = `package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ranakdinesh/spur-platform/httpserver"
	"{{.GoPackage}}/core/ports"
)

// Handler handles all HTTP requests for the {{.Title}} module.
type Handler struct {
	svc ports.{{.Title}}Service
}

func New(svc ports.{{.Title}}Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := uuid.MustParse(httpserver.GetTenantID(r.Context()))

	items, err := h.svc.List(r.Context(), tenantID)
	if err != nil {
		respondError(w, 500, "failed to list")
		return
	}
	respondJSON(w, 200, items)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := uuid.MustParse(httpserver.GetTenantID(r.Context()))
	userID   := uuid.MustParse(httpserver.GetUserID(r.Context()))

	cmd := ports.Create{{.Title}}Cmd{
		TenantID:  tenantID,
		CreatedBy: userID,
	}
	// TODO: decode additional fields from r.Body into cmd

	item, err := h.svc.Create(r.Context(), cmd)
	if err != nil {
		respondError(w, 500, "failed to create")
		return
	}
	respondJSON(w, 201, item)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID := uuid.MustParse(httpserver.GetTenantID(r.Context()))
	id       := uuid.MustParse(chi.URLParam(r, "id"))

	item, err := h.svc.Get(r.Context(), id, tenantID)
	if err != nil || item == nil {
		respondError(w, 404, "not found")
		return
	}
	respondJSON(w, 200, item)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID := uuid.MustParse(httpserver.GetTenantID(r.Context()))
	id       := uuid.MustParse(chi.URLParam(r, "id"))

	if err := h.svc.Delete(r.Context(), id, tenantID); err != nil {
		respondError(w, 500, "failed to delete")
		return
	}
	w.WriteHeader(204)
}

func respondJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
`

var libEmbedGo = `package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
`

var libMigrationSQL = `-- {{.Title}} module — initial migration
-- Schema: public
-- Convention: id UUID PK, tenant_id UUID NOT NULL, timestamps

CREATE TABLE IF NOT EXISTS {{.Name}}s (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()

    -- TODO: add your domain columns
    -- Example:
    -- name        TEXT        NOT NULL,
    -- status      TEXT        NOT NULL DEFAULT 'active',
    -- metadata    JSONB       NOT NULL DEFAULT '{}'
);

-- Required: index on tenant_id — every query filters by it
CREATE INDEX IF NOT EXISTS idx_{{.Name}}s_tenant ON {{.Name}}s(tenant_id);

-- Optional: auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END; $$;

CREATE TRIGGER {{.Name}}s_updated_at
    BEFORE UPDATE ON {{.Name}}s
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
`

var libQueriesSQL = `-- {{.Title}} sqlc queries
-- Run: sqlc generate  after adding queries

-- name: Create{{.Title}} :one
INSERT INTO {{.Name}}s (id, tenant_id)
VALUES ($1, $2)
RETURNING *;

-- name: Get{{.Title}}ByID :one
SELECT * FROM {{.Name}}s
WHERE id = $1 AND tenant_id = $2;

-- name: List{{.Title}}sByTenant :many
SELECT * FROM {{.Name}}s
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: Delete{{.Title}} :exec
DELETE FROM {{.Name}}s
WHERE id = $1 AND tenant_id = $2;
`

var libSpurJSON = `{
  "name": "{{.Name}}",
  "version": "{{.Version}}",
  "description": "{{.Description}}",
  "go_package": "{{.GoPackage}}",
  "private": true,
  "status": "alpha",
  "requires": ["http", "db"],
  "required_env": [],
  "optional_env": [],
  "has_sqlc": true,
  "config_struct": "{{.Title}}Config",
  "config_env_prefix": "{{upper .Name}}_",
  "app_field": "{{.Title}} *{{.Name}}.Module",
  "wire_code": {
    "import": "{{.Name}} \"{{.GoPackage}}\"",
    "init": "{{.Name}}Module, err := {{.Name}}.New(ctx, {{.Name}}.Options{DB: dbPool, Log: log, Cfg: cfg.{{.Title}}, MigrationRunner: infra.Migrations.Run})\nif err != nil { return nil, fmt.Errorf(\"{{.Name}}: %w\", err) }\nidentityModule.Services.ModuleService.RegisterManifest(ctx, domain.Manifest{Name: \"{{.Title}}\", Code: \"{{.Name}}\"})",
    "routes": "{{.Name}}Module.RegisterRoutes(r)"
  }
}
`

var libModuleMD = `# spur-{{.Name}} — {{.Title}} Module

{{.Description}}

---

## Install

` + "```bash" + `
spur add module {{.Name}}
` + "```" + `

Or manually:
` + "```bash" + `
go get {{.GoPackage}}@latest
` + "```" + `

---

## Wire into app.go

` + "```go" + `
import {{.Name}} "{{.GoPackage}}"

{{.Name}}Module, err := {{.Name}}.New(ctx, {{.Name}}.Options{
    DB:              dbPool,
    Log:             log,
    Cfg:             cfg.{{.Title}},
    MigrationRunner: infra.Migrations.Run,
})
if err != nil {
    return nil, fmt.Errorf("{{.Name}}: %w", err)
}
{{.Name}}Module.RegisterRoutes(r)
` + "```" + `

---

## Configuration

` + "```bash" + `
# deployments/.env
# TODO: add your env vars here
# {{upper .Name}}_API_KEY=
` + "```" + `

---

## HTTP Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET    | /{{.Name}}       | List all (tenant-scoped) |
| POST   | /{{.Name}}       | Create new |
| GET    | /{{.Name}}/{id}  | Get by ID |
| DELETE | /{{.Name}}/{id}  | Delete |

---

## Using in Another Module

` + "```go" + `
// Inject via Options
type Options struct {
    {{.Title}}Svc {{.Name}}.ports.{{.Title}}Service
}
` + "```" + `

---

## Development

` + "```bash" + `
# Generate sqlc code after editing sql/queries/
sqlc generate

# Run tests
go test ./...

# Build check
go build ./...
` + "```" + `

---

## Adding to the Registry

After pushing to GitHub, add an entry to
` + "`" + `github.com/ranakdinesh/spur-registry/modules.json` + "`" + `.
Copy the contents of ` + "`" + `spur.json` + "`" + ` in this repo.
`

var libReadmeMD = `# spur-{{.Name}}

{{.Description}}

Part of the [Spur platform](https://github.com/ranakdinesh/spur-template).

## Quick Start

` + "```bash" + `
spur add module {{.Name}}
` + "```" + `

See [MODULE.md](MODULE.md) for full documentation.

## License

MIT © {{.Year}} ranakdinesh
`

var libGitignore = `# Binaries
*.exe
*.test
vendor/

# Keys
*.pem
*.key

# IDE
.idea/
.vscode/
*.swp
.DS_Store
`

// ─── Handler-only scaffold ────────────────────────────────────────────────────

// CreateHandlerFile creates a single handler file for an entity in an existing module.
func CreateHandlerFile(path, entityName string, d ProjectModuleData) error {
	content, err := render(singleHandlerGo, struct {
		Entity    string
		EntityT   string
		Module    string
		ModuleT   string
		GoModule  string
	}{
		Entity:  entityName,
		EntityT: title(entityName),
		Module:  d.Name,
		ModuleT: title(d.Name),
		GoModule: d.GoModule,
	})
	if err != nil {
		return err
	}
	return write(path, content)
}

var singleHandlerGo = `package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"{{.GoModule}}/internal/platform/httpserver"
)

type {{.EntityT}}Handler struct{}

func New{{.EntityT}}Handler() *{{.EntityT}}Handler {
	return &{{.EntityT}}Handler{}
}

func (h *{{.EntityT}}Handler) List(w http.ResponseWriter, r *http.Request) {
	tenantID := uuid.MustParse(httpserver.GetTenantID(r.Context()))
	_ = tenantID
	// TODO: call service
	respondJSON(w, 200, []any{})
}

func (h *{{.EntityT}}Handler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID := uuid.MustParse(httpserver.GetTenantID(r.Context()))
	userID   := uuid.MustParse(httpserver.GetUserID(r.Context()))
	_, _ = tenantID, userID
	// TODO: decode request body, call service
	respondJSON(w, 201, map[string]string{"status": "created"})
}

func (h *{{.EntityT}}Handler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID := uuid.MustParse(httpserver.GetTenantID(r.Context()))
	id       := uuid.MustParse(chi.URLParam(r, "id"))
	_, _ = tenantID, id
	// TODO: call service
	respondJSON(w, 200, map[string]any{})
}

func (h *{{.EntityT}}Handler) Delete(w http.ResponseWriter, r *http.Request) {
	tenantID := uuid.MustParse(httpserver.GetTenantID(r.Context()))
	id       := uuid.MustParse(chi.URLParam(r, "id"))
	_, _ = tenantID, id
	// TODO: call service
	w.WriteHeader(204)
}
`
