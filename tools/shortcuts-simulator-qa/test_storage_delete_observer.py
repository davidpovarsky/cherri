"""Synthetic boundary controls only; never an Apple execution producer."""
import json
from pathlib import Path
import tempfile
import unittest

import storage_delete_observer as subject


def plan():
    return {"schema": "cherri-storage-native-observer-plan/1", "repo": "/owned/repo", "root": "/owned/evidence",
            "controllerCommitSha": "1" * 40, "compilerCommitSha": subject.COMPILER,
            "builtHarnessCommitSha": subject.BUILD, "buildRoot": "/owned/build", "buildQualificationSHA256": "2" * 64,
            "runId": "storage-controlled", "runAttempt": 1, "runNonce": "storage-unique-nonce",
            "deviceUdid": subject.native.DEVICE}


def trace():
    def collector(stage):
        return {"schemaVersion": "1.4", "kind": "COLLECTOR_EVENT", "stage": stage, "runId": subject.FIXTURE,
                "traceTransport": "ndjson-stream", "collectorSentinel": "===CHERRI_QA_TRACE_NDJSON_V14==="}
    def inline(stage, step, action):
        return {"schemaVersion": "1.4", "kind": "INLINE_DEBUG_ACTION", "stage": stage, "label": "delete",
                "stepSequence": step, "actionIdentifier": "is.workflow.actions." + action,
                "timestampPrecision": "apple-currentdate", "valueType": "UNKNOWN_AT_TRACE_EVENT",
                "timestamp": "2026-11-25T00:00:00Z"}
    return [collector("collector_begin"), inline("before", 3, "setvariable"),
            inline("after", 24, "comment"), collector("collector_end")]


def callback(rows=None):
    return {"receiverSelfTest": False, "eventSource": "shortcuts-x-callback", "runId": "storage-unique-nonce",
            "expectedShortcutName": subject.FIXTURE, "fixtureId": subject.FIXTURE, "status": "SUCCESS",
            "resultSummary": subject.EXPECTED, "tracePayload": "\n".join(json.dumps(r) for r in (trace() if rows is None else rows))}


def import_receipts():
    argv = ["xcrun", "simctl", "openurl", subject.native.DEVICE, "file:///owned/qa-storage-delete-v2.shortcut"]
    dispatch = {"command": argv, "outcome": "EXITED", "timedOut": False, "exitCode": 0,
                "processReturncode": 0, "parentReaped": True, "timeoutSeconds": 60}
    receipt = {"id": "import-unique", "operation": "import", "outcome": "IMPORTED", "name": subject.FIXTURE,
               "identityVerified": True, "libraryIdentityVerified": True,
               "importTiming": {"contract": "dispatch-exit-anchored-add-v1", "dispatchState": "EXITED",
                                "dispatchReceipt": "dispatch-import-unique.json",
                                "sheetContract": "requested-identity-and-add-v1", "sheetButtonExists": True,
                                "sheetButtonHittable": True, "sheetIdentityMatched": True, "sheetCanTap": True,
                                "sheetSelectionContract": "current-container-bound-add-v1",
                                "sheetSelectionReason": "UNIQUE_CURRENT_NAMED_CONTAINER",
                                "sheetQueryCandidateCount": 3, "sheetCurrentContainerCount": 1,
                                "sheetMatchedCandidateCount": 1, "sheetSelectedQueryIndex": 2,
                                "sheetSelectedContainerPath": [0, 0], "sheetSelectedTitlePath": [0, 0, 0],
                                "sheetSelectedButtonPath": [0, 0, 2],
                                "addObservedUTC": "2026-11-25T00:00:01.000Z",
                                "lastSheetCheckUTC": "2026-11-25T00:00:02.000Z",
                                "sheetMatchedAfterCaptureUTC": "2026-11-25T00:00:02.001Z"}}
    return argv, dispatch, receipt


class StorageObserverControls(unittest.TestCase):
    def reject(self, function, *args):
        with self.assertRaises((ValueError, TypeError, KeyError)):
            function(*args)

    def test_exact_plan(self):
        self.assertEqual(subject.validate_plan(plan()), plan())

    def test_plan_fixed_identities(self):
        for key, value in (("schema", "other"), ("compilerCommitSha", "1" * 40),
                           ("builtHarnessCommitSha", "3" * 40), ("deviceUdid", "foreign-device")):
            with self.subTest(key=key):
                p = plan(); p[key] = value
                self.reject(subject.validate_plan, p)

    def test_plan_closed_keys_and_replay(self):
        for key, value in (("fallback", True), ("runtimeDispatchCount", 0), ("runAttempt", 2), ("runAttempt", True)):
            with self.subTest(key=key, value=value):
                p = plan(); p[key] = value
                self.reject(subject.validate_plan, p)

    def test_plan_hashes_nonce_and_paths(self):
        for key, value in (("controllerCommitSha", "A" * 40), ("buildQualificationSHA256", "2" * 63),
                           ("runNonce", ""), ("runNonce", "a" * 129), ("runId", "wrong/name"),
                           ("root", "relative"), ("root", "/owned/../elsewhere"), ("root", "/owned/repo")):
            with self.subTest(key=key, value=value):
                p = plan(); p[key] = value
                self.reject(subject.validate_plan, p)

    def test_original_import_positive(self):
        argv, dispatch, receipt = import_receipts()
        subject.validate_import(dispatch, receipt, argv, "import-unique")

    def test_import_requires_actual_process_exit(self):
        for key, value in (("exitCode", False), ("exitCode", 1), ("processReturncode", False),
                           ("processReturncode", 1), ("outcome", "LAUNCHED"), ("timedOut", True),
                           ("parentReaped", False), ("timeoutSeconds", 61)):
            with self.subTest(key=key, value=value):
                argv, dispatch, receipt = import_receipts(); dispatch[key] = value
                self.reject(subject.validate_import, dispatch, receipt, argv, "import-unique")

    def test_import_requires_exact_argv_and_card(self):
        argv, dispatch, receipt = import_receipts()
        dispatch["command"] = ["/usr/bin/xcrun", *argv[1:]]
        self.reject(subject.validate_import, dispatch, receipt, argv, "import-unique")
        for key, value in (("name", "other"), ("id", "wrong"), ("operation", "runtime"),
                           ("identityVerified", False), ("libraryIdentityVerified", False), ("outcome", "OPENED")):
            with self.subTest(key=key):
                argv, dispatch, receipt = import_receipts(); receipt[key] = value
                self.reject(subject.validate_import, dispatch, receipt, argv, "import-unique")

    def test_import_requires_original_timing_join(self):
        for key, value in (("contract", "other"), ("dispatchState", "PENDING"), ("dispatchReceipt", "unrelated.json")):
            with self.subTest(key=key):
                argv, dispatch, receipt = import_receipts(); receipt["importTiming"][key] = value
                self.reject(subject.validate_import, dispatch, receipt, argv, "import-unique")

    def test_exact_four_source_derived_events(self):
        self.assertEqual(subject.exact_storage_trace(callback(), "storage-unique-nonce"), trace())

    def test_callback_identity_and_failure_preserved(self):
        for key, value in (("receiverSelfTest", True), ("eventSource", "self-test"), ("runId", "old-nonce"),
                           ("expectedShortcutName", "other"), ("fixtureId", "other"), ("status", "ERROR"),
                           ("resultSummary", "CHERRI_QA_STORAGE_DELETE_RESULT:BEFORE_OK:VALUE_REMAINS"),
                           ("resultSummary", subject.EXPECTED + " trailing")):
            with self.subTest(key=key):
                c = callback(); c[key] = value
                self.reject(subject.exact_storage_trace, c, "storage-unique-nonce")

    def test_missing_extra_and_reordered_trace(self):
        for rows in (trace()[:3], trace() + [trace()[2]], list(reversed(trace())),
                     [trace()[0], trace()[2], trace()[1], trace()[3]]):
            with self.subTest(rows=rows):
                self.reject(subject.exact_storage_trace, callback(rows), "storage-unique-nonce")

    def test_collector_exact_fields(self):
        for index in (0, 3):
            for key in trace()[index]:
                with self.subTest(index=index, key=key):
                    rows = trace(); rows[index][key] = "wrong"
                    self.reject(subject.exact_storage_trace, callback(rows), "storage-unique-nonce")

    def test_inline_static_fields_and_integer_type(self):
        for index in (1, 2):
            for key in trace()[index]:
                if key == "timestamp":
                    continue
                with self.subTest(index=index, key=key):
                    rows = trace(); rows[index][key] = 3.0 if key == "stepSequence" else "wrong"
                    self.reject(subject.exact_storage_trace, callback(rows), "storage-unique-nonce")

    def test_actual_timestamp_required(self):
        for index in (1, 2):
            for stamp in (None, "", "  ", "\ufffc"):
                with self.subTest(index=index, stamp=stamp):
                    rows = trace(); rows[index]["timestamp"] = stamp
                    self.reject(subject.exact_storage_trace, callback(rows), "storage-unique-nonce")

    def test_duplicate_json_key_rejected(self):
        c = callback(); c["tracePayload"] = c["tracePayload"].replace('"stage": "before"', '"stage": "before", "stage": "before"')
        self.reject(subject.exact_storage_trace, c, "storage-unique-nonce")

    def test_extra_inline_or_collector_field_rejected(self):
        for index in range(4):
            with self.subTest(index=index):
                rows = trace(); rows[index]["invented"] = True
                self.reject(subject.exact_storage_trace, callback(rows), "storage-unique-nonce")

    def test_packet_corruption_rejected_before_native(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            p = root / "builds/qa-storage-delete-v2.cherri"; p.parent.mkdir(); p.write_bytes(b"corrupt source")
            self.reject(subject.packet_inventory, root)

    def test_reviewed_cleanup_and_process_methods_are_reused(self):
        for method in ("capture", "start_observer", "finish", "check_capacity", "start_capacity_watch"):
            with self.subTest(method=method):
                self.assertIs(getattr(subject.StorageObservation, method), getattr(subject.native.NativeObservation, method))

    def test_original_fixture_and_callback_parser_are_pinned(self):
        root = Path(subject.__file__).resolve().parents[2]
        fixture = root / "tools/shortcuts-simulator-qa/fixtures/qa-storage-delete-v2.cherri"
        self.assertEqual(subject.sha(fixture.read_bytes()), subject.PACKET_PINS["builds/qa-storage-delete-v2.cherri"])
        shell = (root / "scripts/ios27_shortcuts_simulator_qa.sh").read_text()
        marker = shell.index('      RUNTIME_EVAL="$(python3 - ')
        begin = shell.index("<<'PY'\n", marker) + len("<<'PY'\n")
        self.assertEqual(subject.sha(shell[begin:shell.index("\nPY\n", begin)].encode()), subject.RUNTIME_EVALUATOR_SHA256)

    def test_fresh_sheet_witnesses_required(self):
        for name, value in (("sheetContract", "old"), ("sheetButtonExists", False),
                            ("sheetButtonHittable", False), ("sheetIdentityMatched", False),
                            ("sheetCanTap", 1), ("sheetMatchedAfterCaptureUTC", None),
                            ("lastSheetCheckUTC", "not-a-timeZ"),
                            ("lastSheetCheckUTC", "2026-11-25T00:00:00.000Z"),
                            ("sheetMatchedAfterCaptureUTC", "2026-11-25T00:00:01.999Z")):
            with self.subTest(name=name, value=value):
                argv, dispatch, receipt = import_receipts()
                receipt["importTiming"][name] = value
                self.reject(subject.validate_import, dispatch, receipt, argv, "import-unique")

    def test_exact_clean_build_metadata(self):
        build, qualification = clean_build_metadata()
        subject.validate_clean_build_metadata(build, qualification)

    def test_unqualified_or_legacy_build_cannot_supply_new_harness(self):
        mutations = (("schema", "other"), ("status", "SOURCE_UNVERIFIED"),
                     ("builtHarnessCommit", "7d129d03c564a96bd6b97eeace8480b9bfa053e0"),
                     ("builtHarnessTree", "0" * 40), ("error", {}), ("sourceGuardError", {}),
                     ("sourceUnchanged", False), ("generatedProjectRetained", False),
                     ("compilerBuilt", True), ("shortcutExecuted", True), ("testsExecuted", True))
        for key, value in mutations:
            with self.subTest(key=key):
                build, qualification = clean_build_metadata()
                build[key] = value
                self.reject(subject.validate_clean_build_metadata, build, qualification)

    def test_clean_qualification_requires_source_and_no_reexecution(self):
        mutations = (("schema", "cherri-standalone-storage-harness-build-qualification/1"),
                     ("status", "SOURCE_UNVERIFIED"), ("builtHarnessCommit", "0" * 40),
                     ("builtHarnessTree", "0" * 40), ("sourceTrackedBytesUnchanged", False),
                     ("sourceCleanAfterRetainedProjectRelocation", False),
                     ("xcodeBuildReexecuted", True), ("compilerRebuilt", True), ("nativeInvocation", True))
        for key, value in mutations:
            with self.subTest(key=key):
                build, qualification = clean_build_metadata()
                qualification[key] = value
                self.reject(subject.validate_clean_build_metadata, build, qualification)

    def test_current_container_selection_positive_and_paths(self):
        argv, dispatch, receipt = import_receipts()
        subject.validate_import(dispatch, receipt, argv, "import-unique")
        timing = receipt["importTiming"]
        timing["sheetSelectedContainerPath"] = [0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0]
        timing["sheetSelectedTitlePath"] = timing["sheetSelectedContainerPath"] + [0]
        timing["sheetSelectedButtonPath"] = timing["sheetSelectedContainerPath"] + [6]
        subject.validate_import(dispatch, receipt, argv, "import-unique")

    def test_current_container_selection_contract_required(self):
        for field, value in (("sheetSelectionContract", "old"),
                             ("sheetSelectionContract", None),
                             ("sheetSelectionReason", "NO_CURRENT_HITTABLE_MATCH"),
                             ("sheetSelectionReason", "AMBIGUOUS_CURRENT_MATCH"),
                             ("sheetSelectionReason", True)):
            with self.subTest(field=field, value=value):
                argv, dispatch, receipt = import_receipts()
                receipt["importTiming"][field] = value
                self.reject(subject.validate_import, dispatch, receipt, argv, "import-unique")

    def test_current_container_selection_integer_bounds(self):
        values = {
            "sheetQueryCandidateCount": (0, 33, True, 3.0, "3"),
            "sheetCurrentContainerCount": (0, 8193, True, 1.0),
            "sheetMatchedCandidateCount": (0, 2, True, 1.0),
            "sheetSelectedQueryIndex": (-1, 3, True, 2.0),
        }
        for field, bad in values.items():
            for value in bad:
                with self.subTest(field=field, value=value):
                    argv, dispatch, receipt = import_receipts()
                    receipt["importTiming"][field] = value
                    self.reject(subject.validate_import, dispatch, receipt, argv, "import-unique")

    def test_current_container_selection_path_shape(self):
        for field in ("sheetSelectedContainerPath", "sheetSelectedTitlePath", "sheetSelectedButtonPath"):
            for value in (None, "", [], [True], [-1], [8192], [0] * 65, [0.0], [0, "1"]):
                with self.subTest(field=field, value=value):
                    argv, dispatch, receipt = import_receipts()
                    receipt["importTiming"][field] = value
                    self.reject(subject.validate_import, dispatch, receipt, argv, "import-unique")

    def test_current_container_selection_cross_sheet_rejected(self):
        for field, value in (("sheetSelectedContainerPath", [1, 0]),
                             ("sheetSelectedTitlePath", [0, 1, 0]),
                             ("sheetSelectedButtonPath", [0, 1, 2]),
                             ("sheetSelectedTitlePath", [0, 0]),
                             ("sheetSelectedButtonPath", [0, 0]),
                             ("sheetSelectedButtonPath", [0, 0, 0]),
                             ("sheetSelectedTitlePath", [0, 0, 2, 1])):
            with self.subTest(field=field, value=value):
                argv, dispatch, receipt = import_receipts()
                receipt["importTiming"][field] = value
                self.reject(subject.validate_import, dispatch, receipt, argv, "import-unique")

    def test_current_container_selection_evidence_complete(self):
        for field in ("sheetSelectionContract", "sheetSelectionReason", "sheetQueryCandidateCount",
                      "sheetCurrentContainerCount", "sheetMatchedCandidateCount", "sheetSelectedQueryIndex",
                      "sheetSelectedContainerPath", "sheetSelectedTitlePath", "sheetSelectedButtonPath"):
            with self.subTest(field=field):
                argv, dispatch, receipt = import_receipts()
                del receipt["importTiming"][field]
                self.reject(subject.validate_import, dispatch, receipt, argv, "import-unique")

    def test_previous_hidden_first_match_build_rejected(self):
        build, qualification = clean_build_metadata()
        for document in (build, qualification):
            document["builtHarnessCommit"] = "43b962d4dc07e9f8e94c8d4394af95abdf12ebe7"
            document["builtHarnessTree"] = "029ff3f51ce6cbaadb5fe41a29e12ccfee13b4b8"
        self.reject(subject.validate_clean_build_metadata, build, qualification)



    def test_scoped_build_truthful_metadata(self):
        for document in ("build", "qualification"):
            for field, value in (("completeRepositoryInventory", True), ("sourceScope", "full-repository"),
                                 ("sourceFiles", 21), ("sourceFiles", 22.0), ("projectInputCount", 13),
                                 ("projectInputCount", True)):
                with self.subTest(document=document, field=field, value=value):
                    build, qualification = clean_build_metadata()
                    (build if document == "build" else qualification)[field] = value
                    self.reject(subject.validate_clean_build_metadata, build, qualification)
        for field, value in (("completeTrackedProject", False), ("scopePaths", [subject.PROJECT]),
                             ("scopePaths", list(reversed(subject.BUILD_SCOPE_PATHS)))):
            with self.subTest(field=field):
                build, qualification = clean_build_metadata(); qualification[field] = value
                self.reject(subject.validate_clean_build_metadata, build, qualification)

    def test_scoped_source_exact_closure_positive(self):
        source = scoped_source()
        self.assertEqual(subject.validate_build_source(source), source["files"])
        self.assertEqual(len(source["files"]), 22)
        self.assertEqual(sum(path.startswith(subject.PROJECT) for path in source["files"]), 14)

    def test_scoped_source_missing_or_added_dependency(self):
        for path in subject.BUILD_INPUT_FILES:
            with self.subTest(path=path):
                source = scoped_source(); del source["files"][path]
                self.reject(subject.validate_build_source, source)
        source = scoped_source()
        source["files"][subject.PROJECT + "unreviewed.swift"] = next(iter(source["files"].values()))
        self.reject(subject.validate_build_source, source)

    def test_scoped_source_identity_and_whole_repo_relabel_rejected(self):
        for field, value in (("commit", "0" * 40), ("tree", "0" * 40),
                             ("completeTrackedSource", True), ("completeTrackedProject", False),
                             ("sourceScope", "full-repository"), ("scopePaths", [subject.PROJECT])):
            with self.subTest(field=field):
                source = scoped_source(); source[field] = value
                self.reject(subject.validate_build_source, source)

    def test_scoped_source_pin_corruption_rejected(self):
        for field, value in (("bytes", -1), ("bytes", True), ("gitBlob", "0" * 39),
                             ("sha256", "A" * 64), ("mode", "120000"), ("extra", "unbound")):
            with self.subTest(field=field):
                source = scoped_source()
                source["files"][subject.BUILD_PROJECT_FILES[0]][field] = value
                self.reject(subject.validate_build_source, source)


    def test_pre_navigation_and_failed_harness_are_rejected(self):
        previous = (
            ("bb9dada0a80c0ef7e83c4097ac2bef5f743541a0", "145b85d8063be775bc44e64fe50fc71e4fd4b352"),
            ("cfd43b3626823dd78d507c8972541bf7000e1786", "f343466ed12c70c2d87f137427dfa963743f23fb"),
        )
        for commit, tree in previous:
            for surface in ("plan", "metadata", "source"):
                with self.subTest(commit=commit, surface=surface):
                    if surface == "plan":
                        value = plan()
                        value["builtHarnessCommitSha"] = commit
                        self.reject(subject.validate_plan, value)
                    elif surface == "metadata":
                        build, qualification = clean_build_metadata()
                        for document in (build, qualification):
                            document["builtHarnessCommit"] = commit
                            document["builtHarnessTree"] = tree
                        self.reject(subject.validate_clean_build_metadata, build, qualification)
                    else:
                        value = scoped_source()
                        value.update(commit=commit, tree=tree)
                        self.reject(subject.validate_build_source, value)


def clean_build_metadata():
    # Synthetic metadata controls; full raw source/process/product joins are
    # additionally required by verify_build before any native installation.
    build = {"schema": "cherri-standalone-storage-harness-build/2", "status": "BUILD_VALIDATED",
             "builtHarnessCommit": subject.BUILD, "builtHarnessTree": subject.BUILD_TREE,
             "sourceUnchanged": True, "generatedProjectRetained": True,
             "compilerBuilt": False, "shortcutExecuted": False, "testsExecuted": False,
             "completeRepositoryInventory": False, "sourceScope": subject.BUILD_SCOPE,
             "sourceFiles": 22, "projectInputCount": 14}
    qualification = {"schema": "cherri-standalone-storage-harness-build-qualification/3",
                     "status": "BUILD_VALIDATED", "builtHarnessCommit": subject.BUILD,
                     "builtHarnessTree": subject.BUILD_TREE, "sourceTrackedBytesUnchanged": True,
                     "sourceCleanAfterRetainedProjectRelocation": True, "xcodeBuildReexecuted": False,
                     "compilerRebuilt": False, "nativeInvocation": False,
                     "completeRepositoryInventory": False, "completeTrackedProject": True,
                     "sourceScope": subject.BUILD_SCOPE, "scopePaths": subject.BUILD_SCOPE_PATHS,
                     "sourceFiles": 22, "projectInputCount": 14}
    return build, qualification

def scoped_source():
    return {"commit": subject.BUILD, "tree": subject.BUILD_TREE,
            "completeTrackedSource": False, "completeTrackedProject": True,
            "sourceScope": subject.BUILD_SCOPE, "scopePaths": list(subject.BUILD_SCOPE_PATHS),
            "files": {path: {"bytes": 1, "gitBlob": "1" * 40, "mode": "100644", "sha256": "2" * 64}
                      for path in subject.BUILD_INPUT_FILES}}


if __name__ == "__main__":
    unittest.main()
