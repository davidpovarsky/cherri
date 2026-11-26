#!/usr/bin/env python3
"""Synthetic SQLite and receipt controls; no Apple import or runtime authority."""
import copy
import datetime
import hashlib
import json
from pathlib import Path
import plistlib
import sqlite3
import sys
import tempfile
import unittest
import uuid
from unittest import mock

import shortcut_library_identity as identity


class ShortcutLibraryIdentityTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="cherri-library-identity-")
        self.root = Path(self.temporary.name).resolve(strict=True)
        self.addCleanup(self.temporary.cleanup)
        self.udid = "D631D98E-2271-4948-B465-8EDBA9BD4D68"
        self.data = self.root / "CoreSimulator/Devices" / self.udid / "data"
        self.database = self.data / "Library/Shortcuts/Shortcuts.sqlite"
        self.database.parent.mkdir(parents=True)
        self.actions = [{"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
                         "WFWorkflowActionParameters": {"WFCommentActionText": "owned synthetic 🧪",
                                                        "number": 0, "flag": False}}]
        self.unsigned = plistlib.dumps({"WFWorkflowActions": self.actions})
        self.signed = b"synthetic-signed-input-not-a-native-signature"
        self.unsigned_path = self.root / "input.shortcut"
        self.signed_path = self.root / "input.signed.shortcut"
        self.unsigned_path.write_bytes(self.unsigned)
        self.signed_path.write_bytes(self.signed)
        self.expected = {"schema": identity.CONTEXT_SCHEMA, "runId": "test-run", "runNonce": "test-nonce",
                         "deviceUdid": self.udid, "phaseId": "import-test", "expectedName": "owned-fixture",
                         "compilerCommitSha": "1" * 40, "builtHarnessCommitSha": "2" * 40,
                         "controllerCommitSha": "3" * 40, "unsignedSHA256": identity.sha(self.unsigned),
                         "signedSHA256": identity.sha(self.signed)}
        with sqlite3.connect(self.database) as c:
            c.execute("CREATE TABLE ZSHORTCUT (Z_PK INTEGER PRIMARY KEY,ZNAME VARCHAR,ZWORKFLOWID VARCHAR,"
                      "ZACTIONCOUNT INTEGER,ZACTIONS INTEGER,ZHIDDENFROMLIBRARYANDSYNC INTEGER,"
                      "ZTOMBSTONED INTEGER,ZCREATIONDATE TIMESTAMP,ZMODIFICATIONDATE TIMESTAMP)")
            c.execute("CREATE TABLE ZSHORTCUTACTIONS (Z_PK INTEGER PRIMARY KEY,ZSHORTCUT INTEGER,ZDATA BLOB)")

    def add(self, number=1, *, name=None, actions=None, hidden=0, tombstoned=0, workflow=None,
            created=817344002.0, modified=817344002.0):
        # These are authored fixture rows, never copies of user/private data.
        actions = self.actions if actions is None else actions
        workflow = str(uuid.UUID(int=number)).upper() if workflow is None else workflow
        with sqlite3.connect(self.database) as c:
            c.execute("INSERT INTO ZSHORTCUT VALUES (?,?,?,?,?,?,?,?,?)",
                      (number, name or self.expected["expectedName"], workflow, len(actions), number,
                       hidden, tombstoned, created, modified))
            c.execute("INSERT INTO ZSHORTCUTACTIONS VALUES (?,?,?)",
                      (number, number, plistlib.dumps(actions, fmt=plistlib.FMT_BINARY)))
        return workflow

    def read(self, name=None):
        return identity.read_library(self.database, name or self.expected["expectedName"], self.unsigned)

    def observation(self, stage, rows=None, *, start="2026-11-26T00:00:00+00:00",
                    end="2026-11-26T00:00:01+00:00"):
        # Explicit synthetic contract double for pure validator tests. It does
        # not report a real Darwin execution or confer native authority.
        measured = self.read()
        if rows is not None:
            measured.update(records=rows, matchingRecordCount=len(rows),
                            activeRecordCount=sum(not row["tombstoned"] for row in rows))
        return {"schema": identity.SCHEMA, "status": "OBSERVED", "stage": stage,
                "context": copy.deepcopy(self.expected), "actualPlatform": "darwin",
                "unsignedSHA256": self.expected["unsignedSHA256"],
                "signedSHA256": self.expected["signedSHA256"], "formalAcceptanceGranted": False,
                "nativeInvocationCount": 0, "readStartedUTC": start, "readCompletedUTC": end, **measured}

    def pair(self):
        before = self.observation("before")
        self.add()
        after = self.observation("after", start="2026-11-26T00:00:04+00:00",
                                 end="2026-11-26T00:00:05+00:00")
        dispatch = {"outcome": "EXITED", "timedOut": False, "exitCode": 0, "parentReaped": True,
                    "startedUTC": "2026-11-26T00:00:01.500000+00:00",
                    "completedUTC": "2026-11-26T00:00:03+00:00"}
        return before, after, dispatch

    def test_absence_uses_actual_complete_exact_name_sql_query(self):
        self.add(name="unrelated")
        before = self.observation("before")
        self.assertEqual(before["records"], [])
        self.assertEqual(identity.admit_absence(before, self.expected)["admission"], "ABSENT_BEFORE_IMPORT")

    def test_existing_one_or_duplicate_records_refuse_admission(self):
        for count in (1, 2):
            with self.subTest(records=count):
                if count == 1:
                    self.add()
                else:
                    self.add(2)
                with self.assertRaisesRegex(ValueError, "PREEXISTING_LIBRARY_RECORD"):
                    identity.admit_absence(self.observation("before"), self.expected)

    def test_hidden_active_record_also_refuses_admission(self):
        self.add(hidden=1)
        with self.assertRaisesRegex(ValueError, "PREEXISTING_LIBRARY_RECORD"):
            identity.admit_absence(self.observation("before"), self.expected)

    def test_tombstoned_record_is_retained_but_not_active(self):
        self.add(tombstoned=1)
        before = self.observation("before")
        self.assertEqual(before["matchingRecordCount"], 1)
        self.assertEqual(before["activeRecordCount"], 0)
        self.assertTrue(before["records"][0]["tombstoned"])
        identity.admit_absence(before, self.expected)

    def test_exact_unicode_and_sql_parameters(self):
        for title in ("שלום 🧪", "e\u0301", "x' OR 1=1 --"):
            with self.subTest(title=title):
                number = 1 + len(list(self.root.glob("marker-*")))
                (self.root / ("marker-" + str(number))).touch()
                self.add(number, name=title)
                self.assertEqual(self.read(title)["matchingRecordCount"], 1)
        self.assertEqual(self.read("é")["matchingRecordCount"], 0)
        self.assertEqual(self.read("owned")["matchingRecordCount"], 0)

    def test_readonly_connection_and_source_bytes(self):
        self.add()
        before = self.database.read_bytes()
        actual_connect = sqlite3.connect
        connections = []
        statements = []
        def connect(*args, **kwargs):
            self.assertTrue(args[0].endswith("?mode=ro"))
            self.assertIs(kwargs["uri"], True)
            c = actual_connect(*args, **kwargs)
            c.set_trace_callback(statements.append)
            connections.append(c)
            return c
        with mock.patch.object(identity.sqlite3, "connect", side_effect=connect):
            self.assertEqual(self.read()["activeRecordCount"], 1)
        self.assertEqual(self.database.read_bytes(), before)
        self.assertEqual(len(connections), 1)
        self.assertTrue(any("query_only=ON" in value for value in statements))
        self.assertFalse(any(value.lstrip().upper().startswith(("INSERT", "UPDATE", "DELETE", "REPLACE", "CREATE", "DROP"))
                             for value in statements))
        with actual_connect(self.database.as_uri() + "?mode=ro", uri=True) as c:
            with self.assertRaises(sqlite3.OperationalError):
                c.execute("DELETE FROM ZSHORTCUT")

    def test_native_record_and_action_owner_are_retained(self):
        native_id = self.add()
        row = self.read()["records"][0]
        self.assertEqual((row["rowId"], row["actionRowId"], row["workflowUUID"]), (1, 1, native_id))
        self.assertEqual(row["actionCount"], 1)
        self.assertTrue(row["actionsMatchUnsigned"])
        self.assertEqual(row["actionsTypedSHA256"], identity.action_input(self.unsigned)[1])

    def test_original_blob_retained_before_semantic_rejection(self):
        self.add()
        raw = b"not a plist"
        with sqlite3.connect(self.database) as c:
            c.execute("UPDATE ZSHORTCUTACTIONS SET ZDATA=?", (raw,))
        retained = []
        with self.assertRaises(Exception):
            identity.read_library(self.database, self.expected["expectedName"], self.unsigned,
                                  save_blob=lambda row, value: retained.append((row, value)) or "raw")
        self.assertEqual(retained, [(1, raw)])

    def test_boolean_number_and_nested_types_are_not_collapsed(self):
        for number, replacement in enumerate((1, False, 0.0, "0", [], {}, b"0"), 1):
            with self.subTest(replacement=repr(replacement)):
                actions = copy.deepcopy(self.actions)
                actions[0]["WFWorkflowActionParameters"]["number"] = replacement
                self.add(number, name="type-" + str(number), actions=actions)
                row = self.read("type-" + str(number))["records"][0]
                self.assertFalse(row["actionsMatchUnsigned"])
        self.assertNotEqual(identity.typed_tree([True]), identity.typed_tree([1]))
        self.assertNotEqual(identity.typed_tree([False]), identity.typed_tree([0]))

    def test_action_order_and_value_changes_are_rejected(self):
        for actions in ([*self.actions, *self.actions],
                        [{"WFWorkflowActionIdentifier": "different", "WFWorkflowActionParameters": {}}]):
            with self.subTest(actions=actions):
                with sqlite3.connect(self.database) as c:
                    c.execute("DELETE FROM ZSHORTCUT"); c.execute("DELETE FROM ZSHORTCUTACTIONS")
                self.add(actions=actions)
                self.assertFalse(self.read()["records"][0]["actionsMatchUnsigned"])

    def test_path_requires_exact_owned_device_and_no_symlink(self):
        self.assertEqual(identity.database_path(self.data, self.udid), self.database)
        with self.assertRaises(ValueError):
            identity.database_path(self.data, "00000000-0000-0000-0000-000000000001")
        alias = self.root / "alias"
        alias.symlink_to(self.data.parent.parent.parent, target_is_directory=True)
        with self.assertRaises(ValueError):
            identity.database_path(alias / "Devices" / self.udid / "data", self.udid)
        original = self.database.with_suffix(".original")
        self.database.rename(original)
        self.database.symlink_to(original)
        with self.assertRaisesRegex(ValueError, "DATABASE_PATH_SYMLINK"):
            identity.database_path(self.data, self.udid)

    def test_missing_database_is_not_absence(self):
        self.database.unlink()
        with self.assertRaises(ValueError):
            self.read()
        with self.assertRaises(FileNotFoundError):
            identity.database_path(self.data, self.udid)

    def test_unknown_schema_is_not_absence(self):
        with sqlite3.connect(self.database) as c:
            c.execute("ALTER TABLE ZSHORTCUT RENAME COLUMN ZNAME TO OTHER")
        with self.assertRaisesRegex(ValueError, "LIBRARY_SCHEMA"):
            self.read()

    def test_wrong_action_owner_and_missing_actions_are_rejected(self):
        self.add()
        with sqlite3.connect(self.database) as c:
            c.execute("UPDATE ZSHORTCUTACTIONS SET ZSHORTCUT=2")
        with self.assertRaisesRegex(ValueError, "LIBRARY_ACTION_OWNER"):
            self.read()
        with sqlite3.connect(self.database) as c:
            c.execute("DELETE FROM ZSHORTCUTACTIONS")
        with self.assertRaisesRegex(ValueError, "LIBRARY_ACTION_OWNER"):
            self.read()

    def test_malformed_record_states_and_dates(self):
        self.add()
        for column, value in (("ZTOMBSTONED", 2), ("ZHIDDENFROMLIBRARYANDSYNC", 2),
                              ("ZACTIONCOUNT", -1), ("ZWORKFLOWID", "not-an-id"),
                              ("ZCREATIONDATE", None)):
            with self.subTest(column=column):
                with sqlite3.connect(self.database) as c:
                    original = c.execute("SELECT " + column + " FROM ZSHORTCUT").fetchone()[0]
                    c.execute("UPDATE ZSHORTCUT SET " + column + "=?", (value,))
                with self.assertRaises((ValueError, TypeError)):
                    self.read()
                with sqlite3.connect(self.database) as c:
                    c.execute("UPDATE ZSHORTCUT SET " + column + "=?", (original,))

    def test_duplicate_uuid_is_rejected(self):
        identifier = self.add()
        self.add(2, workflow=identifier)
        with self.assertRaisesRegex(ValueError, "LIBRARY_RECORD_IDENTITY_DUPLICATE"):
            self.read()

    def test_query_bound_does_not_truncate_to_first_match(self):
        for number in range(1, 34):
            self.add(number)
        with self.assertRaisesRegex(ValueError, "LIBRARY_MATCH_BOUNDS"):
            self.read()

    def test_fresh_absent_to_unique_original_record_join(self):
        before, after, dispatch = self.pair()
        result = identity.bind_new_record(before, after, self.expected, dispatch)
        self.assertTrue(result["beforeExactNameAbsent"])
        self.assertTrue(result["afterExactNameUnique"])
        self.assertTrue(result["storedActionsMatchUnsigned"])
        self.assertFalse(result["formalAcceptanceGranted"])
        self.assertFalse(result["runtimeExecuted"])

    def test_post_import_missing_or_duplicates_are_rejected(self):
        before, after, dispatch = self.pair()
        for rows in ([], [after["records"][0], copy.deepcopy(after["records"][0])]):
            with self.subTest(count=len(rows)):
                value = copy.deepcopy(after)
                value.update(records=rows, activeRecordCount=len(rows), matchingRecordCount=len(rows))
                with self.assertRaisesRegex(ValueError, "NO_UNIQUE_NEW_LIBRARY_RECORD"):
                    identity.bind_new_record(before, value, self.expected, dispatch)

    def test_post_import_hidden_or_changed_actions_are_rejected(self):
        before, after, dispatch = self.pair()
        for field, value in (("hidden", True), ("actionsMatchUnsigned", False),
                             ("actionsTypedSHA256", "0" * 64)):
            with self.subTest(field=field):
                changed = copy.deepcopy(after); changed["records"][0][field] = value
                with self.assertRaisesRegex(ValueError, "NEW_RECORD_ACTION_IDENTITY"):
                    identity.bind_new_record(before, changed, self.expected, dispatch)

    def test_preexisting_record_cannot_be_relabeled_as_fresh(self):
        before, after, dispatch = self.pair()
        before.update(records=copy.deepcopy(after["records"]), activeRecordCount=1, matchingRecordCount=1)
        with self.assertRaisesRegex(ValueError, "PREEXISTING_LIBRARY_RECORD"):
            identity.bind_new_record(before, after, self.expected, dispatch)

    def test_tombstoned_uuid_cannot_be_reused_as_new(self):
        before, after, dispatch = self.pair()
        row = copy.deepcopy(after["records"][0]); row["tombstoned"] = True
        before.update(records=[row], activeRecordCount=0, matchingRecordCount=1)
        with self.assertRaisesRegex(ValueError, "REUSED_LIBRARY_RECORD_UUID"):
            identity.bind_new_record(before, after, self.expected, dispatch)

    def test_context_and_signed_or_unsigned_inputs_are_exact(self):
        before, after, dispatch = self.pair()
        for key in sorted(identity.CONTEXT_KEYS):
            with self.subTest(contextField=key):
                changed = copy.deepcopy(after); changed["context"][key] = "different"
                with self.assertRaises(ValueError):
                    identity.bind_new_record(before, changed, self.expected, dispatch)
        for key in ("signedSHA256", "unsignedSHA256"):
            with self.subTest(input=key):
                changed = copy.deepcopy(after); changed[key] = "f" * 64
                with self.assertRaisesRegex(ValueError, "LIBRARY_OBSERVATION_ARTIFACTS"):
                    identity.bind_new_record(before, changed, self.expected, dispatch)

    def test_platform_scope_counts_and_status_are_not_inherited(self):
        before, after, dispatch = self.pair()
        mutations = {"actualPlatform": "linux", "status": "FAILED", "queryOnly": False,
                     "sourceSQLWrites": 1, "formalAcceptanceGranted": True,
                     "matchingRecordCount": 0, "activeRecordCount": 0,
                     "query": "SELECT FIRST", "queryName": "other", "stage": "before"}
        for key, value in mutations.items():
            with self.subTest(field=key):
                changed = copy.deepcopy(after); changed[key] = value
                with self.assertRaises(ValueError):
                    identity.bind_new_record(before, changed, self.expected, dispatch)

    def test_import_process_failure_is_not_overridden_by_library(self):
        before, after, dispatch = self.pair()
        for field, value in (("outcome", "TIMED_OUT"), ("timedOut", True), ("exitCode", 1),
                             ("exitCode", False), ("parentReaped", False)):
            with self.subTest(field=field, value=value):
                changed = dict(dispatch); changed[field] = value
                with self.assertRaisesRegex(ValueError, "LIBRARY_IMPORT_PROCESS"):
                    identity.bind_new_record(before, after, self.expected, changed)

    def test_preflight_import_and_postflight_timeline_is_required(self):
        before, after, dispatch = self.pair()
        for owner, field, value in (
            ("before", "readCompletedUTC", "2026-11-26T00:00:02+00:00"),
            ("after", "readStartedUTC", "2026-11-26T00:00:02+00:00"),
            ("dispatch", "completedUTC", "2026-11-26T00:00:01+00:00")):
            with self.subTest(owner=owner, field=field):
                values = {"before": copy.deepcopy(before), "after": copy.deepcopy(after),
                          "dispatch": copy.deepcopy(dispatch)}
                values[owner][field] = value
                with self.assertRaises(ValueError):
                    identity.bind_new_record(values["before"], values["after"], self.expected, values["dispatch"])

    def test_record_creation_is_within_this_import(self):
        before, after, dispatch = self.pair()
        for field, value in (("createdUTC", "2026-11-25T23:59:59+00:00"),
                             ("createdUTC", "2026-11-26T00:00:06+00:00"),
                             ("modifiedUTC", "2026-11-26T00:00:01+00:00"),
                             ("modifiedUTC", "2026-11-26T00:00:06+00:00")):
            with self.subTest(field=field, value=value):
                changed = copy.deepcopy(after); changed["records"][0][field] = value
                with self.assertRaisesRegex(ValueError, "NEW_RECORD_CREATION_TIMELINE"):
                    identity.bind_new_record(before, changed, self.expected, dispatch)

    def test_strict_context_json_and_timestamps(self):
        with self.assertRaises(ValueError):
            identity.strict_json(b'{"schema":1,"schema":1}')
        with self.assertRaises(ValueError):
            identity.strict_json(b'{"value":NaN}')
        with self.assertRaises(ValueError):
            identity.context({**self.expected, "extra": True})
        with self.assertRaises(ValueError):
            identity.instant("2026-11-26T00:00:00")
        self.assertEqual(identity.context(copy.deepcopy(self.expected)), self.expected)

    def test_failed_platform_is_retained_with_actual_platform(self):
        with mock.patch.object(identity.sys, "platform", "linux"):
            result = identity.observe(self.data, self.expected, "before", self.unsigned_path,
                                      self.signed_path, self.root / "platform-failure")
        self.assertEqual(result["status"], "FAILED")
        self.assertEqual(result["error"], "DARWIN_REQUIRED")
        self.assertEqual(result["actualPlatform"], "linux")
        self.assertEqual(result["nativeInvocationCount"], 0)
        self.assertEqual(identity.strict_json((self.root/"platform-failure/result.original.json").read_bytes()), result)

    def test_malformed_input_failure_is_retained_in_synthetic_darwin_double(self):
        self.unsigned_path.write_bytes(b"changed")
        with mock.patch.object(identity.sys, "platform", "darwin"):
            result = identity.observe(self.data, self.expected, "before", self.unsigned_path,
                                      self.signed_path, self.root / "input-failure")
        self.assertEqual(result["status"], "FAILED")
        self.assertEqual(result["error"], "LIBRARY_INPUT_HASH")
        self.assertEqual(result["nativeInvocationCount"], 0)
        self.assertTrue((self.root/"input-failure/result.original.json").is_file())

    def test_create_only_output_preserves_existing_originals(self):
        output = self.root / "existing"; output.mkdir(); (output/"keep").write_bytes(b"original")
        with self.assertRaisesRegex(ValueError, "LIBRARY_OUTPUT_CREATE_ONLY"):
            identity.observe(self.data, self.expected, "before", self.unsigned_path, self.signed_path, output)
        self.assertEqual((output/"keep").read_bytes(), b"original")


if __name__ == "__main__":
    unittest.main()
