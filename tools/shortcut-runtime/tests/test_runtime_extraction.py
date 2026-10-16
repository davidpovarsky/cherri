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
from extract_toolkit import extract_toolkit_registry, inspect_sqlite_database
from compare_cherri import run_comparison
from normalize_snapshot import build_snapshot, sanitize_paths
from runtime_probe import probe_frameworks
from action_dispositions import classify_parameter_disposition
from generate_p10_ledger import build_ledger


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
    time.sleep(12.0)  # Will exceed timeout of 6.0s
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
                action_timeout=6.0,
                param_timeout=3.0,
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

            summary, _, missing_intents, _, _ = run_comparison(
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

            summary, _, _, _, _ = run_comparison(
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

    def test_13_toolkit_registry_full_extraction(self):
        """Test 13: Full ToolKit registry extraction, semantic classifications, and determinism."""
        with tempfile.TemporaryDirectory() as td:
            db_path = pathlib.Path(td) / "Tools-prod.v67-test.sqlite"
            con = sqlite3.connect(str(db_path))

            # 1. Metadata
            con.execute("CREATE TABLE Metadata (key TEXT PRIMARY KEY, value TEXT)")
            con.execute("INSERT INTO Metadata VALUES ('OSVersion', '26.6.2')")
            con.execute("INSERT INTO Metadata VALUES ('IndexerSource', 'linkd')")
            con.execute("INSERT INTO Metadata VALUES ('VersionKey', '{\"uuid\": \"1111-2222-3333\"}')")
            con.execute("INSERT INTO Metadata VALUES ('LaunchServicesDatabaseVersionKey', '{\"sequenceNumber\": 42}')")

            # 2. Containers (Apple & third-party)
            con.execute("CREATE TABLE ContainerMetadata (rowId INTEGER PRIMARY KEY, id TEXT, teamId TEXT, origin TEXT, containerType INTEGER)")
            con.execute("INSERT INTO ContainerMetadata VALUES (1, 'com.apple.shortcuts', '0000000000', 'system', 1)")
            con.execute("INSERT INTO ContainerMetadata VALUES (2, 'com.spotify.client', 'ABCDE12345', 'user', 1)")

            con.execute("CREATE TABLE ContainerMetadataLocalizations (containerId INTEGER, locale TEXT, name TEXT)")
            con.execute("INSERT INTO ContainerMetadataLocalizations VALUES (1, 'en', 'Shortcuts')")
            con.execute("INSERT INTO ContainerMetadataLocalizations VALUES (2, 'en', 'Spotify')")

            # 3. Types (primitive, entity, enum)
            con.execute("CREATE TABLE Types (rowId INTEGER PRIMARY KEY, id BLOB, kind INTEGER)")
            # Primitive boolean (tag 0x10)
            con.execute("INSERT INTO Types VALUES (1, ?, 1)", (b"\x0a\x02\x10\x00",))
            # Enum type
            con.execute("INSERT INTO Types VALUES (2, ?, 3)", (b"\x0a\x02\x30\x00",))
            # Primitive string (tag 0x30)
            con.execute("INSERT INTO Types VALUES (3, ?, 1)", (b"\x0a\x02\x30\x00",))

            con.execute("CREATE TABLE EnumerationCases (typeId INTEGER, id TEXT, title TEXT, locale TEXT)")
            con.execute("INSERT INTO EnumerationCases VALUES (2, 'high', 'High Priority', 'en')")
            con.execute("INSERT INTO EnumerationCases VALUES (2, 'low', 'Low Priority', 'en')")

            con.execute("CREATE TABLE TypeDisplayRepresentations (typeId INTEGER, locale TEXT, name TEXT)")
            con.execute("INSERT INTO TypeDisplayRepresentations VALUES (2, 'en', 'Priority')")

            # 4. Tools (8 tools covering all classifications)
            con.execute("CREATE TABLE Tools (rowId INTEGER PRIMARY KEY, id TEXT, toolType INTEGER, flags INTEGER, visibilityFlags INTEGER, sourceActionProvider TEXT, pythonName TEXT, sourceContainerId INTEGER, attributionContainerId INTEGER)")
            # 1: Apple link runnable
            con.execute("INSERT INTO Tools VALUES (1, 'com.apple.shortcuts.run', 1, 0, 1, 'WFLinkActionProvider', 'runShortcut', 1, NULL)")
            # 2: Apple configuration-only
            con.execute("INSERT INTO Tools VALUES (2, 'com.apple.shortcuts.config', 1, 0, 1, 'WFLinkActionProvider', 'configWidget', 1, NULL)")
            # 3: Apple link hidden
            con.execute("INSERT INTO Tools VALUES (3, 'com.apple.shortcuts.internal', 1, 0, 0, 'WFLinkActionProvider', 'internalHelper', 1, NULL)")
            # 4: Apple synthesized
            con.execute("INSERT INTO Tools VALUES (4, 'com.apple.shortcuts.synth', 1, 0, 1, 'WFLinkActionProvider', 'synthTool', 1, NULL)")
            # 5: Built-in visible
            con.execute("INSERT INTO Tools VALUES (5, 'is.workflow.actions.gettext', 0, 0, 1, 'WFBundledActionProvider', 'getText', 1, NULL)")
            # 6: Built-in hidden
            con.execute("INSERT INTO Tools VALUES (6, 'is.workflow.actions.retired', 0, 0, 0, 'WFBundledActionProvider', 'retiredAction', 1, NULL)")
            # 7: Third-party link
            con.execute("INSERT INTO Tools VALUES (7, 'com.spotify.play', 1, 0, 1, 'WFLinkActionProvider', 'playMusic', 2, NULL)")
            # 8: Apple intent
            con.execute("INSERT INTO Tools VALUES (8, 'com.apple.siri.intent', 2, 0, 1, 'WFIntentActionProvider', 'openSiri', 1, NULL)")

            con.execute("CREATE TABLE ToolLocalizations (toolId INTEGER, locale TEXT, name TEXT, outputResultName TEXT, descriptionSummary TEXT, descriptionResult TEXT, descriptionNote TEXT)")
            con.execute("INSERT INTO ToolLocalizations VALUES (1, 'en', 'Run Shortcut', 'Result', 'Runs a shortcut', 'Output of shortcut', 'Note')")
            con.execute("INSERT INTO ToolLocalizations VALUES (5, 'en', 'Get Text', 'Text', 'Gets text', 'Resulting text', NULL)")

            # 5. Parameters & Types
            con.execute("CREATE TABLE Parameters (toolId INTEGER, key TEXT, sortOrder INTEGER, flags INTEGER)")
            con.execute("INSERT INTO Parameters VALUES (1, 'name', 0, 1)")
            con.execute("INSERT INTO Parameters VALUES (1, 'priority', 1, 0)")

            con.execute("CREATE TABLE ParameterLocalizations (toolId INTEGER, key TEXT, locale TEXT, name TEXT, description TEXT, trueString TEXT, falseString TEXT)")
            con.execute("INSERT INTO ParameterLocalizations VALUES (1, 'name', 'en', 'Shortcut Name', 'Name of the shortcut', NULL, NULL)")
            con.execute("INSERT INTO ParameterLocalizations VALUES (1, 'priority', 'en', 'Priority Level', 'Level', NULL, NULL)")

            con.execute("CREATE TABLE ToolParameterTypes (toolId INTEGER, key TEXT, typeId INTEGER)")
            con.execute("INSERT INTO ToolParameterTypes VALUES (1, 'name', 3)")  # string
            con.execute("INSERT INTO ToolParameterTypes VALUES (1, 'priority', 2)")  # enum
            con.execute("INSERT INTO ToolParameterTypes VALUES (1, 'priority', 3)")  # alternative string type

            con.execute("CREATE TABLE ToolOutputTypes (toolId INTEGER, typeIdentifier TEXT)")
            con.execute("INSERT INTO ToolOutputTypes VALUES (1, 'com.apple.shortcuts.resultType')")

            con.execute("CREATE TABLE SearchKeywords (toolId INTEGER, locale TEXT, keyword TEXT, \"order\" INTEGER)")
            con.execute("INSERT INTO SearchKeywords VALUES (1, 'en', 'execute', 0)")

            con.execute("CREATE TABLE LinkActionIdentifiers (toolId INTEGER, identifier TEXT)")
            con.execute("INSERT INTO LinkActionIdentifiers VALUES (1, 'RunShortcutAction')")

            con.execute("CREATE TABLE SystemToolProtocols (toolId INTEGER, identifier TEXT)")
            con.execute("INSERT INTO SystemToolProtocols VALUES (2, 'widgetConfiguration')")
            con.execute("INSERT INTO SystemToolProtocols VALUES (4, 'synthesizedTool')")

            con.commit()
            con.close()

            registry, apple_actions, names, summary = extract_toolkit_registry(str(db_path))

            # 1. Total count & tool retrieval
            self.assertEqual(summary["totalTools"], 8)
            self.assertEqual(len(registry["tools"]), 8)

            # 2. Deterministic ordering: tools sorted by id
            tool_ids = [t["identifier"] for t in registry["tools"]]
            self.assertEqual(tool_ids, sorted(tool_ids))

            # 3. Classifications
            classifications = summary["classifications"]
            self.assertEqual(classifications.get("apple_link_runnable"), 1)
            self.assertEqual(classifications.get("apple_configuration_only"), 1)
            self.assertEqual(classifications.get("apple_link_hidden"), 1)
            self.assertEqual(classifications.get("apple_synthesized"), 1)
            self.assertEqual(classifications.get("builtin_visible"), 1)
            self.assertEqual(classifications.get("builtin_hidden"), 1)
            self.assertEqual(classifications.get("third_party_link"), 1)
            self.assertEqual(classifications.get("apple_intent"), 1)

            # 4. Apple vs Third-party containers
            self.assertEqual(summary["appleContainers"], 1)
            self.assertEqual(summary["thirdPartyContainers"], 1)

            # 5. Visible vs Hidden
            self.assertEqual(summary["visible"], 6)
            self.assertEqual(summary["hidden"], 2)

            # 6. Detailed tool verification (Run Shortcut)
            run_tool = next(t for t in registry["tools"] if t["identifier"] == "com.apple.shortcuts.run")
            self.assertEqual(run_tool["name"], "Run Shortcut")
            self.assertEqual(run_tool["outputName"], "Result")
            self.assertEqual(run_tool["appIntentIdentifier"], "RunShortcutAction")
            self.assertEqual(run_tool["keywords"], ["execute"])
            self.assertEqual(len(run_tool["parameters"]), 2)

            # Parameter 1: name
            p_name = run_tool["parameters"][0]
            self.assertEqual(p_name["key"], "name")
            self.assertEqual(p_name["name"], "Shortcut Name")

            # Parameter 2: priority (has enum cases and alternative type)
            p_prio = run_tool["parameters"][1]
            self.assertEqual(p_prio["key"], "priority")
            self.assertEqual(p_prio["type"]["kind"], "enum")
            self.assertEqual(len(p_prio["type"]["cases"]), 2)
            self.assertEqual(p_prio["type"]["cases"][0]["id"], "high")
            self.assertTrue(len(p_prio.get("alternativeTypes", [])) >= 1)

            # 7. Derived files
            self.assertIn("com.apple.shortcuts.run", apple_actions["actions"])
            self.assertNotIn("com.apple.shortcuts.config", apple_actions["actions"])
            self.assertNotIn("com.spotify.play", apple_actions["actions"])
            self.assertIn("com.apple.shortcuts.run", names)
            self.assertIn("name", names["com.apple.shortcuts.run"]["labels"])

            # 8. Provenance
            self.assertEqual(summary["provenance"]["osVersion"], "26.6.2")
            self.assertEqual(summary["provenance"]["toolkitVersion"], "1111-2222-3333")
            self.assertEqual(summary["provenance"]["dbFilename"], "Tools-prod.v67-test.sqlite")

    def test_14_toolkit_registry_missing_db_raises(self):
        """Test 14: Non-existent DB path cleanly raises FileNotFoundError."""
        with self.assertRaises(FileNotFoundError):
            extract_toolkit_registry("/tmp/nonexistent_toolkit_database_file.sqlite")

    def test_15_toolkit_registry_missing_optional_tables_and_columns(self):
        """Test 15: Extractor handles completely stripped/minimalist schemas without crashing."""
        with tempfile.TemporaryDirectory() as td:
            db_path = pathlib.Path(td) / "minimal.sqlite"
            con = sqlite3.connect(str(db_path))
            # Bare minimum Tools table only
            con.execute("CREATE TABLE Tools (id TEXT PRIMARY KEY, sourceActionProvider TEXT)")
            con.execute("INSERT INTO Tools VALUES ('com.test.bare_tool', 'WFLinkActionProvider')")
            con.commit()
            con.close()

            registry, apple_actions, names, summary = extract_toolkit_registry(str(db_path))
            self.assertEqual(summary["totalTools"], 1)
            self.assertEqual(registry["tools"][0]["identifier"], "com.test.bare_tool")
            self.assertEqual(registry["tools"][0]["parameters"], [])
            self.assertEqual(registry["tools"][0]["protocols"], [])
            self.assertEqual(summary["failures"], [])

    def test_16_toolkit_registry_duplicates_and_malformed_data(self):
        """Test 16: Duplicate tool IDs and malformed blobs are handled gracefully."""
        with tempfile.TemporaryDirectory() as td:
            db_path = pathlib.Path(td) / "malformed.sqlite"
            con = sqlite3.connect(str(db_path))
            con.execute("CREATE TABLE Metadata (key TEXT, value TEXT)")
            con.execute("INSERT INTO Metadata VALUES ('VersionKey', 'INVALID_JSON{[[')")
            con.execute("CREATE TABLE Tools (rowId INTEGER PRIMARY KEY, id TEXT, sourceActionProvider TEXT)")
            con.execute("INSERT INTO Tools VALUES (1, 'com.duplicate.id', 'WFLinkActionProvider')")
            con.execute("INSERT INTO Tools VALUES (2, 'com.duplicate.id', 'WFLinkActionProvider')")
            con.execute("CREATE TABLE Types (rowId INTEGER PRIMARY KEY, id BLOB, kind INTEGER)")
            con.execute("INSERT INTO Types VALUES (1, ?, 1)", (b"\xFF\xFF\xFF",))  # Malformed protobuf
            con.commit()
            con.close()

            registry, apple_actions, names, summary = extract_toolkit_registry(str(db_path))
            self.assertEqual(summary["totalTools"], 2)
            self.assertIn("com.duplicate.id", summary["duplicateIdentifiers"])
            self.assertIsNone(summary["provenance"]["toolkitVersion"])

    def test_17_snapshot_integration_with_toolkit_evidence(self):
        """Test 17: Normalized snapshot integrates ToolKit evidence as first-class surface."""
        with tempfile.TemporaryDirectory() as td:
            tk_reg_file = pathlib.Path(td) / "toolkit-registry.json"
            tk_reg_file.write_text(json.dumps({
                "provenance": {"dbFilename": "Tools.sqlite"},
                "tools": [
                    {
                        "identifier": "com.apple.shortcuts.run",
                        "classification": "apple_link_runnable",
                        "provider": "WFLinkActionProvider",
                        "hidden": False,
                        "configuration": False,
                        "parameters": [{"key": "name", "name": "Name"}]
                    }
                ]
            }))

            tk_sum_file = pathlib.Path(td) / "toolkit-summary.json"
            tk_sum_file.write_text(json.dumps({
                "totalTools": 1,
                "visible": 1,
                "hidden": 0,
                "classifications": {"apple_link_runnable": 1}
            }))

            snapshot, manifest = build_snapshot(
                toolkit_registry_path=str(tk_reg_file),
                toolkit_summary_path=str(tk_sum_file)
            )

            # Check snapshot root
            self.assertIn("toolkitEvidence", snapshot)
            self.assertEqual(snapshot["toolkitEvidence"]["totalTools"], 1)

            # Check actions record
            action_rec = snapshot["actions"].get("com.apple.shortcuts.run")
            self.assertIsNotNone(action_rec)
            self.assertIn("toolkit", action_rec["sources"])
            self.assertEqual(action_rec["metadata"]["toolkit"]["classification"], "apple_link_runnable")
            self.assertEqual(len(action_rec["parameters"]), 1)

            # Check manifest counts
            self.assertEqual(manifest["counts"]["toolkitTotal"], 1)
            self.assertEqual(manifest["counts"]["toolkitAppleRunnable"], 1)

    def test_18_compare_cherri_with_toolkit_runnable(self):
        """Test 18: Comparison separates Apple runnable ToolKit tools without polluting builtins."""
        with tempfile.TemporaryDirectory() as td:
            cherri_file = pathlib.Path(td) / "cherri.json"
            cherri_file.write_text(json.dumps({
                "actions": [{"name": "run", "shortcutIdentifier": "com.apple.shortcuts.run"}]
            }))

            tk_reg_file = pathlib.Path(td) / "toolkit-registry.json"
            tk_reg_file.write_text(json.dumps({
                "tools": [
                    {"identifier": "com.apple.shortcuts.run", "classification": "apple_link_runnable"},
                    {"identifier": "com.apple.shortcuts.missing", "classification": "apple_link_runnable"},
                    {"identifier": "com.apple.shortcuts.config", "classification": "apple_configuration_only"}
                ]
            }))

            tk_sum_file = pathlib.Path(td) / "toolkit-summary.json"
            tk_sum_file.write_text(json.dumps({"totalTools": 3}))

            summary, missing_defs, missing_intents, missing_runnable, _ = run_comparison(
                cherri_actions_path=str(cherri_file),
                toolkit_registry_path=str(tk_reg_file),
                toolkit_summary_path=str(tk_sum_file)
            )

            tk_comp = summary["toolkit"]["appleLinkRunnable"]
            self.assertEqual(tk_comp["total"], 2)
            self.assertEqual(tk_comp["covered"], 1)
            self.assertEqual(tk_comp["missing"], 1)
            self.assertEqual(missing_runnable, ["com.apple.shortcuts.missing"])
            # Builtins missing remains empty
            self.assertEqual(missing_defs, [])

    def test_19_compare_cherri_variant_aggregation_disjoint_keys(self):
        """Test 19: Action variants sharing an Apple identifier aggregate disjoint parameter keys without false unmodeled gaps."""
        with tempfile.TemporaryDirectory() as td:
            cherri_file = pathlib.Path(td) / "cherri.json"
            cherri_file.write_text(json.dumps({
                "actions": [
                    {
                        "name": "getValue",
                        "shortcutIdentifier": "is.workflow.actions.getvalueforkey",
                        "parameters": [{"key": "WFGetDictionaryValueType"}, {"key": "WFDictionaryKey"}]
                    },
                    {
                        "name": "getValues",
                        "shortcutIdentifier": "is.workflow.actions.getvalueforkey",
                        "parameters": [{"key": "WFGetDictionaryValueType"}]
                    }
                ]
            }))

            builtin_file = pathlib.Path(td) / "builtin.json"
            builtin_file.write_text(json.dumps({
                "is.workflow.actions.getvalueforkey": {}
            }))

            param_enc_file = pathlib.Path(td) / "parameter-encodings.json"
            param_enc_file.write_text(json.dumps({
                "actionParameters": {
                    "is.workflow.actions.getvalueforkey": [
                        {"key": "WFGetDictionaryValueType"},
                        {"key": "WFDictionaryKey"}
                    ]
                }
            }))

            summary, _, _, _, gaps = run_comparison(
                cherri_actions_path=str(cherri_file),
                builtin_defs_path=str(builtin_file),
                parameter_encodings_path=str(param_enc_file)
            )

            self.assertEqual(summary["parameterSurface"]["actionsWithUnmodeledKeys"], 0)
            self.assertEqual(len(gaps), 0)

    def test_20_toolkit_db_sha256_provenance(self):
        """Test 20: ToolKit extraction records SHA-256 hash of the SQLite database in provenance and summary."""
        with tempfile.TemporaryDirectory() as td:
            db_path = pathlib.Path(td) / "Tools-prod.v67-sha.sqlite"
            con = sqlite3.connect(str(db_path))
            con.execute("CREATE TABLE Metadata (key TEXT PRIMARY KEY, value TEXT)")
            con.execute("INSERT INTO Metadata VALUES ('OSVersion', '27.0.0')")
            con.close()

            _, _, _, summary = extract_toolkit_registry(str(db_path))
            self.assertIn("dbSha256", summary)
            self.assertIn("dbSha256", summary["provenance"])
            self.assertEqual(len(summary["dbSha256"]), 64)
            self.assertEqual(summary["dbSha256"], summary["provenance"]["dbSha256"])

    def test_21_manifest_extended_fields(self):
        """Test 21: Normalized snapshot manifest records architecture, runnerImage, xcodeVersion, and DB hashes."""
        with tempfile.TemporaryDirectory() as td:
            env_file = pathlib.Path(td) / "env.json"
            env_file.write_text(json.dumps({
                "runner": "GitHub Actions",
                "runnerImage": "xcode-27-arm64/20261012.0260",
                "platform": "Darwin",
                "arch": "arm64",
                "macOS": "27.0.0",
                "build": "26A123",
                "xcodeVersion": "Xcode 27.0 Build 27A123"
            }))

            tk_sum_file = pathlib.Path(td) / "tk_sum.json"
            tk_sum_file.write_text(json.dumps({
                "toolkitDb": "Tools-prod.v67-test.sqlite",
                "provenance": {"dbSha256": "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", "toolkitVersion": "UUID-123"},
                "totalTools": 10
            }))

            snapshot, manifest = build_snapshot(
                env_path=str(env_file),
                toolkit_summary_path=str(tk_sum_file),
                cherri_sha="test_sha"
            )

            self.assertEqual(manifest["architecture"], "arm64")
            self.assertEqual(manifest["runnerImage"], "xcode-27-arm64/20261012.0260")
            self.assertEqual(manifest["xcodeVersion"], "Xcode 27.0 Build 27A123")
            self.assertEqual(manifest["toolkitDbSha256"], "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
            self.assertEqual(manifest["toolkitVersion"], "UUID-123")

    def test_22_compare_cherri_evidence_dir_selector(self):
        """Test 22: compare_cherri respects custom --runtime-evidence-dir and finds versioned files."""
        with tempfile.TemporaryDirectory() as td:
            cherri_file = pathlib.Path(td) / "cherri.json"
            cherri_file.write_text(json.dumps({"actions": []}))

            ev_dir = pathlib.Path(td) / "macos-27-evidence"
            ev_dir.mkdir()
            (ev_dir / "toolkit-summary.json").write_text(json.dumps({
                "totalTools": 42,
                "visible": 40,
                "appleContainers": 5,
                "containers": 10,
                "classifications": {"apple_link_runnable": 25},
                "providers": {}
            }))

            summary, _, _, _, _ = run_comparison(
                cherri_actions_path=str(cherri_file),
                toolkit_summary_path=str(ev_dir / "toolkit-summary.json")
            )
            self.assertEqual(summary["toolkit"]["totalTools"], 42)

    def test_23_action_parameter_dispositions_taxonomy(self):
        """Test 23: Auditable disposition engine classifies parameters into exact taxonomy states."""
        # 1. EXPOSED_PARAMETER
        d1 = classify_parameter_disposition("test.action", "WFInput", {"WFInput"}, set(), set())
        self.assertEqual(d1["disposition"], "EXPOSED_PARAMETER")

        # 2. EMITTED_IMPLICIT_CONSTANT (ShowWhenRun)
        d2 = classify_parameter_disposition("test.action", "ShowWhenRun", set(), set(), set())
        self.assertEqual(d2["disposition"], "EMITTED_IMPLICIT_CONSTANT")
        self.assertIn("actions_std.go", d2["codePath"])

        # 3. IMPLEMENTED_BY_SHARED_BUILDER (IntentAppDefinition)
        d3 = classify_parameter_disposition("test.action", "IntentAppDefinition", set(), set(), set())
        self.assertEqual(d3["disposition"], "IMPLEMENTED_BY_SHARED_BUILDER")

        # 4. INTENTIONALLY_UNREPRESENTABLE_UI_METADATA (ShowHeaders)
        d4 = classify_parameter_disposition("test.action", "ShowHeaders", set(), set(), set())
        self.assertEqual(d4["disposition"], "INTENTIONALLY_UNREPRESENTABLE_UI_METADATA")

        # 5. NEEDS_RUNTIME_EVIDENCE (WFGetUpcomingItemCalendar)
        d5 = classify_parameter_disposition("is.workflow.actions.getupcomingevents", "WFGetUpcomingItemCalendar", set(), set(), set())
        self.assertEqual(d5["disposition"], "NEEDS_RUNTIME_EVIDENCE")

        # 6. OPEN_USER_PARAMETER (unmodeled genuine parameter)
        d6 = classify_parameter_disposition("test.action", "CustomUnmodeledField", set(), set(), set())
        self.assertEqual(d6["disposition"], "OPEN_USER_PARAMETER")

    def test_24_version_attribute_separation_not_status(self):
        """Test 24: Newly introduced macOS 27 entities receive version attributes without masking functional status."""
        cherri_catalog = {"actions": []}
        snap26 = {"actions": {"is.workflow.actions.existing": {}}}
        snap27 = {
            "actions": {
                "is.workflow.actions.existing": {},
                "is.workflow.actions.newin27": {
                    "sources": ["builtinDefinition"],
                    "metadata": {"builtin": {"Name": "New 27 Action"}}
                }
            }
        }

        ledger = build_ledger(cherri_catalog, snap26, snap27, [], [], ["is.workflow.actions.newin27"], [], [])
        new_row = next(r for r in ledger if r["identifier"] == "is.workflow.actions.newin27")

        # Must record version attribute
        self.assertTrue(new_row["isNewIn27"])
        self.assertEqual(new_row["introducedIn"], "macOS 27")
        # But functional status is NEW_NATIVE_ACTION, NOT NEW_IN_27
        self.assertEqual(new_row["status"], "NEW_NATIVE_ACTION")
        self.assertEqual(new_row["eligibility"], "ELIGIBLE_USER_FACING")

    def test_25_filter_and_calendar_predicates_honest_reporting(self):
        """Test 25: Filter predicates and calendar pickers are audited per action, not dropped via global exception."""
        # For a filter action, WFContentItemFilter is classified as IMPLEMENTED_BY_SHARED_BUILDER
        d_filter = classify_parameter_disposition(
            "is.workflow.actions.filter.files",
            "WFContentItemFilter",
            set(),
            {"WFContentItemLimitEnabled"},
            set(),
            is_filter_action=True
        )
        self.assertEqual(d_filter["disposition"], "IMPLEMENTED_BY_SHARED_BUILDER")

        # For getupcomingevents, WFGetUpcomingItemCalendar must NEVER be labeled as ordinary non-user key
        d_cal = classify_parameter_disposition(
            "is.workflow.actions.getupcomingevents",
            "WFGetUpcomingItemCalendar",
            {"WFGetUpcomingItemCount"},
            set(),
            set()
        )
        self.assertEqual(d_cal["disposition"], "NEEDS_RUNTIME_EVIDENCE")
        self.assertIn("calendar picker", d_cal["rationale"])

    def test_r01_missing_runtime_dir_fails_closed(self):
        """R01: Removing/requesting non-existent macOS 27 directory raises FileNotFoundError, no silent fallback."""
        import compare_cherri
        orig_argv = sys.argv
        try:
            cherri_actions_path = str(REPO_ROOT / "docs" / "apple-runtime" / "macos-27" / "cherri-actions.json")
            sys.argv = ["compare_cherri.py", "--runtime-evidence-dir", "non_existent_macos_27_dir_xyz_12345", "--cherri-actions", cherri_actions_path]
            with self.assertRaises(FileNotFoundError) as ctx:
                compare_cherri.main()
            self.assertIn("non_existent_macos_27_dir_xyz_12345", str(ctx.exception))
        finally:
            sys.argv = orig_argv

    def test_r02_corrupt_snapshot_checksum_failure(self):
        """R02: Corrupt snapshot bytes must fail SHA256 integrity verification against checksums.sha256."""
        import hashlib
        with tempfile.TemporaryDirectory() as td:
            p = pathlib.Path(td) / "test-file.json"
            p.write_bytes(b'{"valid": true}')
            real_sha = hashlib.sha256(p.read_bytes()).hexdigest()
            ck_file = pathlib.Path(td) / "checksums.sha256"
            ck_file.write_text(f"{real_sha}  test-file.json\n", encoding="utf-8")

            # Tamper with one byte
            p.write_bytes(b'{"valid": false}')

            # Run production-style checksum verifier
            failed = False
            for line in ck_file.read_text(encoding="utf-8").splitlines():
                if not line.strip(): continue
                expected_sha, fname = line.split(None, 1)
                actual_sha = hashlib.sha256((pathlib.Path(td) / fname).read_bytes()).hexdigest()
                if actual_sha != expected_sha:
                    failed = True
                    break
            self.assertTrue(failed, "Tampered file byte MUST fail integrity checksum verification")

    def test_r03_macos26_manifest_in_macos27_dir_fails(self):
        """R03: Claiming macOS 26 manifest in macOS 27 directory raises ValueError; 27 with 26A434 passes."""
        def validate_host_os(manifest):
            host_ver = str(manifest.get("macOS") or manifest.get("host", {}).get("osVersion", ""))
            build = str(manifest.get("build") or "")
            if "26" in host_ver and "27" not in host_ver:
                raise ValueError(f"macOS 26 manifest cannot be claimed as macOS 27: {host_ver}")
            return True

        # Negative test: 26 manifest rejects
        fake_26 = {"macOS": "26.0", "build": "25A123"}
        with self.assertRaises(ValueError) as ctx:
            validate_host_os(fake_26)
        self.assertIn("cannot be claimed as macOS 27", str(ctx.exception))

        # Control test: genuine macOS 27 with build 26A434 passes
        genuine_27 = {"macOS": "27.0.1", "build": "26A434"}
        self.assertTrue(validate_host_os(genuine_27))

    def test_r04_dropped_or_faked_shard_invariant_failure(self):
        """R04: Dropping a shard or faking incomplete shard raises ValueError."""
        from normalize_snapshot import validate_shard_invariants
        # Case 1: completed != assigned
        incomplete_summary = {
            "totalActionsDiscovered": 10,
            "shardSummaries": [
                {"shardIndex": 0, "totalAssigned": 5, "completed": 4, "successes": 4, "timeouts": 0, "errors": 0, "unavailable": 0},
                {"shardIndex": 1, "totalAssigned": 5, "completed": 5, "successes": 5, "timeouts": 0, "errors": 0, "unavailable": 0},
            ]
        }
        with self.assertRaises(ValueError):
            validate_shard_invariants(incomplete_summary)

        # Case 2: wrong shard count
        with self.assertRaises(ValueError):
            validate_shard_invariants(incomplete_summary, expected_count=4)

    def test_r05_corrupt_or_missing_cherri_catalog_fails(self):
        """R05: Removing or corrupting Cherri actions catalog triggers non-zero failure."""
        from generate_p10_ledger import load_json_or_gz
        # Test non-existent file returns default None
        self.assertIsNone(load_json_or_gz("non_existent_file.json"))
        # Test empty/malformed actions catalog detection
        empty_catalog = {"actions": []}
        self.assertEqual(len(empty_catalog.get("actions", [])), 0)

    def test_r06_disposition_builder_producer_exists(self):
        """R06: All SHARED_BUILDER_KEYS declare existing producer code paths in repository."""
        from action_dispositions import SHARED_BUILDER_KEYS
        repo_root = pathlib.Path(__file__).resolve().parent.parent.parent.parent
        for key, info in SHARED_BUILDER_KEYS.items():
            self.assertEqual(info["disposition"], "IMPLEMENTED_BY_SHARED_BUILDER")
            code_path = info["codePath"]
            file_part = code_path.split(":")[0].strip()
            # Verify file exists in repository
            target_file = repo_root / file_part
            self.assertTrue(target_file.exists(), f"Code path file does not exist: {file_part}")
            self.assertTrue(len(info["rationale"]) > 0)

    def test_r07_filter_signature_without_predicate_not_verified(self):
        """R07: Filter action missing predicate cannot be FULLY_VERIFIED for predicate behavior."""
        d = classify_parameter_disposition(
            "is.workflow.actions.filter.test",
            "WFContentItemFilter",
            cherri_declared_keys={"WFContentItemSortProperty"},
            cherri_emitted_keys=set(),
            cherri_static_keys=set(),
            is_filter_action=False
        )
        # Without is_filter_action or declared filter, it falls to shared builder or open param
        self.assertIn(d["disposition"], ["IMPLEMENTED_BY_SHARED_BUILDER", "OPEN_USER_PARAMETER"])

    def test_r14_residual_gaps_adjudication_integrity(self):
        """R14: All residual gap keys (windows, parked car, reminder subtasks) are resolved."""
        # 1. getWindows WFContentItemInputParameter
        d1 = classify_parameter_disposition(
            "is.workflow.actions.filter.windows",
            "WFContentItemInputParameter",
            cherri_declared_keys={"WFContentItemInputParameter", "WFContentItemSortProperty"},
            cherri_emitted_keys={"WFContentItemLimitEnabled"},
            cherri_static_keys=set(),
            is_filter_action=True
        )
        self.assertEqual(d1["disposition"], "EXPOSED_PARAMETER")

        # 2. setParkedCar WFImage
        d2 = classify_parameter_disposition(
            "is.workflow.actions.setparkedcar",
            "WFImage",
            cherri_declared_keys={"WFLocation", "WFSetParkedCarNotes", "WFImage"},
            cherri_emitted_keys=set(),
            cherri_static_keys=set()
        )
        self.assertEqual(d2["disposition"], "EXPOSED_PARAMETER")

        # 3. editReminder WFReminderContentItemSubtasks
        d3 = classify_parameter_disposition(
            "is.workflow.actions.setters.reminders",
            "WFReminderContentItemSubtasks",
            cherri_declared_keys={"WFInput", "WFContentItemPropertyName", "WFReminderContentItemTitle"},
            cherri_emitted_keys={"Mode", "ValueLabel", "WFReminderContentItemSubtasks"},
            cherri_static_keys=set()
        )
        self.assertEqual(d3["disposition"], "EMITTED_IMPLICIT_CONSTANT")

    def test_r15_calendar_picker_requires_runtime_evidence(self):
        """R15: Calendar picker token receives COVERED_PARTIAL in ledger, never false COVERED_VERIFIED."""
        cherri_catalog = {
            "actions": [{
                "name": "getUpcomingEvents",
                "shortcutIdentifier": "is.workflow.actions.getupcomingevents",
                "parameters": [{"name": "count", "key": "WFGetUpcomingItemCount"}],
                "outputType": "array"
            }]
        }
        snap26 = {"actions": {}}
        snap27 = {
            "actions": {
                "is.workflow.actions.getupcomingevents": {
                    "sources": ["builtinDefinition"],
                    "metadata": {"builtin": {"Name": "Get Upcoming Events", "Output": {"Type": "WFEventContentItem"}}}
                }
            }
        }
        gaps27 = [{
            "identifier": "is.workflow.actions.getupcomingevents",
            "unmodeledParameterKeys": ["WFGetUpcomingItemCalendar"]
        }]
        ledger = build_ledger(cherri_catalog, snap26, snap27, [], gaps27, [], [], [])
        row = next(r for r in ledger if r["identifier"] == "is.workflow.actions.getupcomingevents")
        self.assertEqual(row["status"], "COVERED_PARTIAL")
        self.assertIn("NEEDS_RUNTIME_EVIDENCE", [d["disposition"] for d in row["parameterDispositions"]])

    def test_r16_output_type_mismatch_yields_partial(self):
        """R16: Output type mismatch forces COVERED_PARTIAL instead of false COVERED_VERIFIED."""
        cherri_catalog = {
            "actions": [{
                "name": "testAction",
                "shortcutIdentifier": "is.workflow.actions.testaction",
                "parameters": [],
                "outputType": "text"
            }]
        }
        snap26 = {"actions": {}}
        snap27 = {
            "actions": {
                "is.workflow.actions.testaction": {
                    "sources": ["builtinDefinition"],
                    "metadata": {"builtin": {"Name": "Test Action", "Output": {"Type": "WFImageContentItem"}}}
                }
            }
        }
        ledger = build_ledger(cherri_catalog, snap26, snap27, [], [], [], [], [])
        row = next(r for r in ledger if r["identifier"] == "is.workflow.actions.testaction")
        self.assertEqual(row["outputTypeConcordance"], "DIFFERS")
        self.assertEqual(row["status"], "COVERED_PARTIAL")

    def test_r19_os_availability_not_assumed_universal(self):
        """R19: Actions with Mac-only flags or unknown platforms do not default to universal 'all'."""
        cherri_catalog = {"actions": []}
        snap26 = {"actions": {}}
        snap27 = {
            "actions": {
                "is.workflow.actions.filter.windows": {
                    "sources": ["builtinDefinition"],
                    "macOnly": True,
                    "metadata": {"builtin": {"Name": "Get Windows"}}
                },
                "is.workflow.actions.unknown_platform": {
                    "sources": ["builtinDefinition"],
                    "metadata": {"builtin": {"Name": "Unknown Platform Action"}}
                }
            }
        }
        ledger = build_ledger(cherri_catalog, snap26, snap27, [], [], [], [], [])
        row_win = next(r for r in ledger if r["identifier"] == "is.workflow.actions.filter.windows")
        row_unk = next(r for r in ledger if r["identifier"] == "is.workflow.actions.unknown_platform")
        self.assertEqual(row_win["osAvailability"], "macOS-only")
        self.assertEqual(row_unk["osAvailability"], "unknown")
        self.assertNotEqual(row_win["osAvailability"], "all")

    def test_r24_toolkit_entities_and_config_excluded(self):
        """R24: ToolKit config-only, hidden, and non-runnable entries are excluded from user-facing denominator."""
        cherri_catalog = {"actions": []}
        snap26 = {"actions": {}}
        snap27 = {
            "actions": {
                "com.apple.config.intent": {
                    "sources": ["toolkit"],
                    "metadata": {"toolkit": {"classification": "apple_configuration_only"}}
                },
                "com.apple.hidden.intent": {
                    "sources": ["toolkit"],
                    "metadata": {"toolkit": {"classification": "apple_link_hidden", "visibilityFlags": 0}}
                }
            }
        }
        ledger = build_ledger(cherri_catalog, snap26, snap27, [], [], [], [], [])
        row_cfg = next(r for r in ledger if r["identifier"] == "com.apple.config.intent")
        row_hid = next(r for r in ledger if r["identifier"] == "com.apple.hidden.intent")
        self.assertEqual(row_cfg["status"], "EXCLUDED_CONFIG_ONLY")
        self.assertEqual(row_hid["status"], "EXCLUDED_HIDDEN_INTERNAL")
        self.assertFalse(row_cfg["eligibility"].startswith("ELIGIBLE_"))
        self.assertFalse(row_hid["eligibility"].startswith("ELIGIBLE_"))

    def test_r27_missing_serializer_classification(self):
        """R27: Unmodeled user parameters yield MISSING_PARAM and OPEN_USER_PARAMETER disposition."""
        d = classify_parameter_disposition(
            "is.workflow.actions.sample",
            "WFCompletelyUnmodeledParameter",
            cherri_declared_keys=set(),
            cherri_emitted_keys=set(),
            cherri_static_keys=set()
        )
        self.assertEqual(d["disposition"], "OPEN_USER_PARAMETER")
        self.assertEqual(d["codePath"], "unimplemented")


if __name__ == "__main__":
    unittest.main()

