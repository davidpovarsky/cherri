import copy
import unittest

from editor_inspector import digest, inspect


def specimen(values=("same", "same"), pages=1):
    native, nodes = [], []
    for index, value in enumerate(values):
        uuid = f"00000000-0000-4000-8000-{index:012d}"
        native.append({"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
                       "WFWorkflowActionParameters": {"UUID": uuid, "WFTextActionText": value}})
    for page in range(1, pages + 1):
        nodes.append({"nodeIndex": 0, "parentIndex": None, "sourceScreenIndex": page,
                      "elementTypeName": "ScrollView", "visible": True,
                      "frame": {"x": 0, "y": 0, "width": 400, "height": 800}})
        for index, value in enumerate(values):
            uuid = native[index]["WFWorkflowActionParameters"]["UUID"]
            card = {"nodeIndex": index * 2 + 1, "parentIndex": 0, "sourceScreenIndex": page,
                    "elementTypeName": "Cell", "visible": True, "geometryValid": True,
                    "identifierDigest": digest(uuid), "nativeUUIDDigests": [digest(uuid)],
                    "nativeActionIdentifier": "is.workflow.actions.gettext",
                    "frame": {"x": 10, "y": index * 100 + 50, "width": 380, "height": 90}}
            nodes.append(card)
            nodes.append({"nodeIndex": index * 2 + 2, "parentIndex": index * 2 + 1,
                          "sourceScreenIndex": page, "elementTypeName": "TextView",
                          "visible": True, "value": value,
                          "frame": {"x": 20, "y": index * 100 + 90, "width": 350, "height": 30}})
    coverage = {"editorOpened": True, "editorIdentityVerified": True, "expectedShortcutName": "qa-unit-v2",
                "runId": "run-1", "scrollContainerFrame": {"x": 0, "y": 0, "width": 400, "height": 800},
                "bottomReachedEvidence": "MULTI_SIGNAL_STALL_AT_PAGE_2"}
    return {"WFWorkflowActions": native}, nodes, coverage


class EditorInspectorTests(unittest.TestCase):
    def result(self, source):
        return inspect(*source, "qa-unit-v2", "run-1", "source-sha")[1]

    def test_equal_adjacent_cards_remain_distinct(self):
        report = self.result(specimen())
        self.assertEqual(report["status"], "PASS")
        self.assertEqual(report["observedCardCount"], 2)

    def test_scroll_overlap_requires_unique_identity(self):
        source = specimen(pages=3)
        self.assertEqual(self.result(source)["status"], "PASS")
        for node in source[1]:
            node.pop("identifierDigest", None)
        report = self.result(source)
        self.assertNotEqual(report["status"], "PASS")
        self.assertIn("PAGE_OVERLAP_WITHOUT_STABLE_IDENTITY", report["issues"])

    def test_toolbar_text_is_never_a_card(self):
        source = specimen()
        source[1].extend([
            {"nodeIndex": 100, "parentIndex": None, "sourceScreenIndex": 1, "elementTypeName": "Toolbar"},
            {"nodeIndex": 101, "parentIndex": 100, "sourceScreenIndex": 1, "elementTypeName": "Cell",
             "visible": True, "label": "Text", "frame": {"x": 10, "y": 0, "width": 380, "height": 50}}])
        self.assertEqual(self.result(source)["observedCardCount"], 2)

    def test_reordering_native_actions_is_detected(self):
        source = specimen()
        source[0]["WFWorkflowActions"].reverse()
        self.assertEqual(self.result(source)["summary"]["statusCounts"]["MISMATCH"], 2)

    def test_duplicate_missing_card_cannot_pass(self):
        source = specimen()
        source[1][:] = [n for n in source[1] if n["nodeIndex"] not in (3, 4)]
        self.assertNotEqual(self.result(source)["status"], "PASS")

    def test_corrupted_visible_value_cannot_pass(self):
        source = specimen()
        source[1][2]["value"] = "wrong"
        self.assertNotEqual(self.result(source)["status"], "PASS")

    def test_unexposed_native_identity_is_unknown(self):
        source = specimen()
        for node in source[1]:
            node.pop("nativeUUIDDigests", None)
            node.pop("nativeActionIdentifier", None)
        report = self.result(source)
        self.assertEqual(report["summary"]["statusCounts"]["AX_UNEXPOSED"], 2)

    def test_same_title_elsewhere_does_not_verify_value(self):
        source = specimen(("one", "two"))
        source[1][2]["value"], source[1][4]["value"] = "two", "one"
        self.assertNotEqual(self.result(source)["status"], "PASS")

    def test_stale_run_and_wrong_editor_fail_closed(self):
        for key, value in (("runId", "stale"), ("expectedShortcutName", "other"), ("editorIdentityVerified", False)):
            source = specimen()
            source[2][key] = value
            self.assertNotEqual(self.result(source)["status"], "PASS", key)

    def test_nonfinite_or_partial_geometry_never_passes(self):
        for value in (float("inf"), -50):
            source = specimen()
            source[1][1]["frame"]["y"] = value
            self.assertNotEqual(self.result(source)["status"], "PASS")

    def test_empty_tree_no_pass(self):
        source = specimen()
        source[1].clear()
        self.assertNotEqual(self.result(source)["status"], "PASS")

    def test_hidden_structured_parameter_not_visually_verified(self):
        source = specimen()
        source[0]["WFWorkflowActions"][0]["WFWorkflowActionParameters"]["WFInput"] = {"Value": {"OutputUUID": "opaque"}}
        self.assertNotEqual(self.result(source)["status"], "PASS")

    def test_unicode_rtl_values_are_exact(self):
        source = specimen(("שָׁלוֹם 😀", "שלום"))
        self.assertEqual(self.result(source)["status"], "PASS")
        source[1][2]["value"] = "שלום 😀"
        self.assertNotEqual(self.result(source)["status"], "PASS")

    def test_virtualized_reused_container_never_collapses_new_card(self):
        source = specimen(pages=2)
        for node in source[1]:
            node.pop("nativeUUIDDigests", None)
        self.assertNotEqual(self.result(source)["status"], "PASS")

    def test_negative_mutations_zero_false_pass(self):
        mutations = [lambda s: s[0]["WFWorkflowActions"].pop(),
                     lambda s: s[0]["WFWorkflowActions"].reverse(),
                     lambda s: s[1][2].update(value="corrupt"),
                     lambda s: s[2].update(bottomReachedEvidence="MAX_PAGES_REACHED"),
                     lambda s: s[1].append(copy.deepcopy(s[1][1]))]
        false_passes = 0
        for mutate in mutations:
            source = specimen(("first", "second"))
            mutate(source)
            false_passes += self.result(source)["status"] == "PASS"
        self.assertEqual(false_passes, 0)


if __name__ == "__main__":
    unittest.main()
