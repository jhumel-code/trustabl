# Policy Scoring — Formula Reference

karenctl assigns every finding a **Base Score** on a 0–100 scale. This score
drives finding prioritization in scan output and the per-tool readiness
percentage.

---

## Formula

```
Base Score = round(Severity_Weight × Confidence, 1)
```

### Severity weights

| Severity | Weight |
|----------|--------|
| Critical | 100.0  |
| High     |  75.0  |
| Medium   |  50.0  |
| Low      |  25.0  |

### Score bands

| Range     | Band     | Color  | Meaning |
|-----------|----------|--------|---------|
| 90–100    | Critical | Red    | Block deployment. Unconditional exploit path or irreversible real-world action with no guard. |
| 70–89.9   | High     | Red    | Fix before shipping. Exploitable with one precondition (e.g., prompt injection succeeds). |
| 40–69.9   | Medium   | Amber  | Fix in next sprint. Reliability or availability impact; not directly exploitable. |
| 10–39.9   | Low      | Yellow | Fix when convenient. Quality or routing degradation; no security or reliability consequence. |

### Why flat multiplication

The formula is intentionally simple:

- **No branching.** No CVSS-style attack vector / scope / privilege modifiers.
  Those modifiers require human judgment to fill in correctly and introduce
  scoring inconsistency across reviewers.
- **Confidence as a discount.** A Critical rule at 0.75 confidence is not as
  actionable as a Critical rule at 0.99. Multiplying by confidence encodes
  this directly: `100.0 × 0.75 = 75.0` falls into the High band, which
  correctly signals "review before acting."
- **Reproducible.** Same YAML → same score every time. No environmental
  adjustments, no temporal scoring. Users can diff scores across versions
  to detect rule changes.

### What the score does not encode

- **Exploitability chain.** A Critical finding may require prompt injection
  to land — two preconditions, not one. The score reflects the severity of
  the consequence if exploitation succeeds, discounted by detection
  confidence, not by exploit probability.
- **Business context.** A payment API tool and a logging tool with the same
  rule firing carry different real-world risk. Score normalization for
  business context is out of scope — use severity labels in triage instead.
- **Fix effort.** Low-score findings are not necessarily easier to fix than
  high-score ones.

---

## Per-tool readiness percentage

The scan output shows each tool's readiness as a percentage — separate from
the base score. The readiness percentage measures how many weighted rules
pass; the base score measures the severity of the worst failure.

```
readiness = 1 - (sum of firing weighted severity / saturation)
```

A tool with no findings scores 100%. Saturation is the weighted-severity value
at which the score bottoms out at 0 (currently 3.0 — calibrate against a real
corpus before changing).

The **Readiness score** shown in the scan summary is the base score of the single
worst finding in the entire scan. The base score shown per tool is the worst
finding for that tool.

---

<!-- score-table:begin -->
## All rules scored

### Claude Agent SDK (CSDK)

| ID | Title | Sev | Conf | Score |
|----|-------|-----|------|-------|
| CSDK-001 | Tool has no description                            | Low      | 0.95 |  14.3 |
| CSDK-002 | Tool parameters are not type-annotated             | Medium   | 0.90 |  36.0 |
| CSDK-003 | Network call has no timeout                        | High     | 0.85 |  59.5 |
| CSDK-004 | Path parameter used in I/O without validation      | High     | 0.70 |  49.0 |
| CSDK-005 | Tool raises exceptions without a structured error contract | Medium   | 0.60 |  24.0 |
| CSDK-006 | Mutating tool has no idempotency key               | Medium   | 0.55 |  22.0 |
| CSDK-007 | Ambiguous tool name                                | Low      | 0.90 |  13.5 |
| CSDK-101 | Claude subagent is granted the Bash tool           | High     | 0.80 |  56.0 |

### OpenShell (OSH)

| ID | Title | Sev | Conf | Score |
|----|-------|-----|------|-------|
| OSH-001  | subprocess called with shell=True                  | Critical | 0.99 |  99.0 |
| OSH-002  | Shell invocation without an allowed-command list   | High     | 0.85 |  59.5 |
| OSH-003  | Filesystem write without sandbox restriction       | High     | 0.80 |  56.0 |
| OSH-004  | No OpenShell resource limits configured            | Medium   | 0.95 |  38.0 |
| OSH-005  | Network egress is unrestricted                     | High     | 0.70 |  49.0 |

### OpenAI Agents SDK (OAIS)

| ID | Title | Sev | Conf | Score |
|----|-------|-----|------|-------|
| OAI-003  | Tool sets strict_mode=False                        | Medium   | 0.95 |  38.0 |
| OAI-004  | Tool has no failure_error_function                 | Medium   | 0.70 |  28.0 |
| OAI-005  | Network call has no timeout                        | High     | 0.85 |  59.5 |
| OAI-006  | Tool accepts path without normalization            | High     | 0.70 |  49.0 |
| OAI-101  | Agent has no input_guardrails AND wires shell or filesystem-touching tools | High     | 0.85 |  59.5 |
| OAI-102  | Agent uses tool_use_behavior="stop_on_first_tool"  | High     | 0.95 |  66.5 |
| OAI-103  | tool_choice="required" combined with reset_tool_choice=False | High     | 0.95 |  66.5 |
| OAI-104  | Raw Agent (not SandboxAgent) wires shell or filesystem-touching tools | Medium   | 0.75 |  30.0 |
| OAI-105  | Agent has mcp_servers configured AND no input_guardrails | High     | 0.85 |  59.5 |
| OAI-201  | Project uses default OpenAI tracing                | Medium   | 0.80 |  32.0 |
| OAIS-001 | OpenAI tool has no description                     | Low      | 0.95 |  14.3 |
| OAIS-002 | OpenAI tool parameters are not type-annotated      | Medium   | 0.90 |  36.0 |
| OAIS-005 | OpenAI tool raises exceptions without a structured error contract | Medium   | 0.60 |  24.0 |
| OAIS-006 | Mutating OpenAI tool has no idempotency key        | Medium   | 0.55 |  22.0 |
| OAIS-007 | Ambiguous OpenAI tool name                         | Low      | 0.90 |  13.5 |

### MCP (MCP)

| ID | Title | Sev | Conf | Score |
|----|-------|-----|------|-------|
| MCP-001  | MCP tool accepts injection-prone parameter names   | High     | 0.75 |  52.5 |
| MCP-002  | MCP tool contains eval or exec call                | Critical | 0.90 |  90.0 |
| MCP-003  | MCP tool deserializes data with pickle or marshal  | Critical | 0.95 |  95.0 |
| MCP-004  | MCP tool has no description                        | Low      | 0.95 |  14.3 |

### Catalog capability-class (CATL)

| ID | Title | Sev | Conf | Score |
|----|-------|-----|------|-------|
| CATL-001 | Code execution tool has no sandbox guard           | Critical | 0.80 |  80.0 |
| CATL-002 | Shell execution tool has no command allowlist      | Critical | 0.80 |  80.0 |
| CATL-003 | File write tool has no path validation             | High     | 0.75 |  52.5 |
| CATL-004 | Agent spawn tool has no privilege scoping guard    | High     | 0.70 |  49.0 |
| CATL-005 | Auth tool has no secure credential handling        | High     | 0.70 |  49.0 |
| CATL-006 | Computer use tool has no confirmation gate         | Critical | 0.75 |  75.0 |
| CATL-007 | Data mutation tool has no dry-run or rollback support | Medium   | 0.65 |  26.0 |
| CATL-008 | External API tool has no rate limit guard          | Medium   | 0.60 |  24.0 |
| CATL-009 | Memory write tool has no size or scope limit       | Low      | 0.60 |   9.0 |

---

## Score distribution

```
90–100   █████████  OSH-001 (99.0), MCP-002 (90.0), MCP-003 (95.0)
70– 89   █████████  CATL-001 (80.0), CATL-002 (80.0), CATL-006 (75.0)
40– 69   ████████████████████████████████████████████████
           CSDK-003 (59.5), CSDK-004 (49.0), CSDK-101 (56.0)
           OSH-002 (59.5), OSH-003 (56.0), OSH-005 (49.0)
           OAI-005 (59.5), OAI-006 (49.0), OAI-101 (59.5)
           OAI-102 (66.5), OAI-103 (66.5), OAI-105 (59.5)
           MCP-001 (52.5), CATL-003 (52.5), CATL-004 (49.0)
           CATL-005 (49.0)
10– 39   ████████████████████████████████████████████████
           CSDK-001 (14.3), CSDK-002 (36.0), CSDK-005 (24.0)
           CSDK-006 (22.0), CSDK-007 (13.5), OSH-004 (38.0)
           OAI-003 (38.0), OAI-004 (28.0), OAI-104 (30.0)
           OAI-201 (32.0), OAIS-001 (14.3), OAIS-002 (36.0)
           OAIS-005 (24.0), OAIS-006 (22.0), OAIS-007 (13.5)
           MCP-004 (14.3), CATL-007 (26.0), CATL-008 (24.0)
```
<!-- score-table:end -->

---

## Adding a score to a new rule

When writing a new rule, derive the score directly from the YAML fields:

```
severity: critical  →  weight 100.0
confidence: 0.82    →  multiply

Score = round(100.0 × 0.82, 1) = 82.0  →  High band (70–89.9)
```

No additional fields required. Score is always derived — never stored in the YAML.

---

## Changing a rule's severity or confidence

Changing either field in the YAML automatically changes the score on the next
scan. No secondary update required. The scoring doc table above must be kept
in sync manually — update the row when the YAML changes.

Bump `severity` up if: a new exploit chain is documented that lowers the
precondition count. Bump `confidence` up if: detection precision improves
(e.g., AST analysis replaces body text search).
