"""Offline controls for pagination's macro result and explicit V2 main boundary."""
import copy
import types
import unittest

from verify_paginate import PREFIX, PaginateGraphModel, extract_paginate_graph


def empty_result_graph():
    name = "__cherri_paginate_public_control_results"
    return [
        {"WFWorkflowActionIdentifier": PREFIX + "list",
         "WFWorkflowActionParameters": {
             "WFItems": [], "UUID": "original-list", "CustomOutputName": "original-list"}},
        {"WFWorkflowActionIdentifier": PREFIX + "setvariable",
         "WFWorkflowActionParameters": {
             "WFVariableName": name,
             "WFInput": {"WFSerializationType": "WFTextTokenAttachment",
                         "Value": {"Type": "ActionOutput", "OutputUUID": "original-list",
                                   "OutputName": "original-list"}}}},
        {"WFWorkflowActionIdentifier": PREFIX + "getvariable",
         "WFWorkflowActionParameters": {
             "UUID": "result-read", "CustomOutputName": "Paginated Items",
             "WFVariable": {"WFSerializationType": "WFTextTokenAttachment",
                            "Value": {"Type": "Variable", "VariableName": name}}}},
    ]


def nothing():
    return {"WFWorkflowActionIdentifier": PREFIX + "nothing"}


class PaginateBoundaryTests(unittest.TestCase):
    def test_v2_boundary_accepts_omitted_or_empty_parameters_without_mutation(self):
        for terminal in (nothing(), {**nothing(), "WFWorkflowActionParameters": {}}):
            with self.subTest(terminal=terminal):
                graph = empty_result_graph() + [terminal]
                original = copy.deepcopy(graph)
                body, boundary = extract_paginate_graph(graph, legacy=False)
                self.assertEqual(boundary, "V2_EXPLICIT_VOID_VERIFIED")
                self.assertEqual(body, original[:-1])
                self.assertEqual(graph, original)
                self.assertIs(body[0], graph[0])

    def test_legacy_graph_is_unchanged_and_has_no_void_boundary(self):
        graph = empty_result_graph()
        body, boundary = extract_paginate_graph(graph, legacy=True)
        self.assertIs(body, graph)
        self.assertEqual(boundary, "LEGACY_UNCHANGED")
        with self.assertRaises(AssertionError):
            extract_paginate_graph(graph + [nothing()], legacy=True)

    def test_missing_extra_interior_or_moved_nothing_is_rejected(self):
        body = empty_result_graph()
        graphs = {
            "missing": body,
            "extra": body + [nothing(), nothing()],
            "interior": body[:2] + [nothing()] + body[2:] + [nothing()],
            "moved": body[:2] + [nothing()] + body[2:],
            "empty": [],
            "only-boundary": [nothing()],
            "malformed-terminal": body + [None],
        }
        for name, graph in graphs.items():
            with self.subTest(name=name), self.assertRaises(AssertionError):
                extract_paginate_graph(graph, legacy=False)

    def test_parameterized_or_annotated_terminal_nothing_is_rejected(self):
        terminals = [
            {**nothing(), "WFWorkflowActionParameters": p}
            for p in ({"UUID": "unreviewed-producer"}, {"WFInput": "value"},
                      {"WFOutput": "value"}, None, [], "")
        ] + [
            {**nothing(), "UUID": "unreviewed-top-level"},
            {**nothing(), "unknown": True},
            {"WFWorkflowActionIdentifier": "foreign.nothing"},
        ]
        for terminal in terminals:
            with self.subTest(terminal=terminal), self.assertRaises(AssertionError):
                extract_paginate_graph(empty_result_graph() + [terminal], legacy=False)

    def test_unknown_macro_effects_remain_rejected(self):
        for identifier in (PREFIX + "nothing", PREFIX + "share", "foreign.getvariable"):
            graph = empty_result_graph()
            graph.insert(2, {"WFWorkflowActionIdentifier": identifier})
            with self.subTest(identifier=identifier), self.assertRaises(AssertionError):
                extract_paginate_graph(graph + [nothing()], legacy=False)

    def test_result_must_bind_the_actual_uncoerced_original_list(self):
        def wrong_initializer(g):
            g[0]["WFWorkflowActionIdentifier"] = PREFIX + "dictionary"
        def nonempty_initializer(g):
            g[0]["WFWorkflowActionParameters"]["WFItems"] = ["unreviewed"]
        def missing_producer(g):
            del g[0]["WFWorkflowActionParameters"]["UUID"]
        def wrong_store(g):
            g[1]["WFWorkflowActionParameters"]["WFInput"]["Value"]["OutputUUID"] = "other-list"
        def coerced_store(g):
            g[1]["WFWorkflowActionParameters"]["WFInput"]["Value"]["Aggrandizements"] = [{"Type": "Text"}]
        def wrong_variable(g):
            g[-1]["WFWorkflowActionParameters"]["WFVariable"]["Value"]["VariableName"] = "__cherri_paginate_public_control_seen"
        def coerced_result(g):
            g[-1]["WFWorkflowActionParameters"]["WFVariable"]["Value"]["Aggrandizements"] = [{"Type": "Text"}]
        def wrong_result_action(g):
            g[-1]["WFWorkflowActionIdentifier"] = PREFIX + "gettext"
        def overwritten_initializer(g):
            g.insert(2, copy.deepcopy(g[1]))
        for mutate in (wrong_initializer, nonempty_initializer, missing_producer,
                       wrong_store, coerced_store, wrong_variable, coerced_result,
                       wrong_result_action, overwritten_initializer):
            for legacy in (False, True):
                graph = empty_result_graph()
                mutate(graph)
                if not legacy:
                    graph.append(nothing())
                with self.subTest(mutation=mutate.__name__, legacy=legacy), self.assertRaises(AssertionError):
                    extract_paginate_graph(graph, legacy=legacy)

    def test_zero_budget_keeps_list_result_and_separate_main_completion(self):
        for legacy in (False, True):
            graph = empty_result_graph() + ([] if legacy else [nothing()])
            body, boundary = extract_paginate_graph(graph, legacy=legacy)
            server = types.SimpleNamespace(requests=[])
            result = PaginateGraphModel(server).run(body)
            self.assertEqual(len(body), 3)
            self.assertEqual(len(graph), 3 if legacy else 4)
            self.assertEqual(result.kind, "List")
            self.assertEqual(result.data, [])
            self.assertEqual(server.requests, [])
            self.assertEqual(boundary, "LEGACY_UNCHANGED" if legacy else "V2_EXPLICIT_VOID_VERIFIED")


if __name__ == "__main__":
    unittest.main()
