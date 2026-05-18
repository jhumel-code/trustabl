# Sample agent fixture for karenctl smoke tests.
# Every function below is deliberately broken in a specific way so that the
# corresponding rule fires on it.  Do not "fix" these functions — the test
# asserts each rule fires at least once on this file.
#
# Rule → function mapping:
#   CSDK-001  no_description
#   CSDK-002  untyped_params
#   CSDK-003  fetch_data          (also fires CSDK-004 via path_reader)
#   CSDK-004  path_reader
#   CSDK-005  parse_config
#   CSDK-006  create_record
#   CSDK-007  handle
#   OSH-001   run_shell_unsafe    (also triggers CATL-002 via execute_command)
#   OSH-002   run_shell_no_allowlist
#   OSH-003   write_to_disk       (also fires via write_file / save_memory)
#   OSH-004   singleton — fires on first applicable tool in scan
#   OSH-005   fetch_dynamic
#   OAIS-001  oais_no_description
#   OAIS-002  oais_untyped_params
#   OAIS-005  oais_raise_no_try
#   OAIS-006  send_invoice
#   OAIS-007  process
#   MCP-001   mcp_command_tool
#   MCP-002   mcp_eval_tool
#   MCP-003   mcp_pickle_tool
#   CATL-001  run_code
#   CATL-002  execute_command
#   CATL-003  write_file
#   CATL-004  spawn_agent
#   CATL-005  authenticate
#   CATL-006  click
#   CATL-007  update_database
#   CATL-008  send_email
#   CATL-009  save_memory

import subprocess
import requests


# ─── Claude Agent SDK tools ──────────────────────────────────────────────────

@tool
def no_description(x):
    # CSDK-001: no docstring
    return x


@tool
def untyped_params(x, y):
    # CSDK-002: no type annotations; CSDK-001 also fires (no docstring)
    return x + y


@tool
def fetch_data(url: str) -> str:
    """Fetch data from a URL."""
    # CSDK-003: requests.get without timeout=
    return requests.get(url).text


@tool
def path_reader(file_path: str) -> str:
    """Read contents of a file at the given path."""
    # CSDK-004: file_path flows to open() without .resolve()
    with open(file_path, "r") as f:
        return f.read()


@tool
def parse_config(config_path: str) -> dict:
    """Parse a configuration file."""
    # CSDK-005: raise without try/except
    raise NotImplementedError("not implemented yet")


@tool
def create_record(name: str) -> dict:
    """Create a new record in the database."""
    # CSDK-006: mutating verb prefix, no idempotency_key param
    return {"name": name}


@tool
def handle(data: str) -> str:
    """Process input data."""
    # CSDK-007: ambiguous tool name
    return data


# ─── Shell invocation surfaces ───────────────────────────────────────────────

def run_shell_unsafe(cmd):
    # OSH-001: subprocess with shell=True
    subprocess.run(cmd, shell=True)


def run_shell_no_allowlist(argv):
    # OSH-002: subprocess without ALLOWED_COMMANDS check
    subprocess.run(argv)


def write_to_disk(path, content):
    # OSH-003: filesystem write
    with open(path, "w") as f:
        f.write(content)


def fetch_dynamic(url):
    # OSH-005: dynamic URL in HTTP call, no ALLOWED_HOSTS
    requests.get(url)


# ─── OpenAI Agents SDK tools ─────────────────────────────────────────────────

@function_tool
def oais_no_description(x):
    # OAIS-001: no docstring
    return x


@function_tool
def oais_untyped_params(x, y):
    """Process two values."""
    # OAIS-002: no type annotations
    return x + y


@function_tool
def oais_raise_no_try(data: str) -> str:
    """Process data with a possible error."""
    # OAIS-005: raise without try/except
    raise ValueError("processing failed")


@function_tool
def send_invoice(amount: float) -> bool:
    """Send an invoice to the customer."""
    # OAIS-006: mutating verb prefix (send_), no idempotency_key param
    return True


@function_tool
def process(x: str) -> str:
    """Process the input."""
    # OAIS-007: ambiguous name
    return x


# ─── MCP server tools ────────────────────────────────────────────────────────

@server.tool
def mcp_command_tool(cmd: str) -> str:
    # MCP-001: injection-prone param name "cmd"
    return cmd


@server.tool
def mcp_eval_tool(expression: str) -> str:
    # MCP-002: eval() in body
    return str(eval(expression))


@server.tool
def mcp_pickle_tool(data: bytes) -> object:
    # MCP-003: pickle.loads in body
    import pickle
    return pickle.loads(data)


# ─── Catalog-classified tools (capability_class_in rules) ────────────────────

@tool
def run_code(code: str) -> str:
    """Run user-provided code."""
    # CATL-001: code_execution capability, unguarded
    exec(code)
    return "done"


@tool
def execute_command(command: str) -> str:
    """Execute a system shell command."""
    # CATL-002: shell_execution capability, unconstrained
    # Also triggers OSH-001 (shell=True)
    subprocess.run(command, shell=True)
    return "done"


@tool
def write_file(file_path: str, content: str) -> bool:
    """Write content to a file at the given path."""
    # CATL-003: file_write capability, file_path to open() without .resolve()
    # Also triggers OSH-003 (write call)
    with open(file_path, "w") as f:
        f.write(content)
    return True


@tool
def spawn_agent(task: str) -> str:
    """Spawn a subagent to complete the given task."""
    # CATL-004: agent_spawn capability, no privilege scoping
    return task


@tool
def authenticate(username: str, password: str) -> dict:
    """Authenticate the user and return a session token."""
    # CATL-005: auth_action capability, credentials unprotected
    return {"user": username}


@tool
def click(selector: str) -> bool:
    """Click the UI element matching the given selector."""
    # CATL-006: computer_use capability, no human gate
    return True


@tool
def update_database(query: str) -> bool:
    """Execute a database update query."""
    # CATL-007: data_mutate capability, no undo path
    return True


@tool
def send_email(to: str, subject: str, body: str) -> bool:
    """Send an email via the external mail API."""
    # CATL-008: external_api capability, unconstrained calls
    requests.post(
        "https://api.mail.example.com/send",
        json={"to": to, "subject": subject, "body": body},
    )
    return True


@tool
def save_memory(content: str) -> bool:
    """Save content to the agent's long-term memory store."""
    # CATL-009: memory_write capability, unbounded
    # Also triggers OSH-003 (write call)
    with open("memory.txt", "a") as f:
        f.write(content)
    return True
