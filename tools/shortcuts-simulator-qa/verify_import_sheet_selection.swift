import Foundation
import CoreGraphics

@main enum VerifyImportSheetSelection {
    typealias Node = QAImportSheetSelection.Node
    typealias Candidate = QAImportSheetSelection.Candidate
    static let name = "qa-storage-delete-v2"
    static let appFrame = CGRect(x: 0, y: 0, width: 402, height: 874)
    static let sheetFrame = CGRect(x: 0, y: 72, width: 402, height: 802)
    static let titleFrame = CGRect(x: 58, y: 142, width: 286, height: 34)
    static let buttonFrame = CGRect(x: 24, y: 789, width: 354, height: 50)

    static func node(_ kind: QAImportSheetSelection.Kind, _ label: String = "", frame: CGRect,
                     id: String = "", enabled: Bool = true, selected: Bool = false,
                     children: [Node] = []) -> Node {
        Node(kind: kind, label: label, identifier: id, enabled: enabled, selected: selected,
             frame: frame, children: children)
    }
    static func copy(_ value: Node, kind: QAImportSheetSelection.Kind? = nil, label: String? = nil,
                     id: String? = nil, enabled: Bool? = nil, selected: Bool? = nil,
                     frame: CGRect? = nil, children: [Node]? = nil) -> Node {
        node(kind ?? value.kind, label ?? value.label, frame: frame ?? value.frame,
             id: id ?? value.identifier, enabled: enabled ?? value.enabled,
             selected: selected ?? value.selected, children: children ?? value.children)
    }
    static func button(_ label: String = "Add Shortcut", frame: CGRect = buttonFrame) -> Node {
        node(.button, label, frame: frame, children: [node(.staticText, label, frame: frame),
            node(.unsupported, "Add", frame: CGRect(x: frame.midX-8, y: frame.midY-8, width: 16, height: 16), id: "plus.circle.fill")])
    }
    static func title(_ value: String = name) -> Node { node(.staticText, value, frame: titleFrame) }
    static func sheet(_ value: String = name, add: Node = button()) -> Node {
        node(.other, frame: sheetFrame, children: [title(value),
            node(.staticText, "Add this shortcut", frame: CGRect(x: 24, y: 220, width: 350, height: 30)), add])
    }
    static func app(_ sheets: [Node]) -> Node {
        node(.application, "Shortcuts", frame: appFrame, children: [node(.window, frame: appFrame, children: sheets)])
    }
    static func candidate(_ index: Int, _ value: Node = button(), hittable: Bool = true) -> Candidate {
        Candidate(queryIndex: index, exists: true, hittable: hittable, snapshot: value)
    }

    struct Frame: Decodable { let x: Double; let y: Double; let width: Double; let height: Double }
    struct Flat: Decodable {
        let path: [Int]; let type: Int; let label: String; let identifier: String
        let enabled: Bool; let selected: Bool; let frame: Frame; let childCount: Int
    }
    struct ObservedCandidate: Decodable {
        let queryIndex: Int; let exists: Bool; let hittable: Bool; let snapshot: [Flat]?
    }
    static func fromFlat(_ rows: [Flat]) throws -> Node {
        func key(_ path: [Int]) -> String { path.map(String.init).joined(separator: "/") }
        var indexed: [String: Flat] = [:]
        for row in rows {
            guard row.path.count <= 64, row.path.allSatisfy({ $0 >= 0 }), indexed[key(row.path)] == nil,
                  row.childCount >= 0, row.childCount <= 8192 else {
                throw NSError(domain: "ObservedAX", code: 1)
            }
            indexed[key(row.path)] = row
        }
        var visited = Set<String>()
        func build(_ path: [Int]) throws -> Node {
            guard path.count <= 64, let row = indexed[key(path)], !visited.contains(key(path)) else {
                throw NSError(domain: "ObservedAX", code: 2)
            }
            visited.insert(key(path))
            let kinds: [Int: QAImportSheetSelection.Kind] = [1: .other, 2: .application, 4: .window, 9: .button, 48: .staticText]
            return node(kinds[row.type] ?? .unsupported, row.label,
                frame: CGRect(x: row.frame.x, y: row.frame.y, width: row.frame.width, height: row.frame.height),
                id: row.identifier, enabled: row.enabled, selected: row.selected,
                children: try (0..<row.childCount).map { try build(path + [$0]) })
        }
        let result = try build([])
        guard visited.count == rows.count else { throw NSError(domain: "ObservedAX", code: 3) }
        return result
    }

    static func main() throws {
        var results: [[String: Any]] = []
        func check(_ condition: Bool, _ name: String) {
            results.append(["name": name, "outcome": condition ? "PASS" : "FAIL"])
        }
        func select(_ label: String, tree: Node = app([sheet()]), candidates: [Candidate] = [candidate(0)],
                    expected: String = name, index: Int?, reason: String? = nil) {
            let result = QAImportSheetSelection.select(current: tree, candidates: candidates, expectedName: expected)
            check(result.binding?.queryIndex == index && (reason == nil || result.reason == reason), label)
        }
        let base = button()
        select("one_current_exact_sheet_selects_bound_button", index: 0)
        let binding = QAImportSheetSelection.select(current: app([sheet()]), candidates: [candidate(0)], expectedName: name).binding
        check(binding?.containerPath == [0, 0] && binding?.titlePath == [0, 0, 0] && binding?.buttonPath == [0, 0, 2],
              "binding_retains_same_container_title_button_paths")
        let hiddenA = button(frame: CGRect(x: 38, y: 742, width: 326, height: 46))
        let hiddenB = button(frame: CGRect(x: 38, y: 723, width: 326, height: 46))
        select("hidden_prefix_does_not_capture_first_match", candidates: [candidate(0, hiddenA, hittable: false),
            candidate(1, hiddenB, hittable: false), candidate(2)], index: 2)
        select("first_candidate_hittable_but_not_current_snapshot_is_ignored", candidates: [candidate(0, hiddenA), candidate(2)], index: 2)
        select("query_index_is_preserved_not_array_position", candidates: [candidate(7, hiddenA, hittable: false), candidate(2)], index: 2)
        select("hidden_query_with_identical_geometry_cannot_win", candidates: [candidate(0, base, hittable: false), candidate(1)], index: 1)
        select("only_hidden_add_is_rejected", candidates: [candidate(0, base, hittable: false)], index: nil, reason: "NO_CURRENT_HITTABLE_MATCH")
        select("all_query_matches_from_old_snapshots_rejected", candidates: [candidate(0, hiddenA), candidate(1, hiddenB)], index: nil)
        select("duplicate_hittable_query_matches_are_ambiguous", candidates: [candidate(0), candidate(1)], index: nil, reason: "AMBIGUOUS_CURRENT_MATCH")
        select("duplicate_query_index_rejected", candidates: [candidate(0), candidate(0)], index: nil, reason: "INVALID_OBSERVATION")
        select("negative_query_index_rejected", candidates: [candidate(-1)], index: nil, reason: "INVALID_OBSERVATION")
        select("unbounded_query_index_rejected", candidates: [candidate(32)], index: nil, reason: "INVALID_OBSERVATION")
        select("unbounded_candidate_inventory_rejected", candidates: (0..<33).map { candidate($0) }, index: nil, reason: "INVALID_OBSERVATION")
        select("no_query_matches_rejected", candidates: [], index: nil)
        select("absent_query_match_retained_without_selection", candidates: [Candidate(queryIndex: 0, exists: false, hittable: false, snapshot: nil)], index: nil)
        select("absent_but_hittable_query_rejected", candidates: [Candidate(queryIndex: 0, exists: false, hittable: true, snapshot: nil)], index: nil, reason: "INVALID_OBSERVATION")
        select("present_without_snapshot_rejected", candidates: [Candidate(queryIndex: 0, exists: true, hittable: true, snapshot: nil)], index: nil, reason: "INVALID_OBSERVATION")
        select("wrong_sheet_name_rejected", tree: app([sheet("other")]), index: nil)
        select("name_substring_does_not_prove_identity", tree: app([sheet(name + "-other")]), index: nil)
        select("expected_title_in_other_sheet_cannot_authorize_button", tree: app([
            node(.other, frame: sheetFrame, children: [title(), node(.unsupported, frame: buttonFrame)]), sheet("other")]), index: nil)
        select("application_title_cannot_authorize_foreign_sheet", tree: copy(app([sheet("other")]), label: name), index: nil)
        select("sibling_title_cannot_authorize_foreign_sheet", tree: app([title(), sheet("other")]), index: nil)
        select("two_current_exact_sheets_rejected", tree: app([sheet(), sheet()]), index: nil, reason: "AMBIGUOUS_CURRENT_MATCH")
        select("two_add_controls_in_one_container_rejected", tree: app([copy(sheet(), children: [title(), base, base])]), index: nil)
        select("two_exact_titles_in_one_container_rejected", tree: app([copy(sheet(), children: [title(), title(), base])]), index: nil)
        let wrappedTitle = node(.other, frame: titleFrame, children: [node(.other, frame: titleFrame, children: [title()])])
        let wrappedButton = node(.other, frame: buttonFrame, children: [base])
        select("single_child_generic_wrappers_remain_transparent", tree: app([node(.other, frame: sheetFrame,
            children: [wrappedTitle, wrappedButton])]), index: 0)
        select("title_inside_different_button_is_not_sheet_title", tree: app([node(.other, frame: sheetFrame,
            children: [node(.button, "More", frame: titleFrame, children: [title()]), base])]), index: nil)
        select("disabled_container_rejected", tree: app([copy(sheet(), enabled: false)]), index: nil)
        select("disabled_ancestor_rejected", tree: app([node(.other, frame: appFrame, enabled: false, children: [sheet()])]), index: nil)
        select("disabled_title_rejected", tree: app([node(.other, frame: sheetFrame, children: [copy(title(), enabled: false), base])]), index: nil)
        select("disabled_button_rejected", tree: app([sheet(add: copy(base, enabled: false))]), candidates: [candidate(0, copy(base, enabled: false))], index: nil)
        select("query_frame_must_match_current_button", candidates: [candidate(0, copy(base, frame: buttonFrame.offsetBy(dx: 1, dy: 0)))], index: nil)
        select("query_identifier_must_match_current_button", candidates: [candidate(0, copy(base, id: "other"))], index: nil)
        select("query_selected_state_must_match_current_button", candidates: [candidate(0, copy(base, selected: true))], index: nil)
        select("query_children_must_match_current_button", candidates: [candidate(0, copy(base, children: []))], index: nil)
        select("query_nested_label_must_match_current_button", candidates: [candidate(0, copy(base,
            children: [copy(base.children[0], label: "different"), base.children[1]]))], index: nil)
        select("query_must_be_button", candidates: [candidate(0, copy(base, kind: .staticText))], index: nil, reason: "INVALID_OBSERVATION")
        select("unknown_consent_label_rejected", candidates: [candidate(0, button("Open"))], index: nil, reason: "INVALID_OBSERVATION")
        select("nan_geometry_rejected", candidates: [candidate(0, copy(base, frame: CGRect(x: CGFloat.nan, y: 1, width: 2, height: 3)))], index: nil, reason: "INVALID_OBSERVATION")
        select("zero_area_current_button_rejected", tree: app([sheet(add: copy(base, frame: .zero))]),
               candidates: [candidate(0, copy(base, frame: .zero))], index: nil)
        select("button_outside_container_rejected", tree: app([copy(sheet(), frame: CGRect(x: 0, y: 72, width: 402, height: 600))]), index: nil)
        select("button_outside_application_rejected", tree: copy(app([sheet()]), frame: CGRect(x: 0, y: 0, width: 402, height: 700)), index: nil)
        select("window_cannot_be_global_container", tree: node(.application, "Shortcuts", frame: appFrame,
            children: [node(.window, frame: appFrame, children: [title(), base])]), index: nil)
        select("sheet_kind_container_is_supported", tree: app([copy(sheet(), kind: .sheet)]), index: 0)
        select("scroll_container_is_supported", tree: app([copy(sheet(), kind: .scrollView)]), index: 0)
        let plus = button("+ Add Shortcut")
        select("explicit_plus_add_label_supported", tree: app([sheet(add: plus)]), candidates: [candidate(0, plus)], index: 0)
        let replace = button("Replace")
        select("explicit_replace_label_preserves_old_route", tree: app([sheet(add: replace)]), candidates: [candidate(0, replace)], index: 0)
        select("unicode_title_exact_bytes_supported", tree: app([sheet("שלום🙂")]), expected: "שלום🙂", index: 0)
        select("canonically_equivalent_but_different_title_bytes_rejected", tree: app([sheet("e\u{301}")]), expected: "é", index: nil)
        select("empty_expected_name_rejected", expected: "", index: nil, reason: "INVALID_OBSERVATION")
        select("unbounded_expected_name_rejected", expected: String(repeating: "x", count: 4097), index: nil, reason: "INVALID_OBSERVATION")
        let deep = (0..<66).reduce(sheet()) { result, _ in node(.other, frame: sheetFrame, children: [result]) }
        select("unbounded_snapshot_depth_rejected", tree: app([deep]), index: nil, reason: "INVALID_OBSERVATION")
        select("unbounded_snapshot_nodes_rejected", tree: app([node(.other, frame: sheetFrame,
            children: (0..<8192).map { _ in title() })]), index: nil, reason: "INVALID_OBSERVATION")
        select("unbounded_snapshot_label_rejected", tree: app([copy(sheet(), label: String(repeating: "x", count: 4097))]), index: nil, reason: "INVALID_OBSERVATION")

        var originalReplay = false
        if CommandLine.arguments.count == 3 {
            let current = try JSONDecoder().decode([Flat].self, from: Data(contentsOf: URL(fileURLWithPath: CommandLine.arguments[1])))
            let observed = try JSONDecoder().decode([ObservedCandidate].self, from: Data(contentsOf: URL(fileURLWithPath: CommandLine.arguments[2])))
            let candidates = try observed.map { Candidate(queryIndex: $0.queryIndex, exists: $0.exists, hittable: $0.hittable,
                                                          snapshot: try $0.snapshot.map(fromFlat)) }
            check(current.count == 38 && candidates.count == 3, "actual_ax_original_inventory_38_nodes_three_candidates")
            select("actual_ax_original_selects_only_hittable_current_container", tree: try fromFlat(current), candidates: candidates, index: 2)
            let decision = QAImportSheetSelection.select(current: try fromFlat(current), candidates: candidates, expectedName: name)
            check(decision.binding?.containerPath == [0,0,1,0,0,0,0,1,0,0,0] &&
                  decision.binding?.titlePath == [0,0,1,0,0,0,0,1,0,0,0,0] &&
                  decision.binding?.buttonPath == [0,0,1,0,0,0,0,1,0,0,0,6],
                  "actual_ax_original_title_button_share_exact_observed_container")
            originalReplay = true
        } else if CommandLine.arguments.count != 1 {
            throw NSError(domain: "ImportSheetSelectionArguments", code: 1)
        }
        let failures = results.filter { $0["outcome"] as? String == "FAIL" }
        let output: [String: Any] = ["schema": "cherri-import-sheet-selection-controls/1",
            "checks": results.count, "failures": failures.count, "results": results,
            "nativeAXOriginalReplay": originalReplay, "nativeImportExecuted": false,
            "nativeRuntimeExecuted": false, "formalAcceptanceGranted": false]
        FileHandle.standardOutput.write(try JSONSerialization.data(withJSONObject: output, options: [.sortedKeys]))
        FileHandle.standardOutput.write(Data("\n".utf8))
        if !failures.isEmpty { throw NSError(domain: "ImportSheetSelectionControls", code: failures.count) }
    }
}
