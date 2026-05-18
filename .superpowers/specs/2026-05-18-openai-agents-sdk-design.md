# OpenAI Agents SDK detection (Python)

**Status:** Design approved 2026-05-18, implementation pending.
**Scope:** Add static-analysis detection for the OpenAI Agents SDK (Python tools and agents) to trustabl, end-to-end through the scanner pipeline. Generalize the rules engine from tool-scoped detection only to a three-scope model (tool / agent / repo) and stage the pipeline into two explicit phases (recon + analysis) along the way.

This spec is the source of truth for what we're building and why. The implementation plan that follows (under `.superpowers/plans/`) decomposes this into PR-sized steps.

---

## 1. Overview

Today trustabl scans Claude Agent SDK repositories. Tool discovery already recognizes `@function_tool` (OpenAI Agents SDK) decorators and tags them with `Kind: openai_tool`, but no OpenAI-framed rules are shipped — those tools surface in `manifest.tools` and then no rule fires against them. This spec closes that gap by adding the first OpenAI-specific policy pack, *and* takes the engine generalizations needed to make the rules expressive (agent-scope rules) and the scan honest (per-SDK policy selection, "unaudited SDK" finding for unknown SDKs).

The end state for the user: `trustabl scan ./openai-agent-repo` produces findings keyed to specific `Agent(...)` declarations as well as to tool definitions, identifies which SDKs are present in code, refuses to silently no-op on unknown SDKs, and remains deterministic across repeat runs.

The end state for contributors: one SDK has been spec'd end-to-end (study → discovery strategy → policy pack → tests). Subsequent SDKs (LangGraph, Pydantic AI, Vercel AI SDK, etc.) get separate spec → plan → implementation cycles that template off this one.

---

## 2. Goals and non-goals

### Goals (v1 of this spec)

- Phase 1 reconnaissance produces a typed `RepoProfile` including languages, SDK deps from manifests, and component artifacts.
- Phase 2a per-language inventory produces typed `ToolDef` / `AgentDef` / `GuardrailDef` / `SessionUse` records with edges between them, for Python.
- Phase 2b policy selection loads only the policy packs for SDKs observed in code; emits info findings for unaudited SDKs (META-001), dep drift (META-002), and opaque agent config (META-003).
- Phase 2c rules engine evaluates 12 OpenAI-specific rules across three scopes (6 tool + 5 agent + 1 repo) plus all existing Claude SDK / OpenShell rules.
- Schema changes: required `scope: tool | agent | repo`, `singleton:` removed, `applies_to` value space scope-dependent.
- Three new tool-scope predicates + dotted-path support in `agent_kwarg_value`.
- Detector interface split into three typed interfaces (`ToolDetector`, `AgentDetector`, `RepoDetector`), `Singleton()` removed.
- Test coverage: predicate unit tests, per-rule fire/silent, edge-resolution unit tests, Phase 2b META-finding tests, examples-corpus sweep, byte-equality determinism test.
- Migration: existing 12 rules get `scope:` field set explicitly; OSH-004 becomes `scope: repo`.

### Non-goals (deferred)

- **TypeScript / JavaScript / Go tool discovery.** Phase 2a is Python-only in this spec.
- **Other SDKs.** No LangGraph / Pydantic AI / Vercel AI SDK / Mastra detection. Each becomes its own spec.
- **Hosted-tool rules.** `WebSearchTool`, `ComputerTool`, `CodeInterpreterTool`, `FileSearchTool`, `HostedMCPTool` discovery slot is reserved (`HostedTools []HostedToolDef`) but no rules ship in v1.
- **Cross-scope queries.** Agent rules cannot ask "does this agent's tool have finding X?" — creates ordering dependencies; revisit when a concrete rule needs it.
- **Handoff hygiene rules.** Comparing parent and subagent guardrails requires handoff graph traversal; defer.
- **Session encryption advisory.** `SQLiteSession` vs `EncryptedSession` heuristic too fragile for v1.
- **LLM enrichment** of low-confidence rule hits. `internal/inference/router.go` stays a stub.
- **Corpus eval benchmark.** 20–40 labelled repos is the MVP gate, addressed separately.
- **CI workflow** — flagged as an orthogonal gap in §10, recommend including in the implementation plan but it's not a feature of this spec.

---

## 3. Background: OpenAI Agents SDK security surface

Everything we detect traces back to a mechanism in the SDK. Grouped by where it lives in user code.

### 3.1 Tool definition surface

Three paths to register a tool, all of which discovery must catch:

- **`@function_tool`** decorator. Kwargs: `strict_mode`, `name_override`, `description_override`, `failure_error_function`, `use_docstring_info`. Type hints are mandatory — the SDK auto-generates a JSON schema from annotations + docstring. Missing hints produce a degraded schema the model misuses.
- **`FunctionTool(...)`** class constructor — non-decorator path; same security concerns.
- **Hosted tool classes:** `WebSearchTool`, `FileSearchTool`, `ComputerTool`, `CodeInterpreterTool`, `HostedMCPTool` — privileged primitives instantiated as objects.

Security implications: missing type hints → broken schema → model passes whatever; `strict_mode=False` → relaxed runtime enforcement; `failure_error_function=None` → exceptions surface as opaque strings; hosted tool with no scoping → unbounded capability.

### 3.2 Agent wiring surface

Confirmed `Agent(...)` kwargs (current docs): `name`, `instructions`, `prompt`, `handoff_description`, `handoffs`, `model`, `model_settings`, `tools`, `mcp_servers`, `mcp_config`, `input_guardrails`, `output_guardrails`, `output_type`, `hooks`, `tool_use_behavior`, `reset_tool_choice`.

Security mechanisms:

- **Guardrails.** `input_guardrails=[…]` / `output_guardrails=[…]` accept lists of `@input_guardrail` / `@output_guardrail`-decorated functions returning `GuardrailFunctionOutput(tripwire_triggered=…)`. Tripwire short-circuits execution.
- **`tool_use_behavior`.** `"run_llm_again"` (default, safe), `"stop_on_first_tool"` (first tool's raw output becomes the final response with no model post-processing — exfiltration / prompt-injection vector), `StopAtTools(...)`, or a custom function.
- **`reset_tool_choice`.** Default `True`. Combined with `model_settings.tool_choice="required"` and `reset_tool_choice=False`, the agent loops forever.
- **`model_settings.tool_choice`.** `"auto"` (default), `"required"`, `"none"`, or specific tool name.
- **`handoffs`.** Conversation history passes to the subagent. Privilege escalation if the subagent is less constrained.
- **`mcp_servers` / `mcp_config`.** MCP-supplied tool descriptions are attacker-controlled if the server is untrusted.

### 3.3 Sandbox surface

`SandboxAgent` — `Agent` variant that runs in an isolated workspace defined by a `Manifest`. The SDK's own answer to sandboxing. Repos using raw `Agent` for shell-touching tools are the safety gap.

### 3.4 Session and tracing surface

- Sessions: `SQLiteSession`, `SQLAlchemySession`, `RedisSession`, `MongoDBSession`, `EncryptedSession`, `AdvancedSQLiteSession`. Persist conversation history.
- Tracing: on by default. Inputs, outputs, tool calls flow through. Configurable via env vars and processor registration; default emits to OpenAI's hosted trace backend.

### 3.5 What the SDK does NOT enforce (the detection opportunities)

- No per-tool timeout enforcement.
- No path normalization on tool params.
- No idempotency wrapping for mutating tools.
- No automatic guardrail attachment.
- No automatic redaction in tracing.

---

## 4. Architecture

### 4.1 Approach — minimal extension

The existing rules engine stays. We add:

1. A `scope:` field on every rule (`tool` | `agent` | `repo`) — `singleton:` is removed in the same change.
2. A second discovery pass that extracts `AgentDef`s and resolves edges from agents to tools / guardrails / handoffs.
3. New predicate families for agent and repo scopes.
4. Three typed `Detector` interfaces instead of one.
5. Data-driven policy selection — load only packs for SDKs observed in code.

Not adopting: a sum-typed target on a single interface (Option B in the design discussion). Type safety on the detector boundary compounds with every new detector; the additional ~100 lines of churn pays back the first time a scope mix-up would have shipped in Option B.

### 4.2 Two-phase pipeline

```
target (path or URL)
    │
    ▼
PHASE 1 — Reconnaissance
  Importer + Normalizer (extended)
  Output: RepoProfile {Languages, SDKDeps, Manifest, Components}
    │
    ▼
PHASE 2a — Inventory (per-language AST)
  Discovery (extended): ToolDefs, AgentDefs, GuardrailDefs, SessionUses, edges
  Output: RepoInventory {Tools, Agents, Guardrails, Sessions, SDKsDetected}
    │
    ▼
PHASE 2b — Policy selection (engine-level findings)
  Load policies for SDKsDetected; emit META-001 / META-002 / META-003
  Output: scoped Registry + 0..N META findings
    │
    ▼
PHASE 2c — Analysis
  ToolDetectors  → ToolDef
  AgentDetectors → AgentDef + RepoInventory
  RepoDetectors  → RepoProfile + RepoInventory
  Output: []Finding
    │
    ▼
Findings → Scoring → Generation → Review
```

Per-phase commitments are codified in [CLAUDE.md](../../CLAUDE.md) (root); see "Two-phase scanning pipeline" section.

### 4.3 Three scopes — recap

Every rule classified into exactly one scope:

| Scope    | Fires per                          | Input to predicate                                |
| -------- | ---------------------------------- | ------------------------------------------------- |
| `tool`   | per tool definition                | `ToolDef` + `ParsedFile` + `RepoInventory`        |
| `agent`  | per agent declaration              | `AgentDef` (with edges) + `RepoInventory`         |
| `repo`   | once per scan                      | `RepoProfile` + `RepoInventory`                   |

Findings are attributed to the matching source location: tool file/line, agent constructor call site, or manifest.

### 4.4 What changes vs. current code

| Current                                                  | After this spec                                                   |
| -------------------------------------------------------- | ------------------------------------------------------------------ |
| Importer + Normalizer return `ScanManifest`              | Phase 1 returns a typed `RepoProfile` including `SDKDeps []SDKDep` |
| `DiscoverTools` returns `[]ToolDef, []ParsedFile`        | Phase 2a returns a `RepoInventory` with Tools + Agents + Guardrails + Sessions + edges + SDKsDetected |
| `LoadRegistry(fsys)` loads all embedded YAML eagerly     | `LoadFor(fsys, sdks)` loads only the relevant policy packs (Phase 2b) |
| One `Detector` interface, scope implicit via `Singleton` | Three typed interfaces: `ToolDetector`, `AgentDetector`, `RepoDetector` |
| Per-tool dispatch in `Registry.Run`                      | Three scope-aware dispatch loops in `Registry.Run`                 |
| `singleton: bool` rule flag                              | Required `scope: tool \| agent \| repo` field                      |
| `applies_to` always = ToolKind list                      | `applies_to` value space depends on scope (ToolKind / AgentClass / SDK) |
| `HasClaudeSDKDependency` / `HasOpenShellArtifact` booleans | Replaced by `RepoProfile.SDKDeps []SDKDep` (typed list)            |
| No "we don't audit this" signal                          | META-001 / META-002 / META-003 info findings                       |

---

## 5. Components

### 5.1 New types in `internal/models/`

```go
// Phase 1 output
type RepoProfile struct {
    Languages []Language
    SDKDeps   []SDKDep
    Manifest  ScanManifest  // existing — file inventory + components
}

type SDKDep struct {
    Name       string  // "openai-agents", "claude-agent-sdk", "pydantic-ai", ...
    Source     string  // path to manifest file
    Confidence float64
}

// Phase 2a output
type RepoInventory struct {
    Tools        []ToolDef         // existing, extended with Config map
    Agents       []AgentDef        // NEW
    Guardrails   []GuardrailDef    // NEW
    Sessions    []SessionUse       // NEW
    HostedTools []HostedToolDef    // NEW — reserved for v1.5; v1 emits empty
    SDKsDetected []SDK             // derived from inventory contents
}

type AgentDef struct {
    SDK            SDK                // openai_agents | claude_agent_sdk | ...
    Class          string             // "Agent" | "SandboxAgent" | "AgentDefinition"
    FilePath       string
    Line, EndLine  int
    Name           string             // from name= kwarg if literal
    Kwargs         KwargTree          // nested; supports dotted-path lookup
    ToolRefs       []ToolRef
    HandoffRefs    []AgentRef
    InputGuards    []GuardrailRef
    OutputGuards   []GuardrailRef
    Opaque         bool               // true if Agent(**config) or tools=get_tools()
}

type KwargTree struct {
    // Either a leaf value or a nested tree.
    // Leaf: Value populated.
    // Tree: Children populated (e.g. model_settings.tool_choice).
    Value    Expr
    Children map[string]*KwargTree
}

type ToolRef struct {
    Name     string  // symbol name in source
    Resolved *ToolDef // nil if External
    External bool    // true when symbol can't be resolved
}

// AgentRef and GuardrailRef have the same shape, pointing at AgentDef / GuardrailDef.

type GuardrailDef struct {
    Name     string
    Kind     GuardrailKind  // input | output
    FilePath string
    Line     int
}

type SessionUse struct {
    Class    string  // "SQLiteSession" | "EncryptedSession" | ...
    FilePath string
    Line     int
}

type HostedToolDef struct {
    Class    string  // "WebSearchTool" | "ComputerTool" | ...
    FilePath string
    Line     int
    Kwargs   KwargTree
}
```

`Expr` is a small typed wrapper around tree-sitter nodes so predicates can ask structured questions without re-parsing — kinds include `literal_string`, `literal_int`, `literal_bool`, `name_ref`, `list_of_refs`, `unknown`.

### 5.2 Three Detector interfaces (in `internal/analysis/detectors/`)

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

type Registry struct {
    tool  []ToolDetector
    agent []AgentDetector
    repo  []RepoDetector
}

func (r *Registry) Run(profile models.RepoProfile, inv models.RepoInventory,
                       parsed []analysis.ParsedFile) []models.Finding
```

`Singleton()` is removed. `Category()` is preserved for `--detectors` filtering. The Registry's run loop dispatches tool detectors over `inv.Tools`, agent detectors over `inv.Agents`, and repo detectors once.

### 5.3 Discovery additions (in `internal/analysis/discovery.go`)

The Python AST pass gains three new walks:

- **`DiscoverAgents(files []ParsedFile) []AgentDef`** — walks `Agent(`, `SandboxAgent(`, `AgentDefinition(` constructor calls. Captures kwargs into the `KwargTree`. SDK is inferred from the imported module (`from agents import Agent` → `openai_agents`; `from claude_agent_sdk import AgentDefinition` → `claude_agent_sdk`).
- **`DiscoverGuardrails(files []ParsedFile) []GuardrailDef`** — `@input_guardrail` / `@output_guardrail` decorated functions. (Class-based guardrails are a known v1 limitation; see §7.)
- **`DiscoverSessions(files []ParsedFile) []SessionUse`** — constructor calls for `*Session` classes from the agents module.

The existing tool discovery is extended to capture decorator kwargs into `ToolDef.Config map[string]string` so `tool_decorator_kwarg_value` and `tool_decorator_kwarg_present` predicates can read them as fields.

**Edge resolution** (`ResolveEdges`) is the meatiest new piece. For each `AgentDef`:

1. `Agent.tools=[a, b, ...]` — resolve each element to a `ToolDef`:
   - In-file symbol lookup first (function or assignment in the same module).
   - Cross-module same-repo lookup second (walk imports, locate the imported module's `ParsedFile`, find the symbol).
   - Cross-repo or non-symbol references (e.g. `tools=get_tools()`, `tools=MY_TOOLS[1:]`) — flagged `External=true`.
   - If the entire `tools=` expression is non-list (e.g. `tools=get_tools()`), the `AgentDef.Opaque` flag is set and ToolRefs is empty.
2. `Agent.handoffs=[...]` — resolve to other `AgentDef`s with the same approach.
3. `Agent.input_guardrails=[...]` / `output_guardrails=[...]` — resolve to `GuardrailDef`s.

`Agent(**config)` unpacking sets `AgentDef.Opaque = true` and leaves Kwargs empty. META-003 fires on `Opaque=true` agents.

Edge resolution sorts candidate tools by `(file_path, line)` before picking — keeps the result deterministic when symbol names collide across modules.

### 5.4 Predicate vocab additions

**Agent predicates** (read off `AgentDef`).

Note: `applies_to` uses semantic kind names (`openai_agent`, `openai_sandbox_agent`, `claude_agent_definition`) — they're aliases for the SDK + class combination. The `agent_class:` predicate matches against the literal Python class name (`Agent`, `SandboxAgent`, `AgentDefinition`) for finer control inside a rule's match expression. The two namespaces are deliberately separate: `applies_to` is the rule-eligibility filter; `agent_class` is a match predicate within the rule body.

| Predicate                            | Reads                                      |
| ------------------------------------ | ------------------------------------------ |
| `agent_class: [class, ...]`          | `AgentDef.Class`                           |
| `agent_kwarg_present: [name]`        | `AgentDef.Kwargs` (dotted-path)            |
| `agent_kwarg_missing: [name]`        | `AgentDef.Kwargs` (dotted-path, negated)   |
| `agent_kwarg_value: {kwarg, value}`  | `AgentDef.Kwargs[kwarg]` value (dotted)    |
| `agent_kwarg_list_empty: [name]`     | `AgentDef.Kwargs[name]` is `[]` or absent  |
| `agent_uses_tool_kind: [kind]`       | `AgentDef.ToolRefs[*].Resolved.Kind`       |
| `agent_uses_hosted_tool: [class]`    | (reserved; for v1.5 hosted tools)          |
| `agent_handoff_to_class: [class]`    | `AgentDef.HandoffRefs[*].Resolved.Class`   |

**Repo predicates** (read off `RepoProfile` + `RepoInventory`):

| Predicate                              | Reads                                                |
| -------------------------------------- | ---------------------------------------------------- |
| `repo_has_sdk_dep: [name]`             | `RepoProfile.SDKDeps[*].Name`                        |
| `repo_has_sdk_in_code: [sdk]`          | `RepoInventory.SDKsDetected`                         |
| `repo_has_agent_class: [class]`        | `RepoInventory.Agents[*].Class`                      |
| `repo_has_no_agent_class: [class]`     | (negated) `RepoInventory.Agents[*].Class`            |
| `repo_component_present: [kind]`       | `RepoProfile.Manifest.Components[*].Kind`            |
| `repo_uses_default_tracing`            | derived from inventory (no `add_trace_processor`)    |

**Tool predicates** (new):

| Predicate                                  | Reads                                |
| ------------------------------------------ | ------------------------------------ |
| `tool_decorator_kwarg_value: {kwarg, value}` | `ToolDef.Config[kwarg]`             |
| `tool_decorator_kwarg_present: [name]`     | `ToolDef.Config` keys                |

Plus dotted-path support in `agent_kwarg_value` (e.g. `model_settings.tool_choice`).

Combinators (`all`, `any`, `not`) carry over unchanged.

### 5.5 Loader and policy selection (in `internal/rules/loader.go`)

```go
func LoadFor(fsys fs.FS, sdks []models.SDK) (*detectors.Registry, error)
```

Walks `policies/<sdk>/` for each SDK in the list. Builds three slices (tool / agent / repo detectors based on each rule's `scope:` field) and hands them to `detectors.New(tool, agent, repo)`. The existing `LoadRegistry(fsys)` becomes the "load everything" path used in tests.

Phase 2b ALSO emits engine-level findings:

- **META-001** — for each SDK in `inventory.SDKsDetected` without a shipped policy pack. Info severity. Message: "This repo uses SDK X, which trustabl does not currently audit."
- **META-002** — for each SDK in `profile.SDKDeps` not present in `inventory.SDKsDetected`. Info severity. Message: "openai-agents declared as a dep but no Agent(...) found in code."
- **META-003** — for each `AgentDef` with `Opaque=true`. Info severity. Message: "Agent configuration is opaque (kwargs come from a variable via `**unpack`, or `tools=` is a non-literal expression like a function call); rules cannot evaluate against this agent."

### 5.6 Scanner restage (in `internal/scanner/scanner.go`)

```go
func Run(cfg Config) (ScanResult, error) {
    src     := ingestion.Resolve(cfg.Target);  defer src.Cleanup()
    profile := ingestion.Recon(src)                              // Phase 1
    inv, parsed := analysis.Inventory(profile, src)              // Phase 2a
    reg, metaFindings := rules.LoadFor(rules.DefaultFS(),         // Phase 2b
                                       profile, inv)
    ruleFindings := reg.Run(profile, inv, parsed)                // Phase 2c
    findings := append(metaFindings, ruleFindings...)

    readiness, overall := analysis.Score(inv.Tools, findings)
    artifacts := append(
        generation.GenerateHooks(findings),
        generation.GeneratePolicy(findings, cfg.Version)...,
    )
    return models.ScanResult{...}, nil
}
```

The new ingestion.Recon and analysis.Inventory functions are renames/wrappers around current code, plus the new agent/guardrail/session discovery.

---

## 6. Schema changes (YAML)

### 6.1 Required field — `scope:`

```yaml
scope: tool | agent | repo
```

Loader rejects rules that omit it. No default. All 12 existing rules get the field set explicitly in the same change:

| Rule                          | Scope         |
| ----------------------------- | ------------- |
| CSDK-001..007                 | `tool`        |
| OSH-001, 002, 003, 005        | `tool`        |
| OSH-004 (was `singleton: true`) | `repo`      |

`singleton:` is removed from the schema. `KnownFields(true)` rejects it from any leftover YAML (catches missed migrations).

### 6.2 `applies_to` semantics change by scope

| Scope    | `applies_to` values                                                       |
| -------- | ------------------------------------------------------------------------- |
| `tool`   | `[claude_sdk_tool, openai_tool, mcp_tool, shell_invocation, unknown]`    |
| `agent`  | `[openai_agent, openai_sandbox_agent, claude_agent_definition]`           |
| `repo`   | `[claude_sdk, openai_agents, openshell, mcp, ...]`                        |

The loader validates the values against the scope. A `scope: agent` rule with `applies_to: [claude_sdk_tool]` fails to load.

### 6.3 Required fields per rule (updated)

`id, title, scope, severity, confidence, applies_to, match, explanation, fix`. `language` and `fix_hints` stay optional.

### 6.4 Schema doc update

`internal/rules/schema.yaml` gets the `scope:` field documented, the per-scope `applies_to` value table, and the new predicates referenced in §5.4. Same four-file change pattern as today (schema.go + predicates.go + evaluator.go + schema.yaml).

---

## 7. Rule catalog (v1)

12 OpenAI-specific rules under `internal/rules/policies/openai_sdk/`. Numbering banded by scope: 001–099 tool, 100–199 agent, 200–299 repo.

### Tool scope (6) — `policies/openai_sdk/`

| Rule    | Title                                | Severity | Match summary                                                              |
| ------- | ------------------------------------ | -------- | -------------------------------------------------------------------------- |
| OAI-001 | Tool has no docstring                | low      | `has_docstring: false`                                                     |
| OAI-002 | Tool has untyped params              | medium   | `has_params: true, has_typed_params: false`                                |
| OAI-003 | Tool sets `strict_mode=False`        | medium   | `tool_decorator_kwarg_value: {kwarg: strict_mode, value: "False"}`         |
| OAI-004 | Tool has no `failure_error_function` | medium   | `not: tool_decorator_kwarg_present: [failure_error_function]`              |
| OAI-005 | Network call without timeout          | high     | `call_without_kwarg: {callees: [requests.*, httpx.*], missing: timeout}`   |
| OAI-006 | Unnormalized path in I/O              | high     | `call_uses_unnormalized_path_param`                                        |

### Agent scope (5) — `policies/openai_sdk/`

| Rule    | Title                                                | Severity | Match summary                                                                                                                |
| ------- | ---------------------------------------------------- | -------- | ---------------------------------------------------------------------------------------------------------------------------- |
| OAI-101 | No input_guardrails AND wires shell/FS tools         | high     | `all: [agent_kwarg_list_empty: [input_guardrails], agent_uses_tool_kind: [shell_invocation]]`                                |
| OAI-102 | `tool_use_behavior="stop_on_first_tool"`             | high     | `agent_kwarg_value: {kwarg: tool_use_behavior, value: "stop_on_first_tool"}`                                                  |
| OAI-103 | `tool_choice=required` + `reset_tool_choice=False`   | high     | `all: [agent_kwarg_value: {kwarg: model_settings.tool_choice, value: "required"}, agent_kwarg_value: {kwarg: reset_tool_choice, value: "False"}]` |
| OAI-104 | Raw `Agent` (not `SandboxAgent`) with FS/shell tools | medium   | `all: [agent_class: [Agent], agent_uses_tool_kind: [shell_invocation]]`                                                       |
| OAI-105 | `mcp_servers` configured AND no `input_guardrails`   | high     | `all: [agent_kwarg_present: [mcp_servers], agent_kwarg_list_empty: [input_guardrails]]`                                       |

### Repo scope (1) — `policies/openai_sdk/`

| Rule    | Title                                       | Severity | Match summary                                                                          |
| ------- | ------------------------------------------- | -------- | -------------------------------------------------------------------------------------- |
| OAI-201 | Default tracing in use                       | medium   | `all: [repo_has_sdk_in_code: [openai_agents], repo_uses_default_tracing: true]`         |

### Engine-emitted (not in YAML)

| ID         | When                                                       | Severity |
| ---------- | ---------------------------------------------------------- | -------- |
| `META-001` | SDK observed in inventory but no policy pack shipped       | info     |
| `META-002` | SDK declared as dep but no code use observed                | info     |
| `META-003` | Agent declared via `Agent(**config)` or `tools=` non-literal | info   |

### Explicitly deferred (already noted in §2)

- Hosted-tool rules (ComputerTool, WebSearchTool, etc.).
- Handoff hygiene.
- Session encryption advisory.
- Output-type unstructured + no output_guardrails heuristic.
- Idempotency parallel to CSDK-006 (predicate reusable; not framed for OpenAI yet).

---

## 8. Testing strategy

### 8.1 Predicate unit tests (extends `internal/rules/predicates_test.go`)

One fire/silent pair per new predicate:

- `PredToolDecoratorKwargValue` — `ToolDef{Config: ...}` direct.
- `PredToolDecoratorKwargPresent` — same.
- `PredRepoUsesDefaultTracing` — synthetic `RepoInventory`.
- `PredAgentClass`, `PredAgentKwargPresent`, `PredAgentKwargMissing`, `PredAgentKwargListEmpty`, `PredAgentKwargValue` (incl. dotted), `PredAgentUsesToolKind`, `PredAgentHandoffToClass` — synthetic `AgentDef`s.

Agent and repo predicates use synthetic structs (no AST round-trip) — fast, isolated, predicate semantics verified independently of discovery.

### 8.2 Per-rule fire/silent tests (extends `internal/rules/policies_test.go`)

The `policyRuleCases` table grows to cover all 12 OAI rules. Test setup dispatches on `tc.scope`:

- **Tool-scope** — existing `parsePy(t, src, kind)` path.
- **Agent-scope** — new `parseAgentPy(t, src)` helper: parse a Python snippet, extract the first `AgentDef` (exercises discovery + rule).
- **Repo-scope** — new `buildRepoFixture(t, opts)` helper: build a synthetic `RepoProfile` + `RepoInventory` directly (no AST).

`TestPolicyRules_AllRulesCovered` extends to iterate all rules across SDKs and assert each has a case. Build fails if any rule ships without coverage.

### 8.3 Edge-resolution unit tests (new `internal/analysis/discovery_test.go`)

Coverage cases:

- Tool defined and referenced in same file → resolved.
- Tool imported from another file in the corpus → resolved (cross-module same-repo).
- Tool imported from outside the corpus → flagged `External=true`.
- `tools=MY_TOOLS` where `MY_TOOLS = [a, b]` at module level → resolved through one indirection.
- `tools=get_tools()` → `Opaque=true`, ToolRefs empty.
- `Agent(**config)` → `Opaque=true`, Kwargs empty.
- Two `ToolDef`s named the same in different modules → deterministic pick by `(file_path, line)`.

### 8.4 Phase 2b META finding tests (new `internal/scanner/policy_selection_test.go`)

- META-001 fires / silent.
- META-002 fires / silent.
- META-003 fires / silent.

All built with synthetic `RepoProfile` + `RepoInventory` directly.

### 8.5 End-to-end examples sweep (`internal/scanner/scanner_test.go`)

`TestScanExamples_NoCrash` unchanged in semantics: walks every immediate subdirectory of `examples/`, runs `scanner.Run`, asserts no error and populated manifest. Regression coverage for the extended pipeline (Phase 1 → 2a → 2b → 2c).

### 8.6 Determinism regression test (new `internal/scanner/determinism_test.go`)

```go
func TestScanDeterministic(t *testing.T) {
    fixture := "../../testdata/deterministic-fixture/"
    r1, _ := scanner.Run(scanner.Config{Target: fixture})
    r2, _ := scanner.Run(scanner.Config{Target: fixture})
    for i, a1 := range r1.GeneratedArtifacts {
        a2 := r2.GeneratedArtifacts[i]
        if !bytes.Equal(a1.Content, a2.Content) {
            t.Errorf("artifact %s: not byte-equal across runs", a1.Path)
        }
    }
    if r1.ScanID != r2.ScanID {
        t.Errorf("ScanID drifted: %s vs %s", r1.ScanID, r2.ScanID)
    }
}
```

A `testdata/deterministic-fixture/` directory holds one small synthetic agent — controlled, unlike `examples/`. Reinstates the contract documented in ARCHITECTURE.md §7.

---

## 9. Risks and open questions

### 9.1 Risks

- **OpenAI Agents SDK pre-1.0.** Discovery patterns pinned to current API; decorator renames or kwarg signature changes silently degrade detection. *Mitigation:* document supported SDK version range in `internal/rules/policies/openai_sdk/README.md`.
- **Real-world Python idioms** defeating structural parsing — `Agent(**config)`, `tools=get_tools()`, `Agent(...)` inside a factory function. *Mitigation:* META-003 fires when discovery sees these patterns; the user knows we didn't audit instead of getting a silent zero-coverage clean bill.
- **Cross-module symbol resolution.** Many real repos define tools in `tools.py`, wire them in `agent.py`. Without cross-module resolution, all `agent.tools` shows `External=true`. *Mitigation:* implement the resolver; this is a dedicated step in the implementation plan, not an aside.
- **Class-based guardrails.** `@input_guardrail`-decorated functions detected; custom subclasses returning `GuardrailFunctionOutput` are not. OAI-101 / OAI-105 false-positive risk. *Mitigation:* also detect functions/methods with return type annotation `GuardrailFunctionOutput`. Imperfect; document.
- **Determinism with edge resolution.** Same-name tools across modules — sort candidates by `(file_path, line)`.
- **META-002 noise.** A repo declaring `openai-agents` for non-agent purposes. Info severity prevents blocking; user can suppress.

### 9.2 Resolved design questions (locked here)

- **`AgentDef.Kwargs` shape:** nested tree (`KwargTree`), not flat dotted map. Enables both `agent_kwarg_present: [model_settings]` and `agent_kwarg_value: {model_settings.tool_choice, ...}`.
- **META-003 in v1:** yes. Addresses the largest false-negative source.
- **Hosted-tool discovery in v1:** no, slot reserved (`HostedTools []HostedToolDef`), rules ship in v1.5.
- **META-004 (SDK version newer than supported):** defer; needs SDK introspection.

### 9.3 Orthogonal gaps (carried into the implementation plan)

These are NOT created by this design but remain open and should be addressed:

- **No CI workflow.** Adding tests adds value only if `go test ./...` runs on every PR. Recommend including a minimal `.github/workflows/test.yml` as a step.
- **Vestigial `HasClaudeSDKDependency` / `HasOpenShellArtifact` booleans.** Phase 1 replaces them with `SDKDeps []SDKDep`; the old fields should be removed in the same change.
- **`call_uses_param` predicate** documented as inferior to `call_uses_unnormalized_path_param`. No shipped rule uses it. Delete to remove the footgun before authors write new rules.

---

## 10. Acceptance criteria — "done" for this spec

The spec is implemented when:

1. `go test ./...` passes with the new test files.
2. `trustabl scan ./repo` on a repo with an `Agent(...)` declaration produces at least one OAI-101..105 finding when the agent is mis-wired (e.g., shell tool + no guardrails), attributed to the `Agent(...)` call site line.
3. `trustabl scan ./repo` on a repo with no agents and only `openai-agents` in `pyproject.toml` produces exactly one META-002 finding.
4. `trustabl scan ./repo` on a repo using a SDK not in our policy packs produces exactly one META-001 finding per such SDK.
5. `trustabl scan ./repo` on a repo with `Agent(**config)` or `Agent(tools=get_tools())` produces exactly one META-003 finding per such agent.
6. `TestPolicyRules_AllRulesCovered` passes — every shipped rule has a fire/silent case.
7. `TestScanDeterministic` passes — same input twice produces byte-identical artifacts.
8. `TestScanExamples_NoCrash` passes on the existing corpus, exercising the full Phase 1 → 2c pipeline.
9. `internal/rules/schema.yaml` documents the `scope:` field and all new predicates.
10. ARCHITECTURE.md updated to describe the new pipeline (current implementation, not principles — those are in CLAUDE.md).

---

## 11. Implementation sequencing (preview)

Full plan goes in `.superpowers/plans/2026-05-18-openai-agents-sdk.md` via the writing-plans skill. Rough sequencing:

1. **Schema + Detector interface split.** Add `scope:` field, remove `singleton:`, split `Detector` into three interfaces, migrate the 12 existing rules. Tests stay green.
2. **Two-phase pipeline restage.** Introduce `RepoProfile`, `RepoInventory`. Move existing discovery into Phase 2a. No new detection — refactor only.
3. **Agent / Guardrail / Session discovery + edge resolution.** Adds the meaty new discovery passes. Edge resolution gets its own step.
4. **Cross-module symbol resolution.** Discrete sub-step.
5. **New predicates** (`tool_decorator_kwarg_value`, `tool_decorator_kwarg_present`, agent predicates, repo predicates, dotted-path).
6. **Policy selection + META-001/002/003.** Phase 2b implementation.
7. **OpenAI rule pack** under `policies/openai_sdk/` — 12 rules + their fire/silent test cases.
8. **Determinism regression test** + `testdata/deterministic-fixture/`.
9. **Doc updates** — ARCHITECTURE.md describes the new pipeline; README.md adds OpenAI to the shipped detectors table.
10. **Orthogonal cleanup** — remove `HasClaudeSDKDependency` / `HasOpenShellArtifact` booleans, delete `call_uses_param` predicate.
11. **CI workflow** — `.github/workflows/test.yml`.

Each step is a PR-sized unit with `go test ./...` green at the boundary.

---

## 12. References

- [CLAUDE.md](../../CLAUDE.md) — three-scope detection model, two-phase pipeline, doc precedence.
- [ARCHITECTURE.md](../../ARCHITECTURE.md) — current implementation.
- [internal/rules/policies/CLAUDE.md](../../internal/rules/policies/CLAUDE.md) — rule authoring contract.
- [internal/rules/schema.yaml](../../internal/rules/schema.yaml) — schema reference.
- OpenAI Agents SDK Python docs: <https://openai.github.io/openai-agents-python/>
