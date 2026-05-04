package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ranakdinesh/spur-cli/internal/scaffold"
)

// createCmd handles `spur create module <n>`
// This creates a STANDALONE spur-* library module — NOT a project domain module.
func createCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new standalone Spur module (spur-* library)",
		Long: `Create a standalone spur-* module repository.

This is for building NEW infrastructure modules that can be
downloaded and used by any Spur project via: spur add module <n>

For business logic inside your project, use: spur make module <n>

Examples:
  spur create module notifications   → creates spur-notifications/
  spur create module billing         → creates spur-billing/
  spur create module storage         → creates spur-storage/`,
	}
	cmd.AddCommand(createModuleCmd())
	return cmd
}

func createModuleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "module <n>",
		Short: "Create a standalone spur-<n> module repository",
		Long: `Creates a complete spur-<n> module directory ready to push to GitHub.

The generated module includes:
  - go.mod with correct package path
  - module entry point (New, RegisterRoutes, Services)
  - Domain types, port interfaces, service implementation
  - Postgres store (ready for sqlc)
  - HTTP handlers with correct patterns
  - SQL migration and query files
  - spur.json (CLI manifest for spur add module)
  - MODULE.md (usage documentation)
  - README.md
  - .gitignore`,
		Example: `  spur create module notifications
  spur create module billing
  spur create module audit
  spur create module video`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreateModule(args[0])
		},
	}
}

func runCreateModule(name string) error {
	name = strings.ToLower(strings.TrimSpace(name))

	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)
	dim := color.New(color.FgHiBlack)
	yellow := color.New(color.FgYellow)

	// ── Banner ────────────────────────────────────────────────────────────
	fmt.Println()
	bold.Println("  ╔═══════════════════════════════════════════════╗")
	bold.Printf("  ║   Creating Spur Module: spur-%-16s║\n", name)
	bold.Println("  ╚═══════════════════════════════════════════════╝")
	fmt.Println()
	dim.Println("  This creates a STANDALONE library module")
	dim.Println("  Push it to GitHub → users install with: spur add module " + name)
	fmt.Println()

	// ── Check target directory ────────────────────────────────────────────
	targetDir := fmt.Sprintf("spur-%s", name)
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("directory %q already exists", targetDir)
	}

	// ── Ask for description ───────────────────────────────────────────────
	var description string
	if err := survey.AskOne(&survey.Input{
		Message: "One-line description of this module?",
		Default: fmt.Sprintf("%s support for Spur projects", toTitle(name)),
		Help:    "Shown in `spur list`. Keep it under 60 characters.",
	}, &description, survey.WithValidator(survey.Required)); err != nil {
		return fmt.Errorf("cancelled")
	}

	// ── Ask for Go package path ───────────────────────────────────────────
	var goPkg string
	if err := survey.AskOne(&survey.Input{
		Message: "Go module path?",
		Default: fmt.Sprintf("github.com/ranakdinesh/spur-%s", name),
	}, &goPkg, survey.WithValidator(survey.Required)); err != nil {
		return fmt.Errorf("cancelled")
	}

	// ── Confirm ───────────────────────────────────────────────────────────
	fmt.Println()
	bold.Println("  Summary:")
	fmt.Printf("  Directory:   %s/\n", targetDir)
	fmt.Printf("  Package:     %s\n", goPkg)
	fmt.Printf("  Description: %s\n", description)
	fmt.Println()

	var proceed bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Create module?",
		Default: true,
	}, &proceed); err != nil || !proceed {
		fmt.Println("  Cancelled.")
		return nil
	}

	// ── Scaffold ──────────────────────────────────────────────────────────
	fmt.Println()
	cyan.Printf("  → Scaffolding %s/...\n", targetDir)

	data := scaffold.LibModuleData{
		Name:        name,
		Description: description,
		GoPackage:   goPkg,
	}

	if err := scaffold.CreateLibModule(targetDir, data); err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}
	green.Println("  ✓ All files created")

	// ── Print file tree ───────────────────────────────────────────────────
	fmt.Println()
	dim.Printf("  %s/\n", targetDir)
	tree := []struct{ indent, file string }{
		{"  ├── ", "go.mod"},
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
		{"  ├── ", "sql/"},
		{"  │   ├── ", "migrations/"},
		{"  │   │   ├── ", fmt.Sprintf("0001_%s_init.sql", name)},
		{"  │   │   └── ", "embed.go"},
		{"  │   └── ", "queries/"},
		{"  │       └── ", fmt.Sprintf("%s.sql", name)},
		{"  ├── ", "spur.json   ← CLI manifest"},
		{"  ├── ", "MODULE.md   ← usage documentation"},
		{"  ├── ", "README.md"},
		{"  └── ", ".gitignore"},
	}
	for _, row := range tree {
		dim.Printf("  %s%s\n", row.indent, row.file)
	}

	// ── Git init ──────────────────────────────────────────────────────────
	fmt.Println()
	cyan.Println("  → Initialising git repository...")
	for _, args := range [][]string{
		{"init"},
		{"add", "."},
		{"commit", "-m", fmt.Sprintf("chore: init spur-%s module", name)},
	} {
		c := exec.Command("git", args...)
		c.Dir = targetDir
		_ = c.Run()
	}
	green.Println("  ✓ Git repository initialised")

	// ── Done ─────────────────────────────────────────────────────────────
	fmt.Println()
	bold.Printf("  ✅ spur-%s is ready!\n\n", name)

	yellow.Println("  ── Build your module ────────────────────────────────")
	fmt.Println()
	fmt.Printf("  %s\n", bold.Sprint("1. Define your domain"))
	fmt.Printf("     %s\n", dim.Sprintf("core/domain/domain.go — add your entity fields"))
	fmt.Println()
	fmt.Printf("  %s\n", bold.Sprint("2. Define your interfaces"))
	fmt.Printf("     %s\n", dim.Sprintf("core/ports/ports.go — add repo and service methods"))
	fmt.Println()
	fmt.Printf("  %s\n", bold.Sprint("3. Write your DB schema"))
	fmt.Printf("     %s\n", dim.Sprintf(fmt.Sprintf("sql/migrations/0001_%s_init.sql — add your tables", name)))
	fmt.Println()
	fmt.Printf("  %s\n", bold.Sprint("4. Write sqlc queries"))
	fmt.Printf("     %s\n", dim.Sprintf(fmt.Sprintf("sql/queries/%s.sql — add your queries", name)))
	fmt.Printf("     %s\n", color.CyanString("     sqlc generate"))
	fmt.Println()
	fmt.Printf("  %s\n", bold.Sprint("5. Implement the store"))
	fmt.Printf("     %s\n", dim.Sprintf("adapters/postgres/store.go — use generated sqlc functions"))
	fmt.Println()
	fmt.Printf("  %s\n", bold.Sprint("6. Implement the service"))
	fmt.Printf("     %s\n", dim.Sprintf("core/services/service.go — business logic"))
	fmt.Println()
	fmt.Printf("  %s\n", bold.Sprint("7. Complete the handlers"))
	fmt.Printf("     %s\n", dim.Sprintf("adapters/httpx/handlers/handler.go — add request/response"))
	fmt.Println()
	fmt.Printf("  %s\n", bold.Sprint("8. Build check"))
	fmt.Printf("     %s\n", color.CyanString("     go build ./..."))
	fmt.Println()

	yellow.Println("  ── Publish your module ──────────────────────────────")
	fmt.Println()
	fmt.Printf("  # Push to GitHub\n")
	fmt.Printf("  cd %s\n", targetDir)
	fmt.Printf("  git remote add origin https://github.com/ranakdinesh/%s.git\n", targetDir)
	fmt.Printf("  git push -u origin main\n")
	fmt.Printf("  git tag v1.0.0 && git push --tags\n")
	fmt.Println()
	fmt.Printf("  # Add to the registry\n")
	fmt.Printf("  # Open: github.com/ranakdinesh/spur-registry/modules.json\n")
	fmt.Printf("  # Paste the contents of: %s/spur.json\n", targetDir)
	fmt.Println()
	fmt.Printf("  # Now anyone can install it:\n")
	fmt.Printf("  %s\n", color.GreenString("  spur add module "+name))
	fmt.Println()

	return nil
}

func toTitle(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
