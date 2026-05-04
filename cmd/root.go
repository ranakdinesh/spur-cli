package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "spur",
	Short: "Spur — composable Go backend platform CLI",
	Long: color.New(color.FgCyan, color.Bold).Sprint(`
  ███████╗██████╗ ██╗   ██╗██████╗
  ██╔════╝██╔══██╗██║   ██║██╔══██╗
  ███████╗██████╔╝██║   ██║██████╔╝
  ╚════██║██╔═══╝ ██║   ██║██╔══██╗
  ███████║██║     ╚██████╔╝██║  ██║
  ╚══════╝╚═╝      ╚═════╝ ╚═╝  ╚═╝
`) + `
  Build products, not infrastructure.

` + "  \033[1mProject commands:\033[0m" + `
    spur new <n>                  Create a new Spur project (interactive)
    spur add protocol <n>         Add a protocol to current project
    spur add module <n>           Download and install a module
    spur status                   What is installed in this project
    spur list                     All available modules

` + "  \033[1mInside a project:\033[0m" + `
    spur make module <n>          Scaffold a domain module (your business logic)
    spur make migration <m> <n>   Create a numbered SQL migration file
    spur make handler <m> <e>     Add a CRUD handler to a module

` + "  \033[1mBuild new Spur modules:\033[0m" + `
    spur create module <n>        Create a standalone spur-<n> library repo

` + "  \033[1mSetup:\033[0m" + `
    spur auth                     Configure your GitHub PAT
`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, color.RedString("Error: %s", err))
		return err
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newCmd())
	rootCmd.AddCommand(addCmd())
	rootCmd.AddCommand(makeCmd())
	rootCmd.AddCommand(createCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(authCmd())
}
