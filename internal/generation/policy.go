package generation

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/trustabl/trustabl/internal/models"
)

// GeneratePolicy emits openshell/policy.yaml from OSH-* findings.
// Schema: https://docs.nvidia.com/openshell/reference/policy-schema
func GeneratePolicy(findings []models.Finding, version string) []models.GeneratedArtifact {
	osh := filterCategory(findings, models.CategoryOpenShell)
	if len(osh) == 0 {
		return []models.GeneratedArtifact{{
			RelativePath: "openshell/policy.yaml",
			Contents:     marshalPolicy(buildDefaultsOnlyPolicy()),
			Category:     models.CategoryOpenShell,
			Rationale:    "No OpenShell findings. Emitted defaults-only policy as a starter.",
		}}
	}

	policy := buildPolicy(osh)
	rationale := fmt.Sprintf("Generated from %d OpenShell finding(s).", len(osh))
	return []models.GeneratedArtifact{{
		RelativePath: "openshell/policy.yaml",
		Contents:     marshalPolicy(policy),
		Category:     models.CategoryOpenShell,
		Rationale:    rationale,
	}}
}

func filterCategory(findings []models.Finding, cat models.DetectorCategory) []models.Finding {
	var out []models.Finding
	for _, f := range findings {
		if f.Category == cat {
			out = append(out, f)
		}
	}
	return out
}

// ────────────────────────────────────────────────────────────────────────────
// Policy model — matches https://docs.nvidia.com/openshell/reference/policy-schema
// ────────────────────────────────────────────────────────────────────────────

type policyDoc struct {
	Version          int                      `yaml:"version"`
	FilesystemPolicy *filesystemPolicy        `yaml:"filesystem_policy,omitempty"`
	Landlock         *landlockPolicy          `yaml:"landlock,omitempty"`
	Process          *processPolicy           `yaml:"process,omitempty"`
	NetworkPolicies  map[string]networkPolicy `yaml:"network_policies,omitempty"`
}

type filesystemPolicy struct {
	IncludeWorkdir bool     `yaml:"include_workdir,omitempty"`
	ReadOnly       []string `yaml:"read_only,omitempty"`
	ReadWrite      []string `yaml:"read_write,omitempty"`
}

type landlockPolicy struct {
	Compatibility string `yaml:"compatibility"`
}

type processPolicy struct {
	RunAsUser  string `yaml:"run_as_user"`
	RunAsGroup string `yaml:"run_as_group"`
}

type networkPolicy struct {
	Name      string            `yaml:"name,omitempty"`
	Endpoints []networkEndpoint `yaml:"endpoints"`
	Binaries  []networkBinary   `yaml:"binaries"`
}

type networkEndpoint struct {
	Host        string   `yaml:"host"`
	Port        int      `yaml:"port"`
	Protocol    string   `yaml:"protocol,omitempty"`
	Enforcement string   `yaml:"enforcement,omitempty"`
	Access      string   `yaml:"access,omitempty"`
	AllowedIPs  []string `yaml:"allowed_ips,omitempty"`
}

type networkBinary struct {
	Path string `yaml:"path"`
}

// ────────────────────────────────────────────────────────────────────────────
// builders
// ────────────────────────────────────────────────────────────────────────────

func buildDefaultsOnlyPolicy() policyDoc {
	return policyDoc{
		Version: 1,
		FilesystemPolicy: &filesystemPolicy{
			IncludeWorkdir: true,
			ReadOnly:       []string{"/usr", "/lib", "/etc"},
			ReadWrite:      []string{"/tmp/agent"},
		},
		Landlock: &landlockPolicy{Compatibility: "hard_requirement"},
		Process:  &processPolicy{RunAsUser: "sandbox", RunAsGroup: "sandbox"},
	}
}

func buildPolicy(findings []models.Finding) policyDoc {
	doc := buildDefaultsOnlyPolicy()

	// Track which write prefixes and network policies have been added to dedupe.
	writePrefixSeen := map[string]bool{}
	netPolicies := map[string]networkPolicy{}

	for _, f := range findings {
		switch f.RuleID {
		case "OSH-001":
			// shell=True is a code fix. Harden landlock as an additional sandbox layer.
			doc.Landlock = &landlockPolicy{Compatibility: "hard_requirement"}

		case "OSH-002":
			// No command allowlist in code. OpenShell controls this via network_policies.binaries
			// (which executables can reach the network). Emit a per-tool network policy
			// with a binaries placeholder so the operator knows to restrict it.
			key := netKey(f.ToolName, "egress")
			entry := netPolicies[key]
			entry.Name = f.ToolName + " egress"
			if len(entry.Binaries) == 0 {
				entry.Binaries = []networkBinary{
					{Path: "# TODO: replace ** with the specific binary paths this tool invokes"},
					{Path: "**"},
				}
			}
			if len(entry.Endpoints) == 0 {
				entry.Endpoints = []networkEndpoint{{
					Host:        "# TODO: enumerate allowed hostnames",
					Port:        443,
					Enforcement: "enforce",
					Access:      "read-write",
				}}
			}
			netPolicies[key] = entry

		case "OSH-003":
			// Filesystem write — constrain to a prefix.
			if doc.FilesystemPolicy == nil {
				doc.FilesystemPolicy = &filesystemPolicy{IncludeWorkdir: true}
			}
			prefix := "/tmp/agent"
			if !writePrefixSeen[prefix] {
				doc.FilesystemPolicy.ReadWrite = appendUniq(doc.FilesystemPolicy.ReadWrite, prefix)
				writePrefixSeen[prefix] = true
			}

		case "OSH-004":
			// Resource limits (cpu/memory) are sandbox creation parameters, not policy.yaml
			// fields. Ensure landlock and process identity are set as the policy-layer baseline.
			doc.Landlock = &landlockPolicy{Compatibility: "hard_requirement"}
			if doc.Process == nil {
				doc.Process = &processPolicy{RunAsUser: "sandbox", RunAsGroup: "sandbox"}
			}

		case "OSH-005":
			// Unrestricted network egress — add a per-tool network policy with allowlist.
			key := netKey(f.ToolName, "egress")
			entry := netPolicies[key]
			entry.Name = f.ToolName + " egress"
			if len(entry.Endpoints) == 0 {
				entry.Endpoints = []networkEndpoint{{
					Host:        "# TODO: enumerate allowed hostnames",
					Port:        443,
					Protocol:    "rest",
					Enforcement: "enforce",
					Access:      "read-write",
				}}
			}
			if len(entry.Binaries) == 0 {
				entry.Binaries = []networkBinary{
					{Path: "# TODO: restrict to specific binary paths"},
				}
			}
			netPolicies[key] = entry

		case "OSH-006":
			// No process identity — set non-root sandbox user.
			doc.Process = &processPolicy{RunAsUser: "sandbox", RunAsGroup: "sandbox"}

		case "OSH-007":
			// Landlock not set to hard_requirement.
			doc.Landlock = &landlockPolicy{Compatibility: "hard_requirement"}
		}
	}

	if len(netPolicies) > 0 {
		doc.NetworkPolicies = make(map[string]networkPolicy, len(netPolicies))
		// Sort keys for deterministic output.
		keys := make([]string, 0, len(netPolicies))
		for k := range netPolicies {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			doc.NetworkPolicies[k] = netPolicies[k]
		}
	}

	return doc
}

func netKey(toolName, suffix string) string {
	if toolName == "" {
		return suffix
	}
	return toolName + "_" + suffix
}

func appendUniq(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

func marshalPolicy(doc policyDoc) string {
	b, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Sprintf("# trustabl: failed to marshal policy: %v\n", err)
	}
	return string(b)
}
