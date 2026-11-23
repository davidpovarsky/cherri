"""Causal controls for export-graph and main-output boundaries, not Apple evidence."""
import copy
import unittest

import verify_debug_export as subject


def action(name, parameters=None):
    return {
        "WFWorkflowActionIdentifier": subject.PREFIX + name,
        "WFWorkflowActionParameters": {} if parameters is None else parameters,
    }


def fixture(legacy):
    actions = [
        action("nothing"),
        action("setvariable", {"WFVariableName": "__cherri_debug_log"}),
        action("gettext", {"WFTextActionText": "synthetic input"}),
        action("appendvariable", {"WFVariableName": "__cherri_debug_log"}),
        action("gettext", {"WFTextActionText": "synthetic export"}),
        action("setitemname", {"WFName": "cherri-debug.json", "UUID": "export-output"}),
    ]
    if not legacy:
        actions.append(action("nothing"))
    return actions


class ExportBoundaryTests(unittest.TestCase):
    def test_both_dialects_preserve_every_macro_action_and_original_bytes(self):
        for legacy in (False, True):
            with self.subTest(legacy=legacy):
                actions = fixture(legacy)
                original = copy.deepcopy(actions)
                graph, boundary = subject.extract_export_graph(actions, legacy=legacy)
                self.assertEqual(actions, original)
                self.assertEqual(graph, original[4:6])
                self.assertIs(graph[0], actions[4])
                self.assertIs(graph[-1], actions[5])
                self.assertEqual(boundary, "LEGACY_UNCHANGED" if legacy else
                                 "V2_EXPLICIT_VOID_VERIFIED")
                # The old whole-tail predicate rejects the valid V2 boundary.
                old_predicate = all(
                    a["WFWorkflowActionIdentifier"].removeprefix(subject.PREFIX)
                    in subject.ALLOWED for a in actions[4:])
                self.assertEqual(old_predicate, legacy)

    def test_v2_missing_or_corrupted_void_boundary_is_rejected(self):
        original = fixture(False)
        mutations = {
            "missing": original[:-1],
            "value_terminal": original[:-1] + [action("gettext")],
            "input_on_nothing": original[:-1] + [action("nothing", {"WFInput": "value"})],
            "uuid_on_nothing": original[:-1] + [action("nothing", {"UUID": "unexpected"})],
            "missing_parameters": original[:-1] + [{
                "WFWorkflowActionIdentifier": subject.PREFIX + "nothing"}],
        }
        for name, actions in mutations.items():
            with self.subTest(name=name):
                with self.assertRaises(ValueError):
                    subject.extract_export_graph(actions, legacy=False)
        subject.extract_export_graph(original, legacy=False)

    def test_no_interior_action_is_hidden_by_the_terminal_split(self):
        for legacy in (False, True):
            for name in ("nothing", "downloadurl", "appendvariable", "runworkflow"):
                with self.subTest(legacy=legacy, action=name):
                    actions = fixture(legacy)
                    actions.insert(5, action(name))
                    with self.assertRaises(ValueError):
                        subject.extract_export_graph(actions, legacy=legacy)

    def test_legacy_does_not_inherit_v2_terminal_removal(self):
        with self.assertRaises(ValueError):
            subject.extract_export_graph(fixture(False), legacy=True)

    def test_prelude_reorder_or_addition_is_rejected(self):
        for legacy in (False, True):
            for name in ("reorder", "addition", "short"):
                with self.subTest(legacy=legacy, mutation=name):
                    actions = fixture(legacy)
                    if name == "reorder":
                        actions[2], actions[3] = actions[3], actions[2]
                    elif name == "addition":
                        actions.insert(3, action("downloadurl"))
                    else:
                        actions = actions[:4]
                    with self.assertRaises(ValueError):
                        subject.extract_export_graph(actions, legacy=legacy)

    def test_missing_file_producer_is_rejected(self):
        for legacy in (False, True):
            with self.subTest(legacy=legacy):
                actions = fixture(legacy)
                del actions[5]
                with self.assertRaises(ValueError):
                    subject.extract_export_graph(actions, legacy=legacy)

    def test_noncanonical_identifier_rejected_by_boundary_and_model(self):
        for raw in ("gettext", "foreign.actions.gettext", "is.workflow.actions.downloadurl"):
            with self.subTest(identifier=raw):
                actions = fixture(True)
                actions[4]["WFWorkflowActionIdentifier"] = raw
                with self.assertRaises(ValueError):
                    subject.extract_export_graph(actions, legacy=True)
                with self.assertRaises(ValueError):
                    subject.GraphModel([]).run([actions[4]])


if __name__ == "__main__":
    unittest.main()
