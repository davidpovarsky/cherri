import Foundation
import CoreGraphics

@main enum VerifyImportedEditorNavigation {
    typealias Model = QAImportedEditorNavigation
    typealias Node = QAImportSheetSelection.Node
    typealias Candidate = QAImportSheetSelection.Candidate
    static let name = "qa-storage-delete-v2"
    static let appFrame = CGRect(x: 0, y: 0, width: 402, height: 874)
    static let barFrame = CGRect(x: 0, y: 62, width: 402, height: 54)
    static let backFrame = CGRect(x: 16, y: 62, width: 44, height: 44)

    static func node(_ kind: QAImportSheetSelection.Kind, label: String = "", id: String = "",
                     frame: CGRect, enabled: Bool = true, selected: Bool = false,
                     children: [Node] = []) -> Node {
        Node(kind: kind, label: label, identifier: id, enabled: enabled, selected: selected,
             frame: frame, children: children)
    }
    static func copy(_ n: Node, kind: QAImportSheetSelection.Kind? = nil, label: String? = nil,
                     id: String? = nil, frame: CGRect? = nil, enabled: Bool? = nil,
                     selected: Bool? = nil, children: [Node]? = nil) -> Node {
        node(kind ?? n.kind, label: label ?? n.label, id: id ?? n.identifier,
             frame: frame ?? n.frame, enabled: enabled ?? n.enabled,
             selected: selected ?? n.selected, children: children ?? n.children)
    }
    static func back() -> Node { node(.button, label: "Back", id: "BackButton", frame: backFrame) }
    static func bar(_ value: String = name, buttons: [Node] = [back()]) -> Node {
        node(.navigationBar, id: value, frame: barFrame, children: buttons)
    }
    static func app(_ bars: [Node] = [bar()]) -> Node {
        node(.application, label: "Shortcuts", frame: appFrame,
             children: [node(.window, frame: appFrame, children: bars)])
    }
    static func candidate(_ i: Int = 0, _ n: Node = back(), hittable: Bool = true) -> Candidate {
        Candidate(queryIndex: i, exists: true, hittable: hittable, snapshot: n)
    }
    static func context(_ value: String = name, foreground: Bool = true, dispatched: Bool = true,
                        attempted: Bool = false, now: Double = 7, deadline: Double = 30) -> Model.Context {
        Model.Context(expectedName: value, foreground: foreground, dispatchConfirmed: dispatched,
                      alreadyNavigated: attempted, now: Date(timeIntervalSince1970: now),
                      deadline: Date(timeIntervalSince1970: deadline))
    }

    static func main() throws {
        var events: [[String: Any]] = []
        func check(_ name: String, _ value: Bool) {
            events.append(["name": name, "outcome": value ? "PASS" : "FAIL"])
        }
        func decide(_ tree: Node = app(), _ candidates: [Candidate] = [candidate()],
                    _ ctx: Model.Context = context()) -> Model.Decision {
            Model.select(current: tree, candidates: candidates, context: ctx)
        }
        func allow(_ n: String, tree: Node = app(), candidates: [Candidate] = [candidate()],
                   ctx: Model.Context = context(), index: Int = 0) {
            let d = decide(tree, candidates, ctx)
            check(n, d.binding?.queryIndex == index && d.reason == "UNIQUE_CURRENT_REQUESTED_EDITOR_BACK")
        }
        func deny(_ n: String, tree: Node = app(), candidates: [Candidate] = [candidate()],
                  ctx: Model.Context = context()) {
            check(n, !decide(tree, candidates, ctx).canNavigate)
        }
        allow("exact_requested_editor_back")
        check("exact_same_bar_and_button_paths", decide().binding?.navigationBarPath == [0,0] &&
              decide().binding?.buttonPath == [0,0,0])
        allow("unicode_exact_editor", tree: app([bar("שלום 🧪")]), ctx: context("שלום 🧪"))
        allow("localized_back_label_bound_by_native_identifier", tree: app([bar(buttons: [copy(back(), label: "חזרה")])]),
              candidates: [candidate(0, copy(back(), label: "חזרה"))])
        deny("wrong_editor_name", tree: app([bar("unrelated")]))
        deny("matching_label_without_identifier", tree: app([copy(bar("unrelated"), label: name)]))
        deny("prefix_editor_name", tree: app([bar(name + "-other")]))
        deny("unicode_normalization_not_silently_equal", tree: app([bar("e\u{301}")]), ctx: context("é"))
        deny("global_title_cannot_bind_other_editor", tree: app([bar("unrelated"),
            node(.staticText, label: name, frame: barFrame)]))
        deny("wrong_bar_role", tree: app([copy(bar(), kind: .other)]))
        deny("back_from_other_bar", tree: app([bar(buttons: []), bar("unrelated")]))
        deny("nested_control_not_a_direct_back", tree: app([bar(buttons:
            [node(.other, frame: barFrame, children: [back()])])]))
        deny("ambiguous_two_named_bars", tree: app([bar(), bar()]))
        deny("ambiguous_two_backs_same_bar", tree: app([bar(buttons: [back(), back()])]))
        deny("ambiguous_duplicate_query_matches", candidates: [candidate(0), candidate(1)])
        allow("hidden_prior_query_does_not_bind", candidates: [candidate(0, hittable: false), candidate(1)], index: 1)
        allow("unrelated_current_bar_is_not_selected", tree: app([bar("unrelated"),bar()]))
        deny("not_hittable", candidates: [candidate(0, hittable: false)])
        deny("missing_query", candidates: [])
        deny("nonexistent_query", candidates: [Candidate(queryIndex: 0, exists: false, hittable: false, snapshot: nil)])
        deny("inconsistent_query_exists", candidates: [Candidate(queryIndex: 0, exists: false, hittable: true, snapshot: back())])
        deny("inconsistent_query_snapshot", candidates: [Candidate(queryIndex: 0, exists: true, hittable: false, snapshot: nil)])
        deny("query_wrong_identifier", candidates: [candidate(0, copy(back(), id: "Dismiss"))])
        deny("query_wrong_role", candidates: [candidate(0, copy(back(), kind: .staticText))])
        deny("query_stale_label", candidates: [candidate(0, copy(back(), label: "Back old"))])
        deny("query_stale_frame", candidates: [candidate(0, copy(back(), frame: backFrame.offsetBy(dx: 1, dy: 0)))])
        deny("query_stale_selection", candidates: [candidate(0, copy(back(), selected: true))])
        deny("query_stale_children", candidates: [candidate(0, copy(back(), children:
            [node(.staticText, label: "old", frame: backFrame)]))])
        deny("disabled_back", tree: app([bar(buttons: [copy(back(), enabled: false)])]),
             candidates: [candidate(0, copy(back(), enabled: false))])
        deny("disabled_bar", tree: app([copy(bar(), enabled: false)]))
        deny("disabled_ancestor", tree: node(.application, frame: appFrame,
            children: [node(.window, frame: appFrame, enabled: false, children: [bar()])]))
        deny("disabled_app", tree: copy(app(), enabled: false))
        deny("offscreen_bar", tree: app([copy(bar(), frame: barFrame.offsetBy(dx: 0, dy: 900))]))
        deny("button_outside_named_bar", tree: app([bar(buttons: [copy(back(), frame: backFrame.offsetBy(dx: 0, dy: 90))])]),
             candidates: [candidate(0, copy(back(), frame: backFrame.offsetBy(dx: 0, dy: 90)))])
        deny("nonfinite_frame", tree: app([copy(bar(), frame: CGRect(x: Double.nan, y: 62, width: 402, height: 54))]))
        deny("empty_frame", tree: app([copy(bar(), frame: .zero)]))
        deny("wrong_application_role", tree: copy(app(), kind: .window))
        deny("background_editor", ctx: context(foreground: false))
        deny("pending_dispatch", ctx: context(dispatched: false))
        deny("duplicate_navigation", ctx: context(attempted: true))
        deny("deadline_exactly_reached", ctx: context(now: 30))
        deny("deadline_exceeded", ctx: context(now: 31))
        deny("nonfinite_clock", ctx: context(now: .nan))
        deny("nonfinite_deadline", ctx: context(deadline: .infinity))
        deny("empty_expected_name", ctx: context(""))
        deny("unbounded_expected_name", ctx: context(String(repeating: "x", count: 4097)))
        deny("duplicate_query_index", candidates: [candidate(0),candidate(0)])
        deny("negative_query_index", candidates: [candidate(-1)])
        deny("unbounded_query_index", candidates: [candidate(32)])
        deny("unbounded_query_count", candidates: (0..<33).map { candidate($0) })
        deny("unbounded_tree_depth", tree: app([(0..<65).reduce(bar()) { n,_ in node(.other, frame: appFrame, children: [n]) }]))
        deny("unbounded_tree_count", tree: app([node(.other, frame: appFrame, children: (0..<8192).map { _ in back() })]))
        deny("unbounded_node_text", tree: app([copy(bar(), label: String(repeating: "x", count: 4097))]))
        var observations = 0, captures = 0
        let healthy = decide()
        let missing = decide(app([bar("other")]))
        let fresh = Model.inspectForNavigation(observe: {
            observations += 1; return observations == 1 ? healthy : missing
        }, capture: { captures += 1 })
        check("capture_transition_to_wrong_editor_rejected", !fresh.canNavigate && observations == 2 && captures == 1)
        observations = 0; captures = 0
        let disappeared = Model.inspectForNavigation(observe: {
            observations += 1; return observations == 1 ? healthy : decide(app([]))
        }, capture: { captures += 1 })
        check("capture_disappeared_back_rejected", !disappeared.canNavigate && observations == 2 && captures == 1)
        observations = 0; captures = 0
        let late = Model.inspectForNavigation(observe: {
            observations += 1; return observations == 1 ? healthy : decide(app(), [candidate()], context(now: 30))
        }, capture: { captures += 1 })
        check("capture_expired_original_deadline_rejected", !late.canNavigate && observations == 2 && captures == 1)
        observations = 0; captures = 0
        let live = Model.inspectForNavigation(observe: { observations += 1; return healthy },
                                             capture: { captures += 1 })
        check("healthy_capture_reobserves_before_navigation", live.canNavigate && observations == 2 && captures == 1)
        observations = 0; captures = 0
        let none = Model.inspectForNavigation(observe: { observations += 1; return missing },
                                             capture: { captures += 1 })
        check("no_match_does_not_capture_or_navigate", !none.canNavigate && observations == 1 && captures == 0)
        enum CaptureFailure: Error { case expected }
        var propagated = false
        do { _ = try Model.inspectForNavigation(observe: { healthy }, capture: { throw CaptureFailure.expected }) }
        catch CaptureFailure.expected { propagated = true }
        check("capture_error_is_not_navigation_authority", propagated)
        var originalReplay = false
        if CommandLine.arguments.count == 2 {
            let text = try String(contentsOfFile: CommandLine.arguments[1], encoding: .utf8)
            let lines = text.components(separatedBy: "\n")
            let bars = lines.enumerated().filter { $0.element.contains("NavigationBar,") &&
                $0.element.contains("identifier: '" + name + "'") }
            let originalBack = bars.count == 1 && bars[0].offset + 1 < lines.count &&
                lines[bars[0].offset + 1].contains("identifier: 'BackButton', label: 'Back'") &&
                bars[0].element.contains("{{0.0, 62.0}, {402.0, 54.0}}") &&
                lines[bars[0].offset + 1].contains("{{16.0, 62.0}, {44.0, 44.0}}")
            check("actual_native03_named_editor_back_boundary", originalBack)
            check("actual_native03_has_editor_and_no_library_cell", text.contains("identifier: 'editor'") &&
                  !text.contains("Cell,") && text.contains("Preparing support for Describe"))
            allow("actual_native03_observed_geometry_selects_back")
            originalReplay = true
        } else if CommandLine.arguments.count != 1 {
            throw NSError(domain: "ImportedEditorNavigationArguments", code: 1)
        }
        let failures = events.filter { $0["outcome"] as? String == "FAIL" }
        let output: [String: Any] = ["schema": "cherri-imported-editor-navigation-controls/1",
            "checks": events.count, "failures": failures.count, "results": events,
            "nativeAXOriginalBoundaryReplay": originalReplay, "nativeImportExecuted": false,
            "nativeRuntimeExecuted": false, "libraryIdentityGranted": false, "formalAcceptanceGranted": false]
        FileHandle.standardOutput.write(try JSONSerialization.data(withJSONObject: output, options: [.sortedKeys]))
        FileHandle.standardOutput.write(Data("\n".utf8))
        if !failures.isEmpty { throw NSError(domain: "ImportedEditorNavigationControls", code: failures.count) }
    }
}
