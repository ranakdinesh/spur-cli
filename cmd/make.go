package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ranakdinesh/spur-cli/internal/config"
	"github.com/ranakdinesh/spur-cli/internal/scaffold"
)

func makeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "make",
		Short: "Scaffold components inside the current project",
		Long: `Scaffold domain modules, migrations, and handlers inside your project.

  spur make module <n>              Create a domain module in internal/modules/
  spur make migration <module> <n>  Add a numbered SQL migration file
  spur make handler <module> <entity>  Add a CRUD handler file`,
	}
	cmd.AddCommand(makeModuleCmd())
	cmd.AddCommand(makeMigrationCmd())
	cmd.AddCommand(makeHandlerCmd())
	return cmd
}

// ─── spur make module ────────────────────────────────────────────────────────

func makeModuleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "module <n>",
		Short: "Scaffold a domain module inside this project",
		Example: `  spur make module workforce
  spur make module proctoring
  spur make module catalog`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMakeModule(args[0])
		},
	}
}

func runMakeModule(name string) error {
	name = strings.ToLower(strings.TrimSpace(name))

	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)
	dim := color.New(color.FgHiBlack)

	// Must be inside a Spur project
	state, err := config.LoadProjectState()
	if err != nil {
		return err
	}

	// Check module doesn't already exist
	modPath := fmt.Sprintf("internal/modules/%s", name)
	if _, err := os.Stat(modPath); err == nil {
		return fmt.Errorf("module %q already exists at %s", name, modPath)
	}

	fmt.Println()
	bold.Printf("  Creating domain module: %s\n\n", color.CyanString(name))
	dim.Println("  This module lives INSIDE your project (your business logic)")
	dim.Println("  For a standalone spur-* library module, use: spur create module")
	fmt.Println()

	// Build scaffold data from project state
	data := scaffold.ProjectModuleData{
		Name:        name,
		GoModule:    state.GoModule,
		HasWS:       state.HasProtocol("websocket"),
		HasSSE:      state.HasProtocol("sse"),
		HasTemporal: state.HasProtocol("temporal"),
		HasQueue:    state.HasProtocol("queue"),
		HasGRPC:     state.HasProtocol("grpc"),
		HasI18n:     state.HasModule("i18n"),
	}

	// Create the module
	cyan.Printf("  → Scaffolding internal/modules/%s/...\n", name)
	if err := scaffold.CreateProjectModule(modPath, data); err != nil {
		return fmt.Errorf("scaffold module: %w", err)
	}
	green.Println("  ✓ All files created")

	// Update .spur.json
	state.AddModule(name)
	_ = config.SaveProjectState(state)
	green.Println("  ✓ .spur.json updated")

	// Print the complete file tree
	fmt.Println()
	dim.Printf("  internal/modules/%s/\n", name)
	tree := []struct{ indent, file string }{
		{"  ├── ", fmt.Sprintf("%s-module.go", name)},
		{"  ├── ", "core/"},
		{"  │   ├── ", "domain/domain.go"},
		{"  │   ├── ", "ports/ports.go"},
		{"  │   └── ", "services/service.go"},
		{"  ├── ", "adapters/"},
		{"  │   ├── ", "postgres/store.go"},
		{"  │   └── ", "httpx/"},
		{"  │       ├── ", "routes.go"},
		{"  │       └── ", "handlers/handler.go"},
		{"  └── ", "sql/"},
		{"      ├── ", "migrations/"},
		{"      │   ├── ", fmt.Sprintf("0001_%s_init.sql", name)},
		{"      │   └── ", "embed.go"},
		{"      └── ", "queries/"},
		{"          └── ", fmt.Sprintf("%s.sql", name)},
	}
	for _, row := range tree {
		dim.Printf("  %s%s\n", row.indent, row.file)
	}

	// Print wire instructions
	fmt.Println()
	bold.Printf("  ✅ %s module ready!\n\n", strings.ToUpper(name[:1])+name[1:])

	fmt.Println("  Wire into internal/app/app.go:")
	fmt.Println()
	fmt.Printf("    // After the SPUR:MODULES marker:\n")
	fmt.Printf("    %sMod, err := %s.New(ctx, %s.Options{\n", name, name, name)
	fmt.Printf("        DB:  dbPool,\n")
	fmt.Printf("        Log: log,\n")
	if data.HasWS {
		fmt.Printf("        WS:  infra.WS,\n")
	}
	if data.HasSSE {
		fmt.Printf("        SSE: infra.SSE,\n")
	}
	if data.HasTemporal {
		fmt.Printf("        Temporal: infra.Temporal,\n")
	}
	if data.HasQueue {
		fmt.Printf("        Queue: infra.Queue,\n")
	}
	fmt.Printf("    })\n")
	fmt.Printf("    if err != nil { return nil, fmt.Errorf(\"%s: %%w\", err) }\n", name)
	fmt.Printf("    identityModule.Services.ModuleService.RegisterManifest(ctx, %sMod.Manifest)\n", name)
	fmt.Println()
	fmt.Println("    // After the SPUR:ROUTES marker:")
	fmt.Printf("    %sMod.RegisterRoutes(r)\n", name)
	fmt.Println()

	fmt.Println("  Next steps:")
	fmt.Printf("  1. Add domain fields in     internal/modules/%s/core/domain/domain.go\n", name)
	fmt.Printf("  2. Add interfaces in        internal/modules/%s/core/ports/ports.go\n", name)
	fmt.Printf("  3. Write your DB schema in  internal/modules/%s/sql/migrations/0001_%s_init.sql\n", name, name)
	fmt.Printf("  4. Write sqlc queries in    internal/modules/%s/sql/queries/%s.sql\n", name, name)
	fmt.Printf("  5. Run: %s\n", color.CyanString("make sqlc"))
	fmt.Printf("  6. Implement store methods  adapters/postgres/store.go\n")
	fmt.Printf("  7. Wire into app.go (above)\n")
	fmt.Printf("  8. Run: %s\n", color.CyanString("go build ./..."))
	fmt.Println()

	return nil
}

// ─── spur make migration ─────────────────────────────────────────────────────

func makeMigrationCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migration <module> <n>",
		Short: "Create a numbered SQL migration file for a module",
		Example: `  spur make migration workforce add_employers_table
  spur make migration sessions add_violation_count
  spur make migration jobs add_priority_column`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMakeMigration(args[0], args[1])
		},
	}
}

func runMakeMigration(moduleName, migName string) error {
	migsDir := fmt.Sprintf("internal/modules/%s/sql/migrations", moduleName)

	if _, err := os.Stat(migsDir); os.IsNotExist(err) {
		return fmt.Errorf(
			"module %q not found at %s\n"+
				"Create it first: spur make module %s",
			moduleName, migsDir, moduleName,
		)
	}

	// Find next sequence number
	entries, _ := os.ReadDir(migsDir)
	next := 1
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			next++
		}
	}

	slug := strings.ReplaceAll(strings.ToLower(migName), " ", "_")
	fname := fmt.Sprintf("%s/%04d_%s_%s.sql", migsDir, next, moduleName, slug)

	content := fmt.Sprintf(
		"-- Migration: %s\n-- Module:    %s\n-- Created:   %s\n\n-- TODO: write your SQL\n",
		migName, moduleName, time.Now().Format("2006-01-02"),
	)

	if err := os.WriteFile(fname, []byte(content), 0644); err != nil {
		return fmt.Errorf("create migration: %w", err)
	}

	color.Green("\n  ✓ Created: %s\n\n", fname)
	fmt.Printf("  Open the file and write your SQL.\n")
	fmt.Printf("  Then run: %s\n\n", color.CyanString("make migrate"))
	return nil
}

// ─── spur make handler ───────────────────────────────────────────────────────

func makeHandlerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "handler <module> <entity>",
		Short: "Add a CRUD handler file to a module",
		Example: `  spur make handler workforce job
  spur make handler proctoring event
  spur make handler catalog product`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMakeHandler(args[0], args[1])
		},
	}
}

func runMakeHandler(moduleName, entityName string) error {
	state, err := config.LoadProjectState()
	if err != nil {
		return err
	}

	handlerDir := fmt.Sprintf("internal/modules/%s/adapters/httpx/handlers", moduleName)
	if _, err := os.Stat(handlerDir); os.IsNotExist(err) {
		return fmt.Errorf("module %q not found — run: spur make module %s", moduleName, moduleName)
	}

	handlerPath := fmt.Sprintf("%s/%s.go", handlerDir, entityName)
	if _, err := os.Stat(handlerPath); err == nil {
		return fmt.Errorf("handler %s already exists", handlerPath)
	}

	data := scaffold.ProjectModuleData{
		Name:     moduleName,
		GoModule: state.GoModule,
	}

	if err := scaffold.CreateHandlerFile(handlerPath, entityName, data); err != nil {
		return err
	}

	color.Green("\n  ✓ Created: %s\n\n", handlerPath)
	fmt.Printf("  Register it in: internal/modules/%s/adapters/httpx/routes.go\n\n", moduleName)
	return nil
}

