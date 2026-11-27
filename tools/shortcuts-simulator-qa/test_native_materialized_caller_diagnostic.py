"""Finite byte/oracle controls; these tests launch no native or signing process."""
import ast
import copy
import json
import os
from pathlib import Path
import plistlib
import unittest

import native_materialized_caller_diagnostic as diagnostic


class MaterializedCallerDiagnosticTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        root = Path(os.environ["CHERRI_MATERIALIZED_DIAGNOSTIC_BUNDLE"])
        raw = (root / "intake.original.json").read_bytes()
        cls.intake_raw, cls.intake, cls.files = diagnostic.load_bundle(root, diagnostic.sha(raw))
        cls.packet = diagnostic.library.strict_json(cls.files["diagnostic/packet.derived.json"])
        cls.plan = {"compilerCommitSha": diagnostic.COMPILER}

    def test_complete_authentic_original_bundle_and_value_contract(self):
        packet, canonical, checked, signed, evidence = diagnostic.validate_originals(self.intake, self.files, self.plan)
        self.assertEqual(packet, self.packet)
        self.assertIsNone(canonical)
        self.assertEqual(checked["diagnosticCaseId"], "without_functions_materialized_text")
        self.assertEqual(checked["caseId"], "without_functions")
        self.assertEqual(checked["source"], diagnostic.SOURCE)
        self.assertEqual(checked["unsigned"], self.files["diagnostic/unsigned.original.shortcut"])
        self.assertEqual(signed, self.files["signing/case/signed.original.shortcut"])
        nonce = "1234567890abcdef1234567890abcdef"
        input_text = "CHERRI-CALLER-" + nonce
        expected = diagnostic.caller.expected_clipboard(checked["caseId"], input_text)
        self.assertEqual(expected, (input_text + "|" + input_text).encode())
        self.assertEqual(len(expected), 93)
        self.assertTrue(evidence["diagnosticOnly"])
        self.assertFalse(evidence["nativeExecuted"])
        self.assertFalse(evidence["formalAcceptanceGranted"])

    def test_complete_graph_rejects_causal_corruptions(self):
        original = plistlib.loads(self.files["diagnostic/unsigned.original.shortcut"])
        actions = original["WFWorkflowActions"]
        self.assertEqual(len(actions), 5)
        variants = []
        def variant(name, edit):
            value = copy.deepcopy(original)
            edit(value["WFWorkflowActions"], value)
            variants.append((name, value))
        variant("clipboard-inline-string", lambda a, g: a[3]["WFWorkflowActionParameters"].__setitem__("WFInput", copy.deepcopy(a[2]["WFWorkflowActionParameters"]["WFTextActionText"])))
        variant("literal-separator-removed", lambda a, g: a[2]["WFWorkflowActionParameters"]["WFTextActionText"]["Value"].__setitem__("string", "\ufffc\ufffc"))
        variant("first-and-second-reversed", lambda a, g: a[2]["WFWorkflowActionParameters"]["WFTextActionText"]["Value"]["attachmentsByRange"].update({"{0, 1}": copy.deepcopy(a[2]["WFWorkflowActionParameters"]["WFTextActionText"]["Value"]["attachmentsByRange"]["{2, 1}"])}))
        variant("clipboard-wrong-producer", lambda a, g: a[3]["WFWorkflowActionParameters"]["WFInput"]["Value"].__setitem__("OutputUUID", a[0]["WFWorkflowActionParameters"]["UUID"]))
        variant("duplicate-output-uuid", lambda a, g: a[2]["WFWorkflowActionParameters"].__setitem__("UUID", a[0]["WFWorkflowActionParameters"]["UUID"]))
        variant("missing-extension-input", lambda a, g: a[0]["WFWorkflowActionParameters"].__setitem__("WFVariable", diagnostic.attachment({"Type": "Clipboard"})))
        variant("extra-clipboard-option", lambda a, g: a[3]["WFWorkflowActionParameters"].__setitem__("WFLocalOnly", True))
        variant("final-empty-text", lambda a, g: a.__setitem__(4, diagnostic.action("gettext", {"WFTextActionText": ""})))
        variant("added-action", lambda a, g: a.append(diagnostic.action("nothing")))
        variant("removed-input-read", lambda a, g: a.pop(0))
        variant("invented-workflow-name", lambda a, g: g.__setitem__("WFWorkflowName", diagnostic.NAME))
        variant("fallback-enabled", lambda a, g: g.__setitem__("WFWorkflowHasOutputFallback", True))
        for name, value in variants:
            with self.subTest(corruption=name):
                with self.assertRaises((ValueError, KeyError, TypeError)):
                    diagnostic.validate_complete_graph(plistlib.dumps(value))

    def test_packet_identity_cannot_be_promoted_or_substituted(self):
        variants = {
            "original-case-label": ("caseId", "without_functions"),
            "wrong-compiler": ("compilerSource", diagnostic.BUILD_EQUIVALENT_SOURCE),
            "wrong-prior-source": ("priorCompilerSource", diagnostic.COMPILER),
            "native-claim": ("nativeExecuted", True),
            "acceptance-claim": ("formalAcceptanceGranted", True),
            "original-replacement": ("originalAcceptanceReplacement", True),
            "changed-expected-bytes": ("expectedClipboardBytesForFreshProtocolInput", 0),
            "changed-source": ("sourceBase64", ""),
            "changed-unsigned": ("unsignedBase64", ""),
        }
        for name, (key, value) in variants.items():
            with self.subTest(corruption=name):
                packet = {**self.packet, key: value}
                with self.assertRaises(ValueError):
                    diagnostic.validate_packet(packet, None)
        with self.subTest(corruption="canonical-fixture-substitution"):
            with self.assertRaises(ValueError):
                diagnostic.validate_packet(self.packet, {"schema": "original-fixture"})

    def test_observed_process_receipts_require_full_owned_closure(self):
        result = diagnostic.json_member(self.files, "compile/result.original.json")
        original = result["processes"][0]
        prefix = "compile/commands/compile-materialized-text."
        variants = []
        def variant(name, edit):
            process = copy.deepcopy(original)
            edit(process)
            variants.append((name, process))
        variant("nonzero-exit", lambda p: p.__setitem__("exitCode", 1))
        variant("timeout", lambda p: p.__setitem__("timedOut", True))
        variant("cancelled", lambda p: p.__setitem__("cancelled", True))
        variant("unreaped-parent", lambda p: p["cleanup"].__setitem__("parentReaped", False))
        variant("remaining-group", lambda p: p["cleanup"]["after"].__setitem__("state", "LIVE"))
        variant("truncated-stdout", lambda p: p["stdout"].__setitem__("complete", False))
        variant("changed-stdout", lambda p: p["stdout"].__setitem__("sha256", "0" * 64))
        variant("longer-child-bound", lambda p: p.__setitem__("commandSeconds", 46))
        for name, process in variants:
            with self.subTest(corruption=name):
                files = dict(self.files)
                files[prefix + "process.original.json"] = json.dumps(process).encode()
                with self.assertRaises(ValueError):
                    diagnostic.validate_process(files, prefix, process, original["argv"], 45, 180)

    def test_original_runtime_and_cleanup_ast_are_preserved(self):
        current_text = Path(diagnostic.__file__).read_text()
        base_text = Path(diagnostic.base.__file__).read_text()
        caller_text = Path(diagnostic.caller.__file__).read_text()
        self.assertEqual(diagnostic.sha(base_text.encode()), "043aba13d9b442abdf442b278b89904ef00791cf85e007a5daf251f1e7e66c87")
        self.assertEqual(diagnostic.sha(caller_text.encode()), "d1d04d3b405dd03b3d7d9cb7fe2e798f8bd53166f238bd96b3e106182c00e60b")
        current_ast, base_ast, caller_ast = map(ast.parse, (current_text, base_text, caller_text))
        current_class = next(n for n in current_ast.body if isinstance(n, ast.ClassDef) and n.name == "MaterializedCallerDiagnostic")
        base_class = next(n for n in base_ast.body if isinstance(n, ast.ClassDef) and n.name == "NativeCallerObservation")
        run = next(n for n in current_class.body if isinstance(n, ast.FunctionDef) and n.name == "run")
        original_run = next(n for n in base_class.body if isinstance(n, ast.FunctionDef) and n.name == "run")
        source = "\n".join(current_text.splitlines()[run.lineno-1:run.end_lineno]) + "\n"
        restored = source.replace("self.bind_diagnostic_build(", "bind_build(").replace("self.bind_diagnostic_packet(self.plan)", "bind_packet_bundle(self.plan)").replace("observe_materialized_input(", "caller.observe_caller_input(")
        restored_run = ast.parse("class A:\n" + restored).body[0].body[0]
        self.assertEqual(ast.dump(restored_run, include_attributes=False), ast.dump(original_run, include_attributes=False))
        observe = next(n for n in current_ast.body if isinstance(n, ast.FunctionDef) and n.name == "observe_materialized_input")
        original_observe = next(n for n in caller_ast.body if isinstance(n, ast.FunctionDef) and n.name == "observe_caller_input")
        text = ast.get_source_segment(current_text, observe).replace("def observe_materialized_input(", "def observe_caller_input(")
        added = "    result.update(diagnosticOnly=True, diagnosticCaseId=DIAGNOSTIC_CASE, originalAcceptanceReplacement=False)\n"
        self.assertIn(added, text)
        text = text.replace(added, "")
        for before, after in reversed((("require(", "caller.require("), ("original(", "caller.original("), ("record(", "caller.record("), ("imported_unique(", "caller.imported_unique("), ("dispatch_ui(", "caller.dispatch_ui("), ("PBCOPY_EXEC", "caller.PBCOPY_EXEC"), ("validate_runtime_bytes(", "caller.validate_runtime_bytes("))):
            text = text.replace(after, before)
        self.assertEqual(ast.dump(ast.parse(text).body[0], include_attributes=False), ast.dump(original_observe, include_attributes=False))
        self.assertIs(diagnostic.native, diagnostic.base.native)
        self.assertIs(diagnostic.library, diagnostic.caller.library)
        self.assertIs(diagnostic.fixtures, diagnostic.caller.fixtures)
        self.assertIs(diagnostic.differential, diagnostic.caller.differential)
        self.assertIs(diagnostic.defer_cancellation_signals, diagnostic.base.defer_cancellation_signals)
        self.assertEqual(diagnostic.base.WHOLE_SECONDS, 1200)
        self.assertEqual(diagnostic.base.FINAL_RESERVE_SECONDS, 300)
        self.assertEqual(diagnostic.LIVE_FLOOR_BYTES, 4 * 1024 ** 3)
        self.assertNotEqual(diagnostic.SCHEMA, diagnostic.base.SCHEMA)


if __name__ == "__main__":
    unittest.main()
