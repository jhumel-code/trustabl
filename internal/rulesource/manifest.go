package rulesource

import (
	"io/fs"

	"gopkg.in/yaml.v3"
)

// manifest is the parsed shape of a rule pack's root manifest.yaml.
type manifest struct {
	SchemaVersion int `yaml:"schema_version"`
}

// compatible reports whether the rule pack rooted at fsys can be evaluated by
// an engine supporting schema version `supported`. A missing, malformed, or
// non-positive-version manifest is treated as incompatible — a pack the engine
// cannot vouch for is not used.
func compatible(fsys fs.FS, supported int) bool {
	b, err := fs.ReadFile(fsys, "manifest.yaml")
	if err != nil {
		return false
	}
	var m manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return false
	}
	if m.SchemaVersion <= 0 {
		return false
	}
	return m.SchemaVersion <= supported
}
