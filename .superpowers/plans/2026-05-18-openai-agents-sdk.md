# OpenAI Agents SDK Detection (Python) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship 12 OpenAI Agents SDK detection rules across three scopes (tool / agent / repo) plus the engine generalizations they require: a two-phase scanning pipeline, three typed Detector interfaces, agent/guardrail/session discovery with cross-module symbol resolution, and data-driven policy selection with three engine-emitted META findings.

**Architecture:** Approach A (minimal extension) per [`.superpowers/specs/2026-05-18-openai-agents-sdk-design.md`](../specs/2026-05-18-openai-agents-sdk-design.md). The existing rules engine stays; we add a `scope:` field, split the Detector interface into three typed variants (`ToolDetector`, `AgentDetector`, `RepoDetector`), add a second discovery pass for agents/guardrails/sessions with edge resolution, and stage the scanner into Phase 1 (recon) → Phase 2a (inventory) → Phase 2b (policy selection) → Phase 2c (analysis).

**Tech Stack:** Go 1.22, `go-tree-sitter` (CGO), `embed.FS` for policy bundling, `gopkg.in/yaml.v3` with `KnownFields(true)`, Cobra CLI, `go test` for the test pyramid.

**Spec reference:** Every task ties back to the spec. When in doubt, the spec is source of truth.

---

## File structure

### New files
- `internal/models/agent.go` — `AgentDef`, `KwargTree`, `ToolRef`, `AgentRef`, `GuardrailRef`, `Expr`, `GuardrailDef`, `SessionUse`, `HostedToolDef`, `SDKDep`, `RepoProfile`, `RepoInventory`, `SDK` enum
- `internal/analysis/agents.go` — `DiscoverAgents`, `DiscoverGuardrails`, `DiscoverSessions`, `ResolveEdges`
- `internal/analysis/agents_test.go` — discovery + edge resolution unit tests
- `internal/rules/policies/openai_sdk/tool_definition.yaml` — OAI-001, OAI-002
- `internal/rules/policies/openai_sdk/decorator_config.yaml` — OAI-003, OAI-004
- `internal/rules/policies/openai_sdk/network.yaml` — OAI-005
- `internal/rules/policies/openai_sdk/path_safety.yaml` — OAI-006
- `internal/rules/policies/openai_sdk/agent_safety.yaml` — OAI-101, OAI-102, OAI-103, OAI-104
- `internal/rules/policies/openai_sdk/mcp_safety.yaml` — OAI-105
- `internal/rules/policies/openai_sdk/tracing.yaml` — OAI-201
- `internal/rules/policies/openai_sdk/README.md` — OpenAI rule pack overview + supported SDK version
- `internal/scanner/policy_selection.go` — `LoadFor`, META-001/002/003 emission
- `internal/scanner/policy_selection_test.go` — META finding tests
- `internal/scanner/determinism_test.go` — byte-equality regression test
- `testdata/deterministic-fixture/` — small controlled agent for determinism test
- `.github/workflows/test.yml` — CI workflow

### Modified files
- `internal/models/models.go` — extend `ToolDef` with `Config map[string]string`, add `Language`/`SDK` constants for OpenAI, remove `HasClaudeSDKDependency`/`HasOpenShellArtifact`
- `internal/rules/schema.go` — add `Scope`, remove `Singleton`, add agent/repo predicate fields to `MatchExpr`
- `internal/rules/schema.yaml` — document `scope:` field, agent/repo predicates, per-scope `applies_to`
- `internal/rules/predicates.go` — `PredToolDecoratorKwargValue/Present`, all agent predicates, all repo predicates
- `internal/rules/predicates_test.go` — fire/silent for each new predicate
- `internal/rules/evaluator.go` — dispatch new predicate fields; remove `Singleton` handling
- `internal/rules/loader.go` — validate `scope` and per-scope `applies_to` values; add `LoadFor`
- `internal/rules/rule_detector.go` — split into `toolRuleDetector`, `agentRuleDetector`, `repoRuleDetector`
- `internal/rules/policies_test.go` — extend `policyRuleCases` with scope dispatch; add OAI rules
- `internal/rules/policies/claude_sdk/*.yaml` — add `scope: tool` to each rule
- `internal/rules/policies/openshell/*.yaml` — add `scope: tool` to OSH-001/002/003/005; OSH-004 becomes `scope: repo`
- `internal/analysis/detectors/detector.go` — three typed interfaces, refactored Registry
- `internal/analysis/discovery.go` — capture decorator kwargs into `ToolDef.Config`
- `internal/ingestion/normalizer.go` — emit `SDKDep []SDKDep` instead of vestigial booleans; rename to `Recon` returning `RepoProfile`
- `internal/scanner/scanner.go` — wire Phase 1 → 2a → 2b → 2c
- `ARCHITECTURE.md` — describe new pipeline
- `README.md` — update detector table
- `internal/rules/policies/CLAUDE.md` — per-scope `applies_to` values

### Files deleted (in cleanup task)
- The `call_uses_param` predicate, `CallUsesParamExpr` struct, related YAML schema docs (no shipped rule uses it; documented as inferior to `call_uses_unnormalized_path_param`)

---

# Phase A: Schema + Detector Interface

## Task 1: Add `Scope` field to `RuleDef`

**Files:**
- Modify: `internal/rules/schema.go`
- Modify: `internal/models/models.go` (add `Scope` enum constants)

- [ ] **Step 1: Define the Scope enum**

In `internal/models/models.go`, add:

```go
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
```

- [ ] **Step 2: Add Scope to RuleDef**

In `internal/rules/schema.go`, find the `RuleDef` struct and add the `Scope` field:

```go
type RuleDef struct {
    ID          string                  `yaml:"id"`
    Title       string                  `yaml:"title"`
    Scope       models.Scope            `yaml:"scope"`       // NEW: required
    Severity    models.Severity         `yaml:"severity"`
    Confidence  float64                 `yaml:"confidence"`
    Language    models.Language         `yaml:"language"`
    AppliesTo   []string                `yaml:"applies_to"`
    // Singleton field REMOVED in Task 4
    Match       MatchExpr               `yaml:"match"`
    Explanation string                  `yaml:"explanation"`
    Fix         string                  `yaml:"fix"`
    FixHints    map[string]any          `yaml:"fix_hints,omitempty"`
    Category    models.DetectorCategory `yaml:"-"`
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/models/models.go internal/rules/schema.go
git commit -m "feat(rules): add Scope field to RuleDef (tool/agent/repo)"
```

---

## Task 2: Validate scope at load time

**Files:**
- Modify: `internal/rules/loader.go`
- Modify: `internal/rules/loader_test.go`

- [ ] **Step 1: Write failing tests for scope validation**

Append to `internal/rules/loader_test.go`:

```go
func TestLoad_RejectsMissingScope(t *testing.T) {
    fs := fstest.MapFS{
        "test/rule.yaml": &fstest.MapFile{Data: []byte(`
policy:
  id: test
  name: Test
  category: claude_sdk
rules:
  - id: TEST-001
    title: Missing scope
    severity: low
    confidence: 0.5
    applies_to: [claude_sdk_tool]
    match: {has_docstring: false}
    explanation: x
    fix: x
`)},
    }
    _, err := rules.Load(fs)
    if err == nil || !strings.Contains(err.Error(), "scope") {
        t.Fatalf("expected scope-required error, got %v", err)
    }
}

func TestLoad_RejectsUnknownScope(t *testing.T) {
    fs := fstest.MapFS{
        "test/rule.yaml": &fstest.MapFile{Data: []byte(`
policy:
  id: test
  name: Test
  category: claude_sdk
rules:
  - id: TEST-001
    title: Bad scope
    scope: tooooool
    severity: low
    confidence: 0.5
    applies_to: [claude_sdk_tool]
    match: {has_docstring: false}
    explanation: x
    fix: x
`)},
    }
    _, err := rules.Load(fs)
    if err == nil || !strings.Contains(err.Error(), "scope") {
        t.Fatalf("expected unknown-scope error, got %v", err)
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
go test ./internal/rules/ -run TestLoad_Rejects -v
```
Expected: FAIL — validation not implemented.

- [ ] **Step 3: Add validation in loader.go**

In `internal/rules/loader.go`, find the per-rule validation block (where severity/confidence/applies_to are checked) and add:

```go
if r.Scope == "" {
    errs = append(errs, fmt.Errorf("rule %s: scope is required (tool|agent|repo)", r.ID))
} else if !models.ValidScope(r.Scope) {
    errs = append(errs, fmt.Errorf("rule %s: unknown scope %q (allowed: tool, agent, repo)", r.ID, r.Scope))
}
```

- [ ] **Step 4: Add per-scope `applies_to` validation**

In the same validation block, after the scope check, add:

```go
if models.ValidScope(r.Scope) {
    for _, kind := range r.AppliesTo {
        if !validAppliesToForScope(r.Scope, kind) {
            errs = append(errs, fmt.Errorf("rule %s: applies_to value %q is not valid for scope %q", r.ID, kind, r.Scope))
        }
    }
}
```

And add the helper at the bottom of `loader.go`:

```go
func validAppliesToForScope(scope models.Scope, kind string) bool {
    switch scope {
    case models.ScopeTool:
        switch kind {
        case "claude_sdk_tool", "openai_tool", "mcp_tool", "shell_invocation", "unknown":
            return true
        }
    case models.ScopeAgent:
        switch kind {
        case "openai_agent", "openai_sandbox_agent", "claude_agent_definition":
            return true
        }
    case models.ScopeRepo:
        switch kind {
        case "claude_sdk", "openai_agents", "openshell", "mcp":
            return true
        }
    }
    return false
}
```

- [ ] **Step 5: Run tests to verify pass**

```
go test ./internal/rules/ -run TestLoad_Rejects -v
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/rules/loader.go internal/rules/loader_test.go
git commit -m "feat(rules): validate scope and per-scope applies_to at load"
```

---

## Task 3: Migrate existing rules to set `scope:` explicitly

**Files:**
- Modify: `internal/rules/policies/claude_sdk/tool_definition.yaml`
- Modify: `internal/rules/policies/claude_sdk/network.yaml`
- Modify: `internal/rules/policies/claude_sdk/path_safety.yaml`
- Modify: `internal/rules/policies/claude_sdk/error_handling.yaml`
- Modify: `internal/rules/policies/claude_sdk/idempotency.yaml`
- Modify: `internal/rules/policies/openshell/shell.yaml`
- Modify: `internal/rules/policies/openshell/filesystem.yaml`
- Modify: `internal/rules/policies/openshell/network.yaml`
- Modify: `internal/rules/policies/openshell/resources.yaml` (OSH-004 → scope: repo)

- [ ] **Step 1: Add `scope: tool` to every CSDK rule and most OSH rules**

In each YAML file under `policies/claude_sdk/` and `policies/openshell/shell.yaml`, `filesystem.yaml`, `network.yaml`, add the `scope:` line after `title:`:

```yaml
rules:
  - id: CSDK-001
    title: Tool function has no docstring / description
    scope: tool                                # ADD THIS LINE
    severity: low
    ...
```

Repeat for all rules in: tool_definition.yaml (CSDK-001, 002, 007), network.yaml (CSDK-003), path_safety.yaml (CSDK-004), error_handling.yaml (CSDK-005), idempotency.yaml (CSDK-006), shell.yaml (OSH-001, OSH-002), filesystem.yaml (OSH-003), network.yaml (OSH-005).

- [ ] **Step 2: Set OSH-004 to `scope: repo`**

In `internal/rules/policies/openshell/resources.yaml`, change OSH-004:

```yaml
rules:
  - id: OSH-004
    title: No OpenShell resource limits configured
    scope: repo                                 # NEW
    severity: medium
    confidence: 0.95
    language: python
    applies_to:                                 # CHANGED: was tool kinds
      - openshell
    # singleton: true                           # REMOVE — replaced by scope: repo
    match: {}
    explanation: >
      ...
```

- [ ] **Step 3: Run tests to verify migration**

```
go test ./internal/rules/
```
Expected: PASS (after Task 2 validates scope; before Task 4 removes `singleton:`, the field will still parse).

- [ ] **Step 4: Commit**

```bash
git add internal/rules/policies/
git commit -m "feat(rules): migrate shipped rules to explicit scope field; OSH-004 -> scope: repo"
```

---

## Task 4: Remove `singleton` field from schema and evaluator

**Files:**
- Modify: `internal/rules/schema.go`
- Modify: `internal/rules/evaluator.go`
- Modify: `internal/rules/rule_detector.go`
- Modify: `internal/rules/policies/openshell/resources.yaml` (remove the comment leftover)

- [ ] **Step 1: Remove `Singleton` field from RuleDef**

In `internal/rules/schema.go`, delete the `Singleton bool` field from `RuleDef`.

- [ ] **Step 2: Remove `Singleton()` from any RuleDetector / Detector usage**

Search for `Singleton` in `internal/rules/rule_detector.go` and `internal/rules/evaluator.go`. Remove all references. (The replacement logic comes in Task 6 when we split the Detector interface — for this task, just remove the dead code.)

- [ ] **Step 3: Remove any leftover `singleton:` comments in YAML**

In `internal/rules/policies/openshell/resources.yaml`, remove the stray comment line if present:
```yaml
    # singleton: true                           # REMOVE — replaced by scope: repo
```

- [ ] **Step 4: Run all tests**

```
go test ./...
```
Expected: tests may fail on `Detector.Singleton()` calls in `internal/analysis/detectors/detector.go` — those will be removed in Task 5. For now, expect compile errors there.

- [ ] **Step 5: Commit** (after Task 5 unblocks the compile)

Defer commit until Task 5 lands; this task and Task 5 ship together.

---

## Task 5: Split `Detector` into three typed interfaces

**Files:**
- Modify: `internal/analysis/detectors/detector.go`

- [ ] **Step 1: Define three interfaces**

Replace the body of `internal/analysis/detectors/detector.go` with:

```go
package detectors

import (
    "sort"
    "github.com/trustabl/trustabl/internal/analysis"
    "github.com/trustabl/trustabl/internal/models"
)

type ToolDetector interface {
    RuleID() string
    Category() models.DetectorCategory
    Applies(models.ToolDef) bool
    Detect(models.ToolDef, analysis.ParsedFile, models.RepoInventory) []models.Finding
}

type AgentDetector interface {
    RuleID() string
    Category() models.DetectorCategory
    Applies(models.AgentDef) bool
    Detect(models.AgentDef, models.RepoInventory) []models.Finding
}

type RepoDetector interface {
    RuleID() string
    Category() models.DetectorCategory
    Applies(models.RepoProfile, models.RepoInventory) bool
    Detect(models.RepoProfile, models.RepoInventory) []models.Finding
}

type Registry struct {
    tool  []ToolDetector
    agent []AgentDetector
    repo  []RepoDetector
}

func New(tool []ToolDetector, agent []AgentDetector, repo []RepoDetector) *Registry {
    return &Registry{tool: tool, agent: agent, repo: repo}
}

func (r *Registry) Run(profile models.RepoProfile, inv models.RepoInventory, parsed []analysis.ParsedFile) []models.Finding {
    var out []models.Finding
    for _, d := range r.tool {
        for _, t := range inv.Tools {
            if !d.Applies(t) { continue }
            pf := parsedFor(t.FilePath, parsed)
            out = append(out, d.Detect(t, pf, inv)...)
        }
    }
    for _, d := range r.agent {
        for _, a := range inv.Agents {
            if !d.Applies(a) { continue }
            out = append(out, d.Detect(a, inv)...)
        }
    }
    for _, d := range r.repo {
        if !d.Applies(profile, inv) { continue }
        out = append(out, d.Detect(profile, inv)...)
    }
    sort.SliceStable(out, func(i, j int) bool {
        if out[i].RuleID != out[j].RuleID { return out[i].RuleID < out[j].RuleID }
        if out[i].FilePath != out[j].FilePath { return out[i].FilePath < out[j].FilePath }
        return out[i].Line < out[j].Line
    })
    return out
}

func (r *Registry) Subset(cats ...models.DetectorCategory) *Registry {
    cset := make(map[models.DetectorCategory]bool, len(cats))
    for _, c := range cats { cset[c] = true }
    var sub Registry
    for _, d := range r.tool  { if cset[d.Category()] { sub.tool  = append(sub.tool,  d) } }
    for _, d := range r.agent { if cset[d.Category()] { sub.agent = append(sub.agent, d) } }
    for _, d := range r.repo  { if cset[d.Category()] { sub.repo  = append(sub.repo,  d) } }
    return &sub
}

func parsedFor(filePath string, parsed []analysis.ParsedFile) analysis.ParsedFile {
    for _, pf := range parsed {
        if pf.RelPath == filePath { return pf }
    }
    return analysis.ParsedFile{}
}
```

- [ ] **Step 2: Run build**

```
go build ./...
```
Expected: failures in `internal/rules/rule_detector.go` (next task) and `internal/scanner/scanner.go`. These are fixed in Tasks 6 and 13.

- [ ] **Step 3: Commit (combined with Task 4)**

```bash
git add internal/rules/schema.go internal/rules/evaluator.go internal/rules/rule_detector.go internal/analysis/detectors/detector.go internal/rules/policies/openshell/resources.yaml
git commit -m "refactor(detectors): split Detector into ToolDetector/AgentDetector/RepoDetector; remove Singleton"
```

(This commit will leave the build broken until Task 6 lands. That's intentional — the next task is small and immediate.)

---

## Task 6: Refactor `RuleDetector` into three scope-specific types

**Files:**
- Modify: `internal/rules/rule_detector.go`

- [ ] **Step 1: Replace RuleDetector with three scope-specific types**

Replace the body of `internal/rules/rule_detector.go`:

```go
package rules

import (
    "github.com/trustabl/trustabl/internal/analysis"
    "github.com/trustabl/trustabl/internal/analysis/detectors"
    "github.com/trustabl/trustabl/internal/models"
)

type toolRuleDetector struct{ rule RuleDef }

func (d toolRuleDetector) RuleID() string                       { return d.rule.ID }
func (d toolRuleDetector) Category() models.DetectorCategory    { return d.rule.Category }
func (d toolRuleDetector) Applies(t models.ToolDef) bool {
    if d.rule.Language != "" && d.rule.Language != t.Language { return false }
    for _, k := range d.rule.AppliesTo { if string(t.Kind) == k { return true } }
    return false
}
func (d toolRuleDetector) Detect(t models.ToolDef, pf analysis.ParsedFile, inv models.RepoInventory) []models.Finding {
    if !d.rule.Match.EvaluateTool(t, pf) { return nil }
    return []models.Finding{findingFromRule(d.rule, t.FilePath, t.Line, t.Name)}
}

type agentRuleDetector struct{ rule RuleDef }

func (d agentRuleDetector) RuleID() string                       { return d.rule.ID }
func (d agentRuleDetector) Category() models.DetectorCategory    { return d.rule.Category }
func (d agentRuleDetector) Applies(a models.AgentDef) bool {
    for _, k := range d.rule.AppliesTo { if agentKindMatches(k, a) { return true } }
    return false
}
func (d agentRuleDetector) Detect(a models.AgentDef, inv models.RepoInventory) []models.Finding {
    if !d.rule.Match.EvaluateAgent(a, inv) { return nil }
    return []models.Finding{findingFromRule(d.rule, a.FilePath, a.Line, a.Name)}
}

type repoRuleDetector struct{ rule RuleDef }

func (d repoRuleDetector) RuleID() string                       { return d.rule.ID }
func (d repoRuleDetector) Category() models.DetectorCategory    { return d.rule.Category }
func (d repoRuleDetector) Applies(p models.RepoProfile, inv models.RepoInventory) bool {
    for _, k := range d.rule.AppliesTo {
        for _, sdk := range inv.SDKsDetected { if string(sdk) == k { return true } }
    }
    return false
}
func (d repoRuleDetector) Detect(p models.RepoProfile, inv models.RepoInventory) []models.Finding {
    if !d.rule.Match.EvaluateRepo(p, inv) { return nil }
    return []models.Finding{findingFromRule(d.rule, "", 0, "")}
}

func agentKindMatches(kind string, a models.AgentDef) bool {
    switch kind {
    case "openai_agent":         return a.SDK == models.SDKOpenAIAgents && a.Class == "Agent"
    case "openai_sandbox_agent": return a.SDK == models.SDKOpenAIAgents && a.Class == "SandboxAgent"
    case "claude_agent_definition": return a.SDK == models.SDKClaudeAgentSDK && a.Class == "AgentDefinition"
    }
    return false
}

// Exported constructors so external test packages (e.g. policies_test.go in
// package rules_test) can build typed detectors directly from a RuleDef.
func NewToolRuleDetector(r RuleDef) detectors.ToolDetector   { return toolRuleDetector{r} }
func NewAgentRuleDetector(r RuleDef) detectors.AgentDetector { return agentRuleDetector{r} }
func NewRepoRuleDetector(r RuleDef) detectors.RepoDetector   { return repoRuleDetector{r} }

func findingFromRule(r RuleDef, filePath string, line int, toolName string) models.Finding {
    return models.Finding{
        RuleID:       r.ID,
        Category:     r.Category,
        Severity:     r.Severity,
        ToolName:     toolName,
        FilePath:     filePath,
        Line:         line,
        Title:        r.Title,
        Explanation:  r.Explanation,
        SuggestedFix: r.Fix,
        Confidence:   r.Confidence,
        FixHints:     r.FixHints,
    }
}
```

`EvaluateTool`, `EvaluateAgent`, `EvaluateRepo` are added in Task 29 alongside the evaluator dispatch refactor. For now, this code won't compile because those methods don't exist yet.

**Coupled-refactor note:** Tasks 4 (remove `singleton`), 5 (split `Detector` interface), 6 (split `RuleDetector`), and 29 (split `Evaluate`) are a single coupled refactor. The build will be RED between Task 4 and Task 29, and the existing per-rule tests (which call `d.Detect(tool, pf)` with the old two-arg signature) won't compile until Task 45 updates them. **Do not stop and declare green at the boundary of Tasks 4-6 or 29; the green-at-boundary discipline resumes at Task 30.** All five tasks ship as a single logical change even though they're separate commits.

- [ ] **Step 2: Update LoadRegistry to build three slices**

In `internal/rules/loader.go`, update `LoadRegistry` to return a `*detectors.Registry` built with three slices:

```go
func LoadRegistry(fsys fs.FS) (*detectors.Registry, error) {
    policies, err := Load(fsys)
    if err != nil { return nil, err }
    var tool []detectors.ToolDetector
    var agent []detectors.AgentDetector
    var repo []detectors.RepoDetector
    for _, p := range policies {
        for _, r := range p.Rules {
            switch r.Scope {
            case models.ScopeTool:  tool  = append(tool,  toolRuleDetector{r})
            case models.ScopeAgent: agent = append(agent, agentRuleDetector{r})
            case models.ScopeRepo:  repo  = append(repo,  repoRuleDetector{r})
            }
        }
    }
    return detectors.New(tool, agent, repo), nil
}
```

- [ ] **Step 3: Build**

```
go build ./...
```
Expected: still failing on `Evaluate*` methods. That's OK — Task 33 introduces them.

- [ ] **Step 4: Commit**

```bash
git add internal/rules/rule_detector.go internal/rules/loader.go
git commit -m "refactor(rules): split RuleDetector into per-scope types"
```

---

# Phase B: Two-Phase Pipeline

## Task 7: Define `RepoProfile`, `SDKDep`, `SDK` types

**Files:**
- Modify: `internal/models/models.go`

- [ ] **Step 1: Add SDK enum + SDKDep + RepoProfile**

Append to `internal/models/models.go`:

```go
// SDK identifies a tool/agent SDK we know about.
type SDK string

const (
    SDKClaudeAgentSDK SDK = "claude_agent_sdk"
    SDKOpenAIAgents   SDK = "openai_agents"
    SDKMCP            SDK = "mcp"
)

type SDKDep struct {
    Name       string  `json:"name"`        // "claude-agent-sdk", "openai-agents", ...
    Source     string  `json:"source"`      // path to manifest file
    Confidence float64 `json:"confidence"`
}

// RepoProfile is the output of Phase 1 (reconnaissance).
type RepoProfile struct {
    Languages []Language   `json:"languages"`
    SDKDeps   []SDKDep     `json:"sdk_deps"`
    Manifest  ScanManifest `json:"manifest"`
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/models/models.go
git commit -m "feat(models): add SDK, SDKDep, RepoProfile types for Phase 1"
```

---

## Task 8: Define `RepoInventory` + supporting types

**Files:**
- Create: `internal/models/agent.go`

- [ ] **Step 1: Create agent.go with full type definitions**

```go
package models

// KwargTree represents a kwarg value as either a leaf (Value) or a nested
// tree (Children, e.g. for model_settings.tool_choice).
type KwargTree struct {
    Value    *Expr                  `json:"value,omitempty"`
    Children map[string]*KwargTree  `json:"children,omitempty"`
}

// Expr is a typed wrapper around a parsed AST node.
type Expr struct {
    Kind ExprKind `json:"kind"`
    Text string   `json:"text"`             // raw source text
    List []Expr   `json:"list,omitempty"`   // populated when Kind == ExprList
}

type ExprKind string

const (
    ExprLiteralString ExprKind = "literal_string"
    ExprLiteralInt    ExprKind = "literal_int"
    ExprLiteralBool   ExprKind = "literal_bool"
    ExprLiteralNone   ExprKind = "literal_none"
    ExprNameRef       ExprKind = "name_ref"
    ExprList          ExprKind = "list"
    ExprCall          ExprKind = "call"
    ExprUnknown       ExprKind = "unknown"
)

type ToolRef struct {
    Name     string   `json:"name"`
    Resolved *ToolDef `json:"-"`
    External bool     `json:"external"`
}

type AgentRef struct {
    Name     string    `json:"name"`
    Resolved *AgentDef `json:"-"`
    External bool      `json:"external"`
}

type GuardrailRef struct {
    Name     string        `json:"name"`
    Resolved *GuardrailDef `json:"-"`
    External bool          `json:"external"`
}

type AgentDef struct {
    SDK          SDK                  `json:"sdk"`
    Class        string               `json:"class"`         // "Agent", "SandboxAgent", "AgentDefinition"
    FilePath     string               `json:"file_path"`
    Line         int                  `json:"line"`
    EndLine      int                  `json:"end_line"`
    Name         string               `json:"name"`          // from name= kwarg literal
    Kwargs       *KwargTree           `json:"kwargs"`
    ToolRefs     []ToolRef            `json:"tool_refs"`
    HandoffRefs  []AgentRef           `json:"handoff_refs"`
    InputGuards  []GuardrailRef       `json:"input_guards"`
    OutputGuards []GuardrailRef       `json:"output_guards"`
    Opaque       bool                 `json:"opaque"`        // true if Agent(**config) or tools=non-literal
}

type GuardrailKind string

const (
    GuardrailInput  GuardrailKind = "input"
    GuardrailOutput GuardrailKind = "output"
)

type GuardrailDef struct {
    Name     string        `json:"name"`
    Kind     GuardrailKind `json:"kind"`
    FilePath string        `json:"file_path"`
    Line     int           `json:"line"`
}

type SessionUse struct {
    Class    string `json:"class"`         // "SQLiteSession", "EncryptedSession", ...
    FilePath string `json:"file_path"`
    Line     int    `json:"line"`
}

type HostedToolDef struct {
    Class    string     `json:"class"`     // "WebSearchTool", "ComputerTool", ...
    FilePath string     `json:"file_path"`
    Line     int        `json:"line"`
    Kwargs   *KwargTree `json:"kwargs"`
}

// RepoInventory is the output of Phase 2a.
type RepoInventory struct {
    Tools        []ToolDef       `json:"tools"`
    Agents       []AgentDef      `json:"agents"`
    Guardrails   []GuardrailDef  `json:"guardrails"`
    Sessions     []SessionUse    `json:"sessions"`
    HostedTools  []HostedToolDef `json:"hosted_tools"`
    SDKsDetected []SDK           `json:"sdks_detected"`
}
```

- [ ] **Step 2: Build to verify types compile**

```
go build ./internal/models/
```
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/models/agent.go
git commit -m "feat(models): add AgentDef, KwargTree, RepoInventory, and supporting types"
```

---

## Task 9: Extend `ToolDef` with `Config` map; add Language constant for OpenAI

**Files:**
- Modify: `internal/models/models.go`

- [ ] **Step 1: Add Config map to ToolDef**

In `internal/models/models.go`, find the `ToolDef` struct and add:

```go
type ToolDef struct {
    Name           string            `json:"name"`
    Kind           ToolKind          `json:"kind"`
    Language       Language          `json:"language"`
    FilePath       string            `json:"file_path"`
    Line, EndLine  int               `json:"line"`
    Description    string            `json:"description"`
    HasTypedParams bool              `json:"has_typed_params"`
    ParamNames     []string          `json:"param_names"`
    Facts          map[string]string `json:"facts,omitempty"`
    Config         map[string]string `json:"config,omitempty"` // NEW: decorator kwargs
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/models/models.go
git commit -m "feat(models): add ToolDef.Config for decorator kwargs"
```

---

## Task 10: Implement `ingestion.Recon` — Phase 1

**Files:**
- Modify: `internal/ingestion/normalizer.go`

- [ ] **Step 1: Add Recon function that returns RepoProfile**

Append to `internal/ingestion/normalizer.go`:

```go
// Recon is the Phase 1 entrypoint. It walks the source tree (cheap, no AST)
// and returns a typed RepoProfile capturing languages, SDK deps, and the
// existing ScanManifest.
func Recon(src *Source) (models.RepoProfile, error) {
    manifest, err := Normalize(src)
    if err != nil { return models.RepoProfile{}, err }

    langs := languagesFromManifest(manifest)
    sdks  := detectSDKDeps(src.RootPath)

    return models.RepoProfile{
        Languages: langs,
        SDKDeps:   sdks,
        Manifest:  manifest,
    }, nil
}

func languagesFromManifest(m models.ScanManifest) []models.Language {
    var langs []models.Language
    if len(m.PythonFiles) > 0     { langs = append(langs, models.LanguagePython) }
    if len(m.TypeScriptFiles) > 0 { langs = append(langs, models.LanguageTypeScript) }
    if len(m.JavaScriptFiles) > 0 { langs = append(langs, models.LanguageJavaScript) }
    return langs
}

// detectSDKDeps scans pyproject.toml / requirements.txt / Pipfile / poetry.lock /
// package.json / go.mod for known SDK package names. Returns SDKDep entries
// with the source manifest path.
func detectSDKDeps(root string) []models.SDKDep {
    type needle struct {
        Name      string  // canonical SDK package name
        Pattern   string  // lowercased substring to match
        Manifests []string
    }
    needles := []needle{
        {Name: "claude-agent-sdk", Pattern: "claude-agent-sdk",
         Manifests: []string{"pyproject.toml", "requirements.txt", "Pipfile", "poetry.lock"}},
        {Name: "claude-agent-sdk", Pattern: "claude_agent_sdk",
         Manifests: []string{"pyproject.toml", "requirements.txt", "Pipfile", "poetry.lock"}},
        {Name: "openai-agents", Pattern: "openai-agents",
         Manifests: []string{"pyproject.toml", "requirements.txt", "Pipfile", "poetry.lock"}},
        {Name: "openai-agents", Pattern: "@openai/agents",
         Manifests: []string{"package.json"}},
    }
    seen := make(map[string]bool)
    var out []models.SDKDep
    for _, n := range needles {
        for _, mfile := range n.Manifests {
            path := filepath.Join(root, mfile)
            b, err := os.ReadFile(path)
            if err != nil { continue }
            if strings.Contains(strings.ToLower(string(b)), n.Pattern) {
                key := n.Name + "@" + mfile
                if seen[key] { continue }
                seen[key] = true
                out = append(out, models.SDKDep{
                    Name:       n.Name,
                    Source:     mfile,
                    Confidence: 0.9,
                })
            }
        }
    }
    sort.Slice(out, func(i, j int) bool {
        if out[i].Name != out[j].Name { return out[i].Name < out[j].Name }
        return out[i].Source < out[j].Source
    })
    return out
}
```

- [ ] **Step 2: Add import for `sort` if not already present**

Check the imports block in normalizer.go and add `"sort"` if missing.

- [ ] **Step 3: Commit**

```bash
git add internal/ingestion/normalizer.go
git commit -m "feat(ingestion): add Recon (Phase 1) returning RepoProfile with SDKDeps"
```

---

## Task 11: Remove vestigial `HasClaudeSDKDependency` / `HasOpenShellArtifact`

**Files:**
- Modify: `internal/models/models.go`
- Modify: `internal/ingestion/normalizer.go`

- [ ] **Step 1: Remove fields from ScanManifest**

In `internal/models/models.go`, find `ScanManifest` and remove:
```go
HasClaudeSDKDependency bool
HasOpenShellArtifact   bool
```

- [ ] **Step 2: Remove population logic in Normalize**

In `internal/ingestion/normalizer.go`, remove these lines from `Normalize`:
```go
manifest.HasClaudeSDKDependency = detectClaudeSDKDependency(src.RootPath)
manifest.HasOpenShellArtifact = detectOpenShellArtifact(manifest.YAMLFiles, src.RootPath)
```

And remove the now-unused `detectClaudeSDKDependency` and `detectOpenShellArtifact` functions (the new `detectSDKDeps` from Task 10 replaces them).

- [ ] **Step 3: Build**

```
go build ./...
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/models/models.go internal/ingestion/normalizer.go
git commit -m "refactor(models): remove vestigial HasClaudeSDKDependency/HasOpenShellArtifact (replaced by RepoProfile.SDKDeps)"
```

---

## Task 12: Wire `scanner.Run` through Phase 1 + 2a (existing tool discovery)

**Files:**
- Modify: `internal/scanner/scanner.go`

- [ ] **Step 1: Refactor scanner.Run**

Replace `Run` with:

```go
func Run(cfg Config) (models.ScanResult, error) {
    src, err := ingestion.Resolve(cfg.Target)
    if err != nil { return models.ScanResult{}, err }
    defer src.Cleanup()

    repoLabel := repoLabelFromSource(src)

    // Phase 1: reconnaissance
    profile, err := ingestion.Recon(src)
    if err != nil { return models.ScanResult{}, err }

    // Phase 2a: per-language inventory (Python only for now)
    tools, parsed, err := analysis.DiscoverTools(profile.Manifest)
    if err != nil { return models.ScanResult{}, err }
    inventory := models.RepoInventory{
        Tools:        tools,
        Agents:       nil,        // Phase C + D (Tasks 13-22) fill this in
        Guardrails:   nil,
        Sessions:     nil,
        SDKsDetected: deriveSDKsDetected(tools, nil),
    }

    // Phase 2b: policy selection (full implementation in Task 36)
    registry, err := rules.LoadRegistry(rules.DefaultFS())
    if err != nil { return models.ScanResult{}, err }
    if len(cfg.Categories) > 0 {
        registry = registry.Subset(cfg.Categories...)
    }

    // Phase 2c: analysis
    findings := registry.Run(profile, inventory, parsed)

    readiness, overall := analysis.Score(tools, findings)
    artifacts := append(
        generation.GenerateHooks(findings),
        generation.GeneratePolicy(findings, cfg.Version)...,
    )

    return models.ScanResult{
        ScanID:             scanID(repoLabel, profile.Manifest),
        Repo:               repoLabel,
        Manifest:           profile.Manifest,
        Tools:              tools,
        Findings:           findings,
        Readiness:          readiness,
        OverallScore:       overall,
        GeneratedArtifacts: artifacts,
    }, nil
}

// deriveSDKsDetected scans the inventory for tool/agent kinds that imply
// a specific SDK is in use. Extended in Task 22.
func deriveSDKsDetected(tools []models.ToolDef, agents []models.AgentDef) []models.SDK {
    seen := make(map[models.SDK]bool)
    for _, t := range tools {
        switch t.Kind {
        case models.KindClaudeSDKTool: seen[models.SDKClaudeAgentSDK] = true
        case models.KindOpenAITool:    seen[models.SDKOpenAIAgents]   = true
        case models.KindMCPTool:       seen[models.SDKMCP]            = true
        }
    }
    for _, a := range agents {
        seen[a.SDK] = true
    }
    var out []models.SDK
    for s := range seen { out = append(out, s) }
    sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
    return out
}
```

- [ ] **Step 2: Add `"sort"` import if missing**

- [ ] **Step 3: Run all tests**

```
go test ./...
```
Expected: PASS for tests not touching agent discovery; failures for any test asserting the removed `HasClaudeSDKDependency` (none expected).

- [ ] **Step 4: Commit**

```bash
git add internal/scanner/scanner.go
git commit -m "refactor(scanner): wire Run through Phase 1 -> 2a -> 2b -> 2c"
```

---

# Phase C: Agent / Guardrail / Session Discovery

## Task 13: Skeleton `internal/analysis/agents.go` with empty discovery functions

**Files:**
- Create: `internal/analysis/agents.go`

- [ ] **Step 1: Create the file with placeholder signatures**

```go
package analysis

import (
    "github.com/trustabl/trustabl/internal/models"
)

// DiscoverAgents walks each ParsedFile and returns AgentDef records for every
// Agent(...) / SandboxAgent(...) / AgentDefinition(...) constructor call.
func DiscoverAgents(files []ParsedFile) []models.AgentDef {
    var out []models.AgentDef
    for _, pf := range files {
        out = append(out, discoverAgentsInFile(pf)...)
    }
    return out
}

// DiscoverGuardrails finds @input_guardrail and @output_guardrail decorated
// functions. Class-based guardrails are NOT detected in v1 (documented limitation).
func DiscoverGuardrails(files []ParsedFile) []models.GuardrailDef {
    var out []models.GuardrailDef
    for _, pf := range files {
        out = append(out, discoverGuardrailsInFile(pf)...)
    }
    return out
}

// DiscoverSessions finds construction sites for *Session classes from the
// agents SDK (SQLiteSession, EncryptedSession, RedisSession, etc.).
func DiscoverSessions(files []ParsedFile) []models.SessionUse {
    var out []models.SessionUse
    for _, pf := range files {
        out = append(out, discoverSessionsInFile(pf)...)
    }
    return out
}

// Internal helpers — filled in by Tasks 14, 15, 16.
func discoverAgentsInFile(pf ParsedFile) []models.AgentDef        { return nil }
func discoverGuardrailsInFile(pf ParsedFile) []models.GuardrailDef { return nil }
func discoverSessionsInFile(pf ParsedFile) []models.SessionUse     { return nil }
```

- [ ] **Step 2: Commit**

```bash
git add internal/analysis/agents.go
git commit -m "feat(analysis): skeleton DiscoverAgents/Guardrails/Sessions"
```

---

## Task 14: Implement `DiscoverAgents` — Class + basic Kwargs

**Files:**
- Modify: `internal/analysis/agents.go`
- Create: `internal/analysis/agents_test.go`

- [ ] **Step 1: Write failing test for basic agent discovery**

```go
package analysis_test

import (
    "testing"
    "github.com/trustabl/trustabl/internal/analysis"
    "github.com/trustabl/trustabl/internal/models"
)

func TestDiscoverAgents_FindsOpenAIAgent(t *testing.T) {
    src := `
from agents import Agent

agent = Agent(
    name="ops",
    instructions="Run ops tasks.",
    model="gpt-4",
)
`
    pf := parsePyFile(t, "main.py", src)
    agents := analysis.DiscoverAgents([]analysis.ParsedFile{pf})
    if len(agents) != 1 {
        t.Fatalf("expected 1 agent, got %d", len(agents))
    }
    a := agents[0]
    if a.SDK != models.SDKOpenAIAgents { t.Errorf("SDK = %v, want SDKOpenAIAgents", a.SDK) }
    if a.Class != "Agent"              { t.Errorf("Class = %v, want Agent", a.Class) }
    if a.Name != "ops"                 { t.Errorf("Name = %v, want ops", a.Name) }
    if a.Kwargs == nil || a.Kwargs.Children["model"] == nil {
        t.Errorf("expected model kwarg captured")
    }
}

func TestDiscoverAgents_FindsSandboxAgent(t *testing.T) {
    src := `
from agents import SandboxAgent
agent = SandboxAgent(name="sb")
`
    pf := parsePyFile(t, "main.py", src)
    agents := analysis.DiscoverAgents([]analysis.ParsedFile{pf})
    if len(agents) != 1 || agents[0].Class != "SandboxAgent" {
        t.Fatalf("expected SandboxAgent, got %+v", agents)
    }
}

func TestDiscoverAgents_FindsClaudeAgentDefinition(t *testing.T) {
    src := `
from claude_agent_sdk import AgentDefinition
agent = AgentDefinition(name="claude")
`
    pf := parsePyFile(t, "main.py", src)
    agents := analysis.DiscoverAgents([]analysis.ParsedFile{pf})
    if len(agents) != 1 || agents[0].SDK != models.SDKClaudeAgentSDK {
        t.Fatalf("expected Claude agent, got %+v", agents)
    }
}
```

`parsePyFile` is a helper that wraps the existing tree-sitter Python parser into a `ParsedFile`. Add it to the test file:

```go
func parsePyFile(t *testing.T, path, src string) analysis.ParsedFile {
    t.Helper()
    // Use the same parser path discovery.go uses.
    parser := analysis.NewPythonParser()
    tree, err := parser.Parse([]byte(src))
    if err != nil { t.Fatalf("parse: %v", err) }
    return analysis.ParsedFile{RelPath: path, Source: []byte(src), Tree: tree}
}
```

(If `NewPythonParser` doesn't exist as a public helper, add a thin export in `discovery.go`.)

- [ ] **Step 2: Run tests — they should fail because discoverAgentsInFile is a stub**

```
go test ./internal/analysis/ -run TestDiscoverAgents -v
```
Expected: FAIL.

- [ ] **Step 3: Implement discoverAgentsInFile**

Replace the stub in `internal/analysis/agents.go`:

```go
import (
    "github.com/trustabl/trustabl/internal/analysis/astutil"
    sitter "github.com/smacker/go-tree-sitter"
    "strings"
)

// Map from imported symbol → (SDK, Class).
type agentImport struct {
    SDK   models.SDK
    Class string
}

func collectAgentImports(pf ParsedFile) map[string]agentImport {
    out := make(map[string]agentImport)
    astutil.Walk(pf.Tree.RootNode(), func(n *sitter.Node) bool {
        if n.Type() != "import_from_statement" { return true }
        moduleName := astutil.NodeText(n.ChildByFieldName("module_name"), pf.Source)
        var sdk models.SDK
        switch moduleName {
        case "agents":              sdk = models.SDKOpenAIAgents
        case "claude_agent_sdk":    sdk = models.SDKClaudeAgentSDK
        default:                    return true
        }
        // Walk the imported names; capture Agent, SandboxAgent, AgentDefinition.
        for i := 0; i < int(n.ChildCount()); i++ {
            child := n.Child(i)
            if child.Type() == "dotted_name" || child.Type() == "aliased_import" {
                name := astutil.NodeText(child, pf.Source)
                switch name {
                case "Agent", "SandboxAgent", "AgentDefinition":
                    out[name] = agentImport{SDK: sdk, Class: name}
                }
            }
        }
        return true
    })
    return out
}

func discoverAgentsInFile(pf ParsedFile) []models.AgentDef {
    imports := collectAgentImports(pf)
    if len(imports) == 0 { return nil }

    var out []models.AgentDef
    astutil.Walk(pf.Tree.RootNode(), func(n *sitter.Node) bool {
        if n.Type() != "call" { return true }
        funcName := astutil.NodeText(n.ChildByFieldName("function"), pf.Source)
        imp, ok := imports[funcName]
        if !ok { return true }

        kwargs, opaque := extractCallKwargs(n, pf.Source)
        a := models.AgentDef{
            SDK:      imp.SDK,
            Class:    imp.Class,
            FilePath: pf.RelPath,
            Line:     int(n.StartPoint().Row) + 1,
            EndLine:  int(n.EndPoint().Row) + 1,
            Kwargs:   kwargs,
            Opaque:   opaque,
        }
        if kwargs != nil && kwargs.Children["name"] != nil &&
           kwargs.Children["name"].Value != nil &&
           kwargs.Children["name"].Value.Kind == models.ExprLiteralString {
            a.Name = strings.Trim(kwargs.Children["name"].Value.Text, `"'`)
        }
        out = append(out, a)
        return true
    })
    return out
}
```

- [ ] **Step 4: Implement extractCallKwargs**

Add to `internal/analysis/agents.go`:

```go
// extractCallKwargs walks the argument_list of a `call` node and builds a
// KwargTree from keyword arguments. Returns opaque=true if the call uses
// **unpack (e.g. Agent(**config)).
func extractCallKwargs(callNode *sitter.Node, src []byte) (*models.KwargTree, bool) {
    args := callNode.ChildByFieldName("arguments")
    if args == nil { return nil, false }
    tree := &models.KwargTree{Children: map[string]*models.KwargTree{}}
    opaque := false
    for i := 0; i < int(args.ChildCount()); i++ {
        child := args.Child(i)
        switch child.Type() {
        case "keyword_argument":
            name := astutil.NodeText(child.ChildByFieldName("name"), src)
            value := child.ChildByFieldName("value")
            tree.Children[name] = exprFromNode(value, src)
        case "dictionary_splat":
            opaque = true   // Agent(**config) — kwargs not statically extractable
        }
    }
    if len(tree.Children) == 0 && !opaque { return nil, false }
    return tree, opaque
}

// exprFromNode converts a value AST node into our typed Expr.
func exprFromNode(n *sitter.Node, src []byte) *models.KwargTree {
    if n == nil { return nil }
    e := &models.Expr{Text: astutil.NodeText(n, src)}
    switch n.Type() {
    case "string":  e.Kind = models.ExprLiteralString
    case "integer": e.Kind = models.ExprLiteralInt
    case "true", "false": e.Kind = models.ExprLiteralBool
    case "none":    e.Kind = models.ExprLiteralNone
    case "identifier": e.Kind = models.ExprNameRef
    case "list":
        e.Kind = models.ExprList
        for i := 0; i < int(n.NamedChildCount()); i++ {
            child := n.NamedChild(i)
            childExpr := exprFromNode(child, src)
            if childExpr != nil && childExpr.Value != nil {
                e.List = append(e.List, *childExpr.Value)
            }
        }
    case "call":
        e.Kind = models.ExprCall
    case "attribute":
        // For nested kwargs like ModelSettings(tool_choice=...), the value
        // node may itself be a `call`. We capture it as a call expression.
        e.Kind = models.ExprNameRef
    default:
        e.Kind = models.ExprUnknown
    }
    // Special case: for `ModelSettings(tool_choice="required")` as a kwarg
    // value, descend into the call's kwargs so dotted-path lookups can find
    // model_settings.tool_choice.
    if n.Type() == "call" {
        inner, _ := extractCallKwargs(n, src)
        return &models.KwargTree{Value: e, Children: nilToEmpty(inner).Children}
    }
    return &models.KwargTree{Value: e}
}

func nilToEmpty(t *models.KwargTree) *models.KwargTree {
    if t == nil { return &models.KwargTree{Children: map[string]*models.KwargTree{}} }
    return t
}
```

- [ ] **Step 5: Run tests**

```
go test ./internal/analysis/ -run TestDiscoverAgents -v
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/analysis/agents.go internal/analysis/agents_test.go
git commit -m "feat(analysis): DiscoverAgents extracts Class + Kwargs as KwargTree"
```

---

## Task 15: Implement `DiscoverGuardrails`

**Files:**
- Modify: `internal/analysis/agents.go`
- Modify: `internal/analysis/agents_test.go`

- [ ] **Step 1: Add test**

```go
func TestDiscoverGuardrails_FindsInputAndOutput(t *testing.T) {
    src := `
from agents import input_guardrail, output_guardrail, GuardrailFunctionOutput

@input_guardrail
def check_input(ctx, agent, input):
    return GuardrailFunctionOutput(output_info=None, tripwire_triggered=False)

@output_guardrail
def check_output(ctx, agent, output):
    return GuardrailFunctionOutput(output_info=None, tripwire_triggered=False)
`
    pf := parsePyFile(t, "g.py", src)
    gs := analysis.DiscoverGuardrails([]analysis.ParsedFile{pf})
    if len(gs) != 2 { t.Fatalf("expected 2 guardrails, got %d", len(gs)) }
    var inputCount, outputCount int
    for _, g := range gs {
        if g.Kind == models.GuardrailInput  { inputCount++ }
        if g.Kind == models.GuardrailOutput { outputCount++ }
    }
    if inputCount != 1 || outputCount != 1 {
        t.Errorf("expected 1 input + 1 output, got %d input, %d output", inputCount, outputCount)
    }
}
```

- [ ] **Step 2: Implement**

Replace the stub in `agents.go`:

```go
func discoverGuardrailsInFile(pf ParsedFile) []models.GuardrailDef {
    var out []models.GuardrailDef
    astutil.Walk(pf.Tree.RootNode(), func(n *sitter.Node) bool {
        if n.Type() != "decorated_definition" { return true }
        decoratorText := decoratorBlockText(n, pf.Source)
        var kind models.GuardrailKind
        switch {
        case strings.Contains(decoratorText, "@input_guardrail"):  kind = models.GuardrailInput
        case strings.Contains(decoratorText, "@output_guardrail"): kind = models.GuardrailOutput
        default: return true
        }
        def := n.ChildByFieldName("definition")
        if def == nil { return true }
        name := astutil.FunctionName(def, pf.Source)
        out = append(out, models.GuardrailDef{
            Name: name, Kind: kind,
            FilePath: pf.RelPath, Line: int(def.StartPoint().Row) + 1,
        })
        return true
    })
    return out
}

func decoratorBlockText(decoratedDef *sitter.Node, src []byte) string {
    var b strings.Builder
    for i := 0; i < int(decoratedDef.ChildCount()); i++ {
        c := decoratedDef.Child(i)
        if c.Type() == "decorator" {
            b.WriteString(astutil.NodeText(c, src))
            b.WriteByte('\n')
        }
    }
    return b.String()
}
```

- [ ] **Step 3: Run tests, commit**

```
go test ./internal/analysis/ -run TestDiscoverGuardrails -v
```
PASS expected.

```bash
git add internal/analysis/agents.go internal/analysis/agents_test.go
git commit -m "feat(analysis): DiscoverGuardrails for @input_guardrail / @output_guardrail"
```

---

## Task 16: Implement `DiscoverSessions`

**Files:**
- Modify: `internal/analysis/agents.go`
- Modify: `internal/analysis/agents_test.go`

- [ ] **Step 1: Add test**

```go
func TestDiscoverSessions(t *testing.T) {
    src := `
from agents import SQLiteSession
session = SQLiteSession("convo")
`
    pf := parsePyFile(t, "s.py", src)
    ss := analysis.DiscoverSessions([]analysis.ParsedFile{pf})
    if len(ss) != 1 || ss[0].Class != "SQLiteSession" {
        t.Fatalf("expected SQLiteSession, got %+v", ss)
    }
}
```

- [ ] **Step 2: Implement**

```go
var sessionClasses = map[string]bool{
    "SQLiteSession":         true,
    "SQLAlchemySession":     true,
    "RedisSession":          true,
    "MongoDBSession":        true,
    "EncryptedSession":      true,
    "AdvancedSQLiteSession": true,
}

func discoverSessionsInFile(pf ParsedFile) []models.SessionUse {
    // Collect imports that bring in *Session names from the agents module.
    imported := make(map[string]bool)
    astutil.Walk(pf.Tree.RootNode(), func(n *sitter.Node) bool {
        if n.Type() != "import_from_statement" { return true }
        moduleName := astutil.NodeText(n.ChildByFieldName("module_name"), pf.Source)
        if !strings.HasPrefix(moduleName, "agents") { return true }
        for i := 0; i < int(n.ChildCount()); i++ {
            child := n.Child(i)
            if child.Type() == "dotted_name" {
                name := astutil.NodeText(child, pf.Source)
                if sessionClasses[name] { imported[name] = true }
            }
        }
        return true
    })
    if len(imported) == 0 { return nil }

    var out []models.SessionUse
    astutil.Walk(pf.Tree.RootNode(), func(n *sitter.Node) bool {
        if n.Type() != "call" { return true }
        funcName := astutil.NodeText(n.ChildByFieldName("function"), pf.Source)
        if imported[funcName] {
            out = append(out, models.SessionUse{
                Class: funcName, FilePath: pf.RelPath,
                Line: int(n.StartPoint().Row) + 1,
            })
        }
        return true
    })
    return out
}
```

- [ ] **Step 3: Run tests, commit**

```
go test ./internal/analysis/ -run TestDiscoverSessions -v
git add internal/analysis/agents.go internal/analysis/agents_test.go
git commit -m "feat(analysis): DiscoverSessions for *Session constructions"
```

---

## Task 17: Capture tool decorator kwargs into `ToolDef.Config`

**Files:**
- Modify: `internal/analysis/discovery.go`
- Modify: existing test that asserts ToolDef shape (predicates_test.go's parsePy if needed)

- [ ] **Step 1: Extract decorator kwargs during tool discovery**

In `internal/analysis/discovery.go`, find where `ToolDef` is built (inside `buildTool` or equivalent). After the kind is determined, extract the decorator's call kwargs:

```go
// After kind is determined, capture decorator kwargs.
// Walk the decorator block to find the @function_tool(...) or @tool(...) call.
config := map[string]string{}
for i := 0; i < int(decoratedDef.ChildCount()); i++ {
    decoratorNode := decoratedDef.Child(i)
    if decoratorNode.Type() != "decorator" { continue }
    // The decorator's body is either a `call` (decorator with args) or
    // an attribute/identifier (decorator without args).
    body := decoratorNode.NamedChild(0)
    if body == nil || body.Type() != "call" { continue }
    args := body.ChildByFieldName("arguments")
    if args == nil { continue }
    for j := 0; j < int(args.ChildCount()); j++ {
        arg := args.Child(j)
        if arg.Type() != "keyword_argument" { continue }
        name := astutil.NodeText(arg.ChildByFieldName("name"), src)
        value := astutil.NodeText(arg.ChildByFieldName("value"), src)
        config[name] = value
    }
}
tool.Config = config
```

(Place this block inside `buildTool` after `tool.Kind` is set.)

- [ ] **Step 2: Add test**

In `internal/analysis/agents_test.go` (or wherever discovery tests live):

```go
func TestDiscoverTools_CapturesDecoratorKwargs(t *testing.T) {
    src := `
from agents import function_tool

@function_tool(strict_mode=False, name_override="my_tool")
def do_thing(x: str) -> str:
    """Do a thing."""
    return x
`
    manifest := models.ScanManifest{
        RepoRoot:    t.TempDir(),
        PythonFiles: []string{"t.py"},
    }
    // Write src to the temp dir
    if err := os.WriteFile(filepath.Join(manifest.RepoRoot, "t.py"), []byte(src), 0644); err != nil {
        t.Fatal(err)
    }
    tools, _, err := analysis.DiscoverTools(manifest)
    if err != nil { t.Fatal(err) }
    if len(tools) != 1 { t.Fatalf("expected 1 tool, got %d", len(tools)) }
    if tools[0].Config["strict_mode"] != "False" {
        t.Errorf("strict_mode = %q, want False", tools[0].Config["strict_mode"])
    }
    if tools[0].Config["name_override"] != `"my_tool"` {
        t.Errorf("name_override = %q, want \"my_tool\"", tools[0].Config["name_override"])
    }
}
```

- [ ] **Step 3: Run, commit**

```
go test ./internal/analysis/ -run TestDiscoverTools_Captures -v
git add internal/analysis/discovery.go internal/analysis/agents_test.go
git commit -m "feat(analysis): capture tool decorator kwargs into ToolDef.Config"
```

---

# Phase D: Edge Resolution

## Task 18: In-file symbol resolution for tool references

**Files:**
- Modify: `internal/analysis/agents.go`

- [ ] **Step 1: Add test**

```go
func TestResolveEdges_InFileTool(t *testing.T) {
    src := `
from agents import Agent, function_tool

@function_tool
def my_tool(x: str) -> str:
    """A tool."""
    return x

agent = Agent(name="a", tools=[my_tool])
`
    pf := parsePyFile(t, "main.py", src)
    parsed := []analysis.ParsedFile{pf}
    inv := models.RepoInventory{
        Tools:  analysis.DiscoverToolsFromParsed(parsed),  // helper we add in Task 19
        Agents: analysis.DiscoverAgents(parsed),
    }
    analysis.ResolveEdges(&inv, parsed)
    if len(inv.Agents) != 1 { t.Fatalf("agents = %d", len(inv.Agents)) }
    a := inv.Agents[0]
    if len(a.ToolRefs) != 1 || a.ToolRefs[0].Name != "my_tool" {
        t.Fatalf("ToolRefs = %+v", a.ToolRefs)
    }
    if a.ToolRefs[0].External {
        t.Error("expected ToolRef.External = false (same file)")
    }
    if a.ToolRefs[0].Resolved == nil {
        t.Error("expected ToolRef.Resolved to be non-nil")
    }
}
```

`DiscoverToolsFromParsed` is a small helper added in Step 3 below — it wraps the existing tool discovery to operate on pre-parsed files. (Currently `DiscoverTools` takes a `ScanManifest`.)

- [ ] **Step 2: Add ResolveEdges**

```go
// ResolveEdges resolves the symbol references inside each AgentDef.ToolRefs /
// HandoffRefs / InputGuards / OutputGuards against the inventory. Sets
// Resolved when the symbol is found, External=true otherwise.
func ResolveEdges(inv *models.RepoInventory, parsed []ParsedFile) {
    // Build a quick lookup: per-file, per-symbol → ToolDef.
    toolsByFileSym := make(map[string]map[string]*models.ToolDef)
    for i := range inv.Tools {
        t := &inv.Tools[i]
        if toolsByFileSym[t.FilePath] == nil {
            toolsByFileSym[t.FilePath] = make(map[string]*models.ToolDef)
        }
        toolsByFileSym[t.FilePath][t.Name] = t
    }
    guardsByFileSym := make(map[string]map[string]*models.GuardrailDef)
    for i := range inv.Guardrails {
        g := &inv.Guardrails[i]
        if guardsByFileSym[g.FilePath] == nil {
            guardsByFileSym[g.FilePath] = make(map[string]*models.GuardrailDef)
        }
        guardsByFileSym[g.FilePath][g.Name] = g
    }

    for i := range inv.Agents {
        a := &inv.Agents[i]
        if a.Opaque { continue }

        // Resolve a.Kwargs["tools"] → ToolRefs
        toolsKwarg := agentKwarg(a, "tools")
        if toolsKwarg != nil && toolsKwarg.Value != nil && toolsKwarg.Value.Kind == models.ExprList {
            for _, item := range toolsKwarg.Value.List {
                ref := models.ToolRef{Name: item.Text}
                if td := toolsByFileSym[a.FilePath][item.Text]; td != nil {
                    ref.Resolved = td
                } else {
                    ref.External = true
                }
                a.ToolRefs = append(a.ToolRefs, ref)
            }
        } else if toolsKwarg != nil {
            // tools= is something non-list (function call, variable, etc.) → opaque
            a.Opaque = true
        }

        // Same pattern for input_guardrails / output_guardrails.
        resolveGuardKwarg(a, "input_guardrails",  &a.InputGuards,  guardsByFileSym[a.FilePath])
        resolveGuardKwarg(a, "output_guardrails", &a.OutputGuards, guardsByFileSym[a.FilePath])
    }
}

func agentKwarg(a *models.AgentDef, name string) *models.KwargTree {
    if a.Kwargs == nil { return nil }
    return a.Kwargs.Children[name]
}

func resolveGuardKwarg(a *models.AgentDef, kwargName string, into *[]models.GuardrailRef, lookup map[string]*models.GuardrailDef) {
    kw := agentKwarg(a, kwargName)
    if kw == nil || kw.Value == nil || kw.Value.Kind != models.ExprList { return }
    for _, item := range kw.Value.List {
        ref := models.GuardrailRef{Name: item.Text}
        if g := lookup[item.Text]; g != nil { ref.Resolved = g } else { ref.External = true }
        *into = append(*into, ref)
    }
}
```

- [ ] **Step 3: Add `DiscoverToolsFromParsed` helper**

In `internal/analysis/discovery.go`, expose a function:

```go
// DiscoverToolsFromParsed is the variant of DiscoverTools used when tests
// already have ParsedFile objects (e.g. unit tests for edge resolution).
func DiscoverToolsFromParsed(parsed []ParsedFile) []ToolDef {
    var out []ToolDef
    for _, pf := range parsed {
        out = append(out, discoverToolsInFile(pf)...)
    }
    return out
}
```

If `discoverToolsInFile` doesn't exist, factor the per-file loop out of `DiscoverTools` into this helper first.

- [ ] **Step 4: Run, commit**

```
go test ./internal/analysis/ -run TestResolveEdges_InFileTool -v
git add internal/analysis/agents.go internal/analysis/discovery.go internal/analysis/agents_test.go
git commit -m "feat(analysis): in-file symbol resolution for agent.tools, input/output_guardrails"
```

---

## Task 19: Cross-module same-repo resolution

**Files:**
- Modify: `internal/analysis/agents.go`

- [ ] **Step 1: Add test**

```go
func TestResolveEdges_CrossModuleTool(t *testing.T) {
    toolFile := parsePyFile(t, "tools.py", `
from agents import function_tool

@function_tool
def my_tool(x: str) -> str:
    """A tool."""
    return x
`)
    agentFile := parsePyFile(t, "agent.py", `
from agents import Agent
from tools import my_tool

agent = Agent(name="a", tools=[my_tool])
`)
    parsed := []analysis.ParsedFile{toolFile, agentFile}
    inv := models.RepoInventory{
        Tools:  analysis.DiscoverToolsFromParsed(parsed),
        Agents: analysis.DiscoverAgents(parsed),
    }
    analysis.ResolveEdges(&inv, parsed)
    if len(inv.Agents[0].ToolRefs) != 1 { t.Fatal("expected one tool ref") }
    ref := inv.Agents[0].ToolRefs[0]
    if ref.External { t.Error("expected cross-module resolution, got External=true") }
    if ref.Resolved == nil || ref.Resolved.FilePath != "tools.py" {
        t.Errorf("expected resolved to tools.py, got %+v", ref.Resolved)
    }
}
```

- [ ] **Step 2: Extend ResolveEdges**

Before the in-file lookup, build a cross-file lookup based on imports:

```go
// Build a per-agent-file map of (import_alias → (module, name)) from
// `from <module> import <name>` statements.
importsByFile := buildImportsByFile(parsed)

// ... inside the per-agent loop, after in-file lookup fails:
if td == nil {
    // Try cross-module: look up the import.
    if imp, ok := importsByFile[a.FilePath][item.Text]; ok {
        // Find a tool with this name in the imported module's file.
        for _, candidateFile := range parsed {
            if matchesModule(candidateFile.RelPath, imp.module) {
                if cand := toolsByFileSym[candidateFile.RelPath][imp.name]; cand != nil {
                    td = cand
                    break
                }
            }
        }
    }
}
```

Helpers:

```go
type importBinding struct {
    module string  // dotted module name from `from X import Y`
    name   string  // imported symbol, possibly aliased
}

func buildImportsByFile(parsed []ParsedFile) map[string]map[string]importBinding {
    out := make(map[string]map[string]importBinding)
    for _, pf := range parsed {
        m := make(map[string]importBinding)
        astutil.Walk(pf.Tree.RootNode(), func(n *sitter.Node) bool {
            if n.Type() != "import_from_statement" { return true }
            module := astutil.NodeText(n.ChildByFieldName("module_name"), pf.Source)
            for i := 0; i < int(n.ChildCount()); i++ {
                child := n.Child(i)
                if child.Type() == "dotted_name" {
                    name := astutil.NodeText(child, pf.Source)
                    if name != module {  // skip the module itself
                        m[name] = importBinding{module: module, name: name}
                    }
                } else if child.Type() == "aliased_import" {
                    orig  := astutil.NodeText(child.ChildByFieldName("name"), pf.Source)
                    alias := astutil.NodeText(child.ChildByFieldName("alias"), pf.Source)
                    m[alias] = importBinding{module: module, name: orig}
                }
            }
            return true
        })
        out[pf.RelPath] = m
    }
    return out
}

// matchesModule returns true if the file path corresponds to the dotted
// module name (e.g. "tools.py" matches module "tools", "pkg/sub/m.py"
// matches "pkg.sub.m").
func matchesModule(filePath, module string) bool {
    base := strings.TrimSuffix(filePath, ".py")
    base = strings.ReplaceAll(base, "/", ".")
    return base == module || strings.HasSuffix(base, "."+module)
}
```

- [ ] **Step 3: Run, commit**

```
go test ./internal/analysis/ -run TestResolveEdges_CrossModule -v
git add internal/analysis/agents.go internal/analysis/agents_test.go
git commit -m "feat(analysis): cross-module same-repo symbol resolution"
```

---

## Task 20: Opaque agent detection + External flag tests

**Files:**
- Modify: `internal/analysis/agents_test.go`

- [ ] **Step 1: Tests for opaque + external**

```go
func TestResolveEdges_OpaqueKwargsUnpack(t *testing.T) {
    src := `
from agents import Agent
config = {"name": "x", "tools": []}
agent = Agent(**config)
`
    pf := parsePyFile(t, "main.py", src)
    agents := analysis.DiscoverAgents([]analysis.ParsedFile{pf})
    if len(agents) != 1 || !agents[0].Opaque {
        t.Fatalf("expected Opaque=true, got %+v", agents)
    }
}

func TestResolveEdges_OpaqueToolsFactory(t *testing.T) {
    src := `
from agents import Agent

def get_tools(): return []

agent = Agent(name="x", tools=get_tools())
`
    pf := parsePyFile(t, "main.py", src)
    parsed := []analysis.ParsedFile{pf}
    inv := models.RepoInventory{
        Tools:  analysis.DiscoverToolsFromParsed(parsed),
        Agents: analysis.DiscoverAgents(parsed),
    }
    analysis.ResolveEdges(&inv, parsed)
    if !inv.Agents[0].Opaque {
        t.Errorf("expected Opaque=true after ResolveEdges saw tools=get_tools(), got false")
    }
}

func TestResolveEdges_ExternalTool(t *testing.T) {
    src := `
from agents import Agent
from third_party import some_tool

agent = Agent(name="x", tools=[some_tool])
`
    pf := parsePyFile(t, "main.py", src)
    parsed := []analysis.ParsedFile{pf}
    inv := models.RepoInventory{
        Tools:  analysis.DiscoverToolsFromParsed(parsed),
        Agents: analysis.DiscoverAgents(parsed),
    }
    analysis.ResolveEdges(&inv, parsed)
    if len(inv.Agents[0].ToolRefs) != 1 || !inv.Agents[0].ToolRefs[0].External {
        t.Errorf("expected External=true for unresolvable tool, got %+v", inv.Agents[0].ToolRefs)
    }
}
```

- [ ] **Step 2: Verify discoverAgentsInFile flags opaque correctly**

The Opaque flag for `Agent(**config)` was set in Task 14 inside `extractCallKwargs`. The Opaque flag for `tools=get_tools()` is set in `ResolveEdges` (Task 18). Both tests should pass without additional code.

- [ ] **Step 3: Run, commit**

```
go test ./internal/analysis/ -run TestResolveEdges -v
git add internal/analysis/agents_test.go
git commit -m "test(analysis): Opaque agent and External tool ref cases"
```

---

## Task 21: Determinism — sort tool candidates by (file_path, line)

**Files:**
- Modify: `internal/analysis/agents.go`

- [ ] **Step 1: Add test for deterministic pick**

```go
func TestResolveEdges_DeterministicSameName(t *testing.T) {
    fileA := parsePyFile(t, "a.py", `
from agents import function_tool
@function_tool
def shared(x: str) -> str:
    """Shared name."""
    return x
`)
    fileB := parsePyFile(t, "b.py", `
from agents import function_tool
@function_tool
def shared(x: str) -> str:
    """Shared name."""
    return x
`)
    agentFile := parsePyFile(t, "agent.py", `
from agents import Agent
from a import shared

agent = Agent(name="x", tools=[shared])
`)
    parsed := []analysis.ParsedFile{fileA, fileB, agentFile}
    inv := models.RepoInventory{
        Tools:  analysis.DiscoverToolsFromParsed(parsed),
        Agents: analysis.DiscoverAgents(parsed),
    }
    analysis.ResolveEdges(&inv, parsed)
    if len(inv.Agents[0].ToolRefs) != 1 { t.Fatal("expected one tool ref") }
    // Should resolve to a.py because that's the imported source.
    if inv.Agents[0].ToolRefs[0].Resolved == nil ||
       inv.Agents[0].ToolRefs[0].Resolved.FilePath != "a.py" {
        t.Errorf("expected resolved to a.py, got %+v", inv.Agents[0].ToolRefs[0].Resolved)
    }
}
```

- [ ] **Step 2: Ensure ResolveEdges sorts tool candidates**

In `ResolveEdges`, when building `toolsByFileSym`, the per-file map is naturally indexed by file. The cross-module resolution already walks `parsed` in deterministic order. To be safe, sort `parsed` by `RelPath` at the start of `ResolveEdges`:

```go
func ResolveEdges(inv *models.RepoInventory, parsed []ParsedFile) {
    sort.Slice(parsed, func(i, j int) bool { return parsed[i].RelPath < parsed[j].RelPath })
    // ... rest unchanged
}
```

- [ ] **Step 3: Run, commit**

```
go test ./internal/analysis/ -run TestResolveEdges -v
git add internal/analysis/agents.go internal/analysis/agents_test.go
git commit -m "feat(analysis): deterministic tool resolution by sorted parsed files"
```

---

## Task 22: Wire agent/guardrail/session discovery into scanner.Run

**Files:**
- Modify: `internal/scanner/scanner.go`

- [ ] **Step 1: Update scanner.Run to call new discovery passes**

In `internal/scanner/scanner.go`, inside `Run`, replace the Phase 2a block:

```go
// Phase 2a: per-language inventory
tools, parsed, err := analysis.DiscoverTools(profile.Manifest)
if err != nil { return models.ScanResult{}, err }
agents     := analysis.DiscoverAgents(parsed)
guardrails := analysis.DiscoverGuardrails(parsed)
sessions   := analysis.DiscoverSessions(parsed)
inventory := models.RepoInventory{
    Tools:        tools,
    Agents:       agents,
    Guardrails:   guardrails,
    Sessions:     sessions,
    SDKsDetected: deriveSDKsDetected(tools, agents),
}
analysis.ResolveEdges(&inventory, parsed)
```

- [ ] **Step 2: Run all tests**

```
go test ./...
```
Expected: PASS (rule tests may not exercise agent flow yet — that's fine until rules ship in Phase G).

- [ ] **Step 3: Commit**

```bash
git add internal/scanner/scanner.go
git commit -m "feat(scanner): wire DiscoverAgents/Guardrails/Sessions + ResolveEdges into Phase 2a"
```

---

# Phase E: New Predicates (Tool scope)

## Task 23: `PredToolDecoratorKwargValue` and `PredToolDecoratorKwargPresent`

**Files:**
- Modify: `internal/rules/schema.go` (add MatchExpr fields)
- Modify: `internal/rules/predicates.go`
- Modify: `internal/rules/predicates_test.go`
- Modify: `internal/rules/evaluator.go`

- [ ] **Step 1: Add MatchExpr fields**

In `internal/rules/schema.go`, add to `MatchExpr`:

```go
type MatchExpr struct {
    // ... existing fields ...
    ToolDecoratorKwargValue   *ToolDecoratorKwargValueExpr `yaml:"tool_decorator_kwarg_value,omitempty"`
    ToolDecoratorKwargPresent []string                     `yaml:"tool_decorator_kwarg_present,omitempty"`
}

type ToolDecoratorKwargValueExpr struct {
    Kwarg string `yaml:"kwarg"`
    Value string `yaml:"value"`
}
```

- [ ] **Step 2: Write predicate tests**

```go
func TestPredToolDecoratorKwargValue(t *testing.T) {
    tool := models.ToolDef{Config: map[string]string{"strict_mode": "False"}}
    if !rules.PredToolDecoratorKwargValue(rules.ToolDecoratorKwargValueExpr{Kwarg: "strict_mode", Value: "False"}, tool) {
        t.Error("expected match")
    }
    if rules.PredToolDecoratorKwargValue(rules.ToolDecoratorKwargValueExpr{Kwarg: "strict_mode", Value: "True"}, tool) {
        t.Error("expected no match (value mismatch)")
    }
    if rules.PredToolDecoratorKwargValue(rules.ToolDecoratorKwargValueExpr{Kwarg: "other", Value: "False"}, tool) {
        t.Error("expected no match (kwarg absent)")
    }
}

func TestPredToolDecoratorKwargPresent(t *testing.T) {
    tool := models.ToolDef{Config: map[string]string{"strict_mode": "False"}}
    if !rules.PredToolDecoratorKwargPresent([]string{"strict_mode"}, tool) {
        t.Error("expected present")
    }
    if rules.PredToolDecoratorKwargPresent([]string{"failure_error_function"}, tool) {
        t.Error("expected not present")
    }
}
```

- [ ] **Step 3: Implement predicates**

In `internal/rules/predicates.go`:

```go
func PredToolDecoratorKwargValue(expr ToolDecoratorKwargValueExpr, t models.ToolDef) bool {
    v, ok := t.Config[expr.Kwarg]
    return ok && v == expr.Value
}

func PredToolDecoratorKwargPresent(names []string, t models.ToolDef) bool {
    for _, n := range names {
        if _, ok := t.Config[n]; ok { return true }
    }
    return false
}
```

- [ ] **Step 4: Wire into evaluator**

In `internal/rules/evaluator.go`, find the tool-scope evaluator (after Task 33 will introduce `EvaluateTool`). For now, the existing `Evaluate` becomes `EvaluateTool`. Add:

```go
if e.ToolDecoratorKwargValue != nil {
    if !PredToolDecoratorKwargValue(*e.ToolDecoratorKwargValue, t) { return false }
}
if len(e.ToolDecoratorKwargPresent) > 0 {
    if !PredToolDecoratorKwargPresent(e.ToolDecoratorKwargPresent, t) { return false }
}
```

- [ ] **Step 5: Run, commit**

```
go test ./internal/rules/ -run TestPredToolDecorator -v
git add internal/rules/schema.go internal/rules/predicates.go internal/rules/predicates_test.go internal/rules/evaluator.go
git commit -m "feat(rules): tool_decorator_kwarg_value / _present predicates"
```

---

# Phase F: New Predicates (Agent scope)

## Task 24: `PredAgentClass`

**Files:**
- Modify: `internal/rules/schema.go`
- Modify: `internal/rules/predicates.go`
- Modify: `internal/rules/predicates_test.go`

- [ ] **Step 1: Schema field**

```go
type MatchExpr struct {
    // ...
    AgentClass []string `yaml:"agent_class,omitempty"`
}
```

- [ ] **Step 2: Test**

```go
func TestPredAgentClass(t *testing.T) {
    a := models.AgentDef{Class: "Agent"}
    if !rules.PredAgentClass([]string{"Agent"}, a) { t.Error("expected match") }
    if rules.PredAgentClass([]string{"SandboxAgent"}, a) { t.Error("expected no match") }
}
```

- [ ] **Step 3: Implementation**

```go
func PredAgentClass(classes []string, a models.AgentDef) bool {
    for _, c := range classes { if a.Class == c { return true } }
    return false
}
```

- [ ] **Step 4: Run, commit**

```
go test ./internal/rules/ -run TestPredAgentClass -v
git add internal/rules/schema.go internal/rules/predicates.go internal/rules/predicates_test.go
git commit -m "feat(rules): agent_class predicate"
```

---

## Task 25: `PredAgentKwargPresent`, `PredAgentKwargMissing`, `PredAgentKwargListEmpty` (with dotted-path support)

**Files:**
- Modify: `internal/rules/schema.go`
- Modify: `internal/rules/predicates.go`
- Modify: `internal/rules/predicates_test.go`

- [ ] **Step 1: Schema fields**

```go
type MatchExpr struct {
    // ...
    AgentKwargPresent   []string `yaml:"agent_kwarg_present,omitempty"`
    AgentKwargMissing   []string `yaml:"agent_kwarg_missing,omitempty"`
    AgentKwargListEmpty []string `yaml:"agent_kwarg_list_empty,omitempty"`
}
```

- [ ] **Step 2: Tests**

```go
func TestPredAgentKwargPresent(t *testing.T) {
    a := models.AgentDef{Kwargs: &models.KwargTree{
        Children: map[string]*models.KwargTree{
            "model": {Value: &models.Expr{Kind: models.ExprLiteralString, Text: `"gpt-4"`}},
            "model_settings": {Children: map[string]*models.KwargTree{
                "tool_choice": {Value: &models.Expr{Kind: models.ExprLiteralString, Text: `"required"`}},
            }},
        },
    }}
    if !rules.PredAgentKwargPresent([]string{"model"}, a) { t.Error("expected model present") }
    if !rules.PredAgentKwargPresent([]string{"model_settings.tool_choice"}, a) { t.Error("expected dotted match") }
    if rules.PredAgentKwargPresent([]string{"nope"}, a) { t.Error("expected not present") }
}

func TestPredAgentKwargListEmpty(t *testing.T) {
    // input_guardrails absent → list empty (vacuously)
    a := models.AgentDef{Kwargs: &models.KwargTree{Children: map[string]*models.KwargTree{}}}
    if !rules.PredAgentKwargListEmpty([]string{"input_guardrails"}, a) {
        t.Error("expected list empty when kwarg absent")
    }
    // input_guardrails = [g] → not empty
    a = models.AgentDef{Kwargs: &models.KwargTree{
        Children: map[string]*models.KwargTree{
            "input_guardrails": {Value: &models.Expr{Kind: models.ExprList, List: []models.Expr{
                {Kind: models.ExprNameRef, Text: "g"},
            }}},
        },
    }}
    if rules.PredAgentKwargListEmpty([]string{"input_guardrails"}, a) {
        t.Error("expected list NOT empty")
    }
}
```

- [ ] **Step 3: Implementation**

```go
// lookupKwarg walks a dotted-path like "model_settings.tool_choice" through
// the KwargTree. Returns nil if any segment is missing.
func lookupKwarg(a models.AgentDef, path string) *models.KwargTree {
    if a.Kwargs == nil { return nil }
    parts := strings.Split(path, ".")
    cur := a.Kwargs
    for _, p := range parts {
        if cur.Children == nil { return nil }
        next, ok := cur.Children[p]
        if !ok { return nil }
        cur = next
    }
    return cur
}

func PredAgentKwargPresent(paths []string, a models.AgentDef) bool {
    for _, p := range paths {
        if lookupKwarg(a, p) != nil { return true }
    }
    return false
}

func PredAgentKwargMissing(paths []string, a models.AgentDef) bool {
    for _, p := range paths {
        if lookupKwarg(a, p) == nil { return true }
    }
    return false
}

func PredAgentKwargListEmpty(paths []string, a models.AgentDef) bool {
    for _, p := range paths {
        kw := lookupKwarg(a, p)
        if kw == nil { return true }   // absent counts as empty
        if kw.Value == nil { continue }
        if kw.Value.Kind == models.ExprList && len(kw.Value.List) == 0 { return true }
    }
    return false
}
```

- [ ] **Step 4: Run, commit**

```
go test ./internal/rules/ -run TestPredAgentKwarg -v
git add internal/rules/schema.go internal/rules/predicates.go internal/rules/predicates_test.go
git commit -m "feat(rules): agent_kwarg_present/missing/list_empty with dotted-path"
```

---

## Task 26: `PredAgentKwargValue` (with dotted-path)

**Files:**
- Modify: `internal/rules/schema.go`
- Modify: `internal/rules/predicates.go`
- Modify: `internal/rules/predicates_test.go`

- [ ] **Step 1: Schema field**

```go
type MatchExpr struct {
    // ...
    AgentKwargValue *AgentKwargValueExpr `yaml:"agent_kwarg_value,omitempty"`
}

type AgentKwargValueExpr struct {
    Kwarg string `yaml:"kwarg"`
    Value string `yaml:"value"`  // literal text comparison ("True", "False", "required", ...)
}
```

- [ ] **Step 2: Test**

```go
func TestPredAgentKwargValue_Dotted(t *testing.T) {
    a := models.AgentDef{Kwargs: &models.KwargTree{
        Children: map[string]*models.KwargTree{
            "model_settings": {Children: map[string]*models.KwargTree{
                "tool_choice": {Value: &models.Expr{Kind: models.ExprLiteralString, Text: `"required"`}},
            }},
            "reset_tool_choice": {Value: &models.Expr{Kind: models.ExprLiteralBool, Text: "False"}},
        },
    }}
    if !rules.PredAgentKwargValue(rules.AgentKwargValueExpr{Kwarg: "model_settings.tool_choice", Value: "required"}, a) {
        t.Error("expected dotted match (after stripping quotes)")
    }
    if !rules.PredAgentKwargValue(rules.AgentKwargValueExpr{Kwarg: "reset_tool_choice", Value: "False"}, a) {
        t.Error("expected bool literal match")
    }
}
```

- [ ] **Step 3: Implementation**

```go
func PredAgentKwargValue(expr AgentKwargValueExpr, a models.AgentDef) bool {
    kw := lookupKwarg(a, expr.Kwarg)
    if kw == nil || kw.Value == nil { return false }
    raw := kw.Value.Text
    // Strip surrounding quotes for string literals so YAML can use
    // value: required instead of value: '"required"'.
    if kw.Value.Kind == models.ExprLiteralString {
        raw = strings.Trim(raw, `"'`)
    }
    return raw == expr.Value
}
```

- [ ] **Step 4: Run, commit**

```
go test ./internal/rules/ -run TestPredAgentKwargValue -v
git add internal/rules/schema.go internal/rules/predicates.go internal/rules/predicates_test.go
git commit -m "feat(rules): agent_kwarg_value with dotted-path and quote-stripping"
```

---

## Task 27: `PredAgentUsesToolKind` and `PredAgentHandoffToClass`

**Files:**
- Modify: `internal/rules/schema.go`
- Modify: `internal/rules/predicates.go`
- Modify: `internal/rules/predicates_test.go`

- [ ] **Step 1: Schema fields**

```go
type MatchExpr struct {
    // ...
    AgentUsesToolKind    []string `yaml:"agent_uses_tool_kind,omitempty"`
    AgentHandoffToClass  []string `yaml:"agent_handoff_to_class,omitempty"`
}
```

- [ ] **Step 2: Tests**

```go
func TestPredAgentUsesToolKind(t *testing.T) {
    shellTool := &models.ToolDef{Kind: models.KindShellInvocation, Name: "run"}
    a := models.AgentDef{ToolRefs: []models.ToolRef{{Name: "run", Resolved: shellTool}}}
    if !rules.PredAgentUsesToolKind([]string{"shell_invocation"}, a) {
        t.Error("expected match against shell_invocation tool ref")
    }
    if rules.PredAgentUsesToolKind([]string{"mcp_tool"}, a) {
        t.Error("expected no match")
    }
}

func TestPredAgentHandoffToClass(t *testing.T) {
    sub := &models.AgentDef{Class: "Agent"}
    a := models.AgentDef{HandoffRefs: []models.AgentRef{{Resolved: sub}}}
    if !rules.PredAgentHandoffToClass([]string{"Agent"}, a) { t.Error("expected match") }
    if rules.PredAgentHandoffToClass([]string{"SandboxAgent"}, a) { t.Error("expected no match") }
}
```

- [ ] **Step 3: Implementation**

```go
func PredAgentUsesToolKind(kinds []string, a models.AgentDef) bool {
    for _, ref := range a.ToolRefs {
        if ref.Resolved == nil { continue }
        for _, k := range kinds {
            if string(ref.Resolved.Kind) == k { return true }
        }
    }
    return false
}

func PredAgentHandoffToClass(classes []string, a models.AgentDef) bool {
    for _, ref := range a.HandoffRefs {
        if ref.Resolved == nil { continue }
        for _, c := range classes {
            if ref.Resolved.Class == c { return true }
        }
    }
    return false
}
```

- [ ] **Step 4: Run, commit**

```
go test ./internal/rules/ -run "TestPredAgentUsesToolKind|TestPredAgentHandoffToClass" -v
git add internal/rules/schema.go internal/rules/predicates.go internal/rules/predicates_test.go
git commit -m "feat(rules): agent_uses_tool_kind / agent_handoff_to_class predicates"
```

---

# Phase G: New Predicates (Repo scope)

## Task 28: `PredRepoHasSDKDep`, `PredRepoHasSDKInCode`, `PredRepoHasAgentClass`, `PredRepoHasNoAgentClass`, `PredRepoComponentPresent`, `PredRepoUsesDefaultTracing`

**Files:**
- Modify: `internal/rules/schema.go`
- Modify: `internal/rules/predicates.go`
- Modify: `internal/rules/predicates_test.go`

- [ ] **Step 1: Schema fields**

```go
type MatchExpr struct {
    // ...
    RepoHasSDKDep          []string `yaml:"repo_has_sdk_dep,omitempty"`
    RepoHasSDKInCode       []string `yaml:"repo_has_sdk_in_code,omitempty"`
    RepoHasAgentClass      []string `yaml:"repo_has_agent_class,omitempty"`
    RepoHasNoAgentClass    []string `yaml:"repo_has_no_agent_class,omitempty"`
    RepoComponentPresent   []string `yaml:"repo_component_present,omitempty"`
    RepoUsesDefaultTracing *bool    `yaml:"repo_uses_default_tracing,omitempty"`
}
```

- [ ] **Step 2: Tests**

```go
func TestPredRepoHasSDKDep(t *testing.T) {
    p := models.RepoProfile{SDKDeps: []models.SDKDep{{Name: "openai-agents"}}}
    if !rules.PredRepoHasSDKDep([]string{"openai-agents"}, p) { t.Error("expected match") }
    if rules.PredRepoHasSDKDep([]string{"langgraph"}, p) { t.Error("expected no match") }
}

func TestPredRepoHasSDKInCode(t *testing.T) {
    inv := models.RepoInventory{SDKsDetected: []models.SDK{models.SDKOpenAIAgents}}
    if !rules.PredRepoHasSDKInCode([]string{"openai_agents"}, inv) { t.Error("expected match") }
}

func TestPredRepoUsesDefaultTracing(t *testing.T) {
    // No add_trace_processor anywhere → default tracing.
    inv := models.RepoInventory{}
    if !rules.PredRepoUsesDefaultTracing(true, inv, []analysis.ParsedFile{}) {
        t.Error("expected default tracing when no custom processor")
    }
}
```

- [ ] **Step 3: Implementation**

```go
func PredRepoHasSDKDep(names []string, p models.RepoProfile) bool {
    for _, dep := range p.SDKDeps {
        for _, n := range names {
            if dep.Name == n { return true }
        }
    }
    return false
}

func PredRepoHasSDKInCode(sdks []string, inv models.RepoInventory) bool {
    for _, s := range inv.SDKsDetected {
        for _, want := range sdks {
            if string(s) == want { return true }
        }
    }
    return false
}

func PredRepoHasAgentClass(classes []string, inv models.RepoInventory) bool {
    for _, a := range inv.Agents {
        for _, c := range classes {
            if a.Class == c { return true }
        }
    }
    return false
}

func PredRepoHasNoAgentClass(classes []string, inv models.RepoInventory) bool {
    return !PredRepoHasAgentClass(classes, inv)
}

func PredRepoComponentPresent(kinds []string, p models.RepoProfile) bool {
    for _, c := range p.Manifest.Components {
        for _, k := range kinds {
            if string(c.Kind) == k { return true }
        }
    }
    return false
}

// PredRepoUsesDefaultTracing returns true when no add_trace_processor call
// appears anywhere in the parsed inventory (and the want flag is true).
func PredRepoUsesDefaultTracing(want bool, inv models.RepoInventory, parsed []analysis.ParsedFile) bool {
    hasCustom := false
    for _, pf := range parsed {
        if strings.Contains(string(pf.Source), "add_trace_processor") ||
           strings.Contains(string(pf.Source), "OPENAI_AGENTS_DISABLE_TRACING") {
            hasCustom = true
            break
        }
    }
    return want == !hasCustom
}
```

- [ ] **Step 4: Run, commit**

```
go test ./internal/rules/ -run TestPredRepo -v
git add internal/rules/schema.go internal/rules/predicates.go internal/rules/predicates_test.go
git commit -m "feat(rules): repo-scope predicates"
```

---

# Phase H: Evaluator + Schema Doc Integration

## Task 29: Split `Evaluate` into `EvaluateTool`, `EvaluateAgent`, `EvaluateRepo`

**Files:**
- Modify: `internal/rules/evaluator.go`

- [ ] **Step 1: Rename existing Evaluate to EvaluateTool**

In `internal/rules/evaluator.go`, change the receiver:

```go
func (e MatchExpr) EvaluateTool(t models.ToolDef, pf analysis.ParsedFile) bool {
    // existing body (handles all tool-scope predicates including new tool_decorator_kwarg_*)
}
```

- [ ] **Step 2: Add EvaluateAgent**

```go
func (e MatchExpr) EvaluateAgent(a models.AgentDef, inv models.RepoInventory) bool {
    // Combinators first
    for _, sub := range e.All { if !sub.EvaluateAgent(a, inv) { return false } }
    if len(e.Any) > 0 {
        anyMatch := false
        for _, sub := range e.Any { if sub.EvaluateAgent(a, inv) { anyMatch = true; break } }
        if !anyMatch { return false }
    }
    if e.Not != nil { if e.Not.EvaluateAgent(a, inv) { return false } }

    if len(e.AgentClass) > 0           && !PredAgentClass(e.AgentClass, a)            { return false }
    if len(e.AgentKwargPresent) > 0    && !PredAgentKwargPresent(e.AgentKwargPresent, a){ return false }
    if len(e.AgentKwargMissing) > 0    && !PredAgentKwargMissing(e.AgentKwargMissing, a){ return false }
    if len(e.AgentKwargListEmpty) > 0  && !PredAgentKwargListEmpty(e.AgentKwargListEmpty, a){ return false }
    if e.AgentKwargValue != nil        && !PredAgentKwargValue(*e.AgentKwargValue, a) { return false }
    if len(e.AgentUsesToolKind) > 0    && !PredAgentUsesToolKind(e.AgentUsesToolKind, a){ return false }
    if len(e.AgentHandoffToClass) > 0  && !PredAgentHandoffToClass(e.AgentHandoffToClass, a){ return false }
    return true
}
```

- [ ] **Step 3: Add EvaluateRepo**

```go
func (e MatchExpr) EvaluateRepo(p models.RepoProfile, inv models.RepoInventory) bool {
    for _, sub := range e.All { if !sub.EvaluateRepo(p, inv) { return false } }
    if len(e.Any) > 0 {
        anyMatch := false
        for _, sub := range e.Any { if sub.EvaluateRepo(p, inv) { anyMatch = true; break } }
        if !anyMatch { return false }
    }
    if e.Not != nil { if e.Not.EvaluateRepo(p, inv) { return false } }

    if len(e.RepoHasSDKDep) > 0          && !PredRepoHasSDKDep(e.RepoHasSDKDep, p)            { return false }
    if len(e.RepoHasSDKInCode) > 0       && !PredRepoHasSDKInCode(e.RepoHasSDKInCode, inv)    { return false }
    if len(e.RepoHasAgentClass) > 0      && !PredRepoHasAgentClass(e.RepoHasAgentClass, inv)  { return false }
    if len(e.RepoHasNoAgentClass) > 0    && !PredRepoHasNoAgentClass(e.RepoHasNoAgentClass, inv){ return false }
    if len(e.RepoComponentPresent) > 0   && !PredRepoComponentPresent(e.RepoComponentPresent, p){ return false }
    if e.RepoUsesDefaultTracing != nil   && !PredRepoUsesDefaultTracing(*e.RepoUsesDefaultTracing, inv, nil){ return false }
    return true
}
```

(For now, `PredRepoUsesDefaultTracing` receives nil parsed files; we'll thread them through in Task 30.)

- [ ] **Step 4: Update rule_detector.go to use the new methods**

The toolRuleDetector / agentRuleDetector / repoRuleDetector code from Task 6 already references `EvaluateTool` / `EvaluateAgent` / `EvaluateRepo`. Build should now succeed.

- [ ] **Step 5: Run all tests**

```
go test ./internal/rules/
go build ./...
```
Expected: PASS / build success.

- [ ] **Step 6: Commit**

```bash
git add internal/rules/evaluator.go
git commit -m "feat(rules): split Evaluate into per-scope EvaluateTool/Agent/Repo"
```

---

## Task 30: Thread parsed files through to `PredRepoUsesDefaultTracing`

**Files:**
- Modify: `internal/rules/evaluator.go`
- Modify: `internal/rules/rule_detector.go`

- [ ] **Step 1: Pass parsed files into EvaluateRepo**

Change `EvaluateRepo` signature:

```go
func (e MatchExpr) EvaluateRepo(p models.RepoProfile, inv models.RepoInventory, parsed []analysis.ParsedFile) bool {
    // recurse with same parsed
    // update PredRepoUsesDefaultTracing call to use parsed
}
```

Update repoRuleDetector.Detect to accept parsed:

```go
type repoRuleDetector struct{ rule RuleDef }

func (d repoRuleDetector) Detect(p models.RepoProfile, inv models.RepoInventory) []models.Finding {
    // ...
}
```

Hmm — but the RepoDetector interface signature doesn't take `parsed`. To minimize blast radius, instead of changing the interface, store parsed files on `RepoInventory` for predicates that need them:

In `internal/models/agent.go`, the existing `RepoInventory` is fine; we just need to make sure parsed files reach predicates. The simplest fix: keep `PredRepoUsesDefaultTracing` reading from a new field `RepoInventory.RawSourceText []string` populated by scanner.Run. **Decision:** add a derived flag on RepoInventory instead of passing parsed files.

- [ ] **Step 2: Add `RepoInventory.UsesDefaultTracing` derived flag**

In `internal/models/agent.go`:

```go
type RepoInventory struct {
    // ...
    UsesDefaultTracing bool `json:"uses_default_tracing"`
}
```

In `internal/scanner/scanner.go`, compute it:

```go
inventory.UsesDefaultTracing = computeDefaultTracing(parsed)

func computeDefaultTracing(parsed []analysis.ParsedFile) bool {
    for _, pf := range parsed {
        if strings.Contains(string(pf.Source), "add_trace_processor") ||
           strings.Contains(string(pf.Source), "OPENAI_AGENTS_DISABLE_TRACING") {
            return false
        }
    }
    return true
}
```

Update `PredRepoUsesDefaultTracing` to read from the inventory:

```go
func PredRepoUsesDefaultTracing(want bool, inv models.RepoInventory) bool {
    return inv.UsesDefaultTracing == want
}
```

Update `EvaluateRepo` accordingly:

```go
if e.RepoUsesDefaultTracing != nil && !PredRepoUsesDefaultTracing(*e.RepoUsesDefaultTracing, inv) { return false }
```

- [ ] **Step 3: Run, commit**

```
go test ./...
git add internal/models/agent.go internal/scanner/scanner.go internal/rules/predicates.go internal/rules/evaluator.go
git commit -m "feat: RepoInventory.UsesDefaultTracing derived in Phase 2a"
```

---

## Task 31: Update `internal/rules/schema.yaml` with new fields and scope vocabulary

**Files:**
- Modify: `internal/rules/schema.yaml`

- [ ] **Step 1: Add `scope:` field documentation**

In the annotated example block in `schema.yaml`, add after `title:`:

```yaml
    scope: tool                               # REQUIRED. tool | agent | repo.
                                              # tool   — fires per @function_tool / @tool / @server.tool / shell-invoking function
                                              # agent  — fires per Agent(...) / SandboxAgent(...) / AgentDefinition(...)
                                              # repo   — fires once per scan against the manifest
                                              # The applies_to value space depends on the scope (see below).
```

Update the `applies_to:` documentation to show per-scope values:

```yaml
    applies_to:                               # REQUIRED. Non-empty list. Value space depends on scope:
      - claude_sdk_tool                       # tool scope:  claude_sdk_tool | openai_tool | mcp_tool | shell_invocation | unknown
      - mcp_tool                              # agent scope: openai_agent | openai_sandbox_agent | claude_agent_definition
                                              # repo scope:  claude_sdk | openai_agents | openshell | mcp
```

- [ ] **Step 2: Add the new predicates to the reference section**

Append to the predicate reference section:

```yaml
# ━━━ TOOL-SCOPE PREDICATES ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#
#   tool_decorator_kwarg_value: {kwarg, value}
#       Fires if the tool's decorator (@function_tool, @tool, etc.) was
#       called with kwarg=value. Reads ToolDef.Config[kwarg] populated during
#       discovery.
#
#       kwarg: strict_mode         REQUIRED. Decorator kwarg name.
#       value: "False"             REQUIRED. Literal text comparison
#                                  (quote bool/None as "True"/"False"/"None").
#
#   tool_decorator_kwarg_present: [name, name, ...]
#       Fires if ANY of the named kwargs is present in the decorator call.
#
# ━━━ AGENT-SCOPE PREDICATES ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#
#   agent_class: [Agent, SandboxAgent, ...]
#       Match against AgentDef.Class (literal Python class name).
#
#   agent_kwarg_present: [name, ...]
#       Match against AgentDef.Kwargs. Supports dotted paths like
#       "model_settings.tool_choice" — descends into nested kwargs.
#
#   agent_kwarg_missing: [name, ...]
#       Negation of agent_kwarg_present per name.
#
#   agent_kwarg_value: {kwarg, value}
#       Fires if the kwarg's value (literal) equals `value`. Supports dotted
#       paths. String quotes are stripped before comparison so YAML can use
#       value: required instead of value: '"required"'.
#
#   agent_kwarg_list_empty: [name, ...]
#       Fires if ANY of the named kwargs is absent OR is an empty list.
#       Common pattern: agent_kwarg_list_empty: [input_guardrails].
#
#   agent_uses_tool_kind: [kind, ...]
#       Fires if any resolved tool in the agent's tools list has the given
#       Kind. Unresolved (External=true) tool refs are skipped.
#
#   agent_handoff_to_class: [class, ...]
#       Fires if the agent hands off to a subagent whose Class matches.
#
# ━━━ REPO-SCOPE PREDICATES ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#
#   repo_has_sdk_dep: [name, ...]
#       Fires if any of the named SDKs is declared as a dep in the project's
#       manifests (pyproject.toml, requirements.txt, package.json, etc.).
#       Reads RepoProfile.SDKDeps.
#
#   repo_has_sdk_in_code: [sdk, ...]
#       Fires if any of the named SDKs is observed in code (tools or agents
#       of that SDK present). Reads RepoInventory.SDKsDetected.
#
#   repo_has_agent_class: [class, ...]
#       Fires if any AgentDef has the given class.
#
#   repo_has_no_agent_class: [class, ...]
#       Inverse of the above.
#
#   repo_component_present: [kind, ...]
#       Fires if RepoProfile.Manifest.Components contains an entry of the
#       given Kind (e.g. sandbox_policy, claude_md).
#
#   repo_uses_default_tracing: true|false
#       Fires when no add_trace_processor or OPENAI_AGENTS_DISABLE_TRACING
#       appears anywhere in the parsed source (and want matches).
```

- [ ] **Step 3: Commit**

```bash
git add internal/rules/schema.yaml
git commit -m "docs(schema): document scope field, per-scope applies_to, and new predicates"
```

---

# Phase I: Policy Selection + META Findings

## Task 32: `LoadFor(fsys, sdks)` — filter policies by SDK

**Files:**
- Modify: `internal/rules/loader.go`

- [ ] **Step 1: Add LoadFor function**

```go
// LoadFor returns a Registry containing only the policy packs whose category
// directory matches one of the given SDKs. The mapping from SDK to policy
// directory name is:
//   SDKClaudeAgentSDK -> "claude_sdk"
//   SDKOpenAIAgents   -> "openai_sdk"
//   SDKMCP            -> "mcp"
//   (plus "openshell" always loaded for shell-invocation rules)
//
// If sdks is empty, returns an empty Registry (no rules).
func LoadFor(fsys fs.FS, sdks []models.SDK) (*detectors.Registry, error) {
    wanted := map[string]bool{
        "openshell": true,    // shell-invocation rules apply regardless of agent SDK
    }
    for _, sdk := range sdks {
        switch sdk {
        case models.SDKClaudeAgentSDK: wanted["claude_sdk"] = true
        case models.SDKOpenAIAgents:   wanted["openai_sdk"] = true
        case models.SDKMCP:            wanted["mcp"]        = true
        }
    }
    all, err := Load(fsys)
    if err != nil { return nil, err }
    var tool []detectors.ToolDetector
    var agent []detectors.AgentDetector
    var repo []detectors.RepoDetector
    for _, p := range all {
        if !wanted[string(p.Policy.Category)] { continue }
        for _, r := range p.Rules {
            switch r.Scope {
            case models.ScopeTool:  tool  = append(tool,  toolRuleDetector{r})
            case models.ScopeAgent: agent = append(agent, agentRuleDetector{r})
            case models.ScopeRepo:  repo  = append(repo,  repoRuleDetector{r})
            }
        }
    }
    return detectors.New(tool, agent, repo), nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/rules/loader.go
git commit -m "feat(rules): LoadFor selects policy packs by observed SDKs"
```

---

## Task 33: Phase 2b — META-001 (unaudited SDK)

**Files:**
- Create: `internal/scanner/policy_selection.go`
- Create: `internal/scanner/policy_selection_test.go`

- [ ] **Step 1: Test**

```go
package scanner_test

import (
    "testing"
    "github.com/trustabl/trustabl/internal/models"
    "github.com/trustabl/trustabl/internal/scanner"
)

func TestSelectPolicies_EmitsMETA001ForUnauditedSDK(t *testing.T) {
    profile := models.RepoProfile{}
    inv := models.RepoInventory{SDKsDetected: []models.SDK{models.SDK("langgraph")}}
    findings := scanner.SelectAndEmitMETA(profile, inv)
    if len(findings) != 1 || findings[0].RuleID != "META-001" {
        t.Fatalf("expected one META-001 finding, got %+v", findings)
    }
}
```

- [ ] **Step 2: Implementation**

```go
package scanner

import (
    "fmt"
    "github.com/trustabl/trustabl/internal/models"
)

// shippedPolicySDKs lists the SDKs we have policy packs for. Update when a
// new SDK pack lands under internal/rules/policies/.
var shippedPolicySDKs = map[models.SDK]bool{
    models.SDKClaudeAgentSDK: true,
    models.SDKOpenAIAgents:   true,
    models.SDKMCP:            true,
}

// SelectAndEmitMETA inspects the profile + inventory and emits engine-level
// info findings:
//   META-001 — an SDK observed in code is not currently audited
//   META-002 — an SDK declared as a dep has no observed code use
//   META-003 — an agent has opaque configuration (Agent(**...) or non-list tools=)
func SelectAndEmitMETA(profile models.RepoProfile, inv models.RepoInventory) []models.Finding {
    var out []models.Finding

    // META-001: unaudited SDKs
    for _, sdk := range inv.SDKsDetected {
        if !shippedPolicySDKs[sdk] {
            out = append(out, models.Finding{
                RuleID:     "META-001",
                Severity:   models.SeverityInfo,
                Title:      "Unaudited SDK in use",
                Explanation: fmt.Sprintf(
                    "This repo uses SDK %q, which trustabl does not currently audit. "+
                    "No rules will fire against agents or tools from this SDK.", sdk),
                SuggestedFix: "If detection for this SDK is needed, file an issue or contribute a policy pack under internal/rules/policies/<sdk>/.",
                Confidence:   1.0,
            })
        }
    }
    return out
}
```

- [ ] **Step 3: Run, commit**

```
go test ./internal/scanner/ -run TestSelectPolicies_EmitsMETA001 -v
git add internal/scanner/policy_selection.go internal/scanner/policy_selection_test.go
git commit -m "feat(scanner): META-001 unaudited SDK info finding"
```

---

## Task 34: META-002 (dep drift)

**Files:**
- Modify: `internal/scanner/policy_selection.go`
- Modify: `internal/scanner/policy_selection_test.go`

- [ ] **Step 1: Test**

```go
func TestSelectPolicies_EmitsMETA002ForDepDrift(t *testing.T) {
    profile := models.RepoProfile{SDKDeps: []models.SDKDep{{Name: "openai-agents", Source: "pyproject.toml"}}}
    inv := models.RepoInventory{SDKsDetected: nil}  // no observed code use
    findings := scanner.SelectAndEmitMETA(profile, inv)
    var meta002 int
    for _, f := range findings { if f.RuleID == "META-002" { meta002++ } }
    if meta002 != 1 { t.Errorf("expected 1 META-002, got %d", meta002) }
}

func TestSelectPolicies_SilentWhenDepAndCodeBothPresent(t *testing.T) {
    profile := models.RepoProfile{SDKDeps: []models.SDKDep{{Name: "openai-agents"}}}
    inv := models.RepoInventory{SDKsDetected: []models.SDK{models.SDKOpenAIAgents}}
    findings := scanner.SelectAndEmitMETA(profile, inv)
    for _, f := range findings {
        if f.RuleID == "META-002" { t.Errorf("expected no META-002, got %+v", f) }
    }
}
```

- [ ] **Step 2: Implementation — append to SelectAndEmitMETA**

```go
// depNameToSDK maps the canonical dep name string to our SDK enum.
var depNameToSDK = map[string]models.SDK{
    "claude-agent-sdk": models.SDKClaudeAgentSDK,
    "openai-agents":    models.SDKOpenAIAgents,
}

// ... at the end of SelectAndEmitMETA:

// META-002: declared deps with no observed code use
observed := make(map[models.SDK]bool)
for _, s := range inv.SDKsDetected { observed[s] = true }
seenDrift := make(map[string]bool)
for _, dep := range profile.SDKDeps {
    sdk, known := depNameToSDK[dep.Name]
    if !known { continue }
    if observed[sdk] { continue }
    if seenDrift[dep.Name] { continue }
    seenDrift[dep.Name] = true
    out = append(out, models.Finding{
        RuleID:   "META-002",
        Severity: models.SeverityInfo,
        Title:    "Declared SDK dependency has no observed code use",
        Explanation: fmt.Sprintf(
            "The project declares %q as a dependency (in %s) but trustabl found no "+
            "code that uses it. The corresponding rules will not fire until an "+
            "agent or tool from this SDK appears in code.", dep.Name, dep.Source),
        SuggestedFix: "If the dep was added intentionally for non-agent reasons (type stubs, helpers), suppress this finding. Otherwise, remove the unused dep.",
        Confidence:   1.0,
    })
}
```

- [ ] **Step 3: Run, commit**

```
go test ./internal/scanner/ -run TestSelectPolicies -v
git add internal/scanner/policy_selection.go internal/scanner/policy_selection_test.go
git commit -m "feat(scanner): META-002 dep-drift info finding"
```

---

## Task 35: META-003 (opaque agent)

**Files:**
- Modify: `internal/scanner/policy_selection.go`
- Modify: `internal/scanner/policy_selection_test.go`

- [ ] **Step 1: Test**

```go
func TestSelectPolicies_EmitsMETA003PerOpaqueAgent(t *testing.T) {
    inv := models.RepoInventory{Agents: []models.AgentDef{
        {Class: "Agent", FilePath: "main.py", Line: 5, Opaque: true},
        {Class: "Agent", FilePath: "main.py", Line: 20, Opaque: false},
        {Class: "Agent", FilePath: "main.py", Line: 30, Opaque: true},
    }}
    findings := scanner.SelectAndEmitMETA(models.RepoProfile{}, inv)
    var meta003 int
    for _, f := range findings { if f.RuleID == "META-003" { meta003++ } }
    if meta003 != 2 { t.Errorf("expected 2 META-003 (one per opaque), got %d", meta003) }
}
```

- [ ] **Step 2: Implementation — append**

```go
// META-003: opaque agents
for _, a := range inv.Agents {
    if !a.Opaque { continue }
    out = append(out, models.Finding{
        RuleID:   "META-003",
        Severity: models.SeverityInfo,
        Title:    "Agent configuration is opaque",
        FilePath: a.FilePath,
        Line:     a.Line,
        Explanation: "Agent configuration is opaque (kwargs come from a variable via **unpack, " +
            "or tools= is a non-literal expression like a function call); rules cannot evaluate against this agent.",
        SuggestedFix: "Inline the agent's kwargs at the constructor call site, or move the dynamic configuration into explicit code that trustabl can analyze.",
        Confidence:   1.0,
    })
}
```

- [ ] **Step 3: Run, commit**

```
go test ./internal/scanner/ -run TestSelectPolicies_EmitsMETA003 -v
git add internal/scanner/policy_selection.go internal/scanner/policy_selection_test.go
git commit -m "feat(scanner): META-003 opaque-agent info finding"
```

---

## Task 36: Wire `SelectAndEmitMETA` + `LoadFor` into `scanner.Run`

**Files:**
- Modify: `internal/scanner/scanner.go`

- [ ] **Step 1: Replace LoadRegistry call with LoadFor + META emission**

```go
// Phase 2b: policy selection
registry, err := rules.LoadFor(rules.DefaultFS(), inventory.SDKsDetected)
if err != nil { return models.ScanResult{}, err }
if len(cfg.Categories) > 0 {
    registry = registry.Subset(cfg.Categories...)
}
metaFindings := SelectAndEmitMETA(profile, inventory)

// Phase 2c: analysis
ruleFindings := registry.Run(profile, inventory, parsed)
findings := append(metaFindings, ruleFindings...)
```

- [ ] **Step 2: Run tests, commit**

```
go test ./...
git add internal/scanner/scanner.go
git commit -m "feat(scanner): wire LoadFor + META findings into Run"
```

---

# Phase J: OpenAI Rule Pack

## Task 37: Create `policies/openai_sdk/` structure + README

**Files:**
- Create: `internal/rules/policies/openai_sdk/README.md`

- [ ] **Step 1: Create the README**

```markdown
# OpenAI Agents SDK rule pack

Rules under this directory target the [OpenAI Agents SDK for Python](https://openai.github.io/openai-agents-python/).

## Supported SDK version

This pack is calibrated against the OpenAI Agents SDK as documented at the URL above (snapshot taken 2026-05-18). Since the SDK is pre-1.0, decorator names and `Agent(...)` kwargs may change. If a future SDK version renames `@function_tool` or restructures kwarg names, rule matches will silently degrade. Track upstream releases and bump this README's version line in the same PR.

## Layout

- `tool_definition.yaml` — OAI-001 (no docstring), OAI-002 (no typed params)
- `decorator_config.yaml` — OAI-003 (strict_mode=False), OAI-004 (no failure_error_function)
- `network.yaml` — OAI-005 (network call without timeout)
- `path_safety.yaml` — OAI-006 (unsafe path in I/O)
- `agent_safety.yaml` — OAI-101 (no input_guardrails + shell tools), OAI-102 (stop_on_first_tool), OAI-103 (loop pattern), OAI-104 (raw Agent + FS tools)
- `mcp_safety.yaml` — OAI-105 (mcp_servers + no input_guardrails)
- `tracing.yaml` — OAI-201 (default tracing in use)
```

- [ ] **Step 2: Commit**

```bash
git add internal/rules/policies/openai_sdk/README.md
git commit -m "docs(policies): OpenAI rule pack README and supported version"
```

---

## Task 38: OAI-001, OAI-002 — tool_definition.yaml

**Files:**
- Create: `internal/rules/policies/openai_sdk/tool_definition.yaml`

- [ ] **Step 1: Write the YAML**

```yaml
policy:
  id: openai_sdk_tool_definition
  name: OpenAI Agents SDK tool definition hygiene
  category: openai_sdk
  description: >
    Rules covering the structural hygiene of OpenAI Agents SDK @function_tool
    definitions. These map to the SDK's expectations: the docstring is the
    model-facing description; type hints drive the auto-generated JSON schema.
rules:
  - id: OAI-001
    title: Tool function has no docstring
    scope: tool
    severity: low
    confidence: 0.9
    language: python
    applies_to:
      - openai_tool
    match:
      has_docstring: false
    explanation: >
      The OpenAI Agents SDK uses the @function_tool's docstring as the
      description sent to the model when it decides whether to call this
      tool. Without a docstring, the model has no information to base its
      selection on, leading to incorrect or skipped tool calls.
    fix: >
      Add a one-line docstring describing what this tool does and when the
      model should call it. Optionally describe parameters in a structured
      format (Google or NumPy style) for richer schema metadata.

  - id: OAI-002
    title: Tool function has no type-annotated parameters
    scope: tool
    severity: medium
    confidence: 0.85
    language: python
    applies_to:
      - openai_tool
    match:
      has_params: true
      has_typed_params: false
    explanation: >
      The OpenAI Agents SDK auto-generates a JSON schema for the tool from
      Python type annotations. Without annotations, the schema is degraded
      (parameters become `Any`), and the model receives unconstrained inputs
      that frequently cause runtime errors.
    fix: >
      Add type hints to every parameter (`def fetch(url: str, timeout: int = 10) -> dict`).
      Use Pydantic models or TypedDict for structured inputs when complex.
```

- [ ] **Step 2: Build to check schema validation**

```
go build ./...
go test ./internal/rules/ -run TestLoad -v
```
Expected: PASS (loader accepts the new file).

- [ ] **Step 3: Commit**

```bash
git add internal/rules/policies/openai_sdk/tool_definition.yaml
git commit -m "feat(policies): OAI-001, OAI-002 (tool definition hygiene)"
```

---

## Task 39: OAI-003, OAI-004 — decorator_config.yaml

**Files:**
- Create: `internal/rules/policies/openai_sdk/decorator_config.yaml`

- [ ] **Step 1: Write the YAML**

```yaml
policy:
  id: openai_sdk_decorator_config
  name: OpenAI Agents SDK @function_tool decorator config
  category: openai_sdk
  description: >
    Rules covering the kwargs passed to @function_tool that affect runtime
    safety. These read ToolDef.Config, captured by discovery from the
    decorator's call arguments.
rules:
  - id: OAI-003
    title: Tool sets strict_mode=False
    scope: tool
    severity: medium
    confidence: 0.95
    language: python
    applies_to:
      - openai_tool
    match:
      tool_decorator_kwarg_value:
        kwarg: strict_mode
        value: "False"
    explanation: >
      Setting strict_mode=False on @function_tool relaxes JSON-schema
      enforcement at runtime. The model can then pass arguments that don't
      match the tool's type hints, leading to silent type errors or crashes
      inside the tool body.
    fix: >
      Remove the strict_mode=False kwarg (the default `True` is the safe
      setting). If the relaxation was needed for a specific input shape,
      adjust the type hints to accept that shape explicitly.

  - id: OAI-004
    title: Tool has no failure_error_function
    scope: tool
    severity: medium
    confidence: 0.7
    language: python
    applies_to:
      - openai_tool
    match:
      not:
        tool_decorator_kwarg_present:
          - failure_error_function
    explanation: >
      When a tool raises an exception, the OpenAI Agents SDK surfaces the
      exception's string representation to the model by default. The model
      receives an opaque error with no recovery contract, leading to
      hallucinated retries or confused responses.
      Setting failure_error_function lets you control what the model sees,
      typically a structured JSON describing the failure and what input to
      try instead.
    fix: >
      Pass `failure_error_function=` to the decorator. The function should
      receive the exception and the run context, and return a structured
      string the model can reason about.
```

- [ ] **Step 2: Build/test and commit**

```
go test ./internal/rules/ -run TestLoad -v
git add internal/rules/policies/openai_sdk/decorator_config.yaml
git commit -m "feat(policies): OAI-003, OAI-004 (decorator config safety)"
```

---

## Task 40: OAI-005 — network.yaml

**Files:**
- Create: `internal/rules/policies/openai_sdk/network.yaml`

- [ ] **Step 1: Write the YAML**

```yaml
policy:
  id: openai_sdk_network
  name: OpenAI Agents SDK network hygiene
  category: openai_sdk
  description: >
    Rules covering outbound network calls made from inside OpenAI Agents SDK
    tools. The SDK doesn't enforce timeouts; they must be set explicitly.
rules:
  - id: OAI-005
    title: Network call has no timeout
    scope: tool
    severity: high
    confidence: 0.85
    language: python
    applies_to:
      - openai_tool
    match:
      call_without_kwarg:
        callees:
          - requests.get
          - requests.post
          - requests.put
          - requests.delete
          - requests.patch
          - requests.head
          - httpx.get
          - httpx.post
          - httpx.put
          - httpx.delete
          - httpx.patch
        missing: timeout
    explanation: >
      An OpenAI Agents SDK tool that makes a network request without a
      timeout can hang indefinitely, blocking the agent's run loop and
      consuming the conversation's wall-clock budget. The SDK does not
      enforce timeouts on tool code.
    fix: >
      Pass `timeout=` (typically 5-30 seconds depending on the endpoint).
      Surface timeouts as a structured tool error the model can react to,
      using `failure_error_function` (see OAI-004).
    fix_hints:
      hook: pretooluse_validate
      guard: timeout_required
```

- [ ] **Step 2: Build/test and commit**

```
git add internal/rules/policies/openai_sdk/network.yaml
git commit -m "feat(policies): OAI-005 (network call without timeout)"
```

---

## Task 41: OAI-006 — path_safety.yaml

**Files:**
- Create: `internal/rules/policies/openai_sdk/path_safety.yaml`

- [ ] **Step 1: Write the YAML**

```yaml
policy:
  id: openai_sdk_path_safety
  name: OpenAI Agents SDK path safety
  category: openai_sdk
  description: >
    Rules covering tool I/O that touches the filesystem with a user-supplied
    path.
rules:
  - id: OAI-006
    title: Tool accepts path without normalization
    scope: tool
    severity: high
    confidence: 0.7
    language: python
    applies_to:
      - openai_tool
    match:
      call_uses_unnormalized_path_param:
        callees: [open, Path]
        callee_prefixes: [shutil., os.]
    explanation: >
      The tool accepts a path-like parameter and flows it into a filesystem
      call (`open`, `Path(...)`, `shutil.*`, `os.*`) without calling
      `.resolve()` or `os.path.realpath()` on it first. The agent's input
      may contain `..` segments that escape the intended directory, exposing
      arbitrary files on the host.
    fix: >
      Normalize the path before use: `p = Path(file_path).resolve()`. Then
      check that the resolved path is inside an explicit allowed-root
      directory before opening.
    fix_hints:
      hook: pretooluse_validate
      guard: path_normalization
```

- [ ] **Step 2: Commit**

```bash
git add internal/rules/policies/openai_sdk/path_safety.yaml
git commit -m "feat(policies): OAI-006 (unnormalized path in I/O)"
```

---

## Task 42: OAI-101, OAI-102, OAI-103, OAI-104 — agent_safety.yaml

**Files:**
- Create: `internal/rules/policies/openai_sdk/agent_safety.yaml`

- [ ] **Step 1: Write the YAML**

```yaml
policy:
  id: openai_sdk_agent_safety
  name: OpenAI Agents SDK agent wiring safety
  category: openai_sdk
  description: >
    Agent-scoped rules for the OpenAI Agents SDK. These fire per Agent(...) /
    SandboxAgent(...) call and inspect the constructor kwargs and the
    resolved tools / handoffs / guardrails graph.
rules:
  - id: OAI-101
    title: Agent has no input_guardrails AND wires shell or filesystem-touching tools
    scope: agent
    severity: high
    confidence: 0.85
    language: python
    applies_to:
      - openai_agent
      - openai_sandbox_agent
    match:
      all:
        - agent_kwarg_list_empty: [input_guardrails]
        - agent_uses_tool_kind: [shell_invocation]
    explanation: >
      This agent is wired with tools that execute shell commands or touch
      the filesystem, but has no input_guardrails configured. A prompt-
      injection user input can reach the shell-invoking tool with no
      pre-check; guardrails are the OpenAI Agents SDK's primary defense
      against this class of attack.
    fix: >
      Add at least one @input_guardrail validating that the user input is
      safe to dispatch to shell tools, and wire it via input_guardrails=[...]
      on the Agent(...) constructor. The guardrail should return
      GuardrailFunctionOutput(tripwire_triggered=True, ...) on rejected input.

  - id: OAI-102
    title: Agent uses tool_use_behavior="stop_on_first_tool"
    scope: agent
    severity: high
    confidence: 0.95
    language: python
    applies_to:
      - openai_agent
      - openai_sandbox_agent
    match:
      agent_kwarg_value:
        kwarg: tool_use_behavior
        value: stop_on_first_tool
    explanation: >
      With tool_use_behavior="stop_on_first_tool", the first tool's raw
      output becomes the agent's final response without any model
      post-processing. If the tool returns attacker-controlled data (web
      search results, untrusted file contents, MCP-provided tool output),
      that data is rendered to the user verbatim — an exfiltration and
      prompt-injection vector.
    fix: >
      Remove the tool_use_behavior="stop_on_first_tool" kwarg (the default
      "run_llm_again" is safe) or constrain it via StopAtTools(...) listing
      only tools whose outputs you control.

  - id: OAI-103
    title: tool_choice="required" combined with reset_tool_choice=False
    scope: agent
    severity: high
    confidence: 0.95
    language: python
    applies_to:
      - openai_agent
      - openai_sandbox_agent
    match:
      all:
        - agent_kwarg_value:
            kwarg: model_settings.tool_choice
            value: required
        - agent_kwarg_value:
            kwarg: reset_tool_choice
            value: "False"
    explanation: >
      The agent forces tool use on every turn (tool_choice="required") AND
      does not reset the tool choice after each call (reset_tool_choice=False).
      This is the SDK's documented loop pattern: the model is required to
      call a tool, calls one, and is required to call another, indefinitely.
    fix: >
      Either drop reset_tool_choice=False (the default True breaks the loop)
      or change tool_choice to "auto". If forced tool use is intentional,
      ensure at least one of the tools terminates the agent via
      tool_use_behavior=StopAtTools(...).

  - id: OAI-104
    title: Raw Agent (not SandboxAgent) wires shell or filesystem-touching tools
    scope: agent
    severity: medium
    confidence: 0.75
    language: python
    applies_to:
      - openai_agent
    match:
      all:
        - agent_class: [Agent]
        - agent_uses_tool_kind: [shell_invocation]
    explanation: >
      This agent uses the plain Agent class with tools that touch the
      shell or filesystem. The SDK ships SandboxAgent specifically to gate
      privileged tools inside an isolated workspace; using raw Agent
      surfaces the host environment to the agent directly.
    fix: >
      Switch from Agent(...) to SandboxAgent(...) and define a Manifest
      restricting the file paths and commands the agent can reach. If the
      privileged tools are needed but sandboxing is impractical, document
      the decision and acknowledge the elevated risk explicitly.
```

- [ ] **Step 2: Build/test and commit**

```
go test ./internal/rules/ -run TestLoad -v
git add internal/rules/policies/openai_sdk/agent_safety.yaml
git commit -m "feat(policies): OAI-101..104 (agent wiring safety)"
```

---

## Task 43: OAI-105 — mcp_safety.yaml

**Files:**
- Create: `internal/rules/policies/openai_sdk/mcp_safety.yaml`

- [ ] **Step 1: Write the YAML**

```yaml
policy:
  id: openai_sdk_mcp_safety
  name: OpenAI Agents SDK MCP integration safety
  category: openai_sdk
  description: >
    Rules covering risks from MCP servers wired into an OpenAI agent. MCP
    tool descriptions are attacker-controlled if the MCP server itself is
    untrusted; guardrails are the agent's primary defense.
rules:
  - id: OAI-105
    title: Agent has mcp_servers configured AND no input_guardrails
    scope: agent
    severity: high
    confidence: 0.85
    language: python
    applies_to:
      - openai_agent
      - openai_sandbox_agent
    match:
      all:
        - agent_kwarg_present: [mcp_servers]
        - agent_kwarg_list_empty: [input_guardrails]
    explanation: >
      The agent imports tools from one or more MCP servers (mcp_servers=)
      but has no input_guardrails. MCP tool descriptions are advertised by
      the MCP server, which is a separate trust boundary; an attacker-
      controlled or compromised MCP server can craft tool descriptions that
      bait the agent into harmful actions (tool poisoning).
    fix: >
      Add at least one @input_guardrail that inspects the user's input AND
      the resolved tool list before the model is invoked. Pin MCP servers
      to known-trusted URLs/checksums and document the trust assumption.
```

- [ ] **Step 2: Commit**

```bash
git add internal/rules/policies/openai_sdk/mcp_safety.yaml
git commit -m "feat(policies): OAI-105 (MCP without input_guardrails)"
```

---

## Task 44: OAI-201 — tracing.yaml

**Files:**
- Create: `internal/rules/policies/openai_sdk/tracing.yaml`

- [ ] **Step 1: Write the YAML**

```yaml
policy:
  id: openai_sdk_tracing
  name: OpenAI Agents SDK tracing configuration
  category: openai_sdk
  description: >
    Repo-scoped rules for OpenAI Agents SDK tracing behavior.
rules:
  - id: OAI-201
    title: Project uses default OpenAI tracing
    scope: repo
    severity: medium
    confidence: 0.8
    language: python
    applies_to:
      - openai_agents
    match:
      all:
        - repo_has_sdk_in_code: [openai_agents]
        - repo_uses_default_tracing: true
    explanation: >
      The project uses the OpenAI Agents SDK with default tracing enabled.
      By default, inputs, tool calls, tool outputs, and agent responses are
      sent to OpenAI's hosted tracing backend. For projects handling
      sensitive data (PII, credentials, internal documents), this is a data
      egress channel that's easy to miss.
    fix: >
      Either disable tracing entirely (set OPENAI_AGENTS_DISABLE_TRACING=1
      in the environment) OR register a custom trace processor via
      agents.tracing.add_trace_processor(...) that redacts sensitive fields
      before they leave the process.
```

- [ ] **Step 2: Commit**

```bash
git add internal/rules/policies/openai_sdk/tracing.yaml
git commit -m "feat(policies): OAI-201 (default tracing in use)"
```

---

## Task 45: Extend `policyRuleCases` with all 12 OAI rules + scope-aware test driver

**Files:**
- Modify: `internal/rules/policies_test.go`

- [ ] **Step 1: Add scope dispatch to test case struct**

Replace the `policyRuleCase` struct definition:

```go
type policyRuleCase struct {
    name      string
    ruleID    string
    scope     models.Scope     // NEW: tool | agent | repo
    kind      models.ToolKind  // for tool scope
    src       string           // Python snippet for tool/agent scopes

    // For agent scope:
    agentSrc string

    // For repo scope:
    repoFixture *repoFixture

    wantFires bool
}

type repoFixture struct {
    SDKs               []models.SDK
    DefaultTracing     bool
    Components         []models.AgentComponent
    SDKDeps            []models.SDKDep
}
```

- [ ] **Step 2: Update test driver to dispatch on scope**

```go
func TestPolicyRules(t *testing.T) {
    for _, tc := range policyRuleCases {
        t.Run(tc.name, func(t *testing.T) {
            switch tc.scope {
            case models.ScopeTool:
                runToolRuleCase(t, tc)
            case models.ScopeAgent:
                runAgentRuleCase(t, tc)
            case models.ScopeRepo:
                runRepoRuleCase(t, tc)
            default:
                t.Fatalf("unknown scope %q", tc.scope)
            }
        })
    }
}

func runToolRuleCase(t *testing.T, tc policyRuleCase) {
    d := loadToolRule(t, tc.ruleID)
    tool, pf := parsePy(t, tc.src, tc.kind)
    if !d.Applies(tool) {
        if tc.wantFires { t.Fatalf("rule %s does not Apply to a %s tool", tc.ruleID, tc.kind) }
        return
    }
    findings := d.Detect(tool, pf, models.RepoInventory{})
    assertFires(t, findings, tc.ruleID, tc.wantFires)
}

func runAgentRuleCase(t *testing.T, tc policyRuleCase) {
    d := loadAgentRule(t, tc.ruleID)
    parsed := parsePyFile(t, "main.py", tc.agentSrc)
    inv := models.RepoInventory{
        Tools:  analysis.DiscoverToolsFromParsed([]analysis.ParsedFile{parsed}),
        Agents: analysis.DiscoverAgents([]analysis.ParsedFile{parsed}),
    }
    analysis.ResolveEdges(&inv, []analysis.ParsedFile{parsed})
    if len(inv.Agents) == 0 {
        if tc.wantFires { t.Fatalf("expected an Agent in test src") }
        return
    }
    a := inv.Agents[0]
    if !d.Applies(a) {
        if tc.wantFires { t.Fatalf("rule %s does not Apply to agent class %s", tc.ruleID, a.Class) }
        return
    }
    findings := d.Detect(a, inv)
    assertFires(t, findings, tc.ruleID, tc.wantFires)
}

func runRepoRuleCase(t *testing.T, tc policyRuleCase) {
    d := loadRepoRule(t, tc.ruleID)
    f := tc.repoFixture
    inv := models.RepoInventory{
        SDKsDetected:       f.SDKs,
        UsesDefaultTracing: f.DefaultTracing,
    }
    profile := models.RepoProfile{
        SDKDeps:  f.SDKDeps,
        Manifest: models.ScanManifest{Components: f.Components},
    }
    if !d.Applies(profile, inv) {
        if tc.wantFires { t.Fatalf("rule %s does not Apply to repo fixture", tc.ruleID) }
        return
    }
    findings := d.Detect(profile, inv)
    assertFires(t, findings, tc.ruleID, tc.wantFires)
}

func assertFires(t *testing.T, findings []models.Finding, ruleID string, want bool) {
    t.Helper()
    fired := false
    for _, f := range findings { if f.RuleID == ruleID { fired = true; break } }
    if fired != want { t.Errorf("rule %s: fired=%v, want %v", ruleID, fired, want) }
}

// loadToolRule, loadAgentRule, loadRepoRule walk the embedded policies and
// return the rule wrapped as a typed detector.
func loadToolRule(t *testing.T, ruleID string) detectors.ToolDetector {
    t.Helper()
    policies, err := rules.Load(rules.DefaultFS())
    if err != nil { t.Fatalf("load: %v", err) }
    for _, p := range policies {
        for _, r := range p.Rules {
            if r.ID == ruleID && r.Scope == models.ScopeTool {
                return rules.NewToolRuleDetector(r)
            }
        }
    }
    t.Fatalf("tool-scope rule %s not found", ruleID)
    return nil
}
// loadAgentRule and loadRepoRule follow the same shape.
```

- [ ] **Step 3: Add OAI test cases**

Append to `policyRuleCases`:

```go
// OAI-001
{name: "OAI-001 fires no docstring", ruleID: "OAI-001", scope: models.ScopeTool, kind: models.KindOpenAITool, src: `
def my_tool(x: str) -> str:
    return x
`, wantFires: true},
{name: "OAI-001 silent with docstring", ruleID: "OAI-001", scope: models.ScopeTool, kind: models.KindOpenAITool, src: `
def my_tool(x: str) -> str:
    """A tool."""
    return x
`, wantFires: false},

// OAI-002
{name: "OAI-002 fires untyped", ruleID: "OAI-002", scope: models.ScopeTool, kind: models.KindOpenAITool, src: `
def my_tool(x, y):
    """T."""
    return x
`, wantFires: true},
{name: "OAI-002 silent typed", ruleID: "OAI-002", scope: models.ScopeTool, kind: models.KindOpenAITool, src: `
def my_tool(x: str, y: int) -> str:
    """T."""
    return x
`, wantFires: false},

// OAI-003 (requires decorator kwargs captured into Config)
// NOTE: parsePy must round-trip through DiscoverTools so Config is populated.
{name: "OAI-003 fires strict_mode=False", ruleID: "OAI-003", scope: models.ScopeTool, kind: models.KindOpenAITool, src: `
from agents import function_tool
@function_tool(strict_mode=False)
def my_tool(x: str) -> str:
    """T."""
    return x
`, wantFires: true},
{name: "OAI-003 silent default strict_mode", ruleID: "OAI-003", scope: models.ScopeTool, kind: models.KindOpenAITool, src: `
from agents import function_tool
@function_tool
def my_tool(x: str) -> str:
    """T."""
    return x
`, wantFires: false},

// OAI-004
{name: "OAI-004 fires no failure_error_function", ruleID: "OAI-004", scope: models.ScopeTool, kind: models.KindOpenAITool, src: `
from agents import function_tool
@function_tool
def my_tool(x: str) -> str:
    """T."""
    return x
`, wantFires: true},
{name: "OAI-004 silent with failure_error_function", ruleID: "OAI-004", scope: models.ScopeTool, kind: models.KindOpenAITool, src: `
from agents import function_tool
def err(c, e): return str(e)
@function_tool(failure_error_function=err)
def my_tool(x: str) -> str:
    """T."""
    return x
`, wantFires: false},

// OAI-005
{name: "OAI-005 fires no timeout", ruleID: "OAI-005", scope: models.ScopeTool, kind: models.KindOpenAITool, src: `
import requests
def my_tool(url: str) -> dict:
    """T."""
    return requests.get(url).json()
`, wantFires: true},
{name: "OAI-005 silent with timeout", ruleID: "OAI-005", scope: models.ScopeTool, kind: models.KindOpenAITool, src: `
import requests
def my_tool(url: str) -> dict:
    """T."""
    return requests.get(url, timeout=10).json()
`, wantFires: false},

// OAI-006
{name: "OAI-006 fires unnormalized path", ruleID: "OAI-006", scope: models.ScopeTool, kind: models.KindOpenAITool, src: `
def my_tool(file_path: str) -> str:
    """T."""
    with open(file_path) as f: return f.read()
`, wantFires: true},
{name: "OAI-006 silent with resolve", ruleID: "OAI-006", scope: models.ScopeTool, kind: models.KindOpenAITool, src: `
from pathlib import Path
def my_tool(file_path: str) -> str:
    """T."""
    p = Path(file_path).resolve()
    with open(p) as f: return f.read()
`, wantFires: false},

// OAI-101
{name: "OAI-101 fires shell + no guardrails", ruleID: "OAI-101", scope: models.ScopeAgent, agentSrc: `
from agents import Agent
import subprocess
def run_cmd(c: str) -> str:
    """."""
    return subprocess.check_output(c, shell=True).decode()
agent = Agent(name="x", tools=[run_cmd])
`, wantFires: true},
{name: "OAI-101 silent with guardrails", ruleID: "OAI-101", scope: models.ScopeAgent, agentSrc: `
from agents import Agent, input_guardrail, GuardrailFunctionOutput
import subprocess

@input_guardrail
def g(c, a, i):
    return GuardrailFunctionOutput(output_info=None, tripwire_triggered=False)

def run_cmd(c: str) -> str:
    """."""
    return subprocess.check_output(c, shell=True).decode()

agent = Agent(name="x", tools=[run_cmd], input_guardrails=[g])
`, wantFires: false},

// OAI-102
{name: "OAI-102 fires stop_on_first_tool", ruleID: "OAI-102", scope: models.ScopeAgent, agentSrc: `
from agents import Agent
agent = Agent(name="x", tool_use_behavior="stop_on_first_tool")
`, wantFires: true},
{name: "OAI-102 silent default", ruleID: "OAI-102", scope: models.ScopeAgent, agentSrc: `
from agents import Agent
agent = Agent(name="x")
`, wantFires: false},

// OAI-103
{name: "OAI-103 fires loop pattern", ruleID: "OAI-103", scope: models.ScopeAgent, agentSrc: `
from agents import Agent, ModelSettings
agent = Agent(name="x", model_settings=ModelSettings(tool_choice="required"), reset_tool_choice=False)
`, wantFires: true},
{name: "OAI-103 silent reset_tool_choice default", ruleID: "OAI-103", scope: models.ScopeAgent, agentSrc: `
from agents import Agent, ModelSettings
agent = Agent(name="x", model_settings=ModelSettings(tool_choice="required"))
`, wantFires: false},

// OAI-104
{name: "OAI-104 fires raw Agent + shell tool", ruleID: "OAI-104", scope: models.ScopeAgent, agentSrc: `
from agents import Agent
import subprocess
def run_cmd(c: str) -> str:
    """."""
    return subprocess.check_output(c, shell=True).decode()
agent = Agent(name="x", tools=[run_cmd])
`, wantFires: true},
{name: "OAI-104 silent SandboxAgent", ruleID: "OAI-104", scope: models.ScopeAgent, agentSrc: `
from agents import SandboxAgent
import subprocess
def run_cmd(c: str) -> str:
    """."""
    return subprocess.check_output(c, shell=True).decode()
agent = SandboxAgent(name="x", tools=[run_cmd])
`, wantFires: false},

// OAI-105
{name: "OAI-105 fires mcp + no guardrails", ruleID: "OAI-105", scope: models.ScopeAgent, agentSrc: `
from agents import Agent
agent = Agent(name="x", mcp_servers=["http://example/mcp"])
`, wantFires: true},
{name: "OAI-105 silent mcp + guardrails", ruleID: "OAI-105", scope: models.ScopeAgent, agentSrc: `
from agents import Agent, input_guardrail, GuardrailFunctionOutput

@input_guardrail
def g(c, a, i):
    return GuardrailFunctionOutput(output_info=None, tripwire_triggered=False)

agent = Agent(name="x", mcp_servers=["http://example/mcp"], input_guardrails=[g])
`, wantFires: false},

// OAI-201
{name: "OAI-201 fires default tracing + openai use", ruleID: "OAI-201", scope: models.ScopeRepo,
 repoFixture: &repoFixture{SDKs: []models.SDK{models.SDKOpenAIAgents}, DefaultTracing: true},
 wantFires: true},
{name: "OAI-201 silent custom processor", ruleID: "OAI-201", scope: models.ScopeRepo,
 repoFixture: &repoFixture{SDKs: []models.SDK{models.SDKOpenAIAgents}, DefaultTracing: false},
 wantFires: false},
```

- [ ] **Step 4: Run tests, commit**

```
go test ./internal/rules/ -run TestPolicyRules -v
```
Expected: PASS for all OAI cases. `TestPolicyRules_AllRulesCovered` also passes (every rule has cases).

```bash
git add internal/rules/policies_test.go
git commit -m "test(rules): fire/silent cases for OAI-001..201 with scope dispatch"
```

---

# Phase K: Determinism Test

## Task 46: Create `testdata/deterministic-fixture/`

**Files:**
- Create: `testdata/deterministic-fixture/pyproject.toml`
- Create: `testdata/deterministic-fixture/agent.py`

- [ ] **Step 1: Fixture pyproject.toml**

```toml
[project]
name = "deterministic-fixture"
version = "0.0.1"
dependencies = ["openai-agents>=0.0.1"]
```

- [ ] **Step 2: Fixture agent.py — one tool + one agent, several findings**

```python
"""Deterministic-fixture agent.

This file is the controlled input for TestScanDeterministic. It deliberately
triggers a known set of rules so the artifacts have predictable content.
DO NOT modify casually — changes here change the determinism test's
generated bytes.
"""
from agents import Agent, function_tool
import requests

@function_tool
def fetch(url: str) -> dict:
    """Fetch a URL."""
    return requests.get(url).json()   # OAI-005 fires (no timeout)

agent = Agent(name="fixture", tools=[fetch])  # OAI-101 silent (no shell tool); OAI-104 silent
```

- [ ] **Step 3: Commit**

```bash
git add testdata/deterministic-fixture/
git commit -m "test(fixture): deterministic-fixture for byte-equality test"
```

---

## Task 47: Implement `TestScanDeterministic`

**Files:**
- Create: `internal/scanner/determinism_test.go`

- [ ] **Step 1: Write the test**

```go
package scanner_test

import (
    "bytes"
    "path/filepath"
    "runtime"
    "testing"

    "github.com/trustabl/trustabl/internal/scanner"
)

// TestScanDeterministic asserts that two runs over the same fixture produce
// byte-identical artifacts and the same ScanID. Guards the contract
// documented in ARCHITECTURE.md §7.
func TestScanDeterministic(t *testing.T) {
    _, thisFile, _, _ := runtime.Caller(0)
    fixture := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "deterministic-fixture")

    r1, err := scanner.Run(scanner.Config{Target: fixture})
    if err != nil { t.Fatalf("first run: %v", err) }
    r2, err := scanner.Run(scanner.Config{Target: fixture})
    if err != nil { t.Fatalf("second run: %v", err) }

    if r1.ScanID != r2.ScanID {
        t.Errorf("ScanID drifted: %q vs %q", r1.ScanID, r2.ScanID)
    }
    if len(r1.GeneratedArtifacts) != len(r2.GeneratedArtifacts) {
        t.Fatalf("artifact count differs: %d vs %d", len(r1.GeneratedArtifacts), len(r2.GeneratedArtifacts))
    }
    for i, a1 := range r1.GeneratedArtifacts {
        a2 := r2.GeneratedArtifacts[i]
        if a1.Path != a2.Path {
            t.Errorf("artifact %d path differs: %q vs %q", i, a1.Path, a2.Path)
            continue
        }
        if !bytes.Equal(a1.Content, a2.Content) {
            t.Errorf("artifact %q content not byte-equal across runs (len %d vs %d)",
                a1.Path, len(a1.Content), len(a2.Content))
        }
    }
}
```

- [ ] **Step 2: Run, commit**

```
go test ./internal/scanner/ -run TestScanDeterministic -v
```
Expected: PASS.

```bash
git add internal/scanner/determinism_test.go
git commit -m "test(scanner): byte-equality determinism regression"
```

---

# Phase L: Doc Updates

## Task 48: Update ARCHITECTURE.md to describe the new pipeline

**Files:**
- Modify: `ARCHITECTURE.md`

- [ ] **Step 1: Replace §2 Pipeline diagram**

Replace the existing diagram in §2 with:

```
target (path or URL)
    │
    ▼
PHASE 1 — Reconnaissance (ingestion.Recon)
  Output: RepoProfile {Languages, SDKDeps, Manifest, Components}
    │
    ▼
PHASE 2a — Inventory (analysis: DiscoverTools, DiscoverAgents,
                       DiscoverGuardrails, DiscoverSessions, ResolveEdges)
  Output: RepoInventory {Tools, Agents, Guardrails, Sessions, SDKsDetected}
    │
    ▼
PHASE 2b — Policy selection (rules.LoadFor + SelectAndEmitMETA)
  Loads policy packs for SDKsDetected; emits META-001/002/003
    │
    ▼
PHASE 2c — Analysis (detectors.Registry.Run)
  ToolDetectors over inv.Tools
  AgentDetectors over inv.Agents
  RepoDetectors run once
    │
    ▼
Scoring → Generation → Review
```

- [ ] **Step 2: Update §4 Detectors section to describe three interfaces**

Replace the `type Detector interface { ... }` block with:

```go
type ToolDetector interface {
    RuleID() string
    Category() models.DetectorCategory
    Applies(models.ToolDef) bool
    Detect(models.ToolDef, analysis.ParsedFile, models.RepoInventory) []models.Finding
}
type AgentDetector interface {
    RuleID() string
    Category() models.DetectorCategory
    Applies(models.AgentDef) bool
    Detect(models.AgentDef, models.RepoInventory) []models.Finding
}
type RepoDetector interface {
    RuleID() string
    Category() models.DetectorCategory
    Applies(models.RepoProfile, models.RepoInventory) bool
    Detect(models.RepoProfile, models.RepoInventory) []models.Finding
}
```

And update the prose: "The `detectors` package owns three typed interfaces (`ToolDetector`, `AgentDetector`, `RepoDetector`) and a `Registry` that holds three slices and dispatches per scope." Remove all references to the old single-interface `Detector` and `Singleton()`.

- [ ] **Step 3: Update §3 Data model**

Add `RepoProfile`, `RepoInventory`, `AgentDef`, `GuardrailDef`, `SessionUse`, `SDKDep`, `SDK` to the data model section. Remove `HasClaudeSDKDependency`/`HasOpenShellArtifact` from the `ScanManifest` Go snippet. Add the new fields to `ToolDef` (`Config map[string]string`).

- [ ] **Step 4: Add §5.1 about META findings**

Add a subsection under §5 documenting the engine-emitted META-001, META-002, META-003 findings — what they fire on, why they're not in YAML.

- [ ] **Step 5: Run, commit**

```bash
git add ARCHITECTURE.md
git commit -m "docs(architecture): describe two-phase pipeline, three Detector interfaces, RepoProfile/Inventory"
```

---

## Task 49: Update README.md detector table + SDK coverage

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add OAI rows to the shipped detector table**

In the "Detectors shipped in this skeleton" section, change the table header:

```markdown
Naming: `CSDK-NNN` for Claude SDK, `OSH-NNN` for OpenShell sandbox, `OAI-NNN` for OpenAI Agents SDK. Rules live in YAML under `internal/rules/policies/<category>/<topic>.yaml`.
```

Append OAI rows to the table:

```markdown
| OAI-001 | Tool function has no docstring                       | low      | `openai_sdk/tool_definition.yaml` |
| OAI-002 | Tool function has no type-annotated params           | medium   | `openai_sdk/tool_definition.yaml` |
| OAI-003 | Tool sets `strict_mode=False`                        | medium   | `openai_sdk/decorator_config.yaml` |
| OAI-004 | Tool has no `failure_error_function`                  | medium   | `openai_sdk/decorator_config.yaml` |
| OAI-005 | Network call without timeout (OpenAI-framed)         | high     | `openai_sdk/network.yaml`         |
| OAI-006 | Unnormalized path in I/O (OpenAI-framed)             | high     | `openai_sdk/path_safety.yaml`     |
| OAI-101 | No input_guardrails + shell/FS tools                 | high     | `openai_sdk/agent_safety.yaml`    |
| OAI-102 | `tool_use_behavior="stop_on_first_tool"`             | high     | `openai_sdk/agent_safety.yaml`    |
| OAI-103 | Loop pattern (tool_choice=required + reset=False)    | high     | `openai_sdk/agent_safety.yaml`    |
| OAI-104 | Raw Agent (not SandboxAgent) + shell tools           | medium   | `openai_sdk/agent_safety.yaml`    |
| OAI-105 | mcp_servers + no input_guardrails                    | high     | `openai_sdk/mcp_safety.yaml`      |
| OAI-201 | Default OpenAI tracing in use                         | medium   | `openai_sdk/tracing.yaml`         |
```

- [ ] **Step 2: Update SDK coverage section**

Change "OpenAI Agents SDK tools are discovered ... but no OpenAI-framed rules are shipped yet" to past tense + list. Add a sentence about agent-scope and repo-scope coverage.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs(readme): add OAI detector rows and update SDK coverage"
```

---

## Task 50: Update `internal/rules/policies/CLAUDE.md` with per-scope `applies_to`

**Files:**
- Modify: `internal/rules/policies/CLAUDE.md`

- [ ] **Step 1: Add per-scope applies_to value table**

Append after the existing "SDK scope per rule" section:

```markdown
## Per-scope `applies_to` values

The value space of `applies_to` depends on the rule's `scope:`. The loader
rejects mismatches at load time.

| Scope    | `applies_to` values                                                          |
| -------- | ---------------------------------------------------------------------------- |
| `tool`   | `claude_sdk_tool`, `openai_tool`, `mcp_tool`, `shell_invocation`, `unknown` |
| `agent`  | `openai_agent`, `openai_sandbox_agent`, `claude_agent_definition`            |
| `repo`   | `claude_sdk`, `openai_agents`, `openshell`, `mcp`                            |

Note: the agent-scope values (`openai_agent`, etc.) are semantic kind aliases
for the (SDK, Python class) tuple. The `agent_class:` predicate, by contrast,
matches against the literal Python class name (`Agent`, `SandboxAgent`,
`AgentDefinition`).
```

- [ ] **Step 2: Commit**

```bash
git add internal/rules/policies/CLAUDE.md
git commit -m "docs(policies): per-scope applies_to value table"
```

---

# Phase M: Orthogonal Cleanup

## Task 51: Delete `call_uses_param` predicate

**Files:**
- Modify: `internal/rules/schema.go` (remove `CallUsesParam *CallUsesParamExpr`)
- Modify: `internal/rules/predicates.go` (remove `PredCallUsesParam` + `CallUsesParamExpr`)
- Modify: `internal/rules/evaluator.go` (remove the dispatch case)
- Modify: `internal/rules/predicates_test.go` (remove tests for it)
- Modify: `internal/rules/schema.yaml` (remove the documentation block)

- [ ] **Step 1: Verify no shipped rule uses it**

```
grep -r "call_uses_param" internal/rules/policies/
```
Expected: empty (the `_unnormalized_path_param` variant is the one used).

- [ ] **Step 2: Delete all references**

In each file above, remove every mention of `CallUsesParam`, `CallUsesParamExpr`, `PredCallUsesParam`, and `call_uses_param`.

- [ ] **Step 3: Run all tests, commit**

```
go test ./...
git add internal/rules/
git commit -m "refactor(rules): delete unused call_uses_param predicate (superseded by call_uses_unnormalized_path_param)"
```

---

# Phase N: CI Workflow

## Task 52: Add GitHub Actions test workflow

**Files:**
- Create: `.github/workflows/test.yml`

- [ ] **Step 1: Workflow content**

```yaml
name: test

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true
      - name: go vet
        run: go vet ./...
      - name: go test
        env:
          CGO_ENABLED: '1'
        run: go test -race ./...
      - name: go build
        env:
          CGO_ENABLED: '1'
        run: go build -o /tmp/trustabl ./cmd/trustabl
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/test.yml
git commit -m "ci: add GitHub Actions workflow for go vet/test/build"
```

---

# Acceptance check (run after all tasks)

- [ ] **All tests green**

```
go vet ./...
go test ./...
go build ./cmd/trustabl
```

- [ ] **Spec acceptance criteria verified**

Walk through §10 of [the spec](../specs/2026-05-18-openai-agents-sdk-design.md):

1. `go test ./...` passes ✓
2. `trustabl scan` on a mis-wired OpenAI agent produces OAI-101..105 findings — write a smoke test scenario
3. Repo with only `openai-agents` dep produces exactly one META-002 — covered by `TestSelectPolicies_EmitsMETA002ForDepDrift`
4. Repo with an unknown SDK produces META-001 — covered
5. `Agent(**config)` produces one META-003 per agent — covered
6. `TestPolicyRules_AllRulesCovered` passes ✓
7. `TestScanDeterministic` passes ✓
8. `TestScanExamples_NoCrash` passes with the extended pipeline ✓
9. `schema.yaml` documents `scope:` and all new predicates — Task 31
10. ARCHITECTURE.md updated — Task 48

If any item fails, return to the relevant Phase tasks and resolve before declaring done.
