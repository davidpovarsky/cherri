"""Synthetic causal controls for the D03 collector; no Apple execution credit."""
import contextlib
import copy
import hashlib
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import collect_ios_test_evidence as collector
import documentation_evidence as docs


class DocumentationEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.serial = 0
        self.manifest = {
            "languageVersion": "2.0",
            "docsRepo": "https://github.com/davidpovarsky/cherrilang.org.git",
            "compilerRepo": "https://github.com/davidpovarsky/cherri.git",
            "docsCommit": "b" * 40, "compilerCommit": "c" * 40,
            "schemaFingerprint": "d" * 64, "macroFingerprint": "e" * 64,
        }
        paths = sorted(docs.SOURCE_PATHS)
        self.index = {"formatVersion": 1, "languageVersion": "2.0",
                      **{key: self.manifest[key] for key in docs.PINS},
                      "sourceFiles": [{"path": path, "sha256": hashlib.sha256(path.encode()).hexdigest()}
                                      for path in paths], "sections": []}
        for path in paths:
            for line in range(1, 127):
                self.index["sections"].append({
                    "id": path + ":" + str(line), "sourcePath": path, "sourceLine": line,
                    "documentTitle": "Current documentation",
                    "title": "show" if path.endswith("/actions.md") and line == 1 else "Section " + str(line),
                    "migrationOnly": path.endswith("/migration.md"),
                    "blocks": [{"kind": "code", "language": "cherri", "text": 'show("public synthetic")'}],
                })
        self.write_inputs()

    def write_inputs(self, index=None, manifest=None):
        self.index_bytes = (json.dumps(index or self.index, ensure_ascii=False, indent=2) + "\n").encode()
        (self.root / "index.json").write_bytes(self.index_bytes)
        self.manifest_bytes = (json.dumps(manifest or self.manifest, indent=2) + "\n").encode()
        (self.root / "manifest.json").write_bytes(self.manifest_bytes)
        self.generation = {"docsCommit": self.index["docsCommit"],
                           "compilerCommit": self.index["compilerCommit"],
                           "sourceFiles": 4, "sections": len(self.index["sections"]),
                           "bytes": len(self.index_bytes),
                           "sha256": hashlib.sha256(self.index_bytes).hexdigest()}
        self.generation_bytes = (json.dumps(self.generation) + "\n").encode()
        (self.root / "generation.json").write_bytes(self.generation_bytes)

    def receipt_value(self, **updates):
        value = {"schemaVersion": "1.0", **{key: self.index[key] for key in docs.PINS},
                 "bundleSHA256": hashlib.sha256(self.index_bytes).hexdigest(),
                 "sourcePaths": [row["path"] for row in self.index["sourceFiles"]],
                 "sourceHashes": [row["sha256"] for row in self.index["sourceFiles"]],
                 "sections": len(self.index["sections"]), "exactActionFirst": "show",
                 "defaultMigrationResults": 0, "explicitMigrationResults": 126,
                 "missingResults": 0}
        value.update(updates)
        return value

    def receipt_line(self, **updates):
        return docs.PREFIX + json.dumps(self.receipt_value(**updates))

    def block(self, test, body="", result="passed"):
        suite, method = test.split("/")
        target = ("CherriUITests" if suite == "CherriEditorUITests" else "CherriTests") + "." + suite
        return (f"Test Case '-[{target} {method}]' started.\n{body}\n"
                f"Test Case '-[{target} {method}]' {result} (0.123 seconds).\n")

    def log(self, receipt=None):
        return (self.block(docs.POSITIVE, self.receipt_line() if receipt is None else receipt)
                + self.block(docs.NEGATIVE) + self.block(docs.UI, docs.CHECKPOINT))

    def collect(self, text=None, exit_code=0, implementation="a" * 40,
                run_id="1234", missing=(), generation=True):
        self.serial += 1
        log = self.root / ("input-%d.log" % self.serial)
        log.write_text(self.log() if text is None else text, encoding="utf-8")
        exit_path = self.root / ("exit-%d.txt" % self.serial)
        if exit_code is not None:
            exit_path.write_text(str(exit_code), encoding="utf-8")
        output = self.root / ("output-%d" % self.serial)
        paths = [self.root / "index.json", self.root / "manifest.json",
                 self.root / "generation.json" if generation else None]
        for name in missing:
            paths[{"index": 0, "manifest": 1, "generation": 2}[name]] = self.root / ("missing-" + name)
        with contextlib.redirect_stdout(io.StringIO()):
            diagnostic, records = collector.collect(log, output, implementation, run_id,
                                                     exit_path, *paths)
        return diagnostic, records, {p.name: p.read_bytes() for p in output.iterdir()}

    def assert_rejected(self, result):
        diagnostic, records, _ = result
        self.assertFalse(any("D03" in record["requirements"] for record in records))
        self.assertEqual(diagnostic["documentationEvidence"]["status"], "NOT_VERIFIED")
        self.assertTrue(diagnostic["documentationEvidence"]["errorCodes"])

    def test_three_tests_bind_exact_raw_receipt_and_artifact_bytes(self):
        raw = "  " + docs.PREFIX + json.dumps(self.receipt_value(), separators=(", ", " : ")) + "  "
        diagnostic, records, output = self.collect(self.log(raw))
        self.assertEqual(diagnostic["documentationEvidence"]["status"], "VERIFIED_APP_TESTS")
        self.assertEqual(len(records), 1)
        record = records[0]
        self.assertEqual(record["requirements"], ["D03"])
        self.assertEqual(record["tier"], "integration")
        self.assertEqual(record["test_id"], "CherriUITests/testD03CurrentDocumentationSearchUI")
        self.assertEqual(record["implementation_sha"], "a" * 40)
        self.assertNotEqual(record["implementation_sha"], self.index["compilerCommit"])
        self.assertEqual(record["assertions"], {"current_docs_bundle": "true",
                                              "historical_docs_rejected": "true",
                                              "current_docs_ui_search": "true"})
        self.assertIn((raw + "\n").encode(), output["xcodetest.log"])
        self.assertEqual(output["documentation-index.json"], self.index_bytes)
        self.assertEqual(output["documentation-compatibility-manifest.json"], self.manifest_bytes)
        self.assertEqual(output["documentation-receipt.json"], self.generation_bytes)
        self.assertEqual(len(record["artifacts"]), 5)
        for artifact in record["artifacts"]:
            self.assertEqual(artifact["run_id"], "1234")
            self.assertEqual(artifact["sha256"], hashlib.sha256(output[Path(artifact["path"]).name]).hexdigest())

    def test_each_required_test_missing_or_failed_is_rejected(self):
        for test in docs.TEST_IDS:
            with self.subTest(test=test, mode="missing"):
                blocks = [self.block(t, self.receipt_line() if t == docs.POSITIVE
                                     else docs.CHECKPOINT if t == docs.UI else "")
                          for t in docs.TEST_IDS if t != test]
                self.assert_rejected(self.collect("".join(blocks)))
            with self.subTest(test=test, mode="failed"):
                text = self.log().replace(self.block(test, self.receipt_line() if test == docs.POSITIVE
                                                     else docs.CHECKPOINT if test == docs.UI else ""),
                                         self.block(test, self.receipt_line() if test == docs.POSITIVE
                                                    else docs.CHECKPOINT if test == docs.UI else "", "failed"))
                self.assert_rejected(self.collect(text))

    def test_each_duplicate_case_or_orphan_end_is_rejected(self):
        for test in docs.TEST_IDS:
            with self.subTest(test=test, mode="duplicate"):
                self.assert_rejected(self.collect(self.log() + self.block(test)))
            with self.subTest(test=test, mode="orphan-end"):
                self.assert_rejected(self.collect(self.log() + self.block(test).splitlines()[-1] + "\n"))

    def test_interleaved_and_unclosed_cases_are_rejected(self):
        positive = self.block(docs.POSITIVE, self.receipt_line()).splitlines(keepends=True)
        text = positive[0] + self.block(docs.NEGATIVE) + "".join(positive[1:]) + self.block(docs.UI, docs.CHECKPOINT)
        self.assert_rejected(self.collect(text))
        self.assert_rejected(self.collect(self.log() + self.block(docs.NEGATIVE).splitlines()[0] + "\n"))

    def test_unknown_test_boundary_cannot_hide_inside_positive_test(self):
        body = self.receipt_line() + "\nTest Case '-[OtherTests.OtherSuite testOther]' started.\n"
        self.assert_rejected(self.collect(self.log(body)))

    def test_only_the_exact_ui_checkpoint_inside_its_block_is_accepted(self):
        for text in (self.log().replace(docs.CHECKPOINT, ""),
                     docs.CHECKPOINT + "\n" + self.log(),
                     self.log().replace(docs.CHECKPOINT, docs.CHECKPOINT + " PRIVATE_SUFFIX"),
                     self.log().replace(docs.CHECKPOINT, docs.CHECKPOINT + "\n" + docs.CHECKPOINT),
                     self.log().replace(docs.CHECKPOINT, "[CHECKPOINT:D03_CURRENT_DOCUMENTATION_VERIFIED]"),
                     self.block(docs.POSITIVE, self.receipt_line() + "\n" + docs.CHECKPOINT)
                     + self.block(docs.NEGATIVE) + self.block(docs.UI)):
            with self.subTest(text=text[-100:]):
                self.assert_rejected(self.collect(text))

    def test_receipt_missing_duplicate_wrong_context_and_overflow_are_rejected(self):
        for text in (self.log(""), self.log(self.receipt_line() + "\n" + self.receipt_line()),
                     self.receipt_line() + "\n" + self.log(),
                     self.block(docs.POSITIVE) + self.block(docs.NEGATIVE, self.receipt_line())
                     + self.block(docs.UI, docs.CHECKPOINT),
                     self.log("\n".join([self.receipt_line()] * 10))):
            self.assert_rejected(self.collect(text))

    def test_rejected_arbitrary_fields_never_reach_published_logs(self):
        secret = "private-owner@example.invalid"
        result = self.collect(self.log(self.receipt_line(privateNote=secret)))
        self.assert_rejected(result)
        self.assertNotIn(secret.encode(), b"".join(result[2].values()))
        self.assertNotIn(b"privateNote", result[2]["xcodetest.log"])

    def test_duplicate_json_keys_even_equal_values_are_rejected(self):
        line = self.receipt_line().replace('"schemaVersion": "1.0"', '"schemaVersion": "1.0", "schemaVersion": "1.0"')
        self.assert_rejected(self.collect(self.log(line)))
        line = self.receipt_line().replace('"schemaVersion": "1.0"', '"schemaVersion": "1.0", "\\u0073chemaVersion": "1.0"')
        self.assert_rejected(self.collect(self.log(line)))

    def test_receipt_types_domains_and_nonfinite_values_are_rejected(self):
        changes = [{"sections": True}, {"sections": 500}, {"sections": 10001},
                   {"defaultMigrationResults": False}, {"missingResults": False},
                   {"explicitMigrationResults": 0}, {"explicitMigrationResults": 1.5},
                   {"exactActionFirst": "quicklook"}, {"bundleSHA256": "f" * 63},
                   {"docsCommit": "main"}, {"schemaVersion": "2.0"},
                   {"sections": float("nan")}]
        for update in changes:
            with self.subTest(update=update):
                self.assert_rejected(self.collect(self.log(self.receipt_line(**update))))

    def test_each_receipt_pin_must_match_supplied_manifest_and_index(self):
        for key, size in docs.PINS.items():
            with self.subTest(key=key):
                self.assert_rejected(self.collect(self.log(self.receipt_line(**{key: "f" * size}))))

    def test_source_pairing_unique_paths_and_hashes_are_verified(self):
        value = self.receipt_value()
        changes = [{"sourcePaths": value["sourcePaths"][:3]},
                   {"sourcePaths": [value["sourcePaths"][0]] * 4},
                   {"sourcePaths": ["language/old.md"] + value["sourcePaths"][1:]},
                   {"sourceHashes": ["f" * 64] + value["sourceHashes"][1:]},
                   {"sourceHashes": value["sourceHashes"][::-1]},
                   {"sourceHashes": [True] * 4}]
        for update in changes:
            self.assert_rejected(self.collect(self.log(self.receipt_line(**update))))
        reversed_value = self.receipt_line(sourcePaths=value["sourcePaths"][::-1],
                                          sourceHashes=value["sourceHashes"][::-1])
        self.assertEqual(len(self.collect(self.log(reversed_value))[1]), 1)

    def test_bundle_bytes_and_observed_counts_must_match(self):
        for update in ({"bundleSHA256": "f" * 64}, {"sections": 505},
                       {"explicitMigrationResults": 125}):
            self.assert_rejected(self.collect(self.log(self.receipt_line(**update))))
        original = self.root / "index.json"
        original.write_bytes(self.index_bytes + b" ")
        self.assert_rejected(self.collect())

    def test_required_trusted_inputs_cannot_be_missing_or_invalid(self):
        for key in ("index", "manifest", "generation"):
            self.assert_rejected(self.collect(missing=(key,)))
        original = (self.root / "index.json").read_bytes()
        for data in (b"null", b"[]", b"{", b"{}", b"x" * 2_000_001):
            (self.root / "index.json").write_bytes(data)
            self.assert_rejected(self.collect())
        (self.root / "index.json").write_bytes(original)

    def test_missing_generation_stdout_is_diagnostic_only(self):
        result = self.collect(generation=False)
        self.assert_rejected(result)
        self.assertNotIn("documentation-receipt.json", result[2])
        self.assertEqual(result[0]["documentationEvidence"]["errorCodes"], ["MISSING_GENERATION_RECEIPT"])

    def test_direct_helper_enforces_the_required_test_targets(self):
        for original, replacement in (("CherriTests.DocumentationIndexTests", "OtherTests.DocumentationIndexTests"),
                                      ("CherriUITests.CherriEditorUITests", "OtherTests.CherriEditorUITests")):
            text = self.log().replace(original, replacement)
            with self.assertRaisesRegex(docs.EvidenceError, "WRONG_REQUIRED_TEST_TARGET"):
                docs.receipt_from_log(text)
            self.assert_rejected(self.collect(text))

    def test_index_language_uses_the_apps_exact_lowercase_grammar(self):
        for language in ("CHERRI", "cherri title=sample", "cherri\\nlegacy", "x/y", "a.b"):
            index = copy.deepcopy(self.index)
            index["sections"][0]["blocks"][0]["language"] = language
            self.write_inputs(index=index)
            self.assert_rejected(self.collect())
        self.write_inputs()

    def test_manifest_is_source_authority_not_only_matching_log_strings(self):
        for key, value in (("docsCommit", "f" * 40),
                           ("compilerCommit", "f" * 40),
                           ("docsRepo", "https://example.invalid/docs"),
                           ("compilerRepo", "https://example.invalid/compiler"),
                           ("languageVersion", "1.0")):
            broken = dict(self.manifest, **{key: value})
            self.write_inputs(manifest=broken)
            self.assert_rejected(self.collect())
        self.write_inputs()
        raw = (self.root / "manifest.json").read_text()
        (self.root / "manifest.json").write_text(raw.replace('"languageVersion": "2.0"',
                                                            '"languageVersion": "2.0", "languageVersion": "2.0"'))
        self.assert_rejected(self.collect())

    def test_index_historical_duplicate_and_damaged_rows_are_rejected(self):
        mutations = [
            lambda x: x["sections"][0].update(sourcePath="language/old.md"),
            lambda x: x["sections"].append(copy.deepcopy(x["sections"][0])),
            lambda x: x["sections"][0].update(migrationOnly=True),
            lambda x: x["sections"][0].update(sourceLine=True),
            lambda x: x["sections"][0].update(blocks=[]),
            lambda x: x["sections"][0]["blocks"][0].update(language="CHERRI-LEGACY title=example"),
            lambda x: x["sourceFiles"][0].update(path=x["sourceFiles"][1]["path"]),
            lambda x: x.update(formatVersion=True),
        ]
        for mutate in mutations:
            index = copy.deepcopy(self.index)
            mutate(index)
            self.write_inputs(index=index)
            self.assert_rejected(self.collect())
        self.write_inputs()

    def test_every_generation_fact_is_checked_against_raw_index(self):
        for key, value in (("docsCommit", "f" * 40), ("compilerCommit", "f" * 40),
                           ("sourceFiles", 3), ("sourceFiles", True), ("sections", 503),
                           ("bytes", len(self.index_bytes) - 1), ("sha256", "f" * 64)):
            broken = dict(self.generation, **{key: value})
            (self.root / "generation.json").write_text(json.dumps(broken), encoding="utf-8")
            self.assert_rejected(self.collect())

    def test_process_source_and_run_identity_are_required(self):
        for code in (65, None, "invalid"):
            self.assert_rejected(self.collect(exit_code=code))
        self.assert_rejected(self.collect(implementation="main"))
        self.assert_rejected(self.collect(run_id="../other-run"))

    def test_no_docs_inputs_keep_existing_service_behavior(self):
        with contextlib.redirect_stdout(io.StringIO()):
            text = self.block("CherriCoreIntegrationTests/testE01ContextualCompletionService")
            (self.root / "plain.log").write_text(text)
            (self.root / "exit").write_text("0")
            diagnostic, records = collector.collect(self.root / "plain.log", self.root / "plain",
                                                     "a" * 40, "1234", self.root / "exit")
        self.assertEqual(diagnostic["documentationEvidence"]["status"], "NOT_REQUESTED")
        self.assertEqual(len(records), 1)
        self.assertNotIn("D03", records[0]["requirements"])

    def test_actual_cli_accepts_explicit_inputs_and_reports_negative_without_credit(self):
        (self.root / "cli.log").write_text(self.log(), encoding="utf-8")
        (self.root / "cli-exit").write_text("0")
        base = [sys.executable, str(Path(collector.__file__)), str(self.root / "cli.log")]
        for valid in (True, False):
            output = self.root / ("cli-positive" if valid else "cli-negative")
            args = [*base, str(output), "a" * 40, "1234", str(self.root / "cli-exit"),
                    "--documentation-index", str(self.root / "index.json"),
                    "--documentation-manifest", str(self.root / ("manifest.json" if valid else "missing.json")),
                    "--documentation-receipt", str(self.root / "generation.json")]
            proc = subprocess.run(args, capture_output=True, text=True, timeout=20)
            self.assertEqual(proc.returncode, 0, proc.stderr)
            records = json.loads((output / "ios-test-evidence.json").read_text())["records"]
            self.assertEqual(len(records), 1 if valid else 0)
            diagnostic = json.loads((output / "ios-test-diagnostics.json").read_text())
            self.assertEqual(diagnostic["documentationEvidence"]["status"],
                             "VERIFIED_APP_TESTS" if valid else "NOT_VERIFIED")


if __name__ == "__main__":
    unittest.main()
