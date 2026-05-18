#!/usr/bin/env python3
"""
gather_tools.py — GitHub tool name harvester for karenctl catalog curation.

Searches public GitHub repos that use major agent SDKs, extracts decorated
tool function names and their docstrings, and writes a raw YAML file for
human curation into tools_catalog.yaml.

Usage:
    GITHUB_TOKEN=ghp_... python tools/gather_tools.py [--output tools_raw.yaml]

Requirements:
    pip install requests pyyaml

The output is a raw YAML file (not a policy file) that you review and merge
into internal/catalog/tools_catalog.yaml after curating.
"""

import argparse
import ast
import datetime
import os
import re
import sys
import textwrap
from dataclasses import dataclass, field
from typing import Optional

import requests
import yaml


# ─── Configuration ─────────────────────────────────────────────────────────

# GitHub code search queries, one per SDK.
# Each tuple: (framework_label, search_query)
SEARCH_QUERIES = [
    (
        "anthropic_claude",
        'language:python "@tool" "claude_agent_sdk" OR "anthropic_agents"',
    ),
    (
        "openai_agents",
        'language:python "@function_tool" "openai.agents" OR "openai-agents"',
    ),
    (
        "langchain",
        'language:python "@tool" "from langchain.tools" OR "from langchain_core.tools"',
    ),
    (
        "crewai",
        'language:python "@tool" "from crewai" OR "crewai.tools"',
    ),
    (
        "autogen",
        'language:python "FunctionTool" "from autogen" OR "pyautogen"',
    ),
    (
        "pydantic_ai",
        'language:python "@agent.tool" "pydantic_ai"',
    ),
    (
        "google_adk",
        'language:python "@adk.tool" OR "@genai.tool" "google.adk" OR "google.generativeai"',
    ),
    (
        "mcp",
        'language:python "@server.tool" OR ".register_tool" "mcp" OR "modelcontextprotocol"',
    ),
]

# Decorators that mark tool functions, per framework.
DECORATOR_PATTERNS = {
    "anthropic_claude": [r"@tool\b", r"@claude_tool\b", r"@agent\.tool\b"],
    "openai_agents": [r"@function_tool\b"],
    "langchain": [r"@tool\b"],
    "crewai": [r"@tool\b"],
    "autogen": [r"@tool\b"],
    "pydantic_ai": [r"@agent\.tool\b"],
    "google_adk": [r"@adk\.tool\b", r"@genai\.tool\b"],
    "mcp": [r"@server\.tool\b", r"@mcp\.tool\b"],
}

MAX_RESULTS_PER_QUERY = 30  # GitHub code search caps at 100; be conservative


# ─── Data model ────────────────────────────────────────────────────────────

@dataclass
class RawEntry:
    name: str
    decorator: str
    framework: str
    repo: str
    file_path: str
    docstring_snippet: str = ""


# ─── GitHub API helpers ─────────────────────────────────────────────────────

def gh_headers(token: str) -> dict:
    return {
        "Authorization": f"Bearer {token}",
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
    }


def search_code(token: str, query: str, per_page: int = 30) -> list[dict]:
    url = "https://api.github.com/search/code"
    params = {"q": query, "per_page": per_page}
    resp = requests.get(url, headers=gh_headers(token), params=params, timeout=30)
    if resp.status_code == 403:
        print(f"  [rate-limited] {resp.headers.get('X-RateLimit-Reset', '?')}", file=sys.stderr)
        return []
    resp.raise_for_status()
    return resp.json().get("items", [])


def fetch_file(token: str, repo_full_name: str, file_path: str) -> Optional[str]:
    url = f"https://api.github.com/repos/{repo_full_name}/contents/{file_path}"
    resp = requests.get(url, headers=gh_headers(token), timeout=30)
    if resp.status_code != 200:
        return None
    import base64
    data = resp.json()
    if data.get("encoding") == "base64":
        return base64.b64decode(data["content"]).decode("utf-8", errors="replace")
    return None


# ─── Extraction ────────────────────────────────────────────────────────────

def extract_tools(source: str, framework: str) -> list[tuple[str, str, str]]:
    """Return list of (function_name, decorator_text, docstring_snippet) tuples."""
    patterns = DECORATOR_PATTERNS.get(framework, [r"@tool\b"])
    results = []
    try:
        tree = ast.parse(source)
    except SyntaxError:
        return results

    for node in ast.walk(tree):
        if not isinstance(node, ast.FunctionDef):
            continue
        # Check decorators
        for dec in node.decorator_list:
            dec_src = ast.unparse(dec) if hasattr(ast, "unparse") else ""
            matched = any(re.search(p, "@" + dec_src) for p in patterns)
            if not matched:
                continue
            # Extract docstring
            doc = ""
            if (
                node.body
                and isinstance(node.body[0], ast.Expr)
                and isinstance(node.body[0].value, ast.Constant)
            ):
                raw = node.body[0].value.value or ""
                doc = textwrap.shorten(str(raw).strip(), width=120, placeholder="…")
            results.append((node.name, "@" + dec_src, doc))
            break  # one decorator match per function is enough

    return results


# ─── Main ──────────────────────────────────────────────────────────────────

def run(token: str, output_path: str) -> None:
    entries: list[RawEntry] = []
    seen: set[str] = set()  # dedup by (framework, name)

    for framework, query in SEARCH_QUERIES:
        print(f"Searching: {framework} …", file=sys.stderr)
        items = search_code(token, query, per_page=MAX_RESULTS_PER_QUERY)
        print(f"  {len(items)} code results", file=sys.stderr)

        for item in items:
            repo = item["repository"]["full_name"]
            file_path = item["path"]
            source = fetch_file(token, repo, file_path)
            if not source:
                continue
            for name, dec, doc in extract_tools(source, framework):
                key = f"{framework}:{name}"
                if key in seen:
                    continue
                seen.add(key)
                entries.append(
                    RawEntry(
                        name=name,
                        decorator=dec,
                        framework=framework,
                        repo=repo,
                        file_path=file_path,
                        docstring_snippet=doc,
                    )
                )

    # Sort for stable output.
    entries.sort(key=lambda e: (e.framework, e.name))

    doc = {
        "version": "raw-1",
        "scraped_at": datetime.date.today().isoformat(),
        "note": (
            "Raw output from gather_tools.py. Review and merge into "
            "internal/catalog/tools_catalog.yaml after curation."
        ),
        "entries": [
            {
                "name": e.name,
                "decorator": e.decorator,
                "framework": e.framework,
                "repo": e.repo,
                "file": e.file_path,
                "docstring_snippet": e.docstring_snippet,
            }
            for e in entries
        ],
    }

    with open(output_path, "w", encoding="utf-8") as f:
        yaml.dump(doc, f, allow_unicode=True, sort_keys=False)

    print(f"\nWrote {len(entries)} entries to {output_path}", file=sys.stderr)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--output",
        default="tools_raw.yaml",
        help="Output path for the raw YAML (default: tools_raw.yaml)",
    )
    args = parser.parse_args()

    token = os.environ.get("GITHUB_TOKEN", "")
    if not token:
        print(
            "Error: GITHUB_TOKEN environment variable is required.\n"
            "Create a token at https://github.com/settings/tokens (no scopes needed "
            "for public repo search).",
            file=sys.stderr,
        )
        sys.exit(1)

    run(token, args.output)


if __name__ == "__main__":
    main()
