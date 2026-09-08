#!/usr/bin/env python3
"""Unit tests for CI-only helper scripts."""

from __future__ import annotations

import importlib.util
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parent


def load(name: str):
    spec = importlib.util.spec_from_file_location(name, ROOT / f"{name}.py")
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


classifier = load("ci_change_classifier")
links = load("check_markdown_links")


class ChangeClassifierTests(unittest.TestCase):
    def test_markdown_only_is_docs_only(self):
        result = classifier.classify_paths(["README.md", "docs/roadmap.md"])
        self.assertTrue(result["docs_only"])
        self.assertEqual(result["changed_docs"], ["README.md", "docs/roadmap.md"])

    def test_source_or_empty_changes_are_full(self):
        self.assertFalse(classifier.classify_paths(["cmd/gamed/main.go"])["docs_only"])
        self.assertFalse(classifier.classify_paths([])["docs_only"])

    def test_deleted_markdown_is_safe_docs_only(self):
        self.assertTrue(classifier.classify_paths(["docs/obsolete.md"])["docs_only"])

    def test_ci_and_helpers_are_always_full(self):
        for path in (".github/workflows/ci.yml", "scripts/check_markdown_links.py", "../README.md"):
            self.assertFalse(classifier.classify_paths([path])["docs_only"], path)


class MarkdownLinkTests(unittest.TestCase):
    def test_valid_relative_and_external_links_pass(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "guide.md").write_text("# Guide\n", encoding="utf-8")
            source = root / "README.md"
            source.write_text("[guide](guide.md) [web](https://example.test) [section](#x)\n", encoding="utf-8")
            self.assertEqual(links.check(source), [])

    def test_missing_or_escaping_link_fails(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "README.md"
            source.write_text("[missing](missing.md) [escape](../outside.md)\n", encoding="utf-8")
            errors = links.check(source)
            self.assertEqual(len(errors), 2)

    def test_parent_link_inside_repo_and_empty_destination(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / '.git').mkdir()
            (root / 'README.md').write_text('# Project\n')
            (root / 'docs').mkdir()
            source = root / 'docs' / 'guide.md'
            source.write_text('[home](../README.md)\n')
            self.assertEqual(links.check(source), [])
            self.assertFalse(links.target_exists(source, ' '))

    def test_deleted_markdown_needs_no_link_check(self):
        self.assertEqual(links.check(Path("does-not-exist.md")), [])


if __name__ == "__main__":
    unittest.main()
