import Foundation
import CoreGraphics

@main enum VerifyImportedLibraryIdentity {
    typealias Model = QAImportedLibrarySelection
    typealias Node = QAImportSheetSelection.Node
    typealias Candidate = QAImportSheetSelection.Candidate
    static let name = "qa-storage-delete-v2"
    static let appFrame = CGRect(x: 0, y: 0, width: 402, height: 874)
    static let cellFrame = CGRect(x: 16, y: 160, width: 176, height: 100)

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
    static func cell(_ title: String = name, id: String = "owned-current-cell") -> Node {
        node(.cell, label: title, id: id, frame: cellFrame)
    }
    static func app(_ cells: [Node] = [cell()]) -> Node {
        node(.application, label: "Shortcuts", frame: appFrame,
             children: [node(.window, frame: appFrame, children:
                [node(.collectionView, frame: appFrame, children: cells)])])
    }
    static func candidate(_ value: Node = cell(), index: Int = 0, hittable: Bool = true) -> Candidate {
        Candidate(queryIndex: index, exists: true, hittable: hittable, snapshot: value)
    }
    static func context(_ title: String = name, foreground: Bool = true, dispatched: Bool = true,
                        now: Double = 7, deadline: Double = 30) -> Model.Context {
        Model.Context(expectedName: title, foreground: foreground, dispatchConfirmed: dispatched,
                      now: Date(timeIntervalSince1970: now), deadline: Date(timeIntervalSince1970: deadline))
    }
    static func main() throws {
        var events: [[String: Any]] = []
        func check(_ name: String, _ value: Bool) {
            events.append(["name": name, "outcome": value ? "PASS" : "FAIL"])
        }
        func decide(_ tree: Node = app(), _ candidates: [Candidate] = [candidate()],
                    _ context: Model.Context = context()) -> Model.Decision {
            Model.select(current: tree, candidates: candidates, context: context)
        }
        func allow(_ name: String, tree: Node = app(), candidates: [Candidate] = [candidate()],
                   ctx: Model.Context = context()) {
            let d = decide(tree, candidates, ctx)
            check(name, d.canConfirm && d.reason == "UNIQUE_CURRENT_NAMED_LIBRARY_CELL" &&
                  d.currentNamedCellCount == 1 && d.queryCandidateCount == 1)
        }
        func deny(_ name: String, tree: Node = app(), candidates: [Candidate] = [candidate()],
                  ctx: Model.Context = context()) {
            check(name, !decide(tree, candidates, ctx).canConfirm)
        }
        allow("one_exact_current_hittable_cell")
        check("current_cell_path_is_retained", decide().binding?.cellPath == [0,0,0])
        check("actual_label_and_identifier_are_retained",
              decide().binding?.label == name && decide().binding?.identifier == "owned-current-cell")
        allow("unicode_exact_title", tree: app([cell("שלום 🧪")]),
              candidates: [candidate(cell("שלום 🧪"))], ctx: context("שלום 🧪"))
        let title = node(.staticText, label: name, frame: cellFrame)
        let suffixed = copy(cell(), label: name + ", 31 actions", children: [title])
        allow("metadata_suffix_requires_own_exact_title", tree: app([suffixed]), candidates: [candidate(suffixed)])
        let prefixOnly = copy(cell(), label: name + ", another shortcut")
        deny("metadata_prefix_without_exact_title", tree: app([prefixOnly]), candidates: [candidate(prefixOnly)])
        let wrongTitle = copy(suffixed, children: [copy(title, label: name + "-other")])
        deny("metadata_prefix_wrong_title", tree: app([wrongTitle]), candidates: [candidate(wrongTitle)])
        let childCell = copy(suffixed, children: [cell()])
        deny("another_cell_title_does_not_bind_parent", tree: app([childCell]), candidates: [candidate(childCell)])
        allow("unrelated_current_cell_does_not_count",
              tree: app([cell("unrelated", id: "other"), cell()]))
        deny("two_identical_looking_current_cells_are_ambiguous", tree: app([cell(),cell()]))
        deny("two_distinct_current_record_cells_are_ambiguous",
             tree: app([cell(),cell(id: "second-record")]),
             candidates: [candidate(), candidate(cell(id: "second-record"), index: 1)])
        deny("hidden_duplicate_current_cell_is_not_discarded",
             tree: app([cell(),copy(cell(id: "second-record"), enabled: false)]))
        deny("offscreen_duplicate_current_cell_is_not_discarded",
             tree: app([cell(),copy(cell(id: "second-record"), frame: cellFrame.offsetBy(dx: 900,dy: 0))]))
        deny("multiple_query_matches_are_ambiguous",
             candidates: [candidate(), candidate(cell(), index: 1)])
        deny("hidden_query_duplicate_is_not_discarded",
             candidates: [candidate(), candidate(cell(), index: 1, hittable: false)])
        deny("no_current_named_cell", tree: app([]))
        deny("query_only_cannot_establish_current_cell", tree: app([cell("other")]))
        deny("no_query_cell", candidates: [])
        deny("not_hittable", candidates: [candidate(cell(), hittable: false)])
        deny("nonexistent_query", candidates: [.init(queryIndex: 0, exists: false, hittable: false, snapshot: nil)])
        deny("inconsistent_query_exists", candidates: [.init(queryIndex: 0, exists: false, hittable: true, snapshot: cell())])
        deny("inconsistent_query_snapshot", candidates: [.init(queryIndex: 0, exists: true, hittable: false, snapshot: nil)])
        deny("query_wrong_role", candidates: [candidate(copy(cell(), kind: .button))])
        deny("current_wrong_role", tree: app([copy(cell(), kind: .staticText)]))
        deny("query_wrong_identifier", candidates: [candidate(cell(id: "stale-record"))])
        deny("query_stale_label", candidates: [candidate(copy(cell(), label: name + ", old", children: [title]))])
        deny("query_stale_frame", candidates: [candidate(copy(cell(), frame: cellFrame.offsetBy(dx: 1,dy: 0)))])
        deny("query_stale_selection", candidates: [candidate(copy(cell(), selected: true))])
        deny("query_stale_children", candidates: [candidate(copy(cell(), children: [title]))])
        deny("query_disabled", candidates: [candidate(copy(cell(), enabled: false))])
        deny("current_disabled", tree: app([copy(cell(), enabled: false)]),
             candidates: [candidate(copy(cell(), enabled: false))])
        let disabledParent = node(.collectionView, frame: appFrame, enabled: false, children: [cell()])
        deny("disabled_ancestor", tree: node(.application, frame: appFrame, children: [disabledParent]))
        deny("outside_current_application", tree: app([copy(cell(), frame: cellFrame.offsetBy(dx: 900,dy: 0))]),
             candidates: [candidate(copy(cell(), frame: cellFrame.offsetBy(dx: 900,dy: 0)))])
        deny("zero_cell_frame", tree: app([copy(cell(), frame: .zero)]),
             candidates: [candidate(copy(cell(), frame: .zero))])
        deny("not_foreground", ctx: context(foreground: false))
        deny("dispatch_not_confirmed", ctx: context(dispatched: false))
        deny("exact_deadline_is_rejected", ctx: context(now: 30))
        deny("expired_deadline_is_rejected", ctx: context(now: 31))
        deny("nonfinite_now", ctx: context(now: .nan))
        deny("nonfinite_deadline", ctx: context(deadline: .infinity))
        deny("empty_name", ctx: context(""))
        deny("oversized_name", ctx: context(String(repeating: "x", count: 4097)))
        deny("unicode_normalization_is_not_identity",
             tree: app([cell("e\u{301}")]), candidates: [candidate(cell("e\u{301}"))], ctx: context("é"))
        deny("prefix_only_name_is_not_identity", tree: app([cell(name + "-suffix")]))
        deny("global_title_cannot_bind_another_cell",
             tree: app([cell("unrelated"), title]), candidates: [candidate(cell("unrelated"))])
        deny("wrong_application_role", tree: copy(app(), kind: .window))
        deny("disabled_application", tree: copy(app(), enabled: false))
        deny("empty_application_frame", tree: copy(app(), frame: .zero))
        deny("invalid_nonfinite_frame", tree: copy(app(), frame: CGRect(x: Double.nan, y: 0, width: 402, height: 874)))
        deny("negative_query_index", candidates: [candidate(index: -1)])
        deny("query_index_out_of_bounds", candidates: [candidate(index: 32)])
        deny("duplicate_query_indices", candidates: [candidate(), candidate()])
        deny("too_many_query_matches", candidates: (0..<33).map { candidate(index: $0) })
        deny("oversized_identifier", tree: app([cell(id: String(repeating: "x", count: 4097))]))
        var deep = cell()
        for _ in 0..<65 { deep = node(.other, frame: appFrame, children: [deep]) }
        deny("overdeep_current_tree", tree: node(.application, frame: appFrame, children: [deep]))
        let many = (0..<8193).map { _ in node(.other, frame: .zero) }
        deny("oversized_current_tree", tree: copy(app(), children: many))
        let healthy = decide()
        var calls = 0
        var captures = 0
        func transition(_ name: String, after: Model.Decision) {
            calls = 0; captures = 0
            let result = Model.inspectForConfirmation(observe: {
                calls += 1; return calls == 1 ? healthy : after
            }, capture: { captures += 1 })
            check(name, !result.canConfirm && calls == 2 && captures == 1)
        }
        transition("capture_disappearance_rejected", after: decide(app([])))
        transition("capture_wrong_title_rejected", after: decide(app([cell("other")])))
        transition("capture_duplicate_rejected", after: decide(app([cell(),cell()])))
        transition("capture_deadline_expiry_rejected", after: decide(app(), [candidate()], context(now: 30)))
        transition("capture_new_unique_record_rejected",
                   after: decide(app([cell(id: "replaced-record")]), [candidate(cell(id: "replaced-record"))]))
        transition("capture_changed_path_rejected",
                   after: decide(app([cell("other", id: "other"),cell()])))
        calls = 0; captures = 0
        let unchanged = Model.inspectForConfirmation(observe: { calls += 1; return healthy },
                                                     capture: { captures += 1 })
        check("capture_reobserves_same_identity", unchanged.canConfirm && calls == 2 && captures == 1)
        calls = 0; captures = 0
        let absent = Model.inspectForConfirmation(observe: { calls += 1; return decide(app([])) },
                                                  capture: { captures += 1 })
        check("no_match_does_not_capture", !absent.canConfirm && calls == 1 && captures == 0)
        enum BoundaryFailure: Error { case expected }
        var captureThrew = false
        do {
            _ = try Model.inspectForConfirmation(observe: { healthy }, capture: { throw BoundaryFailure.expected })
        } catch BoundaryFailure.expected { captureThrew = true }
        check("capture_error_never_authorizes_confirmation", captureThrew)
        var observeThrew = false
        calls = 0
        do {
            _ = try Model.inspectForConfirmation(observe: {
                calls += 1
                if calls == 2 { throw BoundaryFailure.expected }
                return healthy
            }, capture: {})
        } catch BoundaryFailure.expected { observeThrew = true }
        check("fresh_observation_error_never_authorizes_confirmation", observeThrew)
        let composed = cell(id: "é")
        let decomposed = cell(id: "e\u{301}")
        var unicodeObservations = 0
        let unicodeTransition = Model.inspectForConfirmation(observe: {
            unicodeObservations += 1
            let value = unicodeObservations == 1 ? composed : decomposed
            return decide(app([value]), [candidate(value)])
        }, capture: {})
        check("capture_unicode_identifier_bytes_change_rejected",
              !unicodeTransition.canConfirm && unicodeObservations == 2)
        let failures = events.filter { $0["outcome"] as? String == "FAIL" }
        let output: [String: Any] = ["schema": "cherri-imported-library-identity-controls/1",
            "checks": events.count, "failures": failures.count, "results": events,
            "nativeImportExecuted": false, "nativeRuntimeExecuted": false,
            "databaseIdentityGranted": false, "formalAcceptanceGranted": false]
        FileHandle.standardOutput.write(try JSONSerialization.data(withJSONObject: output, options: [.sortedKeys]))
        FileHandle.standardOutput.write(Data("\n".utf8))
        if !failures.isEmpty { throw NSError(domain: "ImportedLibraryIdentityControls", code: failures.count) }
    }
}
