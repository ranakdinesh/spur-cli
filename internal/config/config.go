// Package config manages both the CLI user config (~/.spur/config.json)
// and the per-project state (.spur.json in project root).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ─── CLI User Config ──────────────────────────────────────────────────────────

type CLIConfig struct {
	GitHubPAT    string `json:"github_pat"`
	RegistryRepo string `json:"registry_repo"`
	TemplateRepo string `json:"template_repo"`
}

func DefaultCLIConfig() CLIConfig {
	return CLIConfig{
		RegistryRepo: "ranakdinesh/spur-template",
		TemplateRepo: "ranakdinesh/spur-template",
	}
}

func CLIConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".spur", "config.json"), nil
}

func LoadCLIConfig() (CLIConfig, error) {
	cfg := DefaultCLIConfig()
	path, err := CLIConfigPath()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	_ = json.Unmarshal(data, &cfg)
	if pat := os.Getenv("SPUR_GITHUB_PAT"); pat != "" {
		cfg.GitHubPAT = pat
	}
	return cfg, nil
}

func SaveCLIConfig(cfg CLIConfig) error {
	path, err := CLIConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(path, data, 0600)
}

func RequirePAT() (string, error) {
	cfg, err := LoadCLIConfig()
	if err != nil {
		return "", err
	}
	if cfg.GitHubPAT == "" {
		return "", fmt.Errorf("GitHub PAT not configured.\nRun: spur auth")
	}
	return cfg.GitHubPAT, nil
}

// ─── Project State (.spur.json) ───────────────────────────────────────────────
// Written to the project root by spur new and updated by spur add.
// The coding agent reads this to know what's available.

const ProjectStateFile = ".spur.json"

type ProjectState struct {
	// Name is the project name
	Name string `json:"name"`

	// GoModule is the Go module path e.g. "github.com/ranakdinesh/brightlens"
	GoModule string `json:"go_module"`

	// Protocols lists every enabled protocol ID e.g. ["http","grpc","websocket"]
	Protocols []string `json:"protocols"`

	// Modules lists every installed module e.g. ["identity","notifications"]
	Modules []string `json:"modules"`

	// ProtocolPorts maps protocol → port for quick reference
	ProtocolPorts map[string]string `json:"protocol_ports"`

	// CreatedAt is the ISO timestamp of project creation
	CreatedAt string `json:"created_at"`

	// SpurCLIVersion is the CLI version that created this project
	SpurCLIVersion string `json:"spur_cli_version"`
}

func LoadProjectState() (*ProjectState, error) {
	data, err := os.ReadFile(ProjectStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"not inside a Spur project (no .spur.json found).\n" +
					"Run from the project root, or create a project with: spur new <n>",
			)
		}
		return nil, err
	}
	var state ProjectState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("corrupt .spur.json: %w", err)
	}
	return &state, nil
}

func SaveProjectState(state *ProjectState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ProjectStateFile, data, 0644)
}

func (s *ProjectState) HasProtocol(id string) bool {
	for _, p := range s.Protocols {
		if p == id {
			return true
		}
	}
	return false
}

func (s *ProjectState) HasModule(id string) bool {
	for _, m := range s.Modules {
		if m == id {
			return true
		}
	}
	return false
}

func (s *ProjectState) AddProtocol(id string) {
	if !s.HasProtocol(id) {
		s.Protocols = append(s.Protocols, id)
	}
}

func (s *ProjectState) AddModule(id string) {
	if !s.HasModule(id) {
		s.Modules = append(s.Modules, id)
	}
}
