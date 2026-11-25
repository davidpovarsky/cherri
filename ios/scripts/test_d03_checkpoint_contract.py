"""Bind the real Swift D03 emitter to the unchanged strict evidence collector."""
from pathlib import Path
import re
import unittest

import collect_ios_test_evidence as collector
import documentation_evidence as docs


class D03CheckpointContractTests(unittest.TestCase):
    def emitted_checkpoint(self):
        source = (Path(__file__).resolve().parents[1] / "CherriUITests" /
                  "CherriEditorUITests.swift").read_text(encoding="utf-8")
        values = re.findall(
            r'print\("(\[CHECKPOINT:D03_CURRENT_DOCS_SEARCH_VERIFIED\][^"]*)"\)',
            source,
        )
        self.assertEqual(len(values), 1, "Exactly one authored D03 checkpoint emitter is required")
        return values[0]

    def block(self, marker, method="testD03CurrentDocumentationSearchUI"):
        return (f"Test Case '-[CherriUITests.CherriEditorUITests {method}]' started.\n"
                f"{marker}\n"
                f"Test Case '-[CherriUITests.CherriEditorUITests {method}]' passed (0.123 seconds).\n")

    def test_actual_swift_emitter_is_the_exact_protocol_token(self):
        self.assertEqual(self.emitted_checkpoint(), docs.CHECKPOINT)

    def test_actual_swift_token_survives_the_original_collector(self):
        text, _, _ = collector.bounded_log(self.block(self.emitted_checkpoint()))
        self.assertEqual(text.splitlines().count(docs.CHECKPOINT), 1)
        self.assertNotIn(docs.REJECTED_PREFIX, text)

    def test_trailing_text_remains_rejected(self):
        text, _, _ = collector.bounded_log(
            self.block(docs.CHECKPOINT + " untrusted trailing text"))
        self.assertIn(docs.REJECTED_PREFIX + "INVALID_CHECKPOINT", text)
        self.assertNotIn(docs.CHECKPOINT, text.splitlines())

    def test_exact_token_outside_owned_ui_test_remains_rejected(self):
        text, _, _ = collector.bounded_log(
            self.block(docs.CHECKPOINT, "testE01CompletionDisplayAndInsertionUI"))
        self.assertIn(docs.REJECTED_PREFIX + "INVALID_CHECKPOINT", text)
        self.assertNotIn(docs.CHECKPOINT, text.splitlines())


if __name__ == "__main__":
    unittest.main()
