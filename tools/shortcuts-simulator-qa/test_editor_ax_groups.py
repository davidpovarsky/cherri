import copy
import json
from pathlib import Path
import unittest

from editor_inspector import extract_cards, inspect

FIXTURE = Path(__file__).with_name("testdata") / "legacy-ax-groups-44238251950.json"


class ActualLegacyAXGroupingTests(unittest.TestCase):
    def setUp(self):
        self.fixture = json.loads(FIXTURE.read_text(encoding="utf-8"))
        self.nodes = copy.deepcopy(self.fixture["nodes"])
        self.coverage = copy.deepcopy(self.fixture["coverage"])

    def test_real_outer_groups_replace_nested_dictionary_rows(self):
        cards, issues = extract_cards(self.nodes, self.coverage)
        self.assertEqual([c["nodeId"] for c in cards], [
            "p1:n33", "p1:n65", "p1:n97", "p2:n197", "p2:n219", "p2:n233", "p2:n301",
            "p3:n115", "p3:n129", "p3:n197"])
        self.assertTrue(all(c["containerRole"] == "Other" for c in cards))
        self.assertTrue(all(c["containerEvidence"] == "AX_CANVAS_GROUP" for c in cards))
        self.assertTrue(all(not c["nativeUUIDDigests"] and not c["nativeIdentifiers"] for c in cards))
        self.assertIn("PAGE_OVERLAP_WITHOUT_STABLE_IDENTITY", issues)

    def test_number_editable_title_requires_actual_nested_button(self):
        cards, _ = extract_cards(self.nodes, self.coverage)
        self.assertIn("p2:n197", [c["nodeId"] for c in cards])
        for node in self.nodes:
            if node["sourceScreenIndex"] == 2 and node["nodeIndex"] == 204:
                node["elementTypeName"] = "Other"
        cards, _ = extract_cards(self.nodes, self.coverage)
        self.assertNotIn("p2:n197", [c["nodeId"] for c in cards])

    def test_visible_anchor_is_not_a_native_parameter_match_without_identity(self):
        native = [{"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
                   "WFWorkflowActionParameters": {"WFCommentActionText": "ANCHOR_CARD_01_TOP_HEADER"}}]
        observed, compared = inspect({"WFWorkflowActions": native}, self.nodes, self.coverage,
                                     "qa-functional-legacy", self.coverage["runId"], self.fixture["compilerSha"])
        row = compared["actions"][0]
        self.assertEqual(row["status"], "AX_UNEXPOSED")
        self.assertIn("ANCHOR_CARD_01_TOP_HEADER", row["observedTexts"])
        self.assertTrue(row["visibleParameters"][0]["observedTextPresent"])
        self.assertEqual(row["visibleParameters"][0]["status"], "AX_UNEXPOSED")
        self.assertEqual(observed["observedCardCountScope"], "PAGE_OBSERVATIONS_WITH_UNRESOLVED_IDENTITY")

    def test_input_array_order_does_not_change_geometry_observation(self):
        original = extract_cards(self.nodes, self.coverage)
        self.nodes.reverse()
        self.assertEqual(extract_cards(self.nodes, self.coverage), original)

    def test_dictionary_rows_never_become_actions_when_header_is_unexposed(self):
        for node in self.nodes:
            if node["sourceScreenIndex"] == 2 and node["elementTypeName"] in {"Button", "StaticText"}:
                node["elementTypeName"] = "Other"
        cards, _ = extract_cards(self.nodes, self.coverage)
        self.assertFalse(any(c["containerRole"] == "Cell" for c in cards))
        self.assertFalse(any(c["page"] == 2 for c in cards))

    def test_duplicate_node_identity_rejects_page(self):
        self.nodes.append(copy.deepcopy(next(n for n in self.nodes if n["sourceScreenIndex"] == 2 and n["nodeIndex"] == 233)))
        cards, issues = extract_cards(self.nodes, self.coverage)
        self.assertIn("DUPLICATE_OR_MISSING_NODE_INDEX_PAGE_2", issues)
        self.assertFalse(any(c["page"] == 2 for c in cards))

    def test_sibling_order_disagreement_is_reported(self):
        roots = [n for n in self.nodes if n["sourceScreenIndex"] == 1 and n["nodeIndex"] in {33, 65}]
        roots[0]["siblingOrder"], roots[1]["siblingOrder"] = roots[1]["siblingOrder"], roots[0]["siblingOrder"]
        _, issues = extract_cards(self.nodes, self.coverage)
        self.assertIn("AX_GROUP_ORDER_CONFLICT_PAGE_1", issues)

    def test_geometry_overlap_cannot_be_silently_accepted(self):
        for node in self.nodes:
            if node["sourceScreenIndex"] == 1 and node["nodeIndex"] == 33:
                node["frame"]["height"] += 30
        _, issues = extract_cards(self.nodes, self.coverage)
        self.assertIn("AX_GROUP_OVERLAP_PAGE_1", issues)

    def test_repeated_page_keeps_observations_separate_without_native_identity(self):
        duplicate = copy.deepcopy([n for n in self.nodes if n["sourceScreenIndex"] == 1])
        for node in duplicate:
            node["sourceScreenIndex"] = 4
        cards, issues = extract_cards(self.nodes + duplicate, self.coverage)
        self.assertEqual(len(cards), 13)
        self.assertIn("PAGE_OVERLAP_WITHOUT_STABLE_IDENTITY", issues)

    def test_expected_plan_cannot_supply_missing_native_identity(self):
        native = [{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
                   "WFWorkflowActionParameters": {"UUID": str(i), "WFTextActionText": str(i)}} for i in range(10)]
        for actions in (native, list(reversed(native)), native + native[:1]):
            observed, compared = inspect({"WFWorkflowActions": actions}, self.nodes, self.coverage,
                                         "qa-functional-legacy", self.coverage["runId"], self.fixture["compilerSha"])
            self.assertEqual(observed["observedCardCount"], 10)
            self.assertEqual(compared["summary"]["matched"], 0)
            self.assertNotEqual(compared["status"], "PASS")
            self.assertEqual(compared["summary"]["statusCounts"]["AX_UNEXPOSED"], 10)


if __name__ == "__main__":
    unittest.main()
