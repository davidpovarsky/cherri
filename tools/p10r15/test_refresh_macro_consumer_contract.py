#!/usr/bin/env python3
"""Fault controls for the pinned artifact-intake adapter; no compiler simulation."""
import copy
import importlib.util
from pathlib import Path
import unittest

_path = Path(__file__).with_name("refresh_macro_consumer_contract.py")
_spec = importlib.util.spec_from_file_location("macro_refresh", _path)
refresh = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(refresh)


class MacroConsumerRefreshIntakeTests(unittest.TestCase):
    def setUp(self):
        self.caps = {
            "languageVersion": "2.0", "catalogSchemaVersion": "3.0",
            "schemaFingerprint": "1" * 64, "codecFingerprint": "2" * 64,
            "macroFingerprint": "3" * 64, "actionCount": 505,
            "compilerMacros": [{"callableName": name} for name in refresh.MACROS],
        }
        self.manifest = {
            "skillVersion": "2.0.0", "compilerCommit": "a" * 40, "docsCommit": "b" * 40,
            "compilerRepo": "https://github.com/davidpovarsky/cherri.git",
            "docsRepo": "https://github.com/davidpovarsky/cherrilang.org.git",
            "languageVersion": "2.0", "catalogSchemaVersion": "3.0",
            "schemaFingerprint": "1" * 64, "codecFingerprint": "2" * 64,
            "macroFingerprint": "9" * 64,
        }
        tick = chr(96)
        self.reference = (
            "**Schema Fingerprint:** " + tick + "1" * 64 + tick + "  \n"
            "**Total Actions:** 505  \n\n## Compiler macros\n\n"
            "**Compiler Macro Fingerprint:** " + tick + "3" * 64 + tick + "\n\n"
            "**Compiler Macros:** 16\n\n" +
            "\n".join("### " + tick + name + tick + "\n" for name in refresh.MACROS)
        )

    def test_proposal_binds_actual_source_and_keeps_historical_docs_explicit(self):
        before = copy.deepcopy(self.manifest)
        result = refresh.validate_products(self.caps, self.manifest, self.reference)
        self.assertEqual(self.manifest, before)
        self.assertEqual(result["compilerCommit"], refresh.SOURCE)
        self.assertEqual(result["macroFingerprint"], self.caps["macroFingerprint"])
        self.assertEqual(result["docsCommit"], before["docsCommit"])
        self.assertEqual({key for key in result if result[key] != before[key]},
                         {"compilerCommit", "macroFingerprint"})

    def test_identity_and_inventory_faults_rejected(self):
        def changed_caps(key, value):
            caps = copy.deepcopy(self.caps)
            caps[key] = value
            return caps, self.manifest, self.reference
        cases = {
            "wrong_language": changed_caps("languageVersion", "1.0"),
            "wrong_catalog_version": changed_caps("catalogSchemaVersion", "2.0"),
            "missing_macro_identity": changed_caps("macroFingerprint", None),
            "malformed_macro_identity": changed_caps("macroFingerprint", "f" * 63),
            "nonhex_codec": changed_caps("codecFingerprint", "g" * 64),
            "wrong_native_count": changed_caps("actionCount", 504),
            "boolean_count": changed_caps("actionCount", True),
            "missing_macro": changed_caps("compilerMacros", self.caps["compilerMacros"][1:]),
            "duplicate_macro": changed_caps("compilerMacros", self.caps["compilerMacros"][:-1] +
                                           [self.caps["compilerMacros"][0]]),
            "foreign_macro": changed_caps("compilerMacros", [{"callableName": "foreign"}] +
                                         self.caps["compilerMacros"][1:]),
            "foreign_docs_header": (self.caps, self.manifest,
                                    self.reference.replace("3" * 64, "4" * 64)),
            "duplicate_docs_header": (self.caps, self.manifest,
                                      self.reference + "**Compiler Macro Fingerprint:** " +
                                      chr(96) + "3" * 64 + chr(96) + "\n"),
            "stale_docs_count": (self.caps, self.manifest,
                                 self.reference.replace("Compiler Macros:** 16", "Compiler Macros:** 15")),
            "missing_docs_section": (self.caps, self.manifest,
                                     self.reference.replace("## Compiler macros", "## Missing")),
            "foreign_docs_macro": (self.caps, self.manifest,
                                   self.reference.replace("### " + chr(96) + "cache" + chr(96),
                                                          "### " + chr(96) + "other" + chr(96))),
        }
        for name, values in cases.items():
            with self.subTest(name=name), self.assertRaises(ValueError):
                refresh.validate_products(*values)

    def test_mutable_manifest_refs_rejected(self):
        for key in ("compilerCommit", "docsCommit"):
            manifest = dict(self.manifest, **{key: "main"})
            with self.subTest(key=key), self.assertRaises(ValueError):
                refresh.validate_products(self.caps, manifest, self.reference)

    def test_json_duplicates_and_nonfinite_tokens_rejected(self):
        for raw in ('{"macroFingerprint":"a","macroFingerprint":"b"}',
                    '{"value":NaN}', '{"value":Infinity}', '{"value":-Infinity}'):
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                refresh.strict_json(raw)
        self.assertEqual(refresh.strict_json('{"value":false,"zero":0,"empty":""}'),
                         {"value": False, "zero": 0, "empty": ""})


if __name__ == "__main__":
    unittest.main()
