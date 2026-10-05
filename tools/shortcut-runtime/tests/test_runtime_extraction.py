#!/usr/bin/env python3
"""tools/shortcut-runtime/tests/test_runtime_extraction.py

Automated tests for Apple Shortcuts runtime extraction v2 tooling.
Covers all 12 acceptance requirements from the specification without requiring macOS private frameworks.
"""

import json
import os
import pathlib
import sqlite3
import sys
import tempfile
import unittest

# Add parent directory to module search path
REPO_ROOT = pathlib.Path(__file__).resolve().parent.parent.parent.parent
RUNTIME_TOOL_DIR = pathlib.Path(__file__).resolve().parent.parent
sys.path.insert(0, str(RUNTIME_TOOL_DIR))

from extract_parameters import get_shard_actions, ParameterExtractor
from merge_parameters import merge_shards, load_shard
from extract_toolkit import inspect_sqlite_database
from compare_cherri import run_comparison
from normalize_snapshot import build_snapshot, sanitize_paths
from runtime_probe import probe_frameworks


class TestRuntimeExtraction(unittest.TestCase):

    def test_01_deterministic_sharding(self):
        """Test 1: Deterministic shard assignment produces balanced, disjoint partitions."""
        actions = [f"is.workflow.actions.test{i:03d}" for i in range(429)]
        shard_count = 10

        all_sharded = []
        shard_sets = []
        for s in range(shard_count):
            shard = get_shard_actions(actions, s, shard_count)
            # Size must be 42 or 43
            self.assertIn(len(shard), [42, 43])
            shard_sets.append(set(shard))
            all_sharded.extend(shard)

        # Disjoint
        for i in range(shard_count):
            for j in range(i + 1, shard_count):
                self.assertEqual(len(shard_sets[i] & shard_sets[j]), 0, f"Shards {i} and {j} overlap")

        # Union covers all
        self.assertEqual(sorted(all_sharded), sorted(actions))

        # Re-running produces identical partition
        for s in range(shard_count):
            repeat_shard = get_shard_actions(actions, s, shard_count)
            self.assertEqual(repeat_shard, get_shard_actions(actions, s, shard_count))

    def test_02_deterministic_merge_order_independent(self):
        """Test 2: Shard merge is deterministic regardless of input file order."""
        with tempfile.TemporaryDirectory() as td:
            shard0 = pathlib.Path(td) / "parameter-encodings-shard-0.json"
            shard1 = pathlib.Path(td) / "parameter-encodings-shard-1.json"

            data0 = {
                "shardIndex": 0,
                "shardCount": 2,
                "actionParameters": {"is.workflow.actions.b": [{"key": "WFText", "class": "WFVariableStringParameter"}]},
                "parameterClasses": {"WFVariableStringParameter": {"stateClass": "WFVariableStringParameterState", "count": 1, "defaultExamples": ["hi"], "usedBy": ["is.workflow.actions.b"]}},
                "actionOutputNames": {"is.workflow.actions.b": "Text B"},
                "actionDefaults": {"is.workflow.actions.b": {"WFText": "hi"}},
                "unavailableActions": [],
                "actionStatuses": {"is.workflow.actions.b": {"status": "success", "elapsedSeconds": 0.1}},
                "summary": {"shardIndex": 0, "successes": 1, "timeouts": 0, "errors": 0}
            }
            data1 = {
                "shardIndex": 1,
                "shardCount": 2,
                "actionParameters": {"is.workflow.actions.a": [{"key": "WFURL", "class": "WFURLParameter"}]},
                "parameterClasses": {"WFURLParameter": {"stateClass": "WFURLStringParameterState", "count": 1, "defaultExamples": ["https://apple.com"], "usedBy": ["is.workflow.actions.a"]}},
                "actionOutputNames": {"is.workflow.actions.a": "URL A"},
                "actionDefaults": {"is.workflow.actions.a": {"WFURL": "https://apple.com"}},
                "unavailableActions": ["is.workflow.actions.missing"],
                "actionStatuses": {"is.workflow.actions.a": {"status": "success", "elapsedSeconds": 0.2}},
                "summary": {"shardIndex": 1, "successes": 1, "timeouts": 0, "errors": 0}
            }

            shard0.write_text(json.dumps(data0))
            shard1.write_text(json.dumps(data1))

            merge_forward, sum_f, _, _ = merge_shards([str(shard0), str(shard1)], expected_shard_count=2)
            merge_reverse, sum_r, _, _ = merge_shards([str(shard1), str(shard0)], expected_shard_count=2)

            self.assertEqual(json.dumps(merge_forward, sort_keys=True), json.dumps(merge_reverse, sort_keys=True))
            self.assertEqual(json.dumps(sum_f, sort_keys=True), json.dumps(sum_r, sort_keys=True))
            # Check keys sorted
            self.assertEqual(list(merge_forward["actionParameters"].keys()), ["is.workflow.actions.a", "is.workflow.actions.b"])

    def test_03_timeout_status_preserved_and_isolated(self):
        """Test 3: Timeout status is preserved and watchdog does not abort subsequent records."""
        with tempfile.TemporaryDirectory() as td:
            # Create a mock worker script in python
            mock_worker = pathlib.Path(td) / "mock_worker.py"
            mock_worker.write_text("""import sys, time, json, pathlib
action_id = sys.argv[1]
out_file = sys.argv[2]
if "hang" in action_id:
    time.sleep(3.0)  # Will exceed timeout of 0.4s
data = {
    "identifier": action_id,
    "status": "success",
    "parameters": [{"key": "WFInput", "class": "WFStringParameter"}]
}
pathlib.Path(out_file).write_text(json.dumps(data))
""")

            actions_file = pathlib.Path(td) / "actions.json"
            actions_list = ["is.workflow.actions.action1", "is.workflow.actions.hang_action", "is.workflow.actions.action3"]
            actions_file.write_text(json.dumps(actions_list))

            import types
            args = types.SimpleNamespace(
                shard_index=0,
                shard_count=1,
                actions_file=str(actions_file),
                out_dir=td,
                checkpoint_file=None,
                action_timeout=0.4,
                param_timeout=0.2,
                resume=False,
                mock_worker=str(mock_worker),
                prelude_path=None,
                probe_script=None,
                meta_script=None
            )

            extractor = ParameterExtractor(args)
            output = extractor.run(actions_list)

            self.assertEqual(output["summary"]["totalAssigned"], 3)
            self.assertEqual(output["summary"]["completed"], 3)
            self.assertEqual(output["summary"]["successes"], 2)
            self.assertEqual(output["summary"]["timeouts"], 1)

            # Check that hang_action status is timeout
            self.assertEqual(output["actionStatuses"]["is.workflow.actions.hang_action"]["status"], "timeout")
            # And action3 succeeded!
            self.assertEqual(output["actionStatuses"]["is.workflow.actions.action3"]["status"], "success")

    def test_04_duplicate_records_merged_correctly(self):
        """Test 4: Duplicate records across shards are resolved cleanly and deduplicated."""
        shard1_data = {
            "shardIndex": 0,
            "actionParameters": {"is.workflow.actions.dup": [{"key": "K1", "class": "C1"}]},
            "parameterClasses": {"C1": {"stateClass": "S1", "count": 1, "defaultExamples": [], "usedBy": ["is.workflow.actions.dup"]}},
            "actionStatuses": {"is.workflow.actions.dup": {"status": "success"}},
            "summary": {"shardIndex": 0, "successes": 1}
        }
        shard2_data = {
            "shardIndex": 1,
            "actionParameters": {"is.workflow.actions.dup": [{"key": "K1", "class": "C1"}]},
            "parameterClasses": {"C1": {"stateClass": "S1", "count": 1, "defaultExamples": [], "usedBy": ["is.workflow.actions.dup"]}},
            "actionStatuses": {"is.workflow.actions.dup": {"status": "success"}},
            "summary": {"shardIndex": 1, "successes": 1}
        }

        with tempfile.TemporaryDirectory() as td:
            f1 = pathlib.Path(td) / "s1.json"
            f2 = pathlib.Path(td) / "s2.json"
            f1.write_text(json.dumps(shard1_data))
            f2.write_text(json.dumps(shard2_data))

            merged, summary, _, _ = merge_shards([str(f1), str(f2)], expected_shard_count=2)
            self.assertEqual(len(merged["actionParameters"]), 1)
            # usedBy must not contain duplicates
            self.assertEqual(merged["parameterClasses"]["C1"]["usedBy"], ["is.workflow.actions.dup"])

    def test_05_malformed_shard_data_fails_clearly(self):
        """Test 5: Malformed shard file raises clear ValueError."""
        with tempfile.TemporaryDirectory() as td:
            bad_shard = pathlib.Path(td) / "bad.json"
            bad_shard.write_text("{ unclosed json")

            with self.assertRaises(ValueError) as cm:
                merge_shards([str(bad_shard)])
            self.assertIn("Malformed JSON", str(cm.exception))

    def test_06_missing_shard_distinguished_from_timeout(self):
        """Test 6: Missing shard artifact is distinguished from per-action timeouts."""
        with tempfile.TemporaryDirectory() as td:
            s0 = pathlib.Path(td) / "s0.json"
            s2 = pathlib.Path(td) / "s2.json"
            # We have shard 0 and shard 2, but shard 1 is missing
            s0.write_text(json.dumps({"shardIndex": 0, "actionStatuses": {"a": {"status": "timeout"}}, "summary": {"shardIndex": 0, "timeouts": 1}}))
            s2.write_text(json.dumps({"shardIndex": 2, "actionStatuses": {"b": {"status": "success"}}, "summary": {"shardIndex": 2, "successes": 1}}))

            merged, summary, timeouts, _ = merge_shards([str(s0), str(s2)], expected_shard_count=3)
            # Missing shards contains shard 1
            self.assertEqual(summary["missingShards"], [1])
            # Action timeout inside shard 0 is tracked in actionsTimedOut
            self.assertEqual(summary["actionsTimedOut"], 1)

    def test_07_toolkit_status_handling_no_scope_bug(self):
        """Test 7: ToolKit status handling without Python/shell variable scoping bug."""
        with tempfile.TemporaryDirectory() as td:
            db_path = pathlib.Path(td) / "Tools-prod.v67.sqlite"
            con = sqlite3.connect(str(db_path))
            con.execute("CREATE TABLE Tools (id TEXT PRIMARY KEY, sourceActionProvider TEXT, visibilityFlags INTEGER, sourceContainerId INTEGER)")
            con.execute("CREATE TABLE ContainerMetadata (rowId INTEGER PRIMARY KEY, id TEXT)")
            con.execute("CREATE TABLE Parameters (id TEXT)")
            con.execute("INSERT INTO Tools VALUES ('com.apple.tool1', 'WFLinkActionProvider', 1, 1)")
            con.execute("INSERT INTO Tools VALUES ('com.apple.tool2', 'WFLinkActionProvider', 0, 1)")
            con.execute("INSERT INTO ContainerMetadata VALUES (1, 'com.apple.MobileTimer')")
            con.commit()
            con.close()

            # Test with non-zero exit code (e.g. 2)
            summary_err = inspect_sqlite_database(str(db_path), dump_exit_code=2)
            self.assertIn("dump-toolkit-registry exit 2", summary_err["failures"])
            self.assertEqual(summary_err["Tools"], 2)
            self.assertEqual(summary_err["visible"], 1)
            self.assertEqual(summary_err["hidden"], 1)

            # Test with zero exit code
            summary_ok = inspect_sqlite_database(str(db_path), dump_exit_code=0)
            self.assertEqual(summary_ok["failures"], [])
            self.assertEqual(summary_ok["toolkitDb"], "Tools-prod.v67.sqlite")

    def test_08_app_intent_comparison_schema(self):
        """Test 8: App Intent comparison reads app-provided-actions.json and shortcutActionIdentifier."""
        with tempfile.TemporaryDirectory() as td:
            cherri_file = pathlib.Path(td) / "cherri.json"
            cherri_file.write_text(json.dumps({
                "actions": [{"name": "turnOnAlarm", "shortcutIdentifier": "com.apple.mobiletimer.TurnOnAlarm"}]
            }))

            app_file = pathlib.Path(td) / "app-provided-actions.json"
            app_file.write_text(json.dumps([
                {
                    "bundleIdentifier": "com.apple.mobiletimer",
                    "shortcutActionIdentifier": "com.apple.mobiletimer.TurnOnAlarm",
                    "discoverable": True
                },
                {
                    "bundleIdentifier": "com.apple.mobiletimer",
                    "shortcutActionIdentifier": "com.apple.mobiletimer.TurnOffAlarm",
                    "discoverable": True
                }
            ]))

            summary, _, missing_intents, _ = run_comparison(
                cherri_actions_path=str(cherri_file),
                app_intents_path=str(app_file)
            )

            self.assertEqual(summary["appleAppIntents"]["totalExtracted"], 2)
            self.assertEqual(summary["appleAppIntents"]["totalCovered"], 1)
            self.assertEqual(summary["appleAppIntents"]["totalMissing"], 1)
            self.assertEqual(missing_intents, ["com.apple.mobiletimer.TurnOffAlarm"])

    def test_09_discoverable_app_intents_calculated_separately(self):
        """Test 9: Discoverable vs total App Intent coverage is calculated separately."""
        with tempfile.TemporaryDirectory() as td:
            cherri_file = pathlib.Path(td) / "cherri.json"
            cherri_file.write_text(json.dumps({
                "actions": [
                    {"name": "a1", "shortcutIdentifier": "com.apple.app.a1"},
                    {"name": "a3", "shortcutIdentifier": "com.apple.app.a3"}
                ]
            }))

            app_file = pathlib.Path(td) / "app-provided-actions.json"
            app_file.write_text(json.dumps([
                {"bundleIdentifier": "com.apple.app", "shortcutActionIdentifier": "com.apple.app.a1", "discoverable": True},
                {"bundleIdentifier": "com.apple.app", "shortcutActionIdentifier": "com.apple.app.a2", "discoverable": True},
                {"bundleIdentifier": "com.apple.app", "shortcutActionIdentifier": "com.apple.app.a3", "discoverable": False}
            ]))

            summary, _, _, _ = run_comparison(
                cherri_actions_path=str(cherri_file),
                app_intents_path=str(app_file)
            )

            # Total: 3 (a1, a2, a3), 2 covered (a1, a3), 1 missing (a2)
            self.assertEqual(summary["appleAppIntents"]["totalExtracted"], 3)
            self.assertEqual(summary["appleAppIntents"]["totalCovered"], 2)
            self.assertEqual(summary["appleAppIntents"]["totalMissing"], 1)

            # Discoverable: 2 (a1, a2), 1 covered (a1), 1 missing (a2)
            self.assertEqual(summary["appleAppIntents"]["discoverableTotal"], 2)
            self.assertEqual(summary["appleAppIntents"]["discoverableCovered"], 1)
            self.assertEqual(summary["appleAppIntents"]["discoverableMissing"], 1)

    def test_10_framework_load_parsing_non_null(self):
        """Test 10: Framework-load parsing correctly captures JSON output and cannot silently become null."""
        with tempfile.TemporaryDirectory() as td:
            out_file = pathlib.Path(td) / "private-framework-load.json"
            # In non-macOS test environment, probe_frameworks writes valid mock json
            result = probe_frameworks(str(out_file))
            self.assertIsNotNone(result)
            self.assertIn("WorkflowKit", result)
            self.assertIn("ActionKit", result)

            loaded_disk = json.loads(out_file.read_text())
            self.assertEqual(result, loaded_disk)

    def test_11_snapshot_normalization_preserves_provenance(self):
        """Test 11: Snapshot normalization records multi-source provenance."""
        with tempfile.TemporaryDirectory() as td:
            defs_file = pathlib.Path(td) / "builtin.json"
            defs_file.write_text(json.dumps({
                "is.workflow.actions.shared": {"ActionClass": "WFSharedAction"}
            }))

            dyld_file = pathlib.Path(td) / "dyld.txt"
            dyld_file.write_text("is.workflow.actions.shared\nis.workflow.actions.dyld_only\n")

            app_file = pathlib.Path(td) / "app.json"
            app_file.write_text(json.dumps([
                {"bundleIdentifier": "com.apple.app", "shortcutActionIdentifier": "is.workflow.actions.shared"}
            ]))

            snapshot, manifest = build_snapshot(
                builtin_defs_path=str(defs_file),
                dyld_ids_path=str(dyld_file),
                app_intents_path=str(app_file),
                cherri_sha="abc1234"
            )

            actions = snapshot["actions"]
            self.assertIn("is.workflow.actions.shared", actions)
            self.assertIn("is.workflow.actions.dyld_only", actions)

            shared_sources = actions["is.workflow.actions.shared"]["sources"]
            self.assertEqual(shared_sources, ["appIntent", "builtinDefinition", "dyld"])

            dyld_sources = actions["is.workflow.actions.dyld_only"]["sources"]
            self.assertEqual(dyld_sources, ["dyld"])

            self.assertEqual(manifest["cherriGitSha"], "abc1234")

    def test_12_absolute_runner_paths_sanitized(self):
        """Test 12: Absolute runner paths and DB paths are sanitized to ~."""
        raw_data = {
            "path": "/Users/runner/work/cherri/ToolKit/Tools-prod.v67.sqlite",
            "nested": ["/Users/david/some/path", "normal_string"]
        }
        sanitized = sanitize_paths(raw_data)
        self.assertEqual(sanitized["path"], "~/work/cherri/ToolKit/Tools-prod.v67.sqlite")
        self.assertEqual(sanitized["nested"][0], "~/some/path")
        self.assertEqual(sanitized["nested"][1], "normal_string")


if __name__ == "__main__":
    unittest.main()
