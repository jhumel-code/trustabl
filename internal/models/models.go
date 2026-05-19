// Package models holds the typed values that cross module boundaries.
//
// Discipline: anything passed from ingestion → analysis → generation → review
// lives here, not as ad-hoc maps. JSON tags are present because ScanResult is
// emitted for CI piping (--format json).
package models

import "math"

// FixType classifies how a finding can be remediated: by changing agent/tool
// configuration (permissions, guardrails, sandbox policies) or by modifying
// tool source code directly.
type FixType string

const (
	// FixTypeConfig — fix by adding guardrails, hooks, sandbox policies, or
	// agent constructor parameters. No tool source code needs to change.
	FixTypeConfig FixType = "config"
	// FixTypeCode — fix requires modifying tool or agent source code.
	FixTypeCode FixType = "code"
)

// ValidFixType reports whether f is a known fix_type value.
func ValidFixType(f FixType) bool {
	return f == FixTypeConfig || f == FixTypeCode
}

// Scope classifies a rule by the kind of entity it fires against.
type Scope string

const (
	ScopeTool  Scope = "tool"
	ScopeAgent Scope = "agent"
	ScopeRepo  Scope = "repo"
)

// ValidScope reports whether s is a known scope value.
func ValidScope(s Scope) bool {
	switch s {
	case ScopeTool, ScopeAgent, ScopeRepo:
		return true
	}
	return false
}

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// SeverityWeight maps a severity to a numeric weight for scoring. Tweak in
// scoring.go, not here, if the curve needs adjustment.
func SeverityWeight(s Severity) float64 {
	switch s {
	case SeverityCritical:
		return 1.0
	case SeverityHigh:
		return 0.7
	case SeverityMedium:
		return 0.4
	case SeverityLow:
		return 0.15
	default:
		return 0.05
	}
}

// DetectorCategory maps to the two AutoFix categories in architecture §0.
type DetectorCategory string

const (
	CategoryClaudeSDK DetectorCategory = "claude_sdk"
	CategoryOpenAISDK DetectorCategory = "openai_sdk"
	CategoryGoogleADK DetectorCategory = "google_adk"
	CategoryOpenShell DetectorCategory = "openshell"
	CategoryMCP       DetectorCategory = "mcp"
	CategoryCatalog   DetectorCategory = "catalog"
)

// ToolKind drives detector applicability.
type ToolKind string

const (
	KindClaudeSDKTool   ToolKind = "claude_sdk_tool"
	KindOpenAITool      ToolKind = "openai_tool" // OpenAI Agents SDK @function_tool
	KindMCPTool         ToolKind = "mcp_tool"
	KindShellInvocation ToolKind = "shell_invocation"
	KindGoogleADKTool   ToolKind = "google_adk_tool" // Google Agent Development Kit @adk.tool
	KindUnknown         ToolKind = "unknown"
)

// Language identifies the source language of a discovered tool. Rules
// declare a language too, and a rule only applies to a tool of the matching
// language. Empty string is treated as "python" by the loader for
// backwards compatibility.
type Language string

const (
	LanguagePython     Language = "python"
	LanguageTypeScript Language = "typescript"
	LanguageJavaScript Language = "javascript"
	LanguageGo         Language = "go"
)

// ToolDef is one discovered surface that an agent can invoke at runtime.
// Mirrors the Tool Discovery node in architecture §2.
type ToolDef struct {
	Name           string            `json:"name"`
	Kind           ToolKind          `json:"kind"`
	Language       Language          `json:"language"`
	FilePath       string            `json:"file_path"` // relative to repo root
	Line           int               `json:"line"`
	EndLine        int               `json:"end_line"`
	Description    string            `json:"description,omitempty"`
	HasTypedParams bool              `json:"has_typed_params"`
	ParamNames     []string          `json:"param_names,omitempty"`
	Facts           map[string]string `json:"facts,omitempty"`
	Config          map[string]string `json:"config,omitempty"` // decorator kwargs
	CapabilityClass string            `json:"capability_class,omitempty"` // set by catalog enrichment
}

// ComponentKind labels the type of an agent component the normalizer found
// outside of the tool function itself. Components are surfaced for context
// and for future detection passes; today's rule engine only fires against
// ToolDef, not against AgentComponent.
type ComponentKind string

const (
	ComponentMCPConfig             ComponentKind = "mcp_config"
	ComponentClaudeMd              ComponentKind = "claude_md"
	ComponentClaudeSettings        ComponentKind = "claude_settings"
	ComponentSubagent              ComponentKind = "subagent"
	ComponentSlashCommand          ComponentKind = "slash_command"
	ComponentHookScript            ComponentKind = "hook_script"
	ComponentSandboxPolicy         ComponentKind = "sandbox_policy"
	ComponentSystemPrompt          ComponentKind = "system_prompt"
	ComponentDependencyManifest    ComponentKind = "dependency_manifest"
	ComponentClaudeAgentDefinition ComponentKind = "claude_agent_definition" // Python file using AgentDefinition
)

// AgentComponent is a non-tool agent-related artifact discovered during
// normalization: an MCP config, CLAUDE.md, hook script, sandbox policy,
// dependency manifest, etc. These are surfaced in ScanManifest.Components
// so users see the full agent surface, even though detection rules currently
// only run against tools.
type AgentComponent struct {
	Kind     ComponentKind `json:"kind"`
	Path     string        `json:"path"`               // relative to repo root
	Language Language      `json:"language,omitempty"` // for code components only; empty for configs / prompts
	Note     string        `json:"note,omitempty"`     // human-readable hint, e.g. "3 tools registered"
}

// BaseScoreWeight maps severity to the 0–100 flat base score scale.
func BaseScoreWeight(s Severity) float64 {
	switch s {
	case SeverityCritical:
		return 100.0
	case SeverityHigh:
		return 70.0
	case SeverityMedium:
		return 40.0
	case SeverityLow:
		return 15.0
	default:
		return 5.0
	}
}

// Finding is one detector hit on one tool.
type Finding struct {
	RuleID       string             `json:"rule_id"`
	Category     DetectorCategory   `json:"category"`
	Severity     Severity           `json:"severity"`
	FixType      FixType            `json:"fix_type"`
	ToolName     string             `json:"tool_name"`
	FilePath     string             `json:"file_path"`
	Line         int                `json:"line"`
	Title        string             `json:"title"`
	Explanation  string             `json:"explanation"` // "show your work" per doc §7
	SuggestedFix string             `json:"suggested_fix"`
	Confidence   float64            `json:"confidence"` // 0..1
	FixHints     map[string]any     `json:"fix_hints,omitempty"`
}

// BaseScore returns the flat 0–100 score for this finding: BaseScoreWeight × Confidence, 1 dp.
func (f Finding) BaseScore() float64 {
	return math.Round(BaseScoreWeight(f.Severity)*f.Confidence*10) / 10
}

// ToolReadiness is the per-tool score from the Scoring Engine.
type ToolReadiness struct {
	ToolName         string  `json:"tool_name"`
	Score            float64 `json:"score"` // 0..1
	FindingCount     int     `json:"finding_count"`
	WeightedSeverity float64 `json:"weighted_severity"`
	MaxBaseScore     float64 `json:"max_base_score"` // 0–100 flat score of worst finding
}

// ScanManifest is what the Normalizer produces.
type ScanManifest struct {
	RepoRoot               string           `json:"repo_root"`
	IsRemote               bool             `json:"is_remote"`
	RemoteURL              string           `json:"remote_url,omitempty"`
	PythonFiles            []string         `json:"python_files"`
	TypeScriptFiles        []string         `json:"typescript_files,omitempty"`
	JavaScriptFiles        []string         `json:"javascript_files,omitempty"`
	YAMLFiles              []string         `json:"yaml_files"`
	JSONFiles              []string         `json:"json_files,omitempty"`
	MarkdownFiles          []string         `json:"markdown_files,omitempty"`
	HasClaudeSDKDependency bool             `json:"has_claude_sdk_dependency"`
	HasOpenShellArtifact   bool             `json:"has_openshell_artifact"`
	Components             []AgentComponent `json:"components,omitempty"`
}

// SDK identifies a tool/agent SDK we know about.
type SDK string

const (
	SDKClaudeAgentSDK SDK = "claude_agent_sdk"
	SDKOpenAIAgents   SDK = "openai_agents"
	SDKMCP            SDK = "mcp"
	SDKOpenShell      SDK = "openshell"
)

type SDKDep struct {
	Name       string  `json:"name"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
}

// RepoProfile is the output of Phase 1 (reconnaissance).
type RepoProfile struct {
	Languages []Language   `json:"languages"`
	SDKDeps   []SDKDep     `json:"sdk_deps"`
	Manifest  ScanManifest `json:"manifest"`
}

// RepoInventory is the output of Phase 2a.
// AgentDef, GuardrailDef, SessionUse, HostedToolDef are in agent.go.
type RepoInventory struct {
	Tools              []ToolDef       `json:"tools"`
	Agents             []AgentDef      `json:"agents"`
	Guardrails         []GuardrailDef  `json:"guardrails"`
	Sessions           []SessionUse    `json:"sessions"`
	HostedTools        []HostedToolDef `json:"hosted_tools"`
	SDKsDetected       []SDK           `json:"sdks_detected"`
	Manifest           ScanManifest    `json:"manifest"` // convenience copy for repo-scope predicates
	UsesDefaultTracing bool            `json:"uses_default_tracing"`
}

// GeneratedArtifact is a file the generators want to write into the user's repo.
type GeneratedArtifact struct {
	RelativePath string           `json:"relative_path"`
	Contents     string           `json:"contents"`
	Category     DetectorCategory `json:"category"`
	Rationale    string           `json:"rationale"`
}

// ScanResult is the top-level output. JSON-serializable for CI.
type ScanResult struct {
	ScanID             string              `json:"scan_id"`
	Repo               string              `json:"repo"`
	Languages          []Language          `json:"languages"`     // detected by file extension (Phase 1)
	SDKs               []SDK               `json:"sdks"`          // observed in code (Phase 2a)
	Manifest           ScanManifest        `json:"manifest"`
	Tools              []ToolDef           `json:"tools"`
	Agents             []AgentDef          `json:"agents"`
	Findings           []Finding           `json:"findings"`
	Readiness          []ToolReadiness     `json:"readiness"`
	OverallScore       float64             `json:"overall_score"`
	RiskScore          float64             `json:"risk_score"` // 0–100 flat base score of worst finding
	GeneratedArtifacts []GeneratedArtifact `json:"generated_artifacts"`
}
