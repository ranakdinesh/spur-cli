package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ranakdinesh/spur-cli/internal/config"
	"github.com/ranakdinesh/spur-cli/internal/registry"
)

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show all available Spur modules",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList()
		},
	}
}

func runList() error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)
	dim := color.New(color.FgHiBlack)
	yellow := color.New(color.FgYellow)

	modules, err := registry.All()
	if err != nil {
		return fmt.Errorf("load module registry: %w", err)
	}

	// Try to load project state (may not be inside a project)
	var installed map[string]bool
	state, err := config.LoadProjectState()
	if err == nil {
		installed = make(map[string]bool, len(state.Modules))
		for _, m := range state.Modules {
			installed[m] = true
		}
	}

	fmt.Println()
	bold.Println("  Available Spur Modules")
	fmt.Println()

	// Group by status
	stable := filterByStatus(modules, "stable")
	beta := filterByStatus(modules, "beta")
	alpha := filterByStatus(modules, "alpha")

	if len(stable) > 0 {
		bold.Println("  Stable")
		printModules(stable, installed, green, cyan, dim)
		fmt.Println()
	}

	if len(beta) > 0 {
		yellow.Println("  Beta")
		printModules(beta, installed, green, cyan, dim)
		fmt.Println()
	}

	if len(alpha) > 0 {
		dim.Println("  Alpha")
		printModules(alpha, installed, green, cyan, dim)
		fmt.Println()
	}

	// Footer
	if installed != nil {
		dim.Printf("  %d/%d installed in this project\n", len(installed), len(modules))
	} else {
		dim.Println("  Not inside a Spur project — run from project root to see install status")
	}
	fmt.Println()
	fmt.Printf("  Install: %s\n", cyan.Sprint("spur add module <name>"))
	fmt.Printf("  Details: %s\n", cyan.Sprint("spur list <name>"))
	fmt.Println()

	return nil
}

func printModules(modules []registry.Module, installed map[string]bool, green, cyan, dim *color.Color) {
	for _, m := range modules {
		marker := dim.Sprint("  ─ ")
		if installed != nil && installed[m.Name] {
			marker = green.Sprint("  ✓ ")
		}
		fmt.Printf("%s%-16s %s", marker, cyan.Sprint(m.Name), m.Description)
		if m.Version != "" {
			fmt.Printf("  %s", dim.Sprintf("v%s", m.Version))
		}
		fmt.Println()
	}
}

func filterByStatus(modules []registry.Module, status string) []registry.Module {
	var result []registry.Module
	for _, m := range modules {
		if m.Status == status {
			result = append(result, m)
		}
	}
	return result
}
