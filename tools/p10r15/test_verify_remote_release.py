"""Causal controls for Remote Builder's public synthetic workflow observer."""
import copy
import unittest

from verify_remote_release import synthetic_workflow_matches


PREFIX = "is.workflow.actions."
OUTPUT_UUID = "FF0914A0-F36C-4D68-91AC-C48A30880647"


def fixture(dialect):
    legacy = dialect == "legacy"
    reference = {"Type": "ActionOutput", "OutputUUID": OUTPUT_UUID}
    actions = [{
        "WFWorkflowActionIdentifier": PREFIX + "gettext",
        "WFWorkflowActionParameters": {
            "WFTextActionText": "P10R15 staged " + ("Legacy" if legacy else "V2") + " verification",
            "CustomOutputName": "message",
            "UUID": OUTPUT_UUID,
        },
    }]
    if legacy:
        actions.append({
            "WFWorkflowActionIdentifier": PREFIX + "setvariable",
            "WFWorkflowActionParameters": {
                "WFVariableName": "message",
                "WFInput": {"WFSerializationType": "WFTextTokenAttachment", "Value": reference},
            },
        })
    attachment = ({"Type": "Variable", "VariableName": "message"} if legacy else
                  dict(reference, OutputName="message"))
    actions.append({
        "WFWorkflowActionIdentifier": PREFIX + "showresult",
        "WFWorkflowActionParameters": {
            "Text": {"WFSerializationType": "WFTextTokenString",
                     "Value": {"string": "\ufffc", "attachmentsByRange": {"{0, 1}": attachment}}},
        },
    })
    if not legacy:
        # Native V2 main-void lowering omits an empty parameter dictionary.
        actions.append({"WFWorkflowActionIdentifier": PREFIX + "nothing"})
    return actions


class SyntheticWorkflowTests(unittest.TestCase):
    def test_native_v2_void_preserves_original_actions_and_exact_reference(self):
        actions = fixture("v2")
        original = copy.deepcopy(actions)
        self.assertTrue(synthetic_workflow_matches(actions, "v2"))
        self.assertEqual(actions, original)
        self.assertNotIn("WFWorkflowActionParameters", actions[-1])

    def test_explicit_empty_v2_terminal_parameters_are_equivalent(self):
        actions = fixture("v2")
        actions[-1]["WFWorkflowActionParameters"] = {}
        self.assertTrue(synthetic_workflow_matches(actions, "v2"))

    def test_exact_legacy_graph_is_unchanged(self):
        actions = fixture("legacy")
        original = copy.deepcopy(actions)
        self.assertTrue(synthetic_workflow_matches(actions, "legacy"))
        self.assertEqual(actions, original)

    def test_v2_requires_a_terminal_void_action(self):
        for actions in ([], fixture("v2")[:-1], fixture("v2")[-1:]):
            with self.subTest(actions=actions):
                self.assertFalse(synthetic_workflow_matches(actions, "v2"))

    def test_malformed_or_value_bearing_terminal_is_rejected(self):
        malformed = [
            {"WFWorkflowActionIdentifier": PREFIX + "gettext"},
            {},
            None,
            "nothing",
        ]
        for parameters in ({"WFInput": "value"}, {"UUID": "unexpected"}, None, [], ""):
            malformed.append({"WFWorkflowActionIdentifier": PREFIX + "nothing",
                              "WFWorkflowActionParameters": parameters})
        for terminal in malformed:
            with self.subTest(terminal=terminal):
                actions = fixture("v2")[:-1] + [terminal]
                self.assertFalse(synthetic_workflow_matches(actions, "v2"))

    def test_no_extra_or_interior_action_is_hidden(self):
        original = fixture("v2")
        for name in ("nothing", "gettext", "downloadurl", "showresult"):
            for position in (0, 1, 2, 3):
                with self.subTest(name=name, position=position):
                    actions = copy.deepcopy(original)
                    actions.insert(position, {"WFWorkflowActionIdentifier": PREFIX + name})
                    self.assertFalse(synthetic_workflow_matches(actions, "v2"))
        self.assertFalse(synthetic_workflow_matches(
            fixture("legacy") + [{"WFWorkflowActionIdentifier": PREFIX + "nothing"}], "legacy"))

    def test_literal_and_nonempty_producer_uuid_remain_required(self):
        for dialect in ("v2", "legacy"):
            for key, value in (("WFTextActionText", "different text"), ("UUID", ""),
                               ("UUID", None), ("UUID", 7)):
                with self.subTest(dialect=dialect, key=key, value=value):
                    actions = fixture(dialect)
                    actions[0]["WFWorkflowActionParameters"][key] = value
                    self.assertFalse(synthetic_workflow_matches(actions, dialect))

    def test_consumer_token_and_exact_attachment_remain_required(self):
        for dialect in ("v2", "legacy"):
            for mutation in ("serialization", "string", "range", "extra_range", "type", "binding"):
                with self.subTest(dialect=dialect, mutation=mutation):
                    actions = fixture(dialect)
                    display = actions[-1 if dialect == "legacy" else -2]["WFWorkflowActionParameters"]["Text"]
                    value = display["Value"]
                    attachments = value["attachmentsByRange"]
                    reference = attachments["{0, 1}"]
                    if mutation == "serialization":
                        display["WFSerializationType"] = "WFTextTokenAttachment"
                    elif mutation == "string":
                        value["string"] = "literal"
                    elif mutation == "range":
                        value["attachmentsByRange"] = {"{1, 1}": reference}
                    elif mutation == "extra_range":
                        attachments["{1, 1}"] = copy.deepcopy(reference)
                    elif mutation == "type":
                        reference["Type"] = "Other"
                    elif dialect == "legacy":
                        reference["VariableName"] = "other"
                    else:
                        reference["OutputUUID"] = "other"
                    self.assertFalse(synthetic_workflow_matches(actions, dialect))
        actions = fixture("v2")
        actions[1]["WFWorkflowActionParameters"]["Text"]["Value"]["attachmentsByRange"]["{0, 1}"]["OutputName"] = "other"
        self.assertFalse(synthetic_workflow_matches(actions, "v2"))

    def test_legacy_assignment_identity_and_reference_remain_required(self):
        for mutation in ("name", "serialization", "type", "uuid"):
            with self.subTest(mutation=mutation):
                actions = fixture("legacy")
                assignment = actions[1]["WFWorkflowActionParameters"]
                if mutation == "name":
                    assignment["WFVariableName"] = "other"
                elif mutation == "serialization":
                    assignment["WFInput"]["WFSerializationType"] = "WFTextTokenString"
                else:
                    assignment["WFInput"]["Value"]["Type" if mutation == "type" else "OutputUUID"] = "other"
                self.assertFalse(synthetic_workflow_matches(actions, "legacy"))

    def test_dialects_do_not_accept_each_others_graphs(self):
        self.assertFalse(synthetic_workflow_matches(fixture("legacy"), "v2"))
        self.assertFalse(synthetic_workflow_matches(fixture("v2"), "legacy"))


if __name__ == "__main__":
    unittest.main()
