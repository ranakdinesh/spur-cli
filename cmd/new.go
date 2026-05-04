package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ranakdinesh/spur-cli/internal/agent"
	"github.com/ranakdinesh/spur-cli/internal/config"
	"github.com/ranakdinesh/spur-cli/internal/protocol"
)

func newCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <project-name>",
		Short: "Create a new Spur project (interactive)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(args[0])
		},
	}
}

func runNew(name string) error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)
	dim := color.New(color.FgHiBlack)

	// ── Banner ────────────────────────────────────────────────────────────
	fmt.Println()
	bold.Println("  ╔═══════════════════════════════════╗")
	bold.Println("  ║        Spur — New Project         ║")
	bold.Println("  ╚═══════════════════════════════════╝")
	fmt.Println()

	// ── Step 1: Confirm project name ─────────────────────────────────────
	fmt.Printf("  Creating project: %s\n\n", cyan.Sprint(name))

	// ── Step 2: Go module path ────────────────────────────────────────────
	var goModule string
	if err := survey.AskOne(&survey.Input{
		Message: "Go module path?",
		Default: fmt.Sprintf("github.com/ranakdinesh/%s", name),
		Help:    "The Go module path used in go.mod. e.g. github.com/yourorg/yourproject",
	}, &goModule, survey.WithValidator(survey.Required)); err != nil {
		return fmt.Errorf("cancelled")
	}

	// ── Step 3: Protocol selection ────────────────────────────────────────
	fmt.Println()
	bold.Println("  ┌─ Select Protocols ──────────────────────────────────────────┐")
	dim.Println("  │  HTTP is always on. Select what else your project needs.    │")
	bold.Println("  └─────────────────────────────────────────────────────────────┘")
	fmt.Println()

	allProtocols := protocol.All()

	// Build options (skip HTTP — always on)
	var options []string
	var optionIDs []string
	for _, p := range allProtocols {
		if p.AlwaysOn {
			continue
		}
		label := fmt.Sprintf("%-12s %s", p.ID, p.Description)
		options = append(options, label)
		optionIDs = append(optionIDs, p.ID)
	}

	var selectedIdxs []int
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Which protocols do you need?",
		Options: options,
		Help:    "Space to toggle, Enter to confirm. You can add more later with: spur add protocol <n>",
	}, &selectedIdxs); err != nil {
		return fmt.Errorf("cancelled")
	}

	// Build selected protocol IDs (always include HTTP)
	selectedProtocols := []string{"http"}
	for _, idx := range selectedIdxs {
		pid := optionIDs[idx]
		selectedProtocols = append(selectedProtocols, pid)
		// Auto-add required protocols
		p, _ := protocol.Find(pid)
		for _, req := range p.RequiresProtocols {
			found := false
			for _, s := range selectedProtocols {
				if s == req {
					found = true
					break
				}
			}
			if !found {
				selectedProtocols = append(selectedProtocols, req)
				color.Yellow("\n  ⚡ Auto-adding %s (required by %s)", req, pid)
			}
		}
	}

	// ── Step 4: Confirm ───────────────────────────────────────────────────
	fmt.Println()
	bold.Println("  Summary:")
	fmt.Printf("  Project:   %s\n", cyan.Sprint(name))
	fmt.Printf("  Module:    %s\n", cyan.Sprint(goModule))
	fmt.Printf("  Protocols: %s\n", cyan.Sprint(strings.Join(selectedProtocols, ", ")))
	fmt.Println()

	var proceed bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Create project?",
		Default: true,
	}, &proceed); err != nil || !proceed {
		fmt.Println("  Cancelled.")
		return nil
	}

	// ── Step 5: Create project directory ─────────────────────────────────
	targetDir, err := filepath.Abs(name)
	if err != nil {
		return fmt.Errorf("resolve project directory %q: %w", name, err)
	}

	if strings.TrimSpace(targetDir) == "" {
		return fmt.Errorf("resolved project directory is empty for project name %q", name)
	}

	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("directory %q already exists", targetDir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check target directory %q: %w", targetDir, err)
	}
	cliCfg, err := config.LoadCLIConfig()
	if err != nil {
		return err
	}

	fmt.Println()
	cyan.Println("  → Cloning template...")
	if err := cloneTemplate(cliCfg.TemplateRepo, cliCfg.GitHubPAT, targetDir); err != nil {
		return fmt.Errorf("clone template: %w", err)
	}
	_ = os.RemoveAll(filepath.Join(targetDir, ".git"))
	green.Println("  ✓ Template cloned")

	// ── Step 6: Replace module path everywhere ────────────────────────────
	cyan.Println("  → Setting module path...")
	oldPkg := "github.com/ranakdinesh/spur-template"
	if err := replaceInDir(targetDir, oldPkg, goModule); err != nil {
		return err
	}
	if err := replaceInFile(filepath.Join(targetDir, "go.mod"),
		"module "+oldPkg, "module "+goModule); err != nil {
		return err
	}
	green.Printf("  ✓ Module path set to %s\n", goModule)

	// ── Step 7: Generate env example from selected protocols ──────────────
	cyan.Println("  → Generating .env.example...")
	if err := generateEnvExample(targetDir, selectedProtocols); err != nil {
		color.Yellow("  ⚠ Could not write .env.example: %v", err)
	} else {
		green.Println("  ✓ .env.example generated")
	}

	// Copy to .env
	_ = copyFile(
		filepath.Join(targetDir, "deployments", ".env.example"),
		filepath.Join(targetDir, "deployments", ".env"),
	)

	// ── Step 8: Generate docker-compose from selected protocols ───────────
	cyan.Println("  → Generating docker-compose.yml...")
	if err := generateDockerCompose(targetDir, selectedProtocols); err != nil {
		color.Yellow("  ⚠ docker-compose generation failed: %v", err)
	} else {
		green.Println("  ✓ docker-compose.yml generated")
	}

	// ── Step 9: RSA key ───────────────────────────────────────────────────
	keysDir := filepath.Join(targetDir, "keys")
	_ = os.MkdirAll(keysDir, 0700)
	keyPath := filepath.Join(keysDir, "private.pem")
	if out, err := exec.Command("openssl", "genrsa", "-out", keyPath, "2048").CombinedOutput(); err != nil {
		_ = out
		color.Yellow("  ⚠ Generate RSA key manually: openssl genrsa -out %s/keys/private.pem 2048", name)
	} else {
		_ = os.Chmod(keyPath, 0600)
		green.Println("  ✓ RSA key generated")
	}

	// ── Step 10: Write .spur.json (project state) ─────────────────────────
	cyan.Println("  → Writing project state...")
	portMap := buildPortMap(selectedProtocols)
	state := &config.ProjectState{
		Name:           name,
		GoModule:       goModule,
		Protocols:      selectedProtocols,
		Modules:        []string{},
		ProtocolPorts:  portMap,
		CreatedAt:      time.Now().Format(time.RFC3339),
		SpurCLIVersion: "1.0.0",
	}

	// Write .spur.json into the target dir temporarily
	origDir, _ := os.Getwd()
	_ = os.Chdir(targetDir)

	if err := config.SaveProjectState(state); err != nil {
		color.Yellow("  ⚠ Could not write .spur.json: %v", err)
	} else {
		green.Println("  ✓ Project state saved to .spur.json")
	}

	// ── Step 11: Generate GEMINI.md ───────────────────────────────────────
	cyan.Println("  → Generating GEMINI.md...")
	if err := agent.Regenerate(state); err != nil {
		color.Yellow("  ⚠ Could not write GEMINI.md: %v", err)
	} else {
		green.Println("  ✓ GEMINI.md generated (coding agent guide)")
	}

	_ = os.Chdir(origDir)

	// ── Step 12: Git init ─────────────────────────────────────────────────
	_ = gitInit(targetDir)
	green.Println("  ✓ Git repository initialised")

	// ── Done ─────────────────────────────────────────────────────────────
	fmt.Println()
	bold.Printf("  🚀 %s is ready!\n\n", name)

	fmt.Printf("  %s\n\n", cyan.Sprint("Next steps:"))
	fmt.Printf("  cd %s\n", name)
	fmt.Printf("  # Edit deployments/.env\n")
	fmt.Printf("  #   FOSITE_GLOBAL_SECRET=$(openssl rand -hex 32)\n")
	fmt.Printf("  #   AUTH_CLIENT_ID=$(uuidgen)\n\n")
	fmt.Printf("  spur add module identity   %s\n", dim.Sprint("← add auth"))

	if hasProtocol(selectedProtocols, "temporal") {
		fmt.Printf("\n  %s\n", cyan.Sprint("Start with Temporal:"))
		fmt.Printf("  docker compose --profile temporal up -d\n")
	} else {
		fmt.Printf("\n  docker compose -f deployments/docker-compose.yml up -d\n")
	}
	fmt.Printf("  curl http://localhost:8080/healthz\n")
	fmt.Println()
	fmt.Printf("  Add more later:\n")
	fmt.Printf("  spur add protocol grpc\n")
	fmt.Printf("  spur add module billing\n")
	fmt.Printf("  spur status\n")
	fmt.Println()

	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func cloneTemplate(repo, pat, targetDir string) error {
	var url string
	if pat != "" {
		url = fmt.Sprintf("https://%s@github.com/%s.git", pat, repo)
	} else {
		url = fmt.Sprintf("https://github.com/%s.git", repo)
	}
	cmd := exec.Command("git", "clone", "--depth=1", url, targetDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		// sanitise PAT from error
		msg := strings.ReplaceAll(string(out), pat, "***")
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func gitInit(dir string) error {
	for _, args := range [][]string{
		{"init"},
		{"add", "."},
		{"commit", "-m", "chore: init from spur template"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		_ = cmd.Run()
	}
	return nil
}

func replaceInDir(dir, old, newStr string) error {
	exts := map[string]bool{".go": true, ".yaml": true, ".yml": true, ".json": true, ".md": true, ".toml": true}
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if n := info.Name(); n == ".git" || n == "node_modules" || n == ".next" || n == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !exts[filepath.Ext(path)] {
			return nil
		}
		return replaceInFile(path, old, newStr)
	})
}

func replaceInFile(path, old, newStr string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated := strings.ReplaceAll(string(data), old, newStr)
	if updated == string(data) {
		return nil
	}
	return os.WriteFile(path, []byte(updated), 0644)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}

func hasProtocol(list []string, id string) bool {
	for _, p := range list {
		if p == id {
			return true
		}
	}
	return false
}

func buildPortMap(protocols []string) map[string]string {
	defaults := map[string]string{
		"http":      "8080",
		"grpc":      "9090",
		"websocket": "8080 (/ws)",
		"sse":       "8080 (/sse)",
		"hls":       "8888",
		"rtmp":      "1935",
		"temporal":  "7233",
	}
	result := map[string]string{}
	for _, p := range protocols {
		if port, ok := defaults[p]; ok {
			result[p] = port
		}
	}
	return result
}

func generateEnvExample(dir string, protocols []string) error {
	var sb strings.Builder

	sb.WriteString("# ─── Core ────────────────────────────────────────────────────\n")
	sb.WriteString("APP_ENV=development\n")
	sb.WriteString("APP_NAME=Spur\n")
	sb.WriteString("LOG_LEVEL=info\n\n")

	sb.WriteString("# ─── Database ────────────────────────────────────────────────\n")
	sb.WriteString("DATABASE_URL=postgres://spur:changeme@postgres:5432/spur?sslmode=disable\n")
	sb.WriteString("POSTGRES_DB=spur\n")
	sb.WriteString("POSTGRES_USER=spur\n")
	sb.WriteString("POSTGRES_PASSWORD=changeme\n\n")

	sb.WriteString("# ─── Redis ───────────────────────────────────────────────────\n")
	sb.WriteString("REDIS_URL=redis://redis:6379\n\n")

	sb.WriteString("# ─── UI ──────────────────────────────────────────────────────\n")
	sb.WriteString("FRONTEND_URL=http://localhost:3000\n")
	sb.WriteString("NEXT_PUBLIC_THEME=spur\n\n")

	for _, pid := range protocols {
		p, ok := protocol.Find(pid)
		if !ok || p.AlwaysOn || len(p.EnvVars) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("# ─── %s ───────────────────────────────────────────────────\n",
			strings.ToUpper(pid)))
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
		sb.WriteString("\n")
	}

	// Grafana
	sb.WriteString("# ─── Observability ──────────────────────────────────────────\n")
	sb.WriteString("GRAFANA_PASSWORD=admin\n")

	envPath := filepath.Join(dir, "deployments", ".env.example")
	_ = os.MkdirAll(filepath.Dir(envPath), 0755)
	return os.WriteFile(envPath, []byte(sb.String()), 0644)
}

func generateDockerCompose(dir string, protocols []string) error {
	var sb strings.Builder

	sb.WriteString("version: '3.9'\n\nservices:\n\n")

	// Always: postgres + redis + backend + studio + observability
	sb.WriteString(`  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB:       ${POSTGRES_DB:-spur}
      POSTGRES_USER:     ${POSTGRES_USER:-spur}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-changeme}
    volumes:
      - pg_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-spur}"]
      interval: 5s
      retries: 10

  redis:
    image: redis:7-alpine
    restart: unless-stopped
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s

`)

	// Backend ports depend on protocols
	ports := []string{`      - "8080:8080"`}
	if hasProtocol(protocols, "grpc") {
		ports = append(ports, `      - "9090:9090"`)
	}
	if hasProtocol(protocols, "hls") {
		ports = append(ports, `      - "8888:8888"`)
	}
	if hasProtocol(protocols, "rtmp") {
		ports = append(ports, `      - "1935:1935"`)
	}

	sb.WriteString("  backend:\n")
	sb.WriteString("    build:\n")
	sb.WriteString("      context: .\n")
	sb.WriteString("      dockerfile: Dockerfile.backend\n")
	sb.WriteString("    restart: unless-stopped\n")
	sb.WriteString("    env_file: deployments/.env\n")
	sb.WriteString("    environment:\n")
	sb.WriteString("      DATABASE_URL: postgres://${POSTGRES_USER:-spur}:${POSTGRES_PASSWORD:-changeme}@postgres:5432/${POSTGRES_DB:-spur}?sslmode=disable\n")
	sb.WriteString("      REDIS_URL: redis://redis:6379\n")
	sb.WriteString("    ports:\n")
	for _, p := range ports {
		sb.WriteString(p + "\n")
	}
	sb.WriteString("    volumes:\n")
	sb.WriteString("      - ./keys:/app/keys\n")
	sb.WriteString("      - ./data:/app/data\n")
	sb.WriteString("    labels:\n")
	sb.WriteString("      app: spur\n")
	sb.WriteString("      module: backend\n")
	sb.WriteString("    depends_on:\n")
	sb.WriteString("      postgres:\n")
	sb.WriteString("        condition: service_healthy\n")
	sb.WriteString("      redis:\n")
	sb.WriteString("        condition: service_healthy\n")
	sb.WriteString("    logging:\n")
	sb.WriteString("      driver: json-file\n")
	sb.WriteString("      options:\n")
	sb.WriteString("        max-size: \"50m\"\n")
	sb.WriteString("        max-file: \"5\"\n")
	sb.WriteString("        tag: \"{{.Name}}\"\n\n")

	sb.WriteString(`  studio:
    build:
      context: .
      dockerfile: Dockerfile.ui
      args:
        NEXT_PUBLIC_API_URL: ${NEXT_PUBLIC_API_URL:-http://localhost:8080}
        NEXT_PUBLIC_THEME:   ${NEXT_PUBLIC_THEME:-spur}
    restart: unless-stopped
    ports:
      - "3000:3000"
    labels:
      app: spur
      module: studio
    depends_on:
      - backend

`)

	// Temporal services if selected
	if hasProtocol(protocols, "temporal") {
		for _, p := range protocol.All() {
			if p.ID == "temporal" {
				for _, svc := range p.DockerServices {
					sb.WriteString(svc.YAML + "\n\n")
				}
			}
		}
	}

	// Observability (always)
	sb.WriteString(`  loki:
    image: grafana/loki:2.9.0
    restart: unless-stopped
    volumes:
      - ./deployments/loki/config.yaml:/etc/loki/local-config.yaml:ro
      - loki_data:/loki
    command: -config.file=/etc/loki/local-config.yaml

  promtail:
    image: grafana/promtail:2.9.0
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - /var/lib/docker/containers:/var/lib/docker/containers:ro
      - ./deployments/promtail/config.yaml:/etc/promtail/config.yml:ro

  prometheus:
    image: prom/prometheus:v2.47.0
    restart: unless-stopped
    volumes:
      - ./deployments/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus_data:/prometheus

  grafana:
    image: grafana/grafana:10.2.0
    restart: unless-stopped
    ports:
      - "3001:3000"
    volumes:
      - grafana_data:/var/lib/grafana
      - ./deployments/grafana/provisioning:/etc/grafana/provisioning:ro
    environment:
      GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_PASSWORD:-admin}
    depends_on:
      - loki
      - prometheus

volumes:
  pg_data:
  redis_data:
  loki_data:
  prometheus_data:
  grafana_data:
`)

	composePath := filepath.Join(dir, "deployments", "docker-compose.yml")
	_ = os.MkdirAll(filepath.Dir(composePath), 0755)
	return os.WriteFile(composePath, []byte(sb.String()), 0644)
}
