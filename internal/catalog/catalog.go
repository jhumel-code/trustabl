// Package catalog maps known agent tool names to capability classes.
// The embedded tools_catalog.yaml is the curated seed; use tools/gather_tools.py
// to collect raw data for curation passes.
package catalog

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// CapabilityClass is a coarse-grained category describing what a tool can do.
// Used by catalog/ policy rules to apply risk checks regardless of SDK.
type CapabilityClass string

const (
	CapClassCodeExec    CapabilityClass = "code_execution"
	CapClassShellExec   CapabilityClass = "shell_execution"
	CapClassFileRead    CapabilityClass = "file_read"
	CapClassFileWrite   CapabilityClass = "file_write"
	CapClassNetRead     CapabilityClass = "network_read"
	CapClassNetWrite    CapabilityClass = "network_write"
	CapClassAgentSpawn  CapabilityClass = "agent_spawn"
	CapClassMemWrite    CapabilityClass = "memory_write"
	CapClassDataQuery   CapabilityClass = "data_query"
	CapClassDataMutate  CapabilityClass = "data_mutate"
	CapClassAuth        CapabilityClass = "auth_action"
	CapClassComputerUse CapabilityClass = "computer_use"
	CapClassExternalAPI CapabilityClass = "external_api"
)

// CatalogEntry is one row in the catalog.
type CatalogEntry struct {
	ID              string          `yaml:"id"`
	Aliases         []string        `yaml:"aliases"`
	Frameworks      []string        `yaml:"frameworks"`
	CapabilityClass CapabilityClass `yaml:"capability_class"`
	RiskLevel       string          `yaml:"risk_level"`
	Description     string          `yaml:"description"`
}

// Catalog is the loaded tool catalog with a built lookup index.
type Catalog struct {
	Version string         `yaml:"version"`
	Entries []CatalogEntry `yaml:"entries"`
	index   map[string]CapabilityClass
}

// DefaultCatalog loads and returns the embedded catalog.
func DefaultCatalog() (*Catalog, error) {
	data, err := embeddedFS.ReadFile("tools_catalog.yaml")
	if err != nil {
		return nil, err
	}
	var c Catalog
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	c.buildIndex()
	return &c, nil
}

func (c *Catalog) buildIndex() {
	c.index = make(map[string]CapabilityClass, len(c.Entries)*4)
	for _, e := range c.Entries {
		c.index[strings.ToLower(e.ID)] = e.CapabilityClass
		for _, alias := range e.Aliases {
			c.index[strings.ToLower(alias)] = e.CapabilityClass
		}
	}
}

// Lookup returns the CapabilityClass for a tool name (case-insensitive).
// Returns "" if the tool is not in the catalog.
func (c *Catalog) Lookup(toolName string) CapabilityClass {
	return c.index[strings.ToLower(toolName)]
}
