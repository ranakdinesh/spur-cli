package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ranakdinesh/spur-cli/internal/config"
	"github.com/ranakdinesh/spur-cli/internal/protocol"
)

// ─── spur status ─────────────────────────────────────────────────────────────

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show what is installed in the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus()
		},
	}
}

func runStatus() error {
	state, err := config.LoadProjectState()
	if err != nil {
		return err
	}

	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	gray := color.New(color.FgHiBlack)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	bold.Printf("  Project: %s\n", state.Name)
	gray.Printf("  Module:  %s\n", state.GoModule)
	gray.Printf("  Created: %s\n", state.CreatedAt)
	fmt.Println()

	// Protocols
	bold.Println("  Protocols:")
	for _, p := range protocol.All() {
		if state.HasProtocol(p.ID) {
			port := state.ProtocolPorts[p.ID]
			suffix := ""
			if port != "" {
				suffix = gray.Sprintf(" (:%s)", port)
			}
			green.Printf("    ✓ %-12s", p.ID)
			fmt.Printf(" %s%s\n", truncate(p.Description, 50), suffix)
		} else if !p.AlwaysOn {
			gray.Printf("    ─ %-12s %s\n", p.ID, truncate(p.Description, 50))
		}
	}

	fmt.Println()

	// Modules
	bold.Println("  Modules:")
	if len(state.Modules) == 0 {
		gray.Println("    None installed.")
		fmt.Println()
		fmt.Printf("    Get started: %s\n", cyan.Sprint("spur add module identity"))
	} else {
		for _, m := range state.Modules {
			green.Printf("    ✓ %s\n", m)
		}
	}

	fmt.Println()
	bold.Println("  Add more:")
	gray.Println("    spur add protocol grpc")
	gray.Println("    spur add module billing")
	gray.Println("    spur make module mymodule")
	fmt.Println()

	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// ─── spur auth ───────────────────────────────────────────────────────────────

func authCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auth",
		Short: "Configure your GitHub PAT for private module access",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuth()
		},
	}
}

func runAuth() error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	bold.Println("  Spur — Configure GitHub Access")
	fmt.Println()
	fmt.Println("  Your GitHub PAT lets spur download private modules.")
	fmt.Println("  Scope needed: repo (read)")
	fmt.Println("  Create at:    https://github.com/settings/tokens/new")
	fmt.Println()

	cfg, _ := config.LoadCLIConfig()

	hint := ""
	if cfg.GitHubPAT != "" {
		end := 8
		if len(cfg.GitHubPAT) < 8 {
			end = len(cfg.GitHubPAT)
		}
		hint = fmt.Sprintf(" (current: %s...)", cfg.GitHubPAT[:end])
	}

	var pat string
	if err := survey.AskOne(&survey.Password{
		Message: fmt.Sprintf("GitHub PAT%s", hint),
	}, &pat); err != nil {
		return fmt.Errorf("cancelled")
	}

	pat = strings.TrimSpace(pat)
	if pat == "" && cfg.GitHubPAT != "" {
		pat = cfg.GitHubPAT
		fmt.Println("  Keeping existing PAT")
	}
	if pat == "" {
		return fmt.Errorf("PAT cannot be empty")
	}

	// Verify
	cyan.Println("\n  → Verifying with GitHub...")
	username, err := verifyPAT(pat)
	if err != nil {
		return fmt.Errorf("verification failed: %w\nCheck token has `repo` scope", err)
	}
	green.Printf("  ✓ Authenticated as: %s\n\n", username)

	cfg.GitHubPAT = pat
	if err := config.SaveCLIConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	green.Println("  ✓ PAT saved to ~/.spur/config.json")

	// Configure Go for private access
	for _, kv := range [][]string{
		{"GOPRIVATE", "github.com/ranakdinesh/*"},
		{"GONOSUMCHECK", "github.com/ranakdinesh/*"},
		{"GONOSUMDB", "github.com/ranakdinesh/*"},
	} {
		_ = exec.Command("go", "env", "-w", kv[0]+"="+kv[1]).Run()
	}
	green.Println("  ✓ GOPRIVATE configured")

	fmt.Printf("\n  Try: %s\n\n", color.CyanString("spur new myproject"))
	return nil
}

func verifyPAT(pat string) (string, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "token "+pat)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return "", fmt.Errorf("invalid token")
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub returned HTTP %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	needle := `"login":"`
	idx := strings.Index(s, needle)
	if idx == -1 {
		return "unknown", nil
	}
	start := idx + len(needle)
	end := strings.Index(s[start:], `"`)
	if end == -1 {
		return "unknown", nil
	}
	return s[start : start+end], nil
}
