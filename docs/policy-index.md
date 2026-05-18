# Policy index

All detection rules shipped in `internal/rules/policies/`, grouped by category.
Each rule links to its source file. Severity abbreviations: **C** = critical, **H** = high, **M** = medium, **L** = low.

Scores are Base Scores (0–10) derived from `Severity_Weight × Confidence`. See [scoring.md](scoring.md) for the full formula and rationale.

---

## Claude Agent SDK (`claude_sdk`) — CSDK-NNN

Rules targeting `@tool`-decorated Python functions in the Claude Agent SDK.

| ID | Title | Sev | Conf | Score | File |
|----|-------|-----|------|-------|------|
| CSDK-001 | Tool has no description | L | 0.95 | 23.8 | [tool_definition.yaml](../internal/rules/policies/claude_sdk/tool_definition.yaml) |
| CSDK-002 | Tool parameters are not type-annotated | M | 0.90 | 45.0 | [tool_definition.yaml](../internal/rules/policies/claude_sdk/tool_definition.yaml) |
| CSDK-003 | Network call has no timeout | H | 0.85 | 63.8 | [network.yaml](../internal/rules/policies/claude_sdk/network.yaml) |
| CSDK-004 | Path parameter used in I/O without validation | H | 0.70 | 52.5 | [path_safety.yaml](../internal/rules/policies/claude_sdk/path_safety.yaml) |
| CSDK-005 | Tool raises exceptions without a structured error contract | M | 0.60 | 30.0 | [error_handling.yaml](../internal/rules/policies/claude_sdk/error_handling.yaml) |
| CSDK-006 | Mutating tool has no idempotency key | M | 0.55 | 27.5 | [idempotency.yaml](../internal/rules/policies/claude_sdk/idempotency.yaml) |
| CSDK-007 | Ambiguous tool name | L | 0.90 | 22.5 | [tool_definition.yaml](../internal/rules/policies/claude_sdk/tool_definition.yaml) |

### Topic files

| File | Rules | What it covers | Rationale |
|------|-------|----------------|-----------|
| `claude_sdk/tool_definition.yaml` | CSDK-001, 002, 007 | Name, description, parameter shape | [Policy/claude_sdk/tool_definition.md](Policy/claude_sdk/tool_definition.md) |
| `claude_sdk/network.yaml` | CSDK-003 | Outbound HTTP — timeout enforcement | [Policy/claude_sdk/network.md](Policy/claude_sdk/network.md) |
| `claude_sdk/path_safety.yaml` | CSDK-004 | Filesystem path traversal | [Policy/claude_sdk/path_safety.md](Policy/claude_sdk/path_safety.md) |
| `claude_sdk/error_handling.yaml` | CSDK-005 | Exception contract | [Policy/claude_sdk/error_handling.md](Policy/claude_sdk/error_handling.md) |
| `claude_sdk/idempotency.yaml` | CSDK-006 | Retry-safe side effects | [Policy/claude_sdk/idempotency.md](Policy/claude_sdk/idempotency.md) |

---

## OpenShell (`openshell`) — OSH-NNN

Rules targeting subprocess invocations, filesystem writes, and network egress.
Feeds the generated `openshell/policy.yaml` sandbox config.

| ID | Title | Sev | Conf | Score | Singleton | File |
|----|-------|-----|------|-------|-----------|------|
| OSH-001 | subprocess called with `shell=True` | C | 0.99 | 99.0 | no | [shell.yaml](../internal/rules/policies/openshell/shell.yaml) |
| OSH-002 | Shell invocation without an allowed-command list | H | 0.85 | 63.8 | no | [shell.yaml](../internal/rules/policies/openshell/shell.yaml) |
| OSH-003 | Filesystem write without sandbox restriction | H | 0.80 | 60.0 | no | [filesystem.yaml](../internal/rules/policies/openshell/filesystem.yaml) |
| OSH-004 | No OpenShell resource limits configured | M | 0.95 | 47.5 | **yes** | [resources.yaml](../internal/rules/policies/openshell/resources.yaml) |
| OSH-005 | Network egress is unrestricted | H | 0.70 | 52.5 | no | [network.yaml](../internal/rules/policies/openshell/network.yaml) |

> OSH-004 is a **singleton** — fires once per scan regardless of how many tools match.

### Topic files

| File | Rules | What it covers | Rationale |
|------|-------|----------------|-----------|
| `openshell/shell.yaml` | OSH-001, 002 | `shell=True`, missing command allowlist | [Policy/openshell/shell.md](Policy/openshell/shell.md) |
| `openshell/filesystem.yaml` | OSH-003 | Unrestricted filesystem writes | [Policy/openshell/filesystem.md](Policy/openshell/filesystem.md) |
| `openshell/resources.yaml` | OSH-004 | Missing cpu/memory/time limits | [Policy/openshell/resources.md](Policy/openshell/resources.md) |
| `openshell/network.yaml` | OSH-005 | Dynamic URL calls with no host allowlist | [Policy/openshell/network.md](Policy/openshell/network.md) |

---

## OpenAI Agents SDK (`openai_sdk`) — OAIS-NNN

Rules targeting `@function_tool`-decorated functions in the OpenAI Agents SDK.

| ID | Title | Sev | Conf | Score | File |
|----|-------|-----|------|-------|------|
| OAIS-001 | OpenAI tool has no description | L | 0.95 | 23.8 | [tool_definition.yaml](../internal/rules/policies/openai_sdk/tool_definition.yaml) |
| OAIS-002 | OpenAI tool parameters are not type-annotated | M | 0.90 | 45.0 | [tool_definition.yaml](../internal/rules/policies/openai_sdk/tool_definition.yaml) |
| OAIS-005 | OpenAI tool raises exceptions without a structured error contract | M | 0.60 | 30.0 | [error_handling.yaml](../internal/rules/policies/openai_sdk/error_handling.yaml) |
| OAIS-006 | Mutating OpenAI tool has no idempotency key | M | 0.55 | 27.5 | [idempotency.yaml](../internal/rules/policies/openai_sdk/idempotency.yaml) |
| OAIS-007 | Ambiguous OpenAI tool name | L | 0.90 | 22.5 | [tool_definition.yaml](../internal/rules/policies/openai_sdk/tool_definition.yaml) |

> OAIS-003 and OAIS-004 are not yet defined — IDs reserved.

### Topic files

| File | Rules | What it covers | Rationale |
|------|-------|----------------|-----------|
| `openai_sdk/tool_definition.yaml` | OAIS-001, 002, 007 | Name, description, parameter shape | [Policy/openai_sdk/tool_definition.md](Policy/openai_sdk/tool_definition.md) |
| `openai_sdk/error_handling.yaml` | OAIS-005 | Exception contract | [Policy/openai_sdk/error_handling.md](Policy/openai_sdk/error_handling.md) |
| `openai_sdk/idempotency.yaml` | OAIS-006 | Retry-safe side effects | [Policy/openai_sdk/idempotency.md](Policy/openai_sdk/idempotency.md) |

---

## MCP (`mcp`) — MCP-NNN

Rules targeting `@server.tool`-decorated MCP server tools. Focus on injection and unsafe deserialization — MCP tools receive model-controlled inputs from external orchestrators.

| ID | Title | Sev | Conf | Score | File |
|----|-------|-----|------|-------|------|
| MCP-001 | MCP tool accepts injection-prone parameter names | H | 0.75 | 56.3 | [injection.yaml](../internal/rules/policies/mcp/injection.yaml) |
| MCP-002 | MCP tool contains eval or exec call | C | 0.90 | 90.0 | [injection.yaml](../internal/rules/policies/mcp/injection.yaml) |
| MCP-003 | MCP tool deserializes data with pickle or marshal | C | 0.95 | 95.0 | [injection.yaml](../internal/rules/policies/mcp/injection.yaml) |

### Topic files

| File | Rules | What it covers | Rationale |
|------|-------|----------------|-----------|
| `mcp/injection.yaml` | MCP-001, 002, 003 | Param injection, eval/exec, unsafe deserialization | [Policy/mcp/injection.md](Policy/mcp/injection.md) |

---

## Catalog capability-class (`catalog`) — CATL-NNN

Cross-framework rules. Fire based on catalog-assigned capability class — apply regardless of SDK. Each rule checks that a capability class carries its required safety guard.

| ID | Title | Capability class | Sev | Conf | Score | File |
|----|-------|-----------------|-----|------|-------|------|
| CATL-001 | Code execution tool has no sandbox guard | `code_execution` | C | 0.80 | 80.0 | [capability_class.yaml](../internal/rules/policies/catalog/capability_class.yaml) |
| CATL-002 | Shell execution tool has no command allowlist | `shell_execution` | C | 0.80 | 80.0 | [capability_class.yaml](../internal/rules/policies/catalog/capability_class.yaml) |
| CATL-003 | File write tool has no path validation | `file_write` | H | 0.75 | 56.3 | [capability_class.yaml](../internal/rules/policies/catalog/capability_class.yaml) |
| CATL-004 | Agent spawn tool has no privilege scoping guard | `agent_spawn` | H | 0.70 | 52.5 | [capability_class.yaml](../internal/rules/policies/catalog/capability_class.yaml) |
| CATL-005 | Auth tool has no secure credential handling | `auth_action` | H | 0.70 | 52.5 | [capability_class.yaml](../internal/rules/policies/catalog/capability_class.yaml) |
| CATL-006 | Computer use tool has no confirmation gate | `computer_use` | C | 0.75 | 75.0 | [capability_class.yaml](../internal/rules/policies/catalog/capability_class.yaml) |
| CATL-007 | Data mutation tool has no dry-run or rollback support | `data_mutate` | M | 0.65 | 32.5 | [capability_class.yaml](../internal/rules/policies/catalog/capability_class.yaml) |
| CATL-008 | External API tool has no rate limit guard | `external_api` | M | 0.60 | 30.0 | [capability_class.yaml](../internal/rules/policies/catalog/capability_class.yaml) |
| CATL-009 | Memory write tool has no size or scope limit | `memory_write` | L | 0.60 | 15.0 | [capability_class.yaml](../internal/rules/policies/catalog/capability_class.yaml) |

### Topic files

| File | Rules | What it covers | Rationale |
|------|-------|----------------|-----------|
| `catalog/capability_class.yaml` | CATL-001 through 009 | All capability-class safety checks | [Policy/catalog/capability_class.md](Policy/catalog/capability_class.md) |

---

## Summary

| Category | Rules | IDs |
|----------|-------|-----|
| Claude Agent SDK | 7 | CSDK-001–007 |
| OpenShell | 5 | OSH-001–005 |
| OpenAI Agents SDK | 5 | OAIS-001, 002, 005, 006, 007 |
| MCP | 3 | MCP-001–003 |
| Catalog | 9 | CATL-001–009 |
| **Total** | **29** | |

---

## Severity distribution

| Severity | Count | Rules |
|----------|-------|-------|
| Critical | 6 | OSH-001, MCP-002, MCP-003, CATL-001, CATL-002, CATL-006 |
| High | 10 | CSDK-003, CSDK-004, OSH-002, OSH-003, OSH-005, MCP-001, CATL-003, CATL-004, CATL-005 |
| Medium | 9 | CSDK-002, CSDK-005, CSDK-006, OSH-004, OAIS-002, OAIS-005, OAIS-006, CATL-007, CATL-008 |
| Low | 4 | CSDK-001, CSDK-007, OAIS-001, OAIS-007, CATL-009 |
