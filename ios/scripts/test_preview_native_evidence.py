"""Causal synthetic controls for original-backed BRG28 intake.

Synthetic log models qualify only this verifier; they are never Apple evidence.
"""
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

import preview_native_evidence as preview
import collect_ios_test_evidence as collector

ROOT = Path(__file__).resolve().parents[2]


def benchmark_values():
    contract = preview.expected_contract()["benchmark"]
    values = []
    for fixture in contract["fixtures"]:
        samples = [float(i + 1) for i in range(20)]
        value = {
            "schema": "cherri-native-preview-benchmark/1",
            "fixture": fixture["fixture"], "sourceSha256": fixture["sourceSha256"],
            "assetSha256": fixture["assetSha256"], "sampleCount": 20,
            "sampleSourceSha256": [preview.digest((fixture["source"] + contract["sampleSuffix"].format(index=i)).encode())
                                   for i in range(20)],
            "totalCompileMilliseconds": samples, "p50Milliseconds": 10.0, "p95Milliseconds": 19.0,
            "osVersion": "Version 27.0 (Build 24A434)", "simulatorModel": "iPhone17,1",
            "mode": "v2", "buildProfile": "debug",
            "cacheReadsDelta": 1 if fixture["assetBytes"] else 0,
            "cacheEncodesDelta": 1 if fixture["assetBytes"] else 0,
            "cacheHitsDelta": 19 if fixture["assetBytes"] else 0,
            "scope": contract["scope"],
        }
        values.append(value)
    return values


def ui_values():
    ui = preview.expected_contract()["ui"]
    values = []
    for index, stage in enumerate(ui["stages"]):
        value = {"schema": "cherri-native-preview-ui/1", "testId": preview.UI,
                 "stage": stage, "sourceSha256": ui["sourceSha256"] if index == 0 else ui["updatedSourceSha256"]}
        if index == 1:
            value["elapsedSeconds"] = 4.5
        values.append(value)
    return values


def model_log(methods=None):
    rows = []
    for method in methods if methods is not None else preview.METHODS:
        suite, test = method.split("/")
        target = "CherriUITests" if method == preview.UI else "CherriTests"
        name = f"Test Case '-[{target}.{suite} {test}]'"
        rows.append(name + " started.")
        if method == preview.BENCHMARK:
            rows.extend(preview.BENCHMARK_PREFIX + json.dumps(value, sort_keys=True) for value in benchmark_values())
        if method == preview.UI:
            rows.extend(preview.UI_PREFIX + json.dumps(value, sort_keys=True) for value in ui_values())
            rows.append(preview.CHECKPOINT)
        rows.append(name + " passed (1.000 seconds).")
    return "\n".join(rows) + "\n"


def parsed(text):
    guard = preview.LogGuard(ROOT)
    for line in text.splitlines():
        guard.observe(line)
    return guard.verified()


def synthetic_pins():
    result = []
    for name in preview.SOURCE_PATHS:
        path = ROOT / name
        raw = path.read_bytes()
        result.append({"path": name, "bytes": len(raw), "sha256": preview.digest(raw),
                       "gitBlob": hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest(),
                       "gitMode": "100755" if path.stat().st_mode & 0o100 else "100644"})
    return result


class PreviewNativeIntakeControls(unittest.TestCase):
    def test_original_requirement_and_exact_contract_bytes(self):
        raw, contract = preview.load_contract(ROOT)
        self.assertEqual(preview.digest((ROOT / preview.GATE_PATH).read_bytes()), preview.GATE_SHA)
        self.assertEqual(contract["tiers"], ["integration", "ios-ui", "benchmark"])
        self.assertIs(contract["physicalDebugBenchmarkIncluded"], False)
        self.assertIs(contract["formalAcceptanceGrantedByContract"], False)
        self.assertEqual(len(contract["requiredMethods"]), 7)
        self.assertEqual(contract["ui"]["sourceSha256"], "bfd415a4ff534b825a79fba7ec1584c79697addf47ff18bc8530f6bfe216715a")
        self.assertEqual(contract["ui"]["updatedSourceSha256"], "e5a5d7a202fed9b0326918017cef19f85619025807f10a28ebd91b22c90ce65b")
        self.assertGreater(len(raw), 0)

    def test_complete_synthetic_cohort_has_all_sixty_samples(self):
        observations = parsed(model_log())
        self.assertEqual(observations["passedMethods"], preview.METHODS)
        self.assertEqual(len(observations["uiEvents"]), 4)
        self.assertEqual(sum(len(v["totalCompileMilliseconds"]) for v in observations["benchmarkObservations"]), 60)
        self.assertEqual(len(set(h for v in observations["benchmarkObservations"] for h in v["sampleSourceSha256"])), 60)

    def test_only_three_exact_tier_records(self):
        outcomes = [{"testId": test, "observedStatus": "passed"} for test in preview.METHODS]
        with mock.patch.object(preview, "source_pins", return_value=synthetic_pins()):
            summary, files, records = preview.collect(ROOT, model_log().encode(), outcomes, "a" * 40, "123", 1, True, False)
        self.assertEqual(summary["status"], "VERIFIED_NATIVE_PREVIEW")
        self.assertEqual({row["tier"] for row in records}, {"integration", "ios-ui", "benchmark"})
        self.assertEqual(len(records), 3)
        self.assertEqual(set(files), {"preview-document-contract.json", "preview-native-proof.json"})
        for row in records:
            self.assertEqual(row["requirements"], ["BRG28"])
            self.assertEqual(len(row["artifacts"]), 3)  # caller appends actual diagnostics as the fourth.
            self.assertEqual(row["assertions"], preview.PROFILES[row["test_id"]]["assertions"])

    def test_every_missing_method_and_earlier_generic_probe_rejected(self):
        for missing in preview.METHODS:
            with self.subTest(missing=missing), self.assertRaises(preview.Rejected):
                parsed(model_log([test for test in preview.METHODS if test != missing]))
        generic = "Test Case '-[CherriUITests.CherriEditorUITests testE07VisualParameterEditingCanvasUI]' started.\n"
        generic += "[CHECKPOINT:E07_PREVIEW_EDIT_VERIFIED]\n"
        generic += "Test Case '-[CherriUITests.CherriEditorUITests testE07VisualParameterEditingCanvasUI]' passed (1.0 seconds).\n"
        with self.assertRaises(preview.Rejected):
            parsed(generic)

    def test_case_ownership_terminal_and_duplicates_rejected(self):
        original = model_log()
        integration = preview.INTEGRATION.split("/")[1]
        mutations = [
            original.replace(" passed (1.000 seconds).", " failed (1.000 seconds).", 1),
            original.replace("CherriTests.CherriPreviewDocumentContextTests", "ForeignTests.CherriPreviewDocumentContextTests", 1),
            original + original,
            original.replace(" started.", " started. extra", 1),
            original.replace(f" {integration}]' started.\n", f" {integration}]' started.\nTest Case '-[CherriTests.CherriPreviewDocumentContextTests {integration}]' started.\n", 1),
            original.replace(" started.", " passed (1.000 seconds).", 1),
            original + preview.REJECTED + "MODEL\n",
        ]
        for index, text in enumerate(mutations):
            with self.subTest(index=index), self.assertRaises(preview.Rejected):
                parsed(text)

    def test_ui_order_source_checkpoint_and_privacy_rejected(self):
        original = model_log()
        line = next(line for line in original.splitlines() if line.startswith(preview.UI_PREFIX))
        event = json.loads(line[len(preview.UI_PREFIX):])
        bad = copy.deepcopy(event);bad["sourceSha256"] = "0" * 64
        extra = copy.deepcopy(event);extra["privateText"] = "PRIVATE_CONTROL"
        mutations = [
            original.replace(line + "\n", "", 1),
            original.replace(line, line + "\n" + line, 1),
            original.replace(line, preview.UI_PREFIX + json.dumps(bad), 1),
            original.replace(line, preview.UI_PREFIX + json.dumps(extra), 1),
            original.replace(line, " " + line, 1),
            original.replace(preview.CHECKPOINT, preview.CHECKPOINT + " trailing", 1),
            original.replace(preview.CHECKPOINT, "", 1),
            original.replace(preview.CHECKPOINT, preview.CHECKPOINT + "\n" + preview.CHECKPOINT, 1),
            line + "\n" + original,
        ]
        stages = [line for line in original.splitlines() if line.startswith(preview.UI_PREFIX)]
        mutations.append(original.replace(stages[1] + "\n" + stages[2], stages[2] + "\n" + stages[1]))
        for index, text in enumerate(mutations):
            with self.subTest(index=index), self.assertRaises(preview.Rejected):
                parsed(text)
        sanitized, _, _ = collector.bounded_log(mutations[3])
        self.assertNotIn("PRIVATE_CONTROL", sanitized)
        self.assertIn(preview.REJECTED + "INVALID_UI_EVENT", sanitized)

    def test_benchmark_raw_sample_hashes_percentiles_and_cache_rejected(self):
        contract = preview.expected_contract()
        base = benchmark_values()[2]
        controls = []
        def changed(field, value):
            sample = copy.deepcopy(base);sample[field] = value;controls.append(sample)
        changed("sampleCount", True)
        changed("sampleCount", 19)
        changed("totalCompileMilliseconds", [1.0] * 19)
        for bad in (0, -1, float("nan"), float("inf"), True):
            timings = list(base["totalCompileMilliseconds"]);timings[5] = bad
            changed("totalCompileMilliseconds", timings)
        changed("p50Milliseconds", 9.0)
        changed("p95Milliseconds", 20.0)
        changed("p50Milliseconds", True)
        hashes = list(base["sampleSourceSha256"]);hashes[1] = hashes[0]
        changed("sampleSourceSha256", hashes)
        changed("sourceSha256", "a" * 64)
        changed("assetSha256", "")
        changed("fixture", "small")
        changed("mode", "legacy")
        changed("buildProfile", "release")
        changed("scope", "actual-shortcuts-runtime")
        changed("cacheReadsDelta", 0)
        changed("cacheEncodesDelta", 2)
        changed("cacheHitsDelta", 18)
        changed("cacheHitsDelta", True)
        changed("osVersion", "UNOBSERVED")
        changed("simulatorModel", "physical-unreported")
        extra = copy.deepcopy(base);extra["privatePath"] = "PRIVATE_CONTROL";controls.append(extra)
        for index, value in enumerate(controls):
            with self.subTest(index=index), self.assertRaises(preview.Rejected):
                preview.validate_benchmark(value, contract, 2)
        for index in (0, 1):
            value = benchmark_values()[index];value["cacheReadsDelta"] = 1
            with self.subTest(no_asset=index), self.assertRaises(preview.Rejected):
                preview.validate_benchmark(value, contract, index)

    def test_benchmark_order_duplicates_and_owner_rejected(self):
        original = model_log()
        lines = [line for line in original.splitlines() if line.startswith(preview.BENCHMARK_PREFIX)]
        mutations = [
            original.replace(lines[0] + "\n", "", 1),
            original.replace(lines[0], lines[0] + "\n" + lines[0], 1),
            original.replace(lines[0] + "\n" + lines[1], lines[1] + "\n" + lines[0]),
            lines[0] + "\n" + original,
            original.replace(lines[0], lines[0] + " trailing", 1),
            original.replace(lines[0], lines[0].replace('"sampleCount": 20', '"sampleCount": 20, "sampleCount": 20'), 1),
        ]
        for index, text in enumerate(mutations):
            with self.subTest(index=index), self.assertRaises(preview.Rejected):
                parsed(text)

    def test_failed_case_preserves_original_terminal_and_following_case(self):
        text = model_log().replace(" passed (1.000 seconds).", " failed (1.000 seconds).", 1)
        sanitized, _, _ = collector.bounded_log(text)
        self.assertIn(" failed (1.000 seconds).", sanitized)
        self.assertIn(preview.REJECTED + "CASE_TERMINAL", sanitized)
        self.assertEqual(sanitized.count(" started."), 7)
        self.assertEqual(sanitized.count(" passed (1.000 seconds)."), 6)

    def test_canonical_sanitizer_preserves_all_original_safe_observations(self):
        original = model_log()
        sanitized, _, _ = collector.bounded_log(original)
        for line in original.splitlines():
            self.assertIn(line + "\n", sanitized)
        self.assertEqual(parsed(sanitized), parsed(original))

    def test_ineligible_partial_failed_or_wrong_attempt_has_no_records(self):
        outcomes = [{"testId": test, "observedStatus": "passed"} for test in preview.METHODS]
        controls = [
            (False, False, 1, outcomes), (True, True, 1, outcomes),
            (True, False, None, outcomes), (True, False, True, outcomes), (True, False, 0, outcomes),
            (True, False, 1, outcomes[:-1]),
            (True, False, 1, outcomes + [outcomes[0]]),
            (True, False, 1, [dict(outcomes[0], observedStatus="failed")] + outcomes[1:]),
        ]
        for eligible, diagnostic, attempt, results in controls:
            with self.subTest(eligible=eligible, diagnostic=diagnostic, attempt=attempt, count=len(results)):
                summary, files, records = preview.collect(ROOT, model_log().encode(), results, "a" * 40, "123", attempt, eligible, diagnostic)
                self.assertEqual(summary["status"], "NOT_VERIFIED")
                self.assertEqual((files, records), ({}, []))

    def test_immutable_contract_rejects_changed_original_or_fixture(self):
        for change in ("gate", "fixture", "flag"):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as temp:
                root = Path(temp).resolve(strict=True)
                for name in (preview.GATE_PATH, preview.CONTRACT_PATH):
                    target = root / name;target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_bytes((ROOT / name).read_bytes())
                if change == "gate":
                    (root / preview.GATE_PATH).write_bytes((root / preview.GATE_PATH).read_bytes() + b" ")
                else:
                    path = root / preview.CONTRACT_PATH;value = json.loads(path.read_bytes())
                    if change == "fixture":
                        value["ui"]["updatedSource"] += "// not intended\n"
                        value["ui"]["updatedSourceSha256"] = preview.digest(value["ui"]["updatedSource"].encode())
                    else:
                        value["genericTestInheritanceAllowed"] = 0
                    path.write_text(json.dumps(value))
                with self.assertRaises(preview.Rejected):
                    preview.load_contract(root)

    def test_physical_source_bytes_modes_symlinks_and_missing_inventory_rejected(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp).resolve(strict=True);path = root / "producer.py"
            raw = b"PUBLIC_ORIGINAL\n";path.write_bytes(raw)
            blob = hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest()
            tree = f"100644 blob {blob}\tproducer.py\0".encode()
            def git(argv, **_):
                return subprocess.CompletedProcess(argv, 0, b"a" * 40 + b"\n" if "rev-parse" in argv else tree)
            with mock.patch.object(preview, "SOURCE_PATHS", ["producer.py"]), mock.patch.object(preview.subprocess, "run", side_effect=git):
                self.assertEqual(len(preview.source_pins(root, "a" * 40)), 1)
                for change in ("bytes", "mode", "symlink"):
                    with self.subTest(change=change):
                        path.unlink();path.write_bytes(raw);path.chmod(0o644)
                        if change == "bytes":path.write_bytes(raw + b"modified")
                        elif change == "mode":path.chmod(0o755)
                        else:
                            original = root / "original.py";original.write_bytes(raw)
                            path.unlink();path.symlink_to(original)
                        with self.assertRaises(preview.Rejected):
                            preview.source_pins(root, "a" * 40)
                with mock.patch.object(preview, "SOURCE_PATHS", ["producer.py", "missing.py"]), self.assertRaises(preview.Rejected):
                    preview.source_pins(root, "a" * 40)


if __name__ == "__main__":
    unittest.main()
