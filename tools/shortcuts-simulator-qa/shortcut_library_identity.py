#!/usr/bin/env python3
"""Read-only simulator library identity for an explicitly bound engineering import.

A successful UI query is not proof that a new signed workflow was imported. This
module records the complete exact-name SQLite result before and after the native
import, and compares the new stored action tree with the original unsigned bytes.
It never writes the source database, deletes a record, or launches a shortcut.
"""
import argparse
import datetime
import hashlib
import json
import math
import os
from pathlib import Path
import re
import sqlite3
import stat
import sys
import time
import uuid
import plistlib

SCHEMA = "cherri-simulator-library-identity/1"
CONTEXT_SCHEMA = "cherri-simulator-library-import-context/1"
CONTEXT_KEYS = {"schema", "runId", "runNonce", "deviceUdid", "phaseId", "expectedName",
                "compilerCommitSha", "builtHarnessCommitSha", "controllerCommitSha",
                "unsignedSHA256", "signedSHA256"}
COLUMNS = ("Z_PK", "ZNAME", "ZWORKFLOWID", "ZACTIONCOUNT", "ZACTIONS",
           "ZHIDDENFROMLIBRARYANDSYNC", "ZTOMBSTONED", "ZCREATIONDATE", "ZMODIFICATIONDATE")
QUERY = "SELECT " + ",".join(COLUMNS) + " FROM ZSHORTCUT WHERE ZNAME COLLATE BINARY = ? ORDER BY Z_PK LIMIT 33"
MAX_FILE = 4 * 1024 * 1024
MAX_ROWS = 32
MAX_NODES = 100000
READ_SECONDS = 8


def require(condition, code):
    if not condition:
        raise ValueError(code)


def sha(value):
    return hashlib.sha256(value).hexdigest()


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def strict_json(raw):
    def object_pairs(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "DUPLICATE_JSON_KEY")
            result[key] = value
        return result
    return json.loads(raw.decode("utf-8"), object_pairs_hook=object_pairs,
                      parse_constant=lambda value: (_ for _ in ()).throw(ValueError("NONFINITE_JSON")))


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"), allow_nan=False).encode("utf-8")


def read(path, limit=MAX_FILE):
    path = Path(path)
    require(path.is_absolute(), "ABSOLUTE_PATH_REQUIRED")
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and info.st_size <= limit, "REGULAR_BOUNDED_INPUT_REQUIRED")
    with path.open("rb") as stream:
        value = stream.read(limit + 1)
    require(len(value) <= limit and len(value) == info.st_size, "INPUT_SIZE_CHANGED")
    return value


def context(value):
    require(isinstance(value, dict) and set(value) == CONTEXT_KEYS and
            value["schema"] == CONTEXT_SCHEMA, "CONTEXT_FIELDS")
    for key in ("runId", "runNonce", "phaseId"):
        require(isinstance(value[key], str) and re.fullmatch(r"[A-Za-z0-9_-]{1,128}", value[key]) is not None,
                "CONTEXT_RUN_IDENTITY")
    require(isinstance(value["expectedName"], str) and 0 < len(value["expectedName"].encode("utf-8")) <= 4096
            and "\0" not in value["expectedName"], "CONTEXT_NAME")
    require(isinstance(value["deviceUdid"], str) and
            str(uuid.UUID(value["deviceUdid"])).upper() == value["deviceUdid"], "CONTEXT_DEVICE")
    for key in ("compilerCommitSha", "builtHarnessCommitSha", "controllerCommitSha"):
        require(isinstance(value[key], str) and re.fullmatch(r"[0-9a-f]{40}", value[key]) is not None,
                "CONTEXT_SOURCE")
    for key in ("unsignedSHA256", "signedSHA256"):
        require(isinstance(value[key], str) and re.fullmatch(r"[0-9a-f]{64}", value[key]) is not None,
                "CONTEXT_ARTIFACT")
    return value


def database_path(device_data_path, device_udid):
    data = Path(device_data_path)
    require(data.is_absolute() and ".." not in data.parts and data.name == "data"
            and data.parent.name == device_udid and data.parent.parent.name == "Devices"
            and data.parent.parent.parent.name == "CoreSimulator", "DEVICE_DATA_PATH")
    target = data / "Library/Shortcuts/Shortcuts.sqlite"
    for component in (target, *target.parents):
        info = component.lstat()
        require(not stat.S_ISLNK(info.st_mode), "DATABASE_PATH_SYMLINK")
        require(stat.S_ISREG(info.st_mode) if component == target else stat.S_ISDIR(info.st_mode),
                "DATABASE_PATH_KIND")
    return target


def typed_tree(value):
    count = [0]
    def check(item, depth):
        count[0] += 1
        require(count[0] <= MAX_NODES and depth <= 64, "ACTION_TREE_BOUNDS")
        if isinstance(item, dict):
            require(all(type(key) is str for key in item), "ACTION_DICTIONARY_KEYS")
            for child in item.values():
                check(child, depth + 1)
        elif isinstance(item, list):
            for child in item:
                check(child, depth + 1)
        elif type(item) in (str, bytes, bool, int):
            if type(item) in (str, bytes):
                require(len(item.encode("utf-8") if isinstance(item, str) else item) <= MAX_FILE,
                        "ACTION_VALUE_BOUNDS")
        elif type(item) is float:
            require(math.isfinite(item), "ACTION_NONFINITE")
        elif not isinstance(item, (datetime.datetime, plistlib.UID)):
            raise ValueError("ACTION_VALUE_KIND")
    check(value, 0)
    return plistlib.dumps(value, fmt=plistlib.FMT_BINARY, sort_keys=True)


def action_input(raw):
    value = plistlib.loads(raw)
    require(type(value) is dict and type(value.get("WFWorkflowActions")) is list
            and 0 < len(value["WFWorkflowActions"]) <= 5000, "UNSIGNED_ACTIONS")
    actions = value["WFWorkflowActions"]
    return actions, sha(typed_tree(actions))


def apple_date(value):
    require(type(value) in (float, int) and math.isfinite(value), "LIBRARY_DATE")
    return (datetime.datetime(2001, 1, 1, tzinfo=datetime.timezone.utc) +
            datetime.timedelta(seconds=value)).isoformat()


def instant(value):
    require(isinstance(value, str), "TIMESTAMP_KIND")
    parsed = datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))
    require(parsed.tzinfo is not None, "TIMESTAMP_ZONE")
    return parsed.astimezone(datetime.timezone.utc)


def read_library(database, expected_name, unsigned_raw, *, save_blob=None):
    """One actual read transaction, exact-name scope, no record modifications."""
    database = Path(database)
    require(database.is_absolute() and database.is_file() and not database.is_symlink(), "DATABASE_FILE")
    _, expected_digest = action_input(unsigned_raw)
    begin = time.monotonic()
    connection = sqlite3.connect(database.as_uri() + "?mode=ro", uri=True, timeout=2)
    try:
        connection.execute("PRAGMA query_only=ON")
        require(connection.execute("PRAGMA query_only").fetchone() == (1,), "SQLITE_QUERY_ONLY")
        connection.set_progress_handler(lambda: int(time.monotonic() - begin > READ_SECONDS), 512)
        connection.execute("BEGIN")
        columns = {row[1]: row[2].upper() for row in connection.execute("PRAGMA table_info(ZSHORTCUT)")}
        require(all(key in columns for key in COLUMNS), "LIBRARY_SCHEMA")
        action_columns = {row[1] for row in connection.execute("PRAGMA table_info(ZSHORTCUTACTIONS)")}
        require({"Z_PK", "ZSHORTCUT", "ZDATA"} <= action_columns, "LIBRARY_ACTION_SCHEMA")
        connection.row_factory = sqlite3.Row
        rows = connection.execute(QUERY, (expected_name,)).fetchall()
        require(len(rows) <= MAX_ROWS, "LIBRARY_MATCH_BOUNDS")
        records = []
        for row in rows:
            require(time.monotonic() - begin < READ_SECONDS, "LIBRARY_READ_DEADLINE")
            value = dict(row)
            require(type(value["Z_PK"]) is int and value["Z_PK"] > 0 and
                    type(value["ZACTIONS"]) is int and value["ZACTIONS"] > 0, "LIBRARY_RECORD_KEY")
            require(type(value["ZNAME"]) is str and
                    value["ZNAME"].encode("utf-8") == expected_name.encode("utf-8"), "LIBRARY_EXACT_NAME")
            require(type(value["ZWORKFLOWID"]) is str and
                    str(uuid.UUID(value["ZWORKFLOWID"])).upper() == value["ZWORKFLOWID"], "LIBRARY_RECORD_UUID")
            require(type(value["ZACTIONCOUNT"]) is int and 0 <= value["ZACTIONCOUNT"] <= 5000,
                    "LIBRARY_ACTION_COUNT")
            for field in ("ZHIDDENFROMLIBRARYANDSYNC", "ZTOMBSTONED"):
                require(type(value[field]) is int and value[field] in (0, 1), "LIBRARY_RECORD_STATE")
            item = {"rowId": value["Z_PK"], "name": value["ZNAME"], "workflowUUID": value["ZWORKFLOWID"],
                    "actionRowId": value["ZACTIONS"], "actionCount": value["ZACTIONCOUNT"],
                    "hidden": bool(value["ZHIDDENFROMLIBRARYANDSYNC"]), "tombstoned": bool(value["ZTOMBSTONED"]),
                    "createdUTC": apple_date(value["ZCREATIONDATE"]),
                    "modifiedUTC": apple_date(value["ZMODIFICATIONDATE"])}
            action_rows = connection.execute("SELECT Z_PK,ZSHORTCUT,ZDATA FROM ZSHORTCUTACTIONS WHERE Z_PK=?",
                                             (value["ZACTIONS"],)).fetchall()
            require(len(action_rows) == 1 and action_rows[0]["ZSHORTCUT"] == value["Z_PK"], "LIBRARY_ACTION_OWNER")
            blob = action_rows[0]["ZDATA"]
            require(type(blob) is bytes and 0 < len(blob) <= MAX_FILE, "LIBRARY_ACTION_BLOB")
            item["actionsBlobSHA256"] = sha(blob)
            item["actionsBlobBytes"] = len(blob)
            if save_blob is not None:
                item["actionsOriginalFile"] = save_blob(value["Z_PK"], blob)
            parsed = plistlib.loads(blob)
            require(type(parsed) is list and len(parsed) == value["ZACTIONCOUNT"], "LIBRARY_STORED_ACTIONS")
            item["actionsTypedSHA256"] = sha(typed_tree(parsed))
            item["actionsMatchUnsigned"] = item["actionsTypedSHA256"] == expected_digest
            records.append(item)
        require(len({item["rowId"] for item in records}) == len(records)
                and len({item["workflowUUID"] for item in records}) == len(records), "LIBRARY_RECORD_IDENTITY_DUPLICATE")
        connection.rollback()
        return {"query": QUERY, "queryName": expected_name, "records": records,
                "matchingRecordCount": len(records), "activeRecordCount": sum(not row["tombstoned"] for row in records),
                "expectedActionsTypedSHA256": expected_digest, "sqliteVersion": sqlite3.sqlite_version,
                "queryOnly": True, "sourceSQLWrites": 0, "readSeconds": time.monotonic() - begin}
    finally:
        connection.close()


def validate_observation(value, expected, stage):
    expected = context(expected)
    require(type(value) is dict and value.get("schema") == SCHEMA and value.get("status") == "OBSERVED"
            and value.get("stage") == stage and value.get("context") == expected, "LIBRARY_OBSERVATION_CONTEXT")
    require(value.get("unsignedSHA256") == expected["unsignedSHA256"] and
            value.get("signedSHA256") == expected["signedSHA256"], "LIBRARY_OBSERVATION_ARTIFACTS")
    require(value.get("actualPlatform") == "darwin" and value.get("queryOnly") is True and
            type(value.get("sourceSQLWrites")) is int and value["sourceSQLWrites"] == 0
            and value.get("formalAcceptanceGranted") is False, "LIBRARY_OBSERVATION_SCOPE")
    require(value.get("query") == QUERY and value.get("queryName") == expected["expectedName"], "LIBRARY_QUERY_IDENTITY")
    records = value.get("records")
    require(type(records) is list and len(records) <= MAX_ROWS and
            type(value.get("matchingRecordCount")) is int and value["matchingRecordCount"] == len(records),
            "LIBRARY_OBSERVATION_RECORD_COUNT")
    require(all(type(row) is dict and type(row.get("tombstoned")) is bool and
                type(row.get("name")) is str and row["name"].encode("utf-8") == expected["expectedName"].encode("utf-8")
                for row in records), "LIBRARY_OBSERVATION_RECORDS")
    active = [row for row in records if not row["tombstoned"]]
    require(type(value.get("activeRecordCount")) is int and value["activeRecordCount"] == len(active),
            "LIBRARY_OBSERVATION_ACTIVE_COUNT")
    require(instant(value["readStartedUTC"]) <= instant(value["readCompletedUTC"]), "LIBRARY_OBSERVATION_CLOCK")
    return active


def admit_absence(before, expected):
    require(not validate_observation(before, expected, "before"), "PREEXISTING_LIBRARY_RECORD")
    return {"admission": "ABSENT_BEFORE_IMPORT", "context": expected,
            "observationSHA256": sha(canonical(before)), "completedUTC": before["readCompletedUTC"]}


def bind_new_record(before, after, expected, dispatch):
    admit_absence(before, expected)
    active = validate_observation(after, expected, "after")
    require(len(active) == 1, "NO_UNIQUE_NEW_LIBRARY_RECORD")
    row = active[0]
    require(row.get("hidden") is False and row.get("actionsMatchUnsigned") is True and
            row.get("actionsTypedSHA256") == before.get("expectedActionsTypedSHA256") ==
            after.get("expectedActionsTypedSHA256"), "NEW_RECORD_ACTION_IDENTITY")
    require(type(row.get("rowId")) is int and row["rowId"] > 0 and
            type(row.get("workflowUUID")) is str and
            str(uuid.UUID(row["workflowUUID"])).upper() == row["workflowUUID"], "NEW_RECORD_NATIVE_IDENTITY")
    require(row["workflowUUID"] not in {item["workflowUUID"] for item in before["records"]},
            "REUSED_LIBRARY_RECORD_UUID")
    require(dispatch.get("outcome") == "EXITED" and dispatch.get("timedOut") is False and
            type(dispatch.get("exitCode")) is int and dispatch["exitCode"] == 0 and
            dispatch.get("parentReaped") is True, "LIBRARY_IMPORT_PROCESS")
    began, ended = instant(dispatch["startedUTC"]), instant(dispatch["completedUTC"])
    require(instant(before["readCompletedUTC"]) <= began <= ended <= instant(after["readStartedUTC"]),
            "LIBRARY_IMPORT_TIMELINE")
    require(began <= instant(row["createdUTC"]) <= instant(after["readCompletedUTC"]) and
            instant(row["createdUTC"]) <= instant(row["modifiedUTC"]) <= instant(after["readCompletedUTC"]),
            "NEW_RECORD_CREATION_TIMELINE")
    return {"schema": "cherri-new-native-library-record/1", "context": expected, "nativeRecord": row,
            "beforeObservationSHA256": sha(canonical(before)), "afterObservationSHA256": sha(canonical(after)),
            "beforeExactNameAbsent": True, "afterExactNameUnique": True, "storedActionsMatchUnsigned": True,
            "formalAcceptanceGranted": False, "runtimeExecuted": False}


def observe(device_data_path, expected, stage, unsigned, signed, output):
    expected = context(expected)
    require(stage in ("before", "after"), "LIBRARY_STAGE")
    output = Path(output)
    require(output.is_absolute() and not output.exists(), "LIBRARY_OUTPUT_CREATE_ONLY")
    output.mkdir(mode=0o700)
    result = {"schema": SCHEMA, "status": "FAILED", "stage": stage, "context": expected,
              "readStartedUTC": now(), "actualPlatform": sys.platform, "sourceSQLWrites": 0,
              "formalAcceptanceGranted": False, "nativeInvocationCount": 0}
    def original(name, raw):
        with (output / name).open("xb") as stream:
            stream.write(raw)
        return name
    try:
        require(sys.platform == "darwin", "DARWIN_REQUIRED")
        database = database_path(device_data_path, expected["deviceUdid"])
        unsigned_raw, signed_raw = read(unsigned), read(signed)
        require(sha(unsigned_raw) == expected["unsignedSHA256"] and sha(signed_raw) == expected["signedSHA256"],
                "LIBRARY_INPUT_HASH")
        result.update(unsignedSHA256=sha(unsigned_raw), signedSHA256=sha(signed_raw),
                      databasePath=str(database), databaseDevice=database.stat().st_dev,
                      databaseInode=database.stat().st_ino)
        result.update(read_library(database, expected["expectedName"], unsigned_raw,
            save_blob=lambda row, raw: original("record-" + str(row) + "-actions.original.plist", raw)))
        require(read(unsigned) == unsigned_raw and read(signed) == signed_raw, "LIBRARY_INPUT_CHANGED")
        result["status"] = "OBSERVED"
    except Exception as error:
        result.update(errorType=type(error).__name__, error=str(error))
    finally:
        result["readCompletedUTC"] = now()
        original("result.original.json", canonical(result) + b"\n")
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--device-data-path", required=True)
    parser.add_argument("--context", required=True)
    parser.add_argument("--stage", required=True, choices=("before", "after"))
    parser.add_argument("--unsigned", required=True)
    parser.add_argument("--signed", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    os.umask(0o077)
    value = observe(args.device_data_path, strict_json(read(args.context, 65536)), args.stage,
                    args.unsigned, args.signed, args.output)
    sys.stdout.buffer.write(canonical(value) + b"\n")
    return 0 if value["status"] == "OBSERVED" else 1


if __name__ == "__main__":
    raise SystemExit(main())
