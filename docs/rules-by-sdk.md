# Rules by SDK / Tool Type

Maps every tool type to the rules that fire against it. Generated from
`internal/rules/policies/`.

---

## Google ADK

| Tool type | Scope | Rules |
|---|---|---|
| `google_adk_tool` | tool | CATL-001, CATL-002, CATL-003, CATL-004, CATL-005, CATL-006, CATL-007, CATL-008, CATL-009, GADK-001, GADK-002, GADK-003, GADK-004, GADK-005, GADK-006, GADK-007, GADK-008, OSH-001, OSH-002, OSH-003, OSH-005 |
| `google_adk_agent` | agent | GADK-101, GADK-102 |

**Policy files:** `policies/google_adk/`, `policies/catalog/`, `policies/openshell/`

---

## OpenAI Agents SDK

| Tool type | Scope | Rules |
|---|---|---|
| `openai_tool` | tool | CATL-001, CATL-002, CATL-003, CATL-004, CATL-005, CATL-006, CATL-007, CATL-008, CATL-009, OAI-003, OAI-004, OAI-005, OAI-006, OAIS-001, OAIS-002, OAIS-005, OAIS-006, OAIS-007 |
| `openai_agent` | agent | OAI-101, OAI-102, OAI-103, OAI-104, OAI-105 |
| `openai_sandbox_agent` | agent | OAI-101, OAI-102, OAI-103, OAI-105 |
| `openai_agents` | repo | OAI-201 |

**Policy files:** `policies/openai_sdk/`, `policies/catalog/`

> Note: `openai_sandbox_agent` does not receive OAI-104 (only `openai_agent` does).
> OpenShell rules do not apply to OpenAI tool types.

---

## Claude SDK

| Tool type | Scope | Rules |
|---|---|---|
| `claude_sdk_tool` | tool | CATL-001, CATL-002, CATL-003, CATL-004, CATL-005, CATL-006, CATL-007, CATL-008, CATL-009, CSDK-001, CSDK-002, CSDK-003, CSDK-004, CSDK-005, CSDK-006, CSDK-007, OSH-001, OSH-002, OSH-003, OSH-005 |
| `claude_agent_definition` | agent | CSDK-101 |

**Policy files:** `policies/claude_sdk/`, `policies/catalog/`, `policies/openshell/`

---

## MCP (cross-SDK)

| Tool type | Scope | Rules |
|---|---|---|
| `mcp_tool` | tool | CATL-001, CATL-002, CATL-003, CATL-004, CATL-005, CATL-006, CATL-007, CATL-008, CATL-009, CSDK-003, CSDK-004, CSDK-005, CSDK-006, MCP-001, MCP-002, MCP-003, MCP-004, OSH-001, OSH-002, OSH-003, OSH-005 |

**Policy files:** `policies/mcp/`, `policies/claude_sdk/` (error_handling + idempotency + network + path_safety shared), `policies/catalog/`, `policies/openshell/`

---

## OpenShell (sandbox)

| Tool type | Scope | Rules |
|---|---|---|
| `shell_invocation` | tool | CATL-001, CATL-002, CATL-003, OSH-001, OSH-002, OSH-003, OSH-005 |
| `openshell` | repo | OSH-004, OSH-006, OSH-007 |

**Policy files:** `policies/openshell/`, `policies/catalog/` (partial)

---

## Summary count

| SDK | Tool types | Rules |
|---|---|---|
| Google ADK | `google_adk_tool`, `google_adk_agent` | 22 tool + 2 agent |
| OpenAI Agents SDK | `openai_tool`, `openai_agent`, `openai_sandbox_agent`, `openai_agents` | 19 tool + 5 agent + 1 repo |
| Claude SDK | `claude_sdk_tool`, `claude_agent_definition` | 21 tool + 1 agent |
| MCP | `mcp_tool` | 22 tool |
| OpenShell | `shell_invocation`, `openshell` | 7 tool + 3 repo |

---

## CATL rules (catalog — all SDK tools)

CATL-001 through CATL-009 apply to every tool type except `openshell` and agent types.
These are SDK-agnostic capability classification rules in `policies/catalog/capability_class.yaml`.

## OSH injection rules (openshell — most SDK tools)

OSH-001, OSH-002, OSH-003, OSH-005 apply to `claude_sdk_tool`, `google_adk_tool`,
`mcp_tool`, and `shell_invocation`. They do **not** apply to `openai_tool`.

OSH-004, OSH-006, OSH-007 are repo-scoped (`openshell`) — fire once per scan, not per tool.
