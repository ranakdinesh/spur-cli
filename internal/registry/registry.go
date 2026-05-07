// Package registry provides the embedded module catalog.
// The source of truth is .spur/modules.json in the CLI repo root.
// It is embedded at build time so spur list works offline.
package registry

import (
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed modules.json
var registryFS embed.FS

// Module describes an installable Spur module from the registry.
type Module struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	GoPackage   string   `json:"go_package"`
	Private     bool     `json:"private"`
	Status      string   `json:"status"`
	RequiredEnv []string `json:"required_env"`
	OptionalEnv []string `json:"optional_env"`
	HasSqlc     bool     `json:"has_sqlc"`
}

type registryFile struct {
	Modules []Module `json:"modules"`
}

// All returns every module in the embedded registry.
func All() ([]Module, error) {
	data, err := registryFS.ReadFile("modules.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded registry: %w", err)
	}
	var reg registryFile
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	return reg.Modules, nil
}

// Find returns a module by name.
func Find(name string) (Module, bool) {
	mods, err := All()
	if err != nil {
		return Module{}, false
	}
	for _, m := range mods {
		if m.Name == name {
			return m, true
		}
	}
	return Module{}, false
}
