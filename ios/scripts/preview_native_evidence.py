"""Original-backed BRG28 intake: native context, actual WebKit edit and timings.

These are three distinct tiers from one complete seven-method native cohort.
Neither generic E07 nor a partial focused run supplies a BRG28 formal record.
"""
import hashlib
import json
import math
from pathlib import Path
import re
import stat
import subprocess

NATIVE_PATH = "ios/CherriTests/CherriPreviewDocumentContextTests.swift"
UI_PATH = "ios/CherriUITests/CherriPreviewDocumentUITests.swift"
CONTRACT_PATH = "ios/TestFixtures/preview-document-contract.json"
GATE_PATH = "tests/language-v2/backend-recovery-gates.json"
GATE_SHA = "e7fbb3ad177c5142935f71a46ca7e89b84822061bfc0919eec59708811ab7e7c"
NATIVE = "CherriPreviewDocumentContextTests/"
UI = "CherriPreviewDocumentUITests/testBRG28ActualWebKitFieldEditAndNumericRejectionUI"
INTEGRATION = NATIVE + "testBRG28ActualPreviewTransactionPreservesDocumentAndRejectsContextRelabels"
BENCHMARK = NATIVE + "testBRG28NativeSmallLargeAndEmbeddedAssetMeasurements"
METHODS = sorted([
    INTEGRATION, BENCHMARK,
    NATIVE + "testBRG28SavedDocumentAssetsAndImportedOriginsAcrossBothDialects",
    NATIVE + "testBRG28UnsavedMissingAndEscapingResourcesFailThroughActualBridge",
    NATIVE + "testBRG28CoordinatedAssetNotificationsInvalidateSameSizeAndTimestamp",
    NATIVE + "testBRG28CancelledCompileCannotReturnAnArtifact", UI,
])
UI_PREFIX = "CHERRI_NATIVE_PREVIEW_UI "
BENCHMARK_PREFIX = "CHERRI_NATIVE_PREVIEW_BENCHMARK "
CHECKPOINT = "CHERRI_NATIVE_PREVIEW_UI_CHECKPOINT=BRG28_WEBKIT_FIELDS_COMPLETE"
REJECTED = "CHERRI_NATIVE_PREVIEW_REJECTED "
ARTIFACT_BASE = "artifacts/backend-recovery/ci-artifacts/ios-build/ios-build-evidence/"
CASE = re.compile(r"^Test Case '-\[([A-Za-z0-9_]+)\.([A-Za-z0-9_]+) (test[A-Za-z0-9_]+)\]' (started\.|(passed|failed) \([0-9.]+ seconds\)\.)$")
PROFILES = {
    INTEGRATION: {
        "tier": "integration", "sourcePath": NATIVE_PATH,
        "assertions": {"same_run_seven_dedicated_methods": "true",
                       "native_document_and_asset_context_bound": "true",
                       "native_preview_transaction_source_exact": "true",
                       "context_relabels_rejected": "true",
                       "both_dialects_relative_resources": "true",
                       "missing_unsaved_escape_rejected": "true",
                       "same_metadata_asset_invalidation_observed": "true",
                       "pre_entry_cancellation_no_artifact": "true"},
        "scope": "Dedicated native Swift/C ABI cohort verifies saved and unsaved document context, both dialects' relative resource origins, missing and escaping resource rejection, exact preview transaction source/context, URI/UUID/revision/asset-revision relabel rejection, coordinated same-size/same-mtime asset invalidation and pre-entry cancellation without an artifact. It does not claim interruption inside a running C call, Shortcut execution or physical-device performance.",
    },
    UI: {
        "tier": "ios-ui", "sourcePath": UI_PATH,
        "assertions": {"same_run_seven_dedicated_methods": "true",
                       "actual_webkit_contenteditable_keyboard": "true",
                       "only_intended_source_node_changed": "true",
                       "comments_unicode_and_other_nodes_preserved": "true",
                       "native_rebuild_and_render_observed": "true",
                       "invalid_numeric_dom_edit_rejected": "true",
                       "no_stale_signed_artifact": "true"},
        "scope": "Dedicated XCUITest types a fixed Unicode/comment source in an actual document, builds it, edits the actual WebKit contentEditable Text field through the keyboard, blurs through another actual field, checks exact source and rebuilt graph, then rejects invalid numeric DOM input while retaining the committed source and no stale signature. Convenience preview buttons and generic E07 results supply no BRG28 tier.",
    },
    BENCHMARK: {
        "tier": "benchmark", "sourcePath": NATIVE_PATH,
        "assertions": {"same_run_seven_dedicated_methods": "true",
                       "sixty_original_native_compile_samples": "true",
                       "small_large_embedded_fixture_hashes": "true",
                       "independently_recomputed_p50_p95": "true",
                       "actual_native_cache_deltas": "true",
                       "recorded_os_and_simulator_model": "true"},
        "scope": "Dedicated native Swift/C ABI benchmark records twenty actual compile durations each for a one-action source, a 200-action source and a 2MiB embedded asset across distinct unrelated source revisions, with exact fixture/sample hashes, OS/model, independently checked p50/p95 and cache reads/encodes/hits. It measures compilation, not WebKit rendering, Shortcut runtime, a latency budget or the separate physical iPad Debug/Release comparison.",
    },
}
SOURCE_PATHS = sorted({
    GATE_PATH, CONTRACT_PATH, NATIVE_PATH, UI_PATH,
    ".github/workflows/ios-build.yml", "ios/project.yml",
    "ios/scripts/collect_ios_test_evidence.py", "ios/scripts/preview_native_evidence.py",
    "ios/scripts/test_preview_native_evidence.py",
    "ios/CherriApp/App/WorkspaceView.swift",
    "ios/CherriApp/Compiler/CherriCompiler.swift",
    "ios/CherriApp/Compiler/CherriDocumentCompileContext.swift",
    "ios/CherriApp/Documents/CherriDocument.swift",
    "ios/CherriApp/Documents/CherriProjectAssetObserver.swift",
    "ios/CherriApp/Editor/CherriEditorBuildRequest.swift",
    "ios/CherriApp/Editor/CherriEditorView.swift",
    "ios/CherriApp/Preview/CherriPreviewEditTransaction.swift",
    "ios/CherriApp/Preview/CherriPreviewRenderState.swift",
    "ios/CherriApp/Preview/ShortcutPreviewView.swift",
    "ios/CherriApp/Preview/ShortcutPlistEditor.swift",
    "ios/CherriUITests/EditorStartupDeadline.swift",
    "ios/TestSupport/CherriJoinedContract.swift",
    "ios/WebPreview/src/main.ts", "ios/WebPreview/package.json", "ios/WebPreview/package-lock.json",
    "action.go", "actions_std.go", "build_profile.go", "copy_paste.go", "functions.go",
    "includes.go", "ios_bridge.go", "language_emit_adapter.go", "mobile_compiler.go",
    "mobile_document_context.go", "native_document_legacy.go", "parser.go",
    "preview_source_mapping.go", "shortcutgen.go", "internal/language/assets/cache.go",
})


class Rejected(ValueError):
    pass


def require(value, code):
    if not value:
        raise Rejected(code)


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def unique(pairs):
    value = {}
    for key, item in pairs:
        require(key not in value, "DUPLICATE_KEY")
        value[key] = item
    return value


def decode(raw, limit):
    require(type(raw) is bytes and 0 < len(raw) <= limit, "JSON_SIZE")
    try:
        return json.loads(raw.decode("utf-8"), object_pairs_hook=unique,
                          parse_constant=lambda _: (_ for _ in ()).throw(Rejected("NONFINITE")))
    except (UnicodeError, ValueError, TypeError) as error:
        raise Rejected("JSON_INVALID") from error


def expected_contract():
    source = '// BRG28 public source preserved\nshow("BRG28_KEEP")\nshow("BRG28_EDIT")\nwait(2)\n// שלום 🍒 preserve tail\n'
    updated = source.replace('show("BRG28_EDIT")', 'show("BRG28_CHANGED")')
    fixtures = [
        ("small", 'show("BRG28_SMALL")\n', 1, b""),
        ("large", "".join(f'show("BRG28_LARGE_{i}")\n' for i in range(200)), 200, b""),
        ("embedded-2mib", 'embedFile("payload.dat")\n', 0, b"c" * (2 * 1024 * 1024)),
    ]
    return {
        "schema": "cherri-native-preview-contract/1", "requirement": "BRG28",
        "originalGatePath": GATE_PATH, "originalGateSha256": GATE_SHA,
        "tiers": ["integration", "ios-ui", "benchmark"], "requiredMethods": METHODS,
        "genericTestInheritanceAllowed": False, "formalAcceptanceGrantedByContract": False,
        "physicalDebugBenchmarkIncluded": False,
        "cancellationScope": "pre-entry cooperative cancellation; no interruption inside a running C call",
        "ui": {
            "testId": UI, "source": source, "sourceSha256": digest(source.encode()),
            "updatedSource": updated, "updatedSourceSha256": digest(updated.encode()),
            "stages": ["actual-source-built", "source-and-native-rebuild-published",
                       "new-native-graph-rendered", "invalid-dom-number-preserved-source"],
            "checkpoint": CHECKPOINT,
        },
        "benchmark": {
            "testId": BENCHMARK, "sampleCount": 20, "sampleSuffix": "// measured unrelated revision {index}\n",
            "mode": "v2", "buildProfile": "debug",
            "scope": "actual-swift-cabi-compilation-no-render-or-shortcut-runtime",
            "percentileIndicesZeroBased": {"p50": 9, "p95": 18},
            "latencyBudget": None,
            "fixtures": [{"fixture": name, "source": text, "sourceSha256": digest(text.encode()),
                          "expectedShowActions": shows, "assetBytes": len(asset),
                          "assetSha256": digest(asset) if asset else ""}
                         for name, text, shows, asset in fixtures],
        },
    }


def load_contract(root):
    gate_path, contract_path = root / GATE_PATH, root / CONTRACT_PATH
    require(all(path.is_file() and not path.is_symlink() for path in (gate_path, contract_path)), "CONTRACT_FILE")
    gate = gate_path.read_bytes()
    require(digest(gate) == GATE_SHA, "ORIGINAL_GATE_BYTES")
    raw = contract_path.read_bytes()
    contract = decode(raw, 64 * 1024)
    require(contract == expected_contract()
            and raw == (json.dumps(expected_contract(), indent=2, ensure_ascii=False, sort_keys=True) + "\n").encode(),
            "CONTRACT_BINDING")
    return raw, contract


def finite_positive(value):
    return type(value) in (int, float) and math.isfinite(value) and value > 0


def validate_ui(value, contract, index):
    ui = contract["ui"]
    require(type(value) is dict and 0 <= index < 4, "UI_SHAPE")
    expected = {"schema": "cherri-native-preview-ui/1", "testId": UI,
                "stage": ui["stages"][index],
                "sourceSha256": ui["sourceSha256"] if index == 0 else ui["updatedSourceSha256"]}
    if index == 1:
        require(finite_positive(value.get("elapsedSeconds")), "UI_DURATION")
        expected["elapsedSeconds"] = value["elapsedSeconds"]
    require(value == expected, "UI_BINDING")
    return value


def validate_benchmark(value, contract, index):
    benchmark = contract["benchmark"]
    require(type(value) is dict and 0 <= index < 3, "BENCHMARK_SHAPE")
    fixture = benchmark["fixtures"][index]
    keys = {"schema", "fixture", "sourceSha256", "sampleSourceSha256", "assetSha256", "sampleCount",
            "totalCompileMilliseconds", "p50Milliseconds", "p95Milliseconds", "osVersion", "simulatorModel",
            "mode", "buildProfile", "cacheReadsDelta", "cacheEncodesDelta", "cacheHitsDelta", "scope"}
    require(set(value) == keys and value["schema"] == "cherri-native-preview-benchmark/1", "BENCHMARK_KEYS")
    require(all(value[key] == fixture[key] for key in ("fixture", "sourceSha256", "assetSha256"))
            and all(value[key] == benchmark[key] for key in ("mode", "buildProfile", "scope")), "BENCHMARK_FIXTURE")
    require(type(value["sampleCount"]) is int and value["sampleCount"] == 20, "SAMPLE_COUNT")
    expected_hashes = [digest((fixture["source"] + benchmark["sampleSuffix"].format(index=i)).encode())
                       for i in range(20)]
    require(value["sampleSourceSha256"] == expected_hashes, "SAMPLE_SOURCE_BYTES")
    timings = value["totalCompileMilliseconds"]
    require(type(timings) is list and len(timings) == 20 and all(finite_positive(x) for x in timings),
            "SAMPLE_TIMINGS")
    ordered = sorted(timings)
    require(type(value["p50Milliseconds"]) in (int, float) and type(value["p95Milliseconds"]) in (int, float)
            and value["p50Milliseconds"] == ordered[9] and value["p95Milliseconds"] == ordered[18],
            "PERCENTILES")
    require(type(value["osVersion"]) is str and re.fullmatch(r"Version [0-9.]{1,20} \(Build [A-Za-z0-9]{1,24}\)", value["osVersion"])
            and type(value["simulatorModel"]) is str
            and re.fullmatch(r"(iPhone|iPad)[0-9]{1,3},[0-9]{1,3}", value["simulatorModel"]),
            "RECORDED_PLATFORM")
    require(all(type(value[key]) is int and value[key] >= 0
                for key in ("cacheReadsDelta", "cacheEncodesDelta", "cacheHitsDelta")), "CACHE_NUMBERS")
    if fixture["assetBytes"]:
        require(value["cacheReadsDelta"] == value["cacheEncodesDelta"] == 1
                and value["cacheHitsDelta"] >= 19, "ASSET_CACHE")
    else:
        require(value["cacheReadsDelta"] == value["cacheEncodesDelta"] == value["cacheHitsDelta"] == 0,
                "NO_ASSET_CACHE")
    return value


class LogGuard:
    """Retain only known original safe lines; failures retain their own boundary."""
    def __init__(self, root):
        self.raw, self.contract = load_contract(root)
        self.active = None
        self.started, self.passed = set(), set()
        self.ui, self.benchmarks = [], []
        self.checkpoint = False
        self.rejections = []

    def reject(self, code):
        self.rejections.append(code)
        return REJECTED + code

    def observe(self, raw_line):
        line = raw_line.rstrip("\r")
        stripped = line.strip()
        if match := CASE.fullmatch(stripped):
            target, suite, method, state, status = match.groups()
            test = suite + "/" + method
            if state == "started.":
                if self.active in METHODS:
                    return self.reject("INTERLEAVED_CASE")
                self.active = test
                if test in METHODS:
                    expected = "CherriUITests" if test == UI else "CherriTests"
                    if target != expected or test in self.started:
                        self.active = None
                        return self.reject("CASE_IDENTITY")
                    self.started.add(test)
            else:
                if test in METHODS:
                    if self.active != test or test not in self.started or test in self.passed or status != "passed":
                        self.active = None
                        return self.reject("CASE_TERMINAL")
                    self.passed.add(test)
                self.active = None
            return None
        if stripped.startswith("Test Case '"):
            result = self.reject("UNSUPPORTED_CASE") if self.active in METHODS or "CherriPreviewDocument" in stripped else None
            self.active = None
            return result
        if stripped.startswith(REJECTED):
            return self.reject("UNTRUSTED_MARKER")
        if stripped.startswith(UI_PREFIX):
            try:
                require(line == stripped and self.active == UI and not self.checkpoint, "UI_OWNER")
                value = validate_ui(decode(line[len(UI_PREFIX):].encode(), 2048), self.contract, len(self.ui))
                self.ui.append(value)
                return line
            except (Rejected, KeyError, TypeError):
                return self.reject("INVALID_UI_EVENT")
        if stripped.startswith(BENCHMARK_PREFIX):
            try:
                require(line == stripped and self.active == BENCHMARK, "BENCHMARK_OWNER")
                value = validate_benchmark(decode(line[len(BENCHMARK_PREFIX):].encode(), 8192),
                                           self.contract, len(self.benchmarks))
                self.benchmarks.append(value)
                return line
            except (Rejected, KeyError, TypeError):
                return self.reject("INVALID_BENCHMARK")
        if stripped.startswith("CHERRI_NATIVE_PREVIEW_UI_CHECKPOINT"):
            if line != CHECKPOINT or self.active != UI or self.checkpoint or len(self.ui) != 4:
                return self.reject("INVALID_CHECKPOINT")
            self.checkpoint = True
            return line
        return None

    def verified(self):
        require(not self.rejections, "REJECTED_LOG")
        require(self.active not in METHODS and self.started == self.passed == set(METHODS), "COMPLETE_COHORT")
        require(len(self.ui) == 4 and self.checkpoint and len(self.benchmarks) == 3, "COMPLETE_OBSERVATIONS")
        return {"passedMethods": METHODS, "uiEvents": self.ui, "uiCheckpoint": CHECKPOINT,
                "benchmarkObservations": self.benchmarks}


def source_pins(root, implementation_sha):
    def git(*args):
        result = subprocess.run(["git", "--no-replace-objects", "-C", str(root), *args],
                                capture_output=True, timeout=15)
        require(result.returncode == 0, "GIT_PROCESS")
        return result.stdout
    require(git("rev-parse", "HEAD") == (implementation_sha + "\n").encode(), "SOURCE_HEAD")
    raw = git("ls-tree", "-r", "-z", implementation_sha, "--", *SOURCE_PATHS)
    entries = {}
    for entry in raw.split(b"\0"):
        if not entry:
            continue
        meta, name = entry.split(b"\t", 1)
        mode, kind, blob = meta.decode("ascii").split()
        name = name.decode("utf-8")
        require(name in SOURCE_PATHS and name not in entries and kind == "blob"
                and mode in ("100644", "100755"), "SOURCE_TREE")
        path = root / name
        require(path.is_file() and not path.is_symlink()
                and path.resolve().is_relative_to(root.resolve()), "SOURCE_PATH")
        actual_mode = "100755" if path.stat().st_mode & stat.S_IXUSR else "100644"
        data = path.read_bytes()
        require(0 < len(data) <= 2 * 1024 * 1024 and actual_mode == mode
                and hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest() == blob,
                "SOURCE_BYTES")
        entries[name] = {"path": name, "bytes": len(data), "sha256": digest(data),
                         "gitBlob": blob, "gitMode": mode}
    require(set(entries) == set(SOURCE_PATHS), "SOURCE_INVENTORY")
    return [entries[name] for name in SOURCE_PATHS]


def collect(root, log_bytes, test_results, implementation_sha, run_id, attempt,
            eligible, diagnostic_only):
    summary = {"status": "NOT_OBSERVED", "errorCodes": [], "recordTestIds": [],
               "noCrossTierInheritance": True}
    files, records = {}, []
    if not (b"CHERRI_NATIVE_PREVIEW_" in log_bytes
            or any(row.get("testId") in METHODS for row in test_results)):
        return summary, files, records
    summary["status"] = "NOT_VERIFIED"
    try:
        require(eligible and not diagnostic_only and re.fullmatch("[0-9a-f]{40}", implementation_sha)
                and re.fullmatch("[0-9]{1,24}", run_id), "RUN_NOT_ELIGIBLE")
        require(type(attempt) is int and 1 <= attempt <= 9999, "RUN_ATTEMPT")
        guard = LogGuard(root)
        for line in log_bytes.decode("utf-8").splitlines():
            guard.observe(line)
        observations = guard.verified()
        outcomes = {}
        for row in test_results:
            require(row.get("testId") not in outcomes and row.get("observedStatus") == "passed", "TEST_OUTCOME")
            outcomes[row["testId"]] = row["observedStatus"]
        require(all(outcomes.get(test) == "passed" for test in METHODS), "TEST_OUTCOME")
        pins = source_pins(root, implementation_sha)
        proof = {"schema": "cherri-native-preview-proof/1",
                 "implementationSha": implementation_sha, "runId": run_id, "runAttempt": attempt,
                 "xcodebuildExitCode": 0, "logSha256": digest(log_bytes),
                 "contractSha256": digest(guard.raw), "originalGateSha256": GATE_SHA,
                 "sourcePins": pins, "observations": observations, "noCrossTierInheritance": True}
        files["preview-document-contract.json"] = guard.raw
        files["preview-native-proof.json"] = (json.dumps(proof, sort_keys=True, indent=2) + "\n").encode()
        by_path = {row["path"]: row for row in pins}
        for test, profile in PROFILES.items():
            records.append({
                "evidence_id": "ev-ios-preview-" + test.split("/")[1],
                "requirements": ["BRG28"], "tier": profile["tier"],
                "implementation_sha": implementation_sha, "test_id": test,
                "fixture_sha256": by_path[profile["sourcePath"]]["sha256"],
                "command": ["xcodebuild", "test", "-scheme", "Cherri"],
                "exit_code": 0, "result": "passed", "assertions": dict(profile["assertions"]),
                "detail": profile["scope"],
                "artifacts": [{"path": ARTIFACT_BASE + "xcodetest.log",
                               "sha256": digest(log_bytes), "run_id": run_id}] + [
                    {"path": ARTIFACT_BASE + name, "sha256": digest(raw), "run_id": run_id}
                    for name, raw in sorted(files.items())],
            })
        summary.update(status="VERIFIED_NATIVE_PREVIEW", runAttempt=attempt,
                       contractSha256=digest(guard.raw), originalGateSha256=GATE_SHA,
                       proofSha256=digest(files["preview-native-proof.json"]),
                       verifiedTests=METHODS, recordTestIds=sorted(PROFILES))
    except (Rejected, OSError, ValueError, TypeError, KeyError, subprocess.SubprocessError) as error:
        summary["errorCodes"].append(str(error) if isinstance(error, Rejected) else "PREVIEW_SOURCE_OR_LOG_INVALID")
        files, records = {}, []
    return summary, files, records
