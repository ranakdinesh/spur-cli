package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ranakdinesh/spur-cli/internal/agent"
	"github.com/ranakdinesh/spur-cli/internal/config"
	"github.com/ranakdinesh/spur-cli/internal/protocol"
)

func addCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a protocol or module to the current project",
		Example: `  spur add protocol grpc
  spur add protocol temporal
  spur add module billing
  spur add module notifications`,
	}
	cmd.AddCommand(addProtocolCmd())
	cmd.AddCommand(addModuleCmd())
	return cmd
}

// ─── spur add protocol ────────────────────────────────────────────────────────

func addProtocolCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "protocol [<n>]",
		Short: "Add a protocol to the current project",
		Example: `  spur add protocol         ← interactive selector
  spur add protocol grpc
  spur add protocol temporal
  spur add protocol websocket
  spur add protocol hls
  spur add protocol rtmp`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return runAddProtocol(args[0])
			}
			return runAddProtocolInteractive()
		},
	}
}

func runAddProtocolInteractive() error {
	state, err := config.LoadProjectState()
	if err != nil {
		return err
	}

	// Show only protocols not yet enabled
	var options []string
	var optionIDs []string
	for _, p := range protocol.All() {
		if p.AlwaysOn || state.HasProtocol(p.ID) {
			continue
		}
		label := fmt.Sprintf("%-12s %s", p.ID, p.Description)
		options = append(options, label)
		optionIDs = append(optionIDs, p.ID)
	}

	if len(options) == 0 {
		color.Green("\n  All available protocols are already enabled!\n")
		return nil
	}

	var selectedIdxs []int
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Which protocols to add?",
		Options: options,
	}, &selectedIdxs); err != nil || len(selectedIdxs) == 0 {
		fmt.Println("  Nothing added.")
		return nil
	}

	for _, idx := range selectedIdxs {
		if err := runAddProtocol(optionIDs[idx]); err != nil {
			color.Yellow("  ⚠ Failed to add %s: %v", optionIDs[idx], err)
		}
	}
	return nil
}

func runAddProtocol(id string) error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)

	p, ok := protocol.Find(id)
	if !ok {
		// Show available protocols
		var available []string
		for _, p := range protocol.All() {
			if !p.AlwaysOn {
				available = append(available, p.ID)
			}
		}
		return fmt.Errorf("unknown protocol %q\nAvailable: %s", id, strings.Join(available, ", "))
	}

	state, err := config.LoadProjectState()
	if err != nil {
		return err
	}

	if state.HasProtocol(id) {
		color.Yellow("\n  Protocol %q is already enabled.\n", id)
		return nil
	}

	fmt.Println()
	bold.Printf("  Adding protocol: %s\n\n", id)

	// Auto-add dependencies
	for _, req := range p.RequiresProtocols {
		if !state.HasProtocol(req) {
			reqP, _ := protocol.Find(req)
			color.Yellow("  ⚡ Auto-adding %s (required by %s)", req, id)
			if err := addProtocolToProject(state, reqP); err != nil {
				return fmt.Errorf("add required protocol %s: %w", req, err)
			}
		}
	}

	// Add the requested protocol
	cyan.Printf("  → Adding %s...\n", id)
	if err := addProtocolToProject(state, p); err != nil {
		return err
	}
	green.Printf("  ✓ %s enabled\n", id)

	// Regenerate GEMINI.md
	cyan.Println("  → Updating GEMINI.md...")
	if err := agent.Regenerate(state); err != nil {
		color.Yellow("  ⚠ GEMINI.md update failed: %v", err)
	} else {
		green.Println("  ✓ GEMINI.md updated — agent now knows about " + id)
	}

	fmt.Println()
	bold.Printf("  ✅ Protocol %s added!\n\n", id)

	// Show what the user needs to do
	if len(p.EnvVars) > 0 {
		cyan.Println("  Add to deployments/.env:")
		for _, ev := range p.EnvVars {
			if ev.Required {
				fmt.Printf("    %s=\n", ev.Key)
			} else {
				fmt.Printf("    %s=%s\n", ev.Key, ev.Default)
			}
		}
		fmt.Println()
	}

	if id == "temporal" {
		fmt.Println("  Start with Temporal:")
		fmt.Println("    docker compose --profile temporal up -d")
	} else {
		fmt.Println("  Restart your project:")
		fmt.Println("    docker compose -f deployments/docker-compose.yml up -d --build")
	}

	if id == "grpc" {
		fmt.Println("\n  Test gRPC:")
		fmt.Println("    grpcurl -plaintext localhost:9090 list")
	}

	fmt.Println()
	return nil
}

func addProtocolToProject(state *config.ProjectState, p protocol.Protocol) error {
	// 1. Append env vars to .env.example
	if len(p.EnvVars) > 0 {
		appendEnvVars("deployments/.env.example", p)
	}

	// 2. Update docker-compose if protocol needs extra services
	if len(p.DockerServices) > 0 {
		appendDockerServices("deployments/docker-compose.yml", p.DockerServices)
	}

	// 3. Update project state
	state.AddProtocol(p.ID)
	if port, ok := defaultPort(p.ID); ok {
		state.ProtocolPorts[p.ID] = port
	}

	// 4. Save state
	return config.SaveProjectState(state)
}

func appendEnvVars(envPath string, p protocol.Protocol) {
	data, _ := os.ReadFile(envPath)
	content := string(data)

	var sb strings.Builder
	sb.WriteString(content)
	sb.WriteString(fmt.Sprintf("\n# ─── %s ──────────────────────────────────────────\n", strings.ToUpper(p.ID)))
	for _, ev := range p.EnvVars {
		if ev.Description != "" {
			sb.WriteString(fmt.Sprintf("# %s\n", ev.Description))
		}
		if ev.Required {
			sb.WriteString(fmt.Sprintf("%s=\n", ev.Key))
		} else {
			sb.WriteString(fmt.Sprintf("%s=%s\n", ev.Key, ev.Default))
		}
	}
	_ = os.WriteFile(envPath, []byte(sb.String()), 0644)
}

func appendDockerServices(composePath string, services []protocol.DockerService) {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return
	}
	content := string(data)

	// Insert before "volumes:" section
	volumesIdx := strings.LastIndex(content, "\nvolumes:")
	if volumesIdx == -1 {
		// Append at end
		for _, svc := range services {
			content += "\n" + svc.YAML + "\n"
		}
	} else {
		insert := ""
		for _, svc := range services {
			insert += "\n" + svc.YAML + "\n"
		}
		content = content[:volumesIdx] + insert + content[volumesIdx:]
	}

	_ = os.WriteFile(composePath, []byte(content), 0644)
}

func defaultPort(id string) (string, bool) {
	m := map[string]string{
		"grpc": "9090", "hls": "8888",
		"rtmp": "1935", "temporal": "7233",
		"websocket": "8080 (/ws)", "sse": "8080 (/sse)",
	}
	p, ok := m[id]
	return p, ok
}

// ─── spur add module ──────────────────────────────────────────────────────────

func addModuleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "module <n>",
		Short: "Download and wire a Spur module",
		Example: `  spur add module identity
  spur add module notifications
  spur add module billing
  spur add module storage
  spur add module agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddModule(args[0])
		},
	}
}

func runAddModule(name string) error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)

	state, err := config.LoadProjectState()
	if err != nil {
		return err
	}

	if state.HasModule(name) {
		color.Yellow("\n  Module %q is already installed.\n", name)
		return nil
	}

	cliCfg, err := config.LoadCLIConfig()
	if err != nil {
		return err
	}

	fmt.Println()
	bold.Printf("  Adding module: %s\n\n", name)

	// Configure Go for private access
	if cliCfg.GitHubPAT != "" {
		for _, kv := range [][]string{
			{"GOPRIVATE", "github.com/ranakdinesh/*"},
			{"GONOSUMCHECK", "github.com/ranakdinesh/*"},
		} {
			_ = exec.Command("go", "env", "-w", kv[0]+"="+kv[1]).Run()
		}
		// Configure git credential
		configureGitCred(cliCfg.GitHubPAT)
	}

	// go get the module
	pkg := fmt.Sprintf("github.com/ranakdinesh/spur-%s@latest", name)
	cyan.Printf("  → Downloading %s...\n", pkg)

	goGet := exec.Command("go", "get", pkg)
	goGet.Env = append(os.Environ(),
		"GONOSUMCHECK=github.com/ranakdinesh/*",
		"GOPRIVATE=github.com/ranakdinesh/*",
		"GOAUTH=gitcredentials",
	)
	if out, err := goGet.CombinedOutput(); err != nil {
		return fmt.Errorf("go get failed:\n%s\n\nTip: run `spur auth` to update your PAT", sanitise(string(out), cliCfg.GitHubPAT))
	}
	green.Printf("  ✓ Downloaded spur-%s\n", name)

	// ── NEW LOGIC ──────────────────────────────────────

	// Step 1: Read the module manifest
	modPath := "github.com/ranakdinesh/spur-" + name
	getModDir := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", modPath)
	outDir, err := getModDir.Output()
	if err != nil {
		return fmt.Errorf("could not find module in cache (did 'go get' fail?): %w", err)
	}
	dir := strings.TrimSpace(string(outDir))
	spurJsonPath := filepath.Join(dir, "spur.json")

	data, err := os.ReadFile(spurJsonPath)
	if err == nil {
		var manifest struct {
			WireCode struct {
				Import string `json:"import"`
				Init   string `json:"init"`
				Routes string `json:"routes"`
			} `json:"wire_code"`
			AppField        string   `json:"app_field"` // ← already there
			AppValue        string   `json:"app_value"`
			ConfigStruct    string   `json:"config_struct"`
			ConfigEnvPrefix string   `json:"config_env_prefix"`
			RequiredEnv     []string `json:"required_env"`
			OptionalEnv     []string `json:"optional_env"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			return fmt.Errorf("parse spur.json: %w", err)
		}

		// Steps 2, 3, 4, 5: Inject into app.go
		appPath := filepath.Join("internal", "app", "app.go")
		appData, err := os.ReadFile(appPath)
		if err != nil {
			return fmt.Errorf("read app.go: %w", err)
		}
		content := string(appData)

		// Import
		if manifest.WireCode.Import != "" {
			content, err = insertBefore(content, "// SPUR:IMPORTS:END", "\t"+manifest.WireCode.Import)
			if err != nil {
				return err
			}
		}
		// Module init code
		if manifest.WireCode.Init != "" {
			content, err = insertBefore(content, "// SPUR:MODULES:END", "\t"+manifest.WireCode.Init)
			if err != nil {
				return err
			}
		}
		// Route registration
		if manifest.WireCode.Routes != "" {
			content, err = insertBefore(content, "// SPUR:ROUTES:END", "\t"+manifest.WireCode.Routes)
			if err != nil {
				return err
			}
		}
		// Struct field declaration (e.g. "Identity *identity.Module")
		if manifest.AppField != "" {
			content, err = insertBefore(content, "// SPUR:APP_VALUES:END", "\t"+manifest.AppField)
			if err != nil {
				return err
			}
		}
		// Return statement value (e.g. "Identity: identityModule,")
		if manifest.AppValue != "" {
			content, err = insertBefore(content, "// SPUR:APP_RETURN:END", "\t"+manifest.AppValue)
			if err != nil {
				return err
			}
		}

		if err := os.WriteFile(appPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("write app.go: %w", err)
		}

		if err := os.WriteFile(appPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("write app.go: %w", err)
		}

		// Step 6: Add env vars to deployments/.env.example
		envPath := filepath.Join("deployments", ".env.example")
		envData, err := os.ReadFile(envPath)
		if err == nil {
			var sb strings.Builder
			sb.WriteString(string(envData))
			if len(manifest.RequiredEnv) > 0 || len(manifest.OptionalEnv) > 0 {
				sb.WriteString(fmt.Sprintf("\n# ── %s ──────────────────────────────────────────────────\n", name))
				for _, k := range manifest.RequiredEnv {
					sb.WriteString(fmt.Sprintf("%s=\n", k))
				}
				for _, k := range manifest.OptionalEnv {
					sb.WriteString(fmt.Sprintf("# %s=\n", k))
				}
			}
			os.WriteFile(envPath, []byte(sb.String()), 0644)
		}
	} else if os.IsNotExist(err) {
		color.Yellow("  ⚠ spur.json not found in module. Please wire it manually.")
	} else {
		return fmt.Errorf("read spur.json: %w", err)
	}
	// ── END NEW LOGIC ──────────────────────────────────

	// go mod tidy
	cyan.Println("  → Tidying modules...")
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Env = append(os.Environ(), "GONOSUMCHECK=github.com/ranakdinesh/*", "GOPRIVATE=github.com/ranakdinesh/*")
	_ = tidy.Run()
	green.Println("  ✓ go.mod updated")

	// Try automatic migration run if Makefile exists in project root
	if _, err := os.Stat("Makefile"); err == nil {
		cyan.Println("  → Running migrations...")
		migrate := exec.Command("make", "migrate")
		if out, err := migrate.CombinedOutput(); err != nil {
			color.Yellow("  ⚠ Auto migration failed: %v", err)
			if len(out) > 0 {
				color.Yellow("  %s", sanitise(string(out), cliCfg.GitHubPAT))
			}
			color.Yellow("  Run manually after configuring deployments/.env")
		} else {
			green.Println("  ✓ Database migrations applied")
		}
	}

	// Update state
	state.AddModule(name)
	if err := config.SaveProjectState(state); err != nil {
		color.Yellow("  ⚠ Could not update .spur.json")
	}

	// Regenerate GEMINI.md
	cyan.Println("  → Updating GEMINI.md...")
	if err := agent.Regenerate(state); err != nil {
		color.Yellow("  ⚠ GEMINI.md update failed: %v", err)
	} else {
		green.Printf("  ✓ GEMINI.md updated — agent knows about %s module\n", name)
	}

	fmt.Println()
	bold.Printf("  ✅ Module spur-%s added!\n\n", name)
	fmt.Printf("  Wire it in internal/app/app.go (see GEMINI.md Section 5)\n")
	fmt.Printf("  Then: make sqlc && go run ./cmd/main.go\n")
	fmt.Printf("  If migrations were skipped/failed: make migrate\n\n")

	return nil
}

func configureGitCred(pat string) {
	if pat == "" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	netrcPath := filepath.Join(home, ".netrc")
	entry := fmt.Sprintf(
		"\nmachine github.com login %s password %s\n",
		"x-oauth-basic", pat,
	)
	// Append only if not already present
	existing, _ := os.ReadFile(netrcPath)
	if strings.Contains(string(existing), "machine github.com") {
		return
	}
	f, err := os.OpenFile(netrcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(entry)
}

func sanitise(s, pat string) string {
	if pat == "" {
		return s
	}
	return strings.ReplaceAll(s, pat, "***")
}

func insertBefore(content, marker, lines string) (string, error) {
	lines = strings.TrimRight(lines, "\n")

	if lines == "" {
		return content, nil
	}

	if strings.Contains(content, lines) {
		return content, nil
	}

	idx := strings.Index(content, marker)
	if idx == -1 {
		return content, fmt.Errorf(
			"marker %q not found. Your project was probably created from an older spur-template. Add the marker to internal/app/app.go or regenerate the project from the latest template",
			marker,
		)
	}

	lineStart := strings.LastIndex(content[:idx], "\n") + 1
	return content[:lineStart] + lines + "\n" + content[lineStart:], nil

}
