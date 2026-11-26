#!/usr/bin/env python3
"""Source and parser controls using unfiltered historical Docs originals.

The four-parent input is not a BRG29 producer run. Its original bytes and
process receipt remain intact; no Go/compiler/generator is invoked here.
"""
from __future__ import annotations
import ast
import copy
import hashlib
import json
from pathlib import Path
import re
import unittest

import documentation_recovery_intake as subject

ROOT = Path(subject.__file__).resolve().parents[2]
FIXTURES = Path(__file__).resolve().parent / "testdata/documentation-recovery-s708"
PACKAGE = "github.com/electrikmilk/cherri/tools/language-docs"
EXTRA_DOCS_PARENT = "TestGeneratedReferenceIncludesSeparateMacroContracts"
ORIGINAL_PARENTS = ("TestP15D01PrimaryParameterContract",
                    "TestP15D01AllGeneratedSignaturesAndCategories",
                    "TestP15D01CheckGateProducesAndRejectsFaults")
RAW_SHA256 = "96bd2bac51ece5f7d18793038e23bb1da12500894315339f5b98307e885b82ae"
PROCESS_SHA256 = "34be61664c09971b43229c64c1ca7a70ebf01ee19a070bc87b6b014d4147fe59"


class DocumentationGenerationInventoryControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.raw = (FIXTURES / "go-controls.stdout.original").read_bytes()
        process_raw = (FIXTURES / "go-controls.process.original.json").read_bytes()
        assert hashlib.sha256(cls.raw).hexdigest() == RAW_SHA256
        assert hashlib.sha256(process_raw).hexdigest() == PROCESS_SHA256
        original = json.loads(process_raw)
        assert original["returncode"] == 0 and original["passed"] is True
        assert original["timed_out"] is False and original["cleanup_complete"] is True
        assert original["orphaned_group"] is False
        assert original["stdout"]["sha256"] == RAW_SHA256
        # An explicit field-name view for go_events only; original stays intact.
        cls.interval = {"startedUTC": original["started_utc"],
                        "endedUTC": original["finished_utc"],
                        "elapsedSeconds": original["elapsed_seconds"],
                        "budgetSeconds": original["budget_seconds"]}
        cls.rows = [json.loads(line) for line in cls.raw.splitlines()]
        cls.actual = tuple(row["Test"] for row in cls.rows
                           if row["Action"] == "run" and row.get("Test"))
        assert len(cls.actual) == len(set(cls.actual)) == 14
        assert set(cls.actual) == {row["Test"] for row in cls.rows
                                  if row["Action"] == "pass" and row.get("Test")}

    def required_with_original_extra_parent(self):
        return subject.GEN_EVENTS + (EXTRA_DOCS_PARENT,)

    def test_current_source_inventory_and_original_parent_selection(self):
        source = (ROOT / "tools/language-docs/p15_contract_test.go").read_text()
        body = source.split("func " + ORIGINAL_PARENTS[2] + "(", 1)[1].split("\nfunc ", 1)[0]
        children = tuple(re.findall(r'^\s*\{"([a-z_]+)",', body, re.M))
        self.assertEqual(len(children), 10)
        self.assertEqual(tuple(subject.GEN_TESTS), ORIGINAL_PARENTS)
        self.assertEqual(subject.GEN_EVENTS,
                         ORIGINAL_PARENTS + tuple(ORIGINAL_PARENTS[2]+"/"+n for n in children))
        pipeline = ast.parse((ROOT / "tools/p10r15/documentation_recovery_pipeline.py").read_bytes())
        selections = [ast.literal_eval(node.value) for node in ast.walk(pipeline)
                      if isinstance(node, ast.Assign)
                      and any(isinstance(t, ast.Name) and t.id == "canonical_tests" for t in node.targets)]
        self.assertEqual(selections, [ORIGINAL_PARENTS])
        macros = source.split("wantMacros := []string{", 1)[1].split("}", 1)[0]
        macro_names = re.findall(r'"([^"]+)"', macros)
        self.assertEqual(len(macro_names), len(set(macro_names)))
        native_count = int(re.search(r"len\(reg\.AllActions\(\)\) != (\d+)", source).group(1))
        scope = (ROOT / "tools/language-acceptance/documentation_recovery_execution.go").read_text()
        self.assertIn(str(native_count+len(macro_names))+" signature contracts with"+
                      str(len(children))+" real drift rejections", scope)

    def test_unfiltered_original_with_explicit_four_parent_inventory(self):
        subject.go_events(self.raw, PACKAGE, self.required_with_original_extra_parent(), self.interval)
        self.assertEqual(set(self.actual), set(self.required_with_original_extra_parent()))

    def test_unfiltered_four_parent_original_cannot_claim_brg29_execution(self):
        with self.assertRaisesRegex(ValueError, "unselected generation event"):
            subject.go_events(self.raw, PACKAGE, subject.GEN_EVENTS, self.interval)

    def test_missing_each_actual_fault_and_changed_scoped_terminals_rejected(self):
        required = self.required_with_original_extra_parent()
        subject.go_events(self.raw, PACKAGE, required, self.interval)
        children = [name for name in self.actual if "/" in name]
        for name in children:
            with self.subTest(missing=name):
                rows = [r for r in self.rows if r.get("Test") != name]
                changed = b"\n".join(json.dumps(r).encode() for r in rows) + b"\n"
                with self.assertRaisesRegex(ValueError, "orphan/reordered pass"):
                    subject.go_events(changed, PACKAGE, required, self.interval)
        for leaf in ("scoped_dialect", "scoped_void_result"):
            name = ORIGINAL_PARENTS[2]+"/"+leaf
            for action in ("fail", "skip"):
                with self.subTest(terminal=action, child=name):
                    rows = copy.deepcopy(self.rows)
                    changed = [r for r in rows if r.get("Test") == name and r["Action"] == "pass"]
                    self.assertEqual(len(changed), 1)
                    changed[0]["Action"] = action
                    raw = b"\n".join(json.dumps(r).encode() for r in rows) + b"\n"
                    with self.assertRaisesRegex(ValueError, "failed/skipped/unknown"):
                        subject.go_events(raw, PACKAGE, required, self.interval)


if __name__ == "__main__":
    unittest.main()

