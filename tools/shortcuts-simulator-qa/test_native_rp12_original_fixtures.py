"""Synthetic transport controls; these never claim a native execution."""
from __future__ import annotations

import base64
import copy
import json
from pathlib import Path
import plistlib
import unittest
from unittest import mock

import native_rp12_original_fixtures as f


def blob(raw: bytes, prefix: str) -> dict:
    return {prefix + "Base64": base64.b64encode(raw).decode("ascii"),
            prefix + "Sha256": f.sha(raw), prefix + "Bytes": len(raw)}


def fixture(case: f.Case) -> dict:
    # Deliberately synthetic: full source bytes and metadata are original, while
    # the tiny graph only exercises the collector boundary, not the Go oracles.
    document = {"WFWorkflowActions": [
        {"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
         "WFWorkflowActionParameters": {"WFTextActionText": "SYNTHETIC_CONTROL"}},
        {"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"},
    ]}
    raw = plistlib.dumps(document)
    return {
        "schema": f.EXPORT_SCHEMA, "requirement": "RP12", "caseId": case.name,
        "originalMapSha256": f.ORIGINAL_MAP_SHA256, "originalCase": copy.deepcopy(case.original_case),
        **blob(case.source, "source"), **blob(raw, "unsigned"),
        "actionCount": 2, "workflowNamePresent": False, "workflowName": "",
        "nativeExecuted": False, "formalAcceptanceGranted": False,
    }


def output(case: f.Case, value: dict | None = None) -> str:
    encoded = json.dumps(fixture(case) if value is None else value, ensure_ascii=False, separators=(",", ":")).encode()
    return ("    p15_native_rp12_original_export_test.go:420: " + f.MARKER_TEXT
            + " sha256=" + f.sha(encoded) + " bytes=" + str(len(encoded))
            + " base64=" + base64.b64encode(encoded).decode() + "\n")


def events() -> list[dict]:
    rows = [{"Action": "start", "Package": f.PACKAGE}]
    children: dict[str, list[str]] = {name: [] for name in f.REQUIRED_EVENTS}
    roots = []
    for name in sorted(f.REQUIRED_EVENTS):
        ancestors = [parent for parent in f.REQUIRED_EVENTS if name.startswith(parent + "/")]
        if ancestors:
            children[max(ancestors, key=len)].append(name)
        else:
            roots.append(name)

    def walk(name: str) -> None:
        rows.append({"Action": "run", "Package": f.PACKAGE, "Test": name})
        if name in f.CASE_BY_TEST:
            rows.append({"Action": "output", "Package": f.PACKAGE, "Test": name, "Output": output(f.CASE_BY_TEST[name])})
        for child in children[name]:
            walk(child)
        rows.append({"Action": "pass", "Package": f.PACKAGE, "Test": name})

    for root in roots:
        walk(root)
    rows.append({"Action": "pass", "Package": f.PACKAGE})
    return rows


def encoded(rows: list[dict]) -> bytes:
    return b"".join(json.dumps(row, ensure_ascii=False, separators=(",", ":")).encode() + b"\n" for row in rows)


class NativeRP12OriginalFixtureTests(unittest.TestCase):
    def parse(self, rows: list[dict]) -> tuple[dict, ...]:
        raw = encoded(rows)
        return f.parse_exports(raw, expected_sha256=f.sha(raw))

    def rejected(self, rows: list[dict], diagnosis: str | None = None) -> None:
        with self.assertRaises(f.FixtureRejected) as caught:
            self.parse(rows)
        if diagnosis is not None:
            self.assertEqual(str(caught.exception), diagnosis)

    def changed(self, case: f.Case, change) -> list[dict]:
        rows = events()
        packet = fixture(case)
        change(packet)
        for row in rows:
            if row.get("Test") == case.export_test and row["Action"] == "output":
                row["Output"] = output(case, packet)
        return rows

    def test_all_fifty_instances_keep_partition_and_duplicate_obligations(self):
        result = self.parse(events())
        self.assertEqual([row["caseId"] for row in result], [case.name for case in f.CASES])
        self.assertEqual(len(f.ORIGINAL_EVENTS), 65)
        self.assertEqual(len(f.REQUIRED_EVENTS), 116)
        self.assertEqual({kind: sum(case.partition == kind for case in f.CASES)
                          for kind in ("self-contained", "provider", "image")},
                         {"self-contained": 37, "provider": 8, "image": 5})
        self.assertEqual(len({case.source for case in f.CASES}), 48)
        duplicates = [case for case in f.CASES if case.source == f.CASES[9].source]
        self.assertEqual([case.name for case in duplicates], ["default_effects/call_0", "default_guards/calls[0]"])
        for row in result:
            self.assertIs(row["nativeExecuted"], False)
            self.assertIs(row["formalAcceptanceGranted"], False)
            self.assertEqual(row["originalCase"]["runtimeEvidence"], "UNOBSERVED")

    def test_every_original_local_event_is_required(self):
        original = events()
        for name in f.ORIGINAL_EVENTS:
            with self.subTest(original=name):
                self.rejected([row for row in original if row.get("Test") != name])

    def test_no_provider_image_or_silent_case_can_be_dropped(self):
        selected = ("written_order/custom_reverse", "written_order/native_action_healthy_control",
                    "snapshot_boundaries/standalone", "aggregate_snapshots/list", "default_guards/calls[0]")
        for name in selected:
            with self.subTest(case=name):
                self.rejected([row for row in events() if row.get("Test") != f.EXPORT_PARENT + "/" + name])

    def test_original_sources_and_observation_authority_are_exact(self):
        case = f.CASES[4]
        changes = {
            "case": lambda p: p.__setitem__("caseId", f.CASES[0].name),
            "source": lambda p: p.update(blob(case.source + b"\nreturn text(\"substitute\")", "source")),
            "map": lambda p: p.__setitem__("originalMapSha256", "0" * 64),
            "provider": lambda p: p["originalCase"].__setitem__("requiredProvider", "is.workflow.actions.gettext"),
            "partition": lambda p: p["originalCase"].__setitem__("prerequisitePartition", "self-contained"),
            "observation": lambda p: p["originalCase"].__setitem__("observationRequired", "completion only"),
            "original_file": lambda p: p["originalCase"].__setitem__("originalSourceFileSha256", "0" * 64),
            "compile_name": lambda p: p["originalCase"].__setitem__("compileFilename", "substitute.cherri"),
            "scope": lambda p: p["originalCase"].__setitem__("mainBoundary", "returned value proves all effects"),
        }
        for label, change in changes.items():
            with self.subTest(change=label):
                self.rejected(self.changed(case, change), "EXPORT_CASE_BINDING")

    def test_optional_parameters_are_preserved_and_malformed_present_values_reject(self):
        case = f.CASES[0]
        packet = fixture(case)
        original_raw = base64.b64decode(packet["unsignedBase64"])
        f.validate_fixture(packet, case)
        self.assertEqual(base64.b64decode(packet["unsignedBase64"]), original_raw)
        self.assertNotIn("WFWorkflowActionParameters", plistlib.loads(original_raw)["WFWorkflowActions"][1])

        def graph(change):
            result = fixture(case)
            document = plistlib.loads(base64.b64decode(result["unsignedBase64"]))
            change(document["WFWorkflowActions"])
            result.update(blob(plistlib.dumps(document), "unsigned"))
            return result

        empty = graph(lambda actions: actions[1].__setitem__("WFWorkflowActionParameters", {}))
        f.validate_fixture(empty, case)
        for wrong in ("bad", [], 1, True):
            with self.subTest(present_parameters=wrong):
                with self.assertRaisesRegex(f.FixtureRejected, "WORKFLOW_ACTION_PARAMETERS"):
                    f.validate_fixture(graph(lambda actions: actions[1].__setitem__("WFWorkflowActionParameters", wrong)), case)
        with self.assertRaisesRegex(f.FixtureRejected, "NOTHING_HAS_PARAMETERS"):
            f.validate_fixture(graph(lambda actions: actions[1].__setitem__("WFWorkflowActionParameters", {"unexpected": "field"})), case)

    def test_packet_bytes_name_and_json_scalar_types_remain_bound(self):
        case = f.CASES[0]
        changes = {
            "source_base64": lambda p: p.__setitem__("sourceBase64", p["sourceBase64"][:-4] + "AAAA"),
            "unsigned_hash": lambda p: p.__setitem__("unsignedSha256", "0" * 64),
            "unsigned_size": lambda p: p.__setitem__("unsignedBytes", p["unsignedBytes"] + 1),
            "unsigned_base64": lambda p: p.__setitem__("unsignedBase64", "!invalid!"),
            "action_count": lambda p: p.__setitem__("actionCount", True),
            "workflow_name": lambda p: p.__setitem__("workflowName", "forged"),
            "name_presence_type": lambda p: p.__setitem__("workflowNamePresent", 0),
            "unknown_field": lambda p: p.__setitem__("nativeAccepted", True),
        }
        for label, change in changes.items():
            with self.subTest(change=label):
                self.rejected(self.changed(case, change))
        for field in ("nativeExecuted", "formalAcceptanceGranted"):
            for value in (True, 0, "false"):
                with self.subTest(field=field, value=value):
                    self.rejected(self.changed(case, lambda p: p.__setitem__(field, value)), "EXPORT_OVERCLAIM")

    def test_named_event_ownership_duplicates_and_terminals_reject(self):
        case = f.CASES[0]
        for problem in ("duplicate", "wrong_case", "fail", "skip", "package", "after_terminal"):
            rows = events()
            at = next(i for i, row in enumerate(rows) if row.get("Test") == case.export_test and row["Action"] == "output")
            if problem == "duplicate":
                rows.insert(at, copy.deepcopy(rows[at]))
            elif problem == "wrong_case":
                rows[at]["Test"] = f.CASES[1].export_test
            elif problem in ("fail", "skip"):
                next(row for row in rows if row.get("Test") == case.export_test and row["Action"] == "pass")["Action"] = problem
            elif problem == "package":
                rows[at]["Package"] = "github.com/example/substitute"
            else:
                rows.append({"Action": "output", "Package": f.PACKAGE, "Output": "late\n"})
            with self.subTest(problem=problem):
                self.rejected(rows)

    def test_chunked_marker_is_joined_but_incomplete_or_ambiguous_streams_reject(self):
        rows = events()
        at = next(i for i, row in enumerate(rows) if row["Action"] == "output")
        row = rows[at]
        value = row["Output"]
        rows[at:at + 1] = [dict(row, Output=value[:37]), dict(row, Output=value[37:])]
        self.assertEqual(len(self.parse(rows)), 50)
        raw = encoded(events())
        for damaged in (raw[:-1], raw + b"\n", raw.replace(b'"Action":"start"', b'"Action":"start","Action":"start"', 1)):
            with self.subTest(stream_bytes=len(damaged)):
                with self.assertRaises(f.FixtureRejected):
                    f.parse_exports(damaged, expected_sha256=f.sha(damaged))
        with self.assertRaisesRegex(f.FixtureRejected, "GO_STREAM_HASH"):
            f.parse_exports(raw, expected_sha256="0" * 64)

    def test_original_source_catalogue_and_shared_parser_pins_are_enforced(self):
        real_read = Path.read_bytes
        root = Path(f.__file__).resolve().parents[2]
        selected = [root / f.CATALOGUE_PATH,
                    root / "p15_call_order_test.go",
                    root / "tools/language-acceptance/capture_order_inventory.json",
                    Path(f.__file__).parent / "native_go_fixture_stream.py"]
        for path in selected:
            def read(actual):
                raw = real_read(actual)
                return raw + b"\n" if actual == path else raw
            with self.subTest(path=str(path.relative_to(root))):
                with mock.patch.object(Path, "read_bytes", read):
                    with self.assertRaises(f.FixtureRejected):
                        if path == root / f.CATALOGUE_PATH:
                            f.read_catalogue()
                        else:
                            f.original_cases()


if __name__ == "__main__":
    unittest.main()
