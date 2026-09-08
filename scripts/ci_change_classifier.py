#!/usr/bin/env python3
"""Classify a GitHub Actions change set without third-party dependencies."""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import PurePosixPath
from typing import Sequence

ALWAYS_FULL_PREFIXES = (".github/workflows/", "scripts/")


def classify_paths(paths: Sequence[str]) -> dict[str, object]:
    """Return CI inputs for changed repository-relative paths.

    Empty, malformed, workflow, and helper-script changes always take the full
    path. Only a non-empty set of Markdown files can use the documentation path.
    """
    normalized = [path for path in paths if path]
    unsafe = any(
        path.startswith(ALWAYS_FULL_PREFIXES)
        or path.startswith("/")
        or ".." in PurePosixPath(path).parts
        for path in normalized
    )
    docs_only = bool(normalized) and not unsafe and all(path.endswith(".md") for path in normalized)
    changed_docs = [path for path in normalized if path.endswith(".md")]
    return {"docs_only": docs_only, "changed_docs": changed_docs}


def changed_paths(base: str, head: str) -> list[str]:
    result = subprocess.run(
        ["git", "diff", "--name-only", "-z", f"{base}...{head}", "--"],
        check=True,
        text=True,
        stdout=subprocess.PIPE,
    )
    return [path for path in result.stdout.split('\0') if path]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", required=True)
    parser.add_argument("--head", required=True)
    args = parser.parse_args()
    try:
        print(json.dumps(classify_paths(changed_paths(args.base, args.head))))
    except subprocess.CalledProcessError as error:
        print(f"cannot classify changes: {error}", file=sys.stderr)
        return error.returncode or 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
