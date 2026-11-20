"""Reject altered public diagnostic sources/contracts before any native signing."""
import hashlib
import json
from pathlib import Path
import tempfile
import unittest

import date_difference_diagnostic as d


class DateDifferenceSourceContractTests(unittest.TestCase):
    def setUp(self):
        self.repo = Path(__file__).resolve().parents[2]
        self.source = (self.repo / d.SOURCE_PATH).read_bytes()
        self.contract = (self.repo / d.SOURCE_PATH).with_suffix(".expected.json").read_bytes()

    def test_reviewed_source_is_diagnostic_only(self):
        report = d.verify_source(self.repo, d.FIXTURE_ID, self.source)
        self.assertEqual(report["appleRuntime"], "NOT_RUN")
        self.assertIs(report["formalAcceptanceGranted"], False)
        self.assertEqual(report["sourceSha256"], hashlib.sha256(self.source).hexdigest())

    def test_changed_source_is_rejected(self):
        changed = self.source.replace(b'number(0)', b'number(1)')
        self.assertNotEqual(self.source, changed)
        with self.assertRaisesRegex(ValueError, "reviewed public source bytes"):
            d.verify_source(self.repo, d.FIXTURE_ID, changed)

    def test_changed_zero_oracle_is_rejected(self):
        with tempfile.TemporaryDirectory(prefix="cherri-date-contract-") as directory:
            root = Path(directory)
            path = root / d.SOURCE_PATH
            path.parent.mkdir(parents=True)
            path.write_bytes(self.source)
            contract = json.loads(self.contract)
            contract["expected_fields_for_numeric_date_contract"]["ZERO_SECONDS_TEXT64"] = ""
            path.with_suffix(".expected.json").write_text(json.dumps(contract), encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "independent contract bytes"):
                d.verify_source(root, d.FIXTURE_ID, self.source)

    def test_private_or_unregistered_identity_is_not_authorized(self):
        self.assertIsNone(d.verify_source(self.repo, "unregistered-date", self.source))
        self.assertIsNone(d.verify_graph("unregistered-date", []))

    def test_missing_graph_cannot_be_reported_as_verified(self):
        with self.assertRaises(ValueError):
            d.verify_graph(d.FIXTURE_ID, [])


if __name__ == "__main__":
    unittest.main()
