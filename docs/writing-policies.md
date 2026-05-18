# Writing detection policies

Reference guide for adding rules to `internal/rules/policies/`.

Read [`internal/rules/policies/README.md`](../internal/rules/policies/README.md) for layout and loader behavior.
Read [`internal/rules/schema.yaml`](../internal/rules/schema.yaml) for the authoritative field list.

---

## Policy file template

```yaml
policy:
  id: <unique_policy_id>               # snake_case, unique across all policy files
  name: <Human-readable name>
  category: <category>                 # claude_sdk | openshell | openai_sdk | mcp | catalog
  description: >
    One or two sentences covering what surface this policy protects and why.

rules:
  - id: <PREFIX-NNN>                   # e.g. CSDK-001, OSH-003, OAIS-007
    title: <Short one-line title>
    severity: <severity>               # critical | high | medium | low
    confidence: <0.0–1.0>
    language: python                   # python | typescript | javascript | go
    applies_to:
      - claude_sdk_tool                # pick the kinds this rule targets
    singleton: false                   # true = fire at most once per scan
    match:
      <predicate>: <value>
    explanation: >
      Name the consequence, not the pattern. One paragraph, plain language.
    fix: >
      Prescribe the concrete change. One paragraph, no bullets.
    fix_hints:                         # optional — drives code generator
      <key>: <value>
```

---

## ID prefix by category

| Category   | Prefix  | Typical `applies_to`                                         |
|------------|---------|--------------------------------------------------------------|
| claude_sdk | CSDK-   | `claude_sdk_tool`                                            |
| openshell  | OSH-    | `claude_sdk_tool`, `mcp_tool`, `shell_invocation`            |
| openai_sdk | OAIS-   | `openai_tool`                                                |
| mcp        | MCP-    | `mcp_tool`                                                   |
| catalog    | CATL-   | `claude_sdk_tool`, `openai_tool`, `mcp_tool`, `google_adk_tool` |

All `applies_to` values: `claude_sdk_tool` · `openai_tool` · `mcp_tool` · `google_adk_tool` · `shell_invocation`

---

## Match predicates

### Bool predicates

Pointer fields — `false` is meaningful and tested against.

```yaml
has_docstring: false          # tool has no docstring/description
has_params: true              # tool declares at least one parameter
has_typed_params: false       # parameters lack type annotations
has_raise: true               # body contains a raise statement
has_try_except: true          # body contains try/except
has_shell_call: true          # body calls subprocess.* / os.system / os.popen
has_write_call: true          # body calls open(..., "w"/"a"/"x") or shutil write ops
has_dynamic_url_call: true    # body passes a non-literal URL to an HTTP function
always: true                  # unconditionally matches (singleton sentinel rules)
```

### String-list predicates

```yaml
name_in:                      # exact, case-insensitive match against tool.Name
  - process
  - handle

name_has_prefix:              # tool name starts with any prefix (case-insensitive)
  - create_
  - update_

has_body_text:                # function body text contains ANY of these strings
  - ALLOWED_COMMANDS          # ⚠ comments inside the function count — see gotcha below
  - allowlist

capability_class_in:          # catalog-assigned capability class matches any value
  - code_execution
  - shell_execution
  - file_read
  - file_write
  - network_read
  - network_write
  - agent_spawn
  - memory_write
  - data_query
  - data_mutate
  - auth_action
  - computer_use
  - external_api
```

### Nested struct predicates

```yaml
param_name_matches:
  exact:    [password, token]
  contains: [key, secret]
  suffixes: [_id, _path]
  prefixes: [file_, user_]

call_without_kwarg:
  callees:        [requests.get, requests.post]
  callee_prefixes: [requests.]
  missing:        timeout          # fires when this kwarg is absent

call_with_kwarg_value:
  callee_prefix: subprocess.       # OR use callees: [subprocess.run]
  kwarg:  shell
  value:  "True"

call_uses_unnormalized_path_param:
  callees:        [open, Path]
  callee_prefixes: [shutil., os.]
  # fires when a path-like param flows to these callees without .resolve()
```

### Combinators

```yaml
all:                           # AND — all sub-expressions must match
  - has_shell_call: true
  - not:
      has_body_text:
        - ALLOWED_COMMANDS

any:                           # OR — at least one sub-expression matches
  - name_has_prefix: [create_]
  - name_has_prefix: [update_]

not:                           # negates one sub-expression
  has_body_text:
    - sandbox
```

---

## Severity and confidence

| Severity | When to use                                                        |
|----------|--------------------------------------------------------------------|
| critical | Direct exploit path: shell injection, code exec, arbitrary write   |
| high     | Likely exploitable under adversarial prompt or at production scale |
| medium   | Reliability gap; not a security issue but causes observable failures |
| low      | Quality / clarity issue; degrades tool selection accuracy          |
| info     | Reserved — do not ship rules at this level                         |

| Confidence | When to use                                          |
|------------|------------------------------------------------------|
| 0.95–0.99  | Structural check, near-zero false positives          |
| 0.80–0.94  | Strong heuristic, rare false positives               |
| 0.65–0.79  | Heuristic, some false positives expected             |
| 0.50–0.64  | Weak signal — reconsider before shipping             |

---

## Common `fix_hints` keys

```yaml
fix_hints:
  add_docstring: true          # generator adds a docstring stub
  add_input_schema: true       # generator adds Pydantic input model
  policy_action: deny_shell_true
  policy_emit: command_allowlist
  hook: pretooluse_validate
  guard: path_under_root
  guard: subagent_tool_scope
  guard: require_human_approval
  guard: dry_run_check
  guard: rate_limit_check
  guard: memory_scope_limit
  guard: redact_credentials
```

---

## ⚠ `has_body_text` gotcha

`PredHasBodyText` scans the full text of the function node — **including inline comments**.

If you describe absent guard keywords in a comment, the predicate will find them and the rule will not fire.

```python
# Bad — "sandbox" is in the comment, has_body_text fires, not: flips false
def run_code(code: str) -> str:
    # CATL-001: code_execution, no sandbox guard
    exec(code)

# Good — no guard keywords in the comment
def run_code(code: str) -> str:
    # CATL-001: code_execution, unguarded
    exec(code)
```

---

## Checklist after writing a rule

1. Add a triggering function to [`examples/sample_agent/tools.py`](../examples/sample_agent/tools.py).
   Comments must not contain the guard keywords the rule checks for (see gotcha above).
2. Add the rule ID to `expectedRules` in [`internal/scanner/scanner_test.go`](../internal/scanner/scanner_test.go).
3. Run `go test ./...` (requires `CGO_ENABLED=1` and GCC in `PATH`).

---

## Complete example

```yaml
policy:
  id: openai_sdk_tool_definition
  name: OpenAI Agents SDK tool definition hygiene
  category: openai_sdk
  description: >
    Rules that govern how an OpenAI Agents SDK @function_tool presents itself
    to the model. Violations degrade the model's ability to select the right
    tool and to validate its own outputs before invoking it.

rules:
  - id: OAIS-001
    title: OpenAI tool has no description
    severity: low
    confidence: 0.95
    language: python
    applies_to:
      - openai_tool
    singleton: false
    match:
      has_docstring: false
    explanation: >
      The OpenAI Agents SDK surfaces the @function_tool's docstring as the
      tool description shown to the model. With no description the model must
      guess from the function name when to call this tool — which causes
      mis-selection under ambiguous prompts.
    fix: >
      Add a one-paragraph docstring describing inputs, outputs, and when to
      use this tool.
    fix_hints:
      add_docstring: true

  - id: OAIS-002
    title: OpenAI tool parameters are not type-annotated
    severity: medium
    confidence: 0.9
    language: python
    applies_to:
      - openai_tool
    singleton: false
    match:
      all:
        - has_params: true
        - has_typed_params: false
    explanation: >
      Without parameter type annotations the SDK cannot generate a JSON schema
      for the tool, so the model cannot validate its own output before
      invoking it. Hallucinated parameter shapes surface as TypeErrors at
      runtime instead of clean validation errors pre-invocation.
    fix: >
      Annotate every parameter with a type. Use Pydantic models for nested
      or complex arguments.
    fix_hints:
      add_input_schema: true

  - id: OAIS-007
    title: Ambiguous OpenAI tool name
    severity: low
    confidence: 0.9
    language: python
    applies_to:
      - openai_tool
    singleton: false
    match:
      name_in:
        - process
        - handle
        - run
        - do
        - execute
        - perform
        - work
        - go
        - thing
        - stuff
    explanation: >
      Tool names like `process`, `handle`, or `run` give the model no signal
      about intent. The model will either call this tool for the wrong job or
      refuse to call it at all when a better-named alternative is present.
    fix: >
      Rename to a verb-object form, e.g. `summarize_invoice`, `fetch_weather`.
```
