#!/usr/bin/env python3
"""Validate local Markdown links in selected repository files."""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path, PurePosixPath

LINK = re.compile(r"(?<!!)\[[^]]*\]\(([^)]+)\)")


def target_exists(source: Path, destination: str) -> bool:
    destination = destination.strip()
    if not destination:
        return False
    destination = destination.split(maxsplit=1)[0]
    if not destination or destination.startswith(("#", "http:", "https:", "mailto:")):
        return True
    path = destination.split("#", 1)[0].split("?", 1)[0]
    if not path:
        return True
    if path.startswith("/"):
        return False
    target = (source.parent / path).resolve()
    root = next((p for p in source.resolve().parents if (p / '.git').exists()), source.parent.resolve())
    if not target.is_relative_to(root):
        return False
    return target.exists()


def check(path: Path) -> list[str]:
    # A Markdown file deleted by this change set has no links left to inspect.
    if not path.exists():
        return []
    errors: list[str] = []
    try:
        text = path.read_text(encoding="utf-8")
    except UnicodeDecodeError:
        return [f"{path}: not valid UTF-8"]
    for line_number, line in enumerate(text.splitlines(), 1):
        for match in LINK.finditer(line):
            if not target_exists(path, match.group(1)):
                errors.append(f"{path}:{line_number}: missing local link {match.group(1)!r}")
    return errors


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("files", nargs="+")
    args = parser.parse_args()
    paths = [Path(value) for value in args.files]
    errors = [error for path in paths for error in check(path)]
    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1
    print(f"checked {len(paths)} Markdown file(s)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
