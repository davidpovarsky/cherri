"""Strict source-bound intake for the four dedicated joined native editor cases.

Only three records are emitted: BRG25 ios-build, BRG25 ios-ui and BRG27 ios-ui.
The BRG27 bridge case is required companion proof, never LSP/service evidence.
"""
import hashlib
import json
from pathlib import Path
import re
import subprocess

PREFIX = "[CHERRI_JOINED_CONTRACT] "
REJECTED = "[CHERRI_JOINED_REJECTED] "
CONTRACT_PATH = "ios/TestFixtures/joined-editor-contract.json"
BRG25_BRIDGE = "CherriJoinedBridgeTests/testBRG25ActualMobileRouteJoinedContract"
BRG27_BRIDGE = "CherriJoinedBridgeTests/testBRG27ActualEditorBridgeJoinedContract"
BRG25_UI = "CherriJoinedContractUITests/testBRG25ActualAppRejectedAndLiteralRoutesUI"
BRG27_UI = "CherriJoinedContractUITests/testBRG27ActualEditorJoinedWorkflowUI"
BRIDGE_PATH = "ios/CherriTests/CherriJoinedBridgeTests.swift"
UI_PATH = "ios/CherriUITests/CherriJoinedContractUITests.swift"
PROFILES = {
    BRG25_BRIDGE: {
        "requirement": "BRG25", "producer": "ios-bridge", "tier": "ios-build",
        "checkpoint": "BRG25_NATIVE_MOBILE_JOINED_VERIFIED", "sourcePath": BRIDGE_PATH,
        "assertions": {"actual_mobile_entrypoints": "true", "literal_v2_auto_parity": "true",
                       "all_mobile_failure_routes_rejected": "true",
                       "no_plist_or_signature_on_failure": "true", "exact_utf16_diagnostics": "true"},
        "scope": "Dedicated hosted Swift test executes actual unsigned and signed mobile entrypoints in explicit v2 and auto, preserves three exact literals and rejects both invalid fixtures with exact UTF16 diagnostics and no plist or signature. CLI, Legacy invocation counters and actual UI require separate records.",
    },
    BRG27_BRIDGE: {
        "requirement": "BRG27", "producer": "ios-bridge", "tier": None,
        "checkpoint": "BRG27_NATIVE_EDITOR_BRIDGE_JOINED_VERIFIED", "sourcePath": BRIDGE_PATH,
        "assertions": {},
        "scope": "Auxiliary native bridge proof only; it grants no service, integration, LSP or UI tier.",
    },
    BRG25_UI: {
        "requirement": "BRG25", "producer": "ios-ui", "tier": "ios-ui",
        "checkpoint": "BRG25_NATIVE_APP_ROUTES_JOINED_VERIFIED", "sourcePath": UI_PATH,
        "assertions": {"actual_app_build_sign_controls": "true", "literal_app_builds": "true",
                       "current_invalid_build_sign_rejected": "true",
                       "no_compiled_or_signed_artifact_on_failure": "true",
                       "exact_rendered_utf16_diagnostics": "true"},
        "scope": "Dedicated XCUITest opens and clears one real document, types three literal fixtures, uses actual Build and Sign controls, observes both exact diagnostic ranges and verifies invalid current source yields no compiled or signed artifact. Mobile and CLI route proofs remain separate.",
    },
    BRG27_UI: {
        "requirement": "BRG27", "producer": "ios-ui", "tier": "ios-ui",
        "checkpoint": "BRG27_NATIVE_EDITOR_UI_JOINED_VERIFIED", "sourcePath": UI_PATH,
        "assertions": {"actual_editor_joined_workflow_ui": "true", "shared_textedit_applied_ui": "true",
                       "current_diagnostics_exact_utf16_ui": "true", "current_revision_only": "true",
                       "hover_rename_format_ui": "true", "same_run_native_bridge_proof": "true",
                       "comments_literals_preserved": "true"},
        "scope": "Dedicated XCUITest drives current Unicode diagnostics, shared parameter and enum completion edits, rapid edits, Hover, Rename, Format and unsigned Build on an opened document; same-run dedicated native bridge proof supplies exact stale-version and request-identity controls. Actual LSP and performance tiers remain separate.",
    },
}
SOURCE_PATHS = sorted({
    CONTRACT_PATH, BRIDGE_PATH, UI_PATH, "ios/TestSupport/CherriJoinedContract.swift",
    "ios/CherriUITests/EditorStartupDeadline.swift", "ios/project.yml",
    "ios/CherriApp/App/WorkspaceView.swift", "ios/CherriApp/Compiler/CherriCompiler.swift",
    "ios/CherriApp/Documents/CherriDocument.swift", "ios/CherriApp/Editor/CherriCompletionRequest.swift",
    "ios/CherriApp/Editor/CherriEditorActionRequest.swift", "ios/CherriApp/Editor/CherriEditorView.swift",
    "ios/scripts/collect_ios_test_evidence.py", "ios/scripts/joined_editor_evidence.py",
    ".github/workflows/ios-build.yml", "ios_bridge.go", "mobile_compiler.go",
    "mobile_language_service.go", "language_cli_adapter.go",
})
CASE = re.compile(r"^Test Case '-\[([A-Za-z0-9_]+)\.([A-Za-z0-9_]+) (test[A-Za-z0-9_]+)\]' (started\.|(passed|failed) \([0-9.]+ seconds\)\.)$")
HEX = re.compile(r"^[0-9a-f]{64}$")
ARTIFACT_BASE = "artifacts/backend-recovery/ci-artifacts/ios-build/ios-build-evidence/"


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
    require(isinstance(raw, bytes) and 0 < len(raw) <= limit, "JSON_SIZE")
    try:
        return json.loads(raw.decode("utf-8"), object_pairs_hook=unique,
                          parse_constant=lambda _: (_ for _ in ()).throw(Rejected("NONFINITE")))
    except (UnicodeError, ValueError, TypeError) as error:
        raise Rejected("JSON_INVALID") from error


def load_contract(root):
    path = root / CONTRACT_PATH
    require(path.is_file() and not path.is_symlink(), "CONTRACT_FILE")
    raw = path.read_bytes()
    contract = decode(raw, 128 * 1024)
    require(type(contract) is dict and contract.get("schema") == "cherri-joined-editor-contract/1"
            and contract.get("eventSchema") == "cherri-ios-joined-contract-event/1"
            and contract.get("eventPrefix") == PREFIX
            and contract.get("genericTestInheritanceAllowed") is False
            and contract.get("formalAcceptanceGrantedByContract") is False
            and contract.get("positionEncoding") == "zero-based-UTF16", "CONTRACT_SCHEMA")
    methods, fixtures = contract.get("methods"), contract.get("fixtures")
    require(type(methods) is dict and set(methods) == set(PROFILES)
            and type(fixtures) is dict and 1 <= len(fixtures) <= 64, "CONTRACT_INVENTORY")
    for fixture in fixtures.values():
        require(type(fixture) is dict, "FIXTURE_SHAPE")
        for field in ("source", "expectedSource"):
            value = fixture.get(field)
            require(type(value) is str and len(value.encode("utf-8")) <= 16384
                    and digest(value.encode("utf-8")) == fixture.get(field + "Sha256"), "FIXTURE_HASH")
    for test, profile in PROFILES.items():
        method = methods[test]
        require(type(method) is dict and all(method.get(k) == profile[k]
                for k in ("requirement", "producer", "checkpoint")), "METHOD_IDENTITY")
        stages = method.get("stages")
        require(type(stages) is list and 1 <= len(stages) <= 32, "STAGE_COUNT")
        names = set()
        for stage in stages:
            require(type(stage) is dict and set(stage) == {"id", "fixtureId", "observedSourceField"}
                    and type(stage["id"]) is str and re.fullmatch("[a-z0-9-]{1,96}", stage["id"])
                    and stage["id"] not in names and stage["fixtureId"] in fixtures
                    and stage["observedSourceField"] in ("source", "expectedSource"), "STAGE_SHAPE")
            names.add(stage["id"])
    return raw, contract


def expected_event(contract, contract_sha, test, sequence):
    method = contract["methods"][test]
    stages = method["stages"]
    require(type(sequence) is int and 0 <= sequence < len(stages), "STAGE_SEQUENCE")
    stage = stages[sequence]
    fixture = contract["fixtures"][stage["fixtureId"]]
    return {"schema": contract["eventSchema"], "testId": test,
            "requirement": method["requirement"], "producer": method["producer"],
            "stage": stage["id"], "sequence": sequence, "fixtureId": stage["fixtureId"],
            "inputSha256": fixture["sourceSha256"],
            "observedSourceSha256": fixture[stage["observedSourceField"] + "Sha256"],
            "contractSha256": contract_sha}


class LogGuard:
    """Publish only original, exact known event lines; reject with fixed tokens."""
    def __init__(self, root):
        self.raw, self.contract = load_contract(root)
        self.contract_sha = digest(self.raw)
        self.active = None
        self.started = set()
        self.passed = set()
        self.events = {}
        self.checkpoints = {}
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
                if self.active in PROFILES:
                    return self.reject("INTERLEAVED_CASE")
                self.active = test
                if test in PROFILES:
                    expected_target = "CherriTests" if PROFILES[test]["producer"] == "ios-bridge" else "CherriUITests"
                    if target != expected_target or test in self.started:
                        self.active = None
                        return self.reject("CASE_IDENTITY")
                    self.started.add(test)
                    self.events[test] = []
            else:
                if test in PROFILES:
                    expected_target = "CherriTests" if PROFILES[test]["producer"] == "ios-bridge" else "CherriUITests"
                    if target != expected_target or self.active != test or test not in self.started or status != "passed" or test in self.passed:
                        # A failed test still ends its actual ownership interval.
                        # Keep the rejection while allowing later cases to retain
                        # their independent original diagnostic boundaries.
                        self.active = None
                        return self.reject("CASE_TERMINAL")
                    self.passed.add(test)
                self.active = None
            return None
        if stripped.startswith("Test Case '"):
            result = self.reject("UNSUPPORTED_CASE") if self.active in PROFILES or "CherriJoined" in stripped else None
            self.active = None
            return result
        if stripped.startswith(REJECTED):
            return self.reject("UNTRUSTED_MARKER")
        if stripped.startswith(PREFIX):
            try:
                require(line == stripped and self.active in PROFILES, "EVENT_OWNER")
                event = decode(line[len(PREFIX):].encode(), 4096)
                expected = expected_event(self.contract, self.contract_sha, self.active,
                                          len(self.events[self.active]))
                require(type(event) is dict and type(event.get("sequence")) is int
                        and event == expected and not self.checkpoints.get(self.active), "EVENT_BINDING")
                self.events[self.active].append(event)
                return line
            except (Rejected, KeyError, TypeError):
                return self.reject("INVALID_EVENT")
        if stripped.startswith(("[CHECKPOINT:BRG25_NATIVE_", "[CHECKPOINT:BRG27_NATIVE_")):
            if self.active not in PROFILES or self.active not in self.events:
                return self.reject("CHECKPOINT_OWNER")
            checkpoint = "[CHECKPOINT:" + PROFILES[self.active]["checkpoint"] + "]"
            if (line != checkpoint or self.checkpoints.get(self.active)
                    or len(self.events[self.active]) != len(self.contract["methods"][self.active]["stages"])):
                return self.reject("INVALID_CHECKPOINT")
            self.checkpoints[self.active] = 1
            return line
        return None

    def verified(self):
        require(not self.rejections, "REJECTED_LOG")
        require(self.active not in PROFILES, "INCOMPLETE_CASE")
        methods = {}
        for test in self.started:
            require(test in self.passed and self.checkpoints.get(test) == 1
                    and len(self.events[test]) == len(self.contract["methods"][test]["stages"]), "INCOMPLETE_METHOD")
            methods[test] = {"testId": test, "producer": PROFILES[test]["producer"],
                             "requirement": PROFILES[test]["requirement"], "status": "passed",
                             "checkpoint": "[CHECKPOINT:" + PROFILES[test]["checkpoint"] + "]",
                             "events": self.events[test]}
        return methods


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
        data = path.read_bytes()
        require(0 < len(data) <= 2 * 1024 * 1024
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
    if not (PREFIX.encode() in log_bytes or REJECTED.encode() in log_bytes
            or any(row.get("testId") in PROFILES for row in test_results)):
        return summary, files, records
    summary["status"] = "NOT_VERIFIED"
    try:
        require(eligible and not diagnostic_only and re.fullmatch("[0-9a-f]{40}", implementation_sha)
                and re.fullmatch("[0-9]{1,24}", run_id), "RUN_NOT_ELIGIBLE")
        require(type(attempt) is int and 1 <= attempt <= 9999, "RUN_ATTEMPT")
        guard = LogGuard(root)
        for line in log_bytes.decode("utf-8").splitlines():
            guard.observe(line)
        methods = guard.verified()
        require(methods, "NO_JOINED_METHOD")
        outcomes = {}
        for row in test_results:
            require(row.get("testId") not in outcomes and row.get("observedStatus") == "passed", "TEST_OUTCOME")
            outcomes[row["testId"]] = row["observedStatus"]
        require(all(outcomes.get(test) == "passed" for test in methods), "TEST_OUTCOME")
        pins = source_pins(root, implementation_sha)
        proof = {"schema": "cherri-joined-native-editor-proof/1",
                 "implementationSha": implementation_sha, "runId": run_id, "runAttempt": attempt,
                 "xcodebuildExitCode": 0, "logSha256": digest(log_bytes),
                 "contractSha256": guard.contract_sha, "sourcePins": pins, "methods": methods,
                 "noCrossTierInheritance": True}
        files["joined-editor-contract.json"] = guard.raw
        files["joined-editor-proof.json"] = (json.dumps(proof, sort_keys=True, indent=2) + "\n").encode()
        by_path = {row["path"]: row for row in pins}
        for test, profile in PROFILES.items():
            if not profile["tier"] or test not in methods:
                continue
            if test == BRG27_UI and BRG27_BRIDGE not in methods:
                continue
            records.append({
                "evidence_id": "ev-ios-joined-" + test.split("/")[1],
                "requirements": [profile["requirement"]], "tier": profile["tier"],
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
        summary.update(status="VERIFIED_JOINED_NATIVE", runAttempt=attempt,
                       contractSha256=guard.contract_sha,
                       proofSha256=digest(files["joined-editor-proof.json"]),
                       verifiedTests=sorted(methods),
                       recordTestIds=sorted(row["test_id"] for row in records))
    except (Rejected, OSError, ValueError, TypeError, KeyError, subprocess.SubprocessError) as error:
        summary["errorCodes"].append(str(error) if isinstance(error, Rejected) else "JOINED_SOURCE_OR_LOG_INVALID")
        files, records = {}, []
    return summary, files, records
