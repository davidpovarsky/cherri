import Foundation
import CoreGraphics

@main enum VerifyExistingEditorNavigation {
    typealias Model = QAExistingEditorNavigation
    typealias Node = QAImportSheetSelection.Node
    typealias Candidate = QAImportSheetSelection.Candidate
    static let name = "CherriFN04Materialized_9f4ac00a6d8c4acba51a9f37a9ce232d"
    static let appFrame = CGRect(x: 0, y: 0, width: 402, height: 874)
    static let barFrame = CGRect(x: 0, y: 62, width: 402, height: 60)
    static let editFrame = CGRect(x: 318, y: 70, width: 62, height: 44)
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
    static func edit() -> Node { node(.button, label: "Editor", frame: editFrame) }
    static func bar(_ value: String = name, buttons: [Node] = [edit()]) -> Node {
        node(.navigationBar, id: value, frame: barFrame, children: buttons)
    }
    static func app(_ bars: [Node] = [bar()]) -> Node {
        node(.application, label: "Shortcuts", frame: appFrame,
             children: [node(.window, frame: appFrame, children: bars)])
    }
    static func candidate(_ n: Node = edit(), index: Int = 0, hittable: Bool = true) -> Candidate {
        Candidate(queryIndex: index, exists: true, hittable: hittable, snapshot: n)
    }
    static func context(_ value: String = name, foreground: Bool = true,
                        attempted: Bool = false, now: Double = 7, deadline: Double = 25) -> Model.Context {
        Model.Context(expectedName: value, foreground: foreground, alreadyInvoked: attempted,
                      now: Date(timeIntervalSince1970: now), deadline: Date(timeIntervalSince1970: deadline))
    }
    static func capturedTree(_ path: String) throws -> Node {
        guard let rows = try JSONSerialization.jsonObject(with: Data(contentsOf: URL(fileURLWithPath: path))) as? [[String: Any]],
              rows.count == 86 else { throw NSError(domain: "ExistingEditorControls", code: 1) }
        func key(_ values: [Int]) -> String { values.map(String.init).joined(separator: "/") }
        var indexed: [String: [String: Any]] = [:]
        for row in rows {
            guard let path = row["path"] as? [Int], indexed[key(path)] == nil else {
                throw NSError(domain: "ExistingEditorControls", code: 2)
            }
            indexed[key(path)] = row
        }
        func visit(_ path: [Int]) throws -> Node {
            guard let row = indexed[key(path)], let type = row["type"] as? Int,
                  let label = row["label"] as? String, let id = row["identifier"] as? String,
                  let enabled = row["enabled"] as? Bool, let selected = row["selected"] as? Bool,
                  let frame = row["frame"] as? [String: Double], let x = frame["x"], let y = frame["y"],
                  let width = frame["width"], let height = frame["height"], let count = row["childCount"] as? Int else {
                throw NSError(domain: "ExistingEditorControls", code: 3)
            }
            let kind: QAImportSheetSelection.Kind
            switch type {
            case 2: kind = .application
            case 4: kind = .window
            case 21: kind = .navigationBar
            case 9: kind = .button
            case 48: kind = .staticText
            default: kind = .other
            }
            return node(kind, label: label, id: id, frame: CGRect(x: x, y: y, width: width, height: height),
                        enabled: enabled, selected: selected, children: try (0..<count).map { try visit(path + [$0]) })
        }
        return try visit([])
    }
    static func matches(_ tree: Node, _ predicate: (Node) -> Bool) -> [Node] {
        (predicate(tree) ? [tree] : []) + tree.children.flatMap { matches($0, predicate) }
    }
    static func main() throws {
        guard CommandLine.arguments.count == 2 else { throw NSError(domain: "ExistingEditorControls", code: 4) }
        var events: [[String: Any]] = []
        func check(_ name: String, _ value: Bool) {
            events.append(["name": name, "outcome": value ? "PASS" : "FAIL"])
        }
        func decide(_ tree: Node = app(), _ candidates: [Candidate] = [candidate()],
                    _ ctx: Model.Context = context()) -> Model.Decision {
            Model.select(current: tree, candidates: candidates, context: ctx)
        }
        func deny(_ n: String, tree: Node = app(), candidates: [Candidate] = [candidate()], ctx: Model.Context = context()) {
            check(n, !decide(tree, candidates, ctx).canNavigate)
        }
        let actual = try capturedTree(CommandLine.arguments[1])
        let actualButtons = matches(actual) { $0.kind == .button && $0.label == "Editor" }
        guard actualButtons.count == 1 else { throw NSError(domain: "ExistingEditorControls", code: 5) }
        let actualDecision = decide(actual, [candidate(actualButtons[0])])
        check("actual_86_node_capture_selects_only_editor", actualDecision.canNavigate &&
              actualDecision.reason == "UNIQUE_CURRENT_REQUESTED_EDITOR_BUTTON" &&
              actualDecision.binding?.navigationBarPath == [0,0,2,0,0,0,0,0,0] &&
              actualDecision.binding?.buttonPath == [0,0,2,0,0,0,0,0,0,2])
        check("simple_exact_identity", decide().canNavigate)
        deny("wrong_exact_name", ctx: context(name + "-other"))
        deny("matching_label_cannot_replace_identifier", tree: app([copy(bar("other"), label: name)]))
        deny("two_named_bars_even_one_has_button", tree: app([bar(),bar(buttons: [])]))
        deny("two_editor_buttons", tree: app([bar(buttons: [edit(),edit()])]))
        deny("editor_from_unrelated_bar", tree: app([bar(buttons: []),bar("other")]))
        deny("global_editor_not_direct_child", tree: app([bar(buttons: [node(.other, frame: barFrame, children: [edit()])])]))
        for label in ["play", "Run", "Add Shortcut", "Replace", "Back", "Edit"] {
            let wrong = copy(edit(), label: label)
            deny("wrong_button_" + label, tree: app([bar(buttons: [wrong])]), candidates: [candidate(wrong)])
        }
        deny("not_hittable", candidates: [candidate(hittable: false)])
        deny("query_not_current_frame", candidates: [candidate(copy(edit(), frame: editFrame.offsetBy(dx: 1, dy: 0)))])
        deny("query_not_current_selection", candidates: [candidate(copy(edit(), selected: true))])
        deny("query_not_current_children", candidates: [candidate(copy(edit(), children: [edit()]))])
        deny("duplicate_query", candidates: [candidate(index: 0),candidate(index: 1)])
        deny("duplicate_query_index", candidates: [candidate(),candidate()])
        deny("query_absent", candidates: [])
        deny("query_inconsistent_existence", candidates: [Candidate(queryIndex: 0, exists: false, hittable: true, snapshot: edit())])
        deny("button_has_unobserved_identifier", tree: app([bar(buttons: [copy(edit(), id: "Run")])]), candidates: [candidate(copy(edit(), id: "Run"))])
        deny("disabled_navigation_bar", tree: app([copy(bar(), enabled: false)]))
        deny("background", ctx: context(foreground: false))
        deny("already_invoked", ctx: context(attempted: true))
        deny("deadline_reached", ctx: context(now: 25))
        deny("deadline_exceeded", ctx: context(now: 26))
        deny("nonfinite_clock", ctx: context(now: .nan))
        deny("nonfinite_deadline", ctx: context(deadline: .infinity))
        deny("nonfinite_frame", tree: app([copy(bar(), frame: CGRect(x: Double.nan, y: 0, width: 402, height: 60))]))
        deny("frame_outside_app", tree: app([copy(bar(), frame: barFrame.offsetBy(dx: 0, dy: 900))]))
        deny("button_outside_bar", tree: app([bar(buttons: [copy(edit(), frame: editFrame.offsetBy(dx: 0, dy: 200))])]),
             candidates: [candidate(copy(edit(), frame: editFrame.offsetBy(dx: 0, dy: 200)))])
        deny("depth_overflow", tree: app([(0..<65).reduce(bar()) { n,_ in node(.other, frame: appFrame, children: [n]) }]))
        deny("node_overflow", tree: app([node(.other, frame: appFrame, children: (0..<8192).map { _ in edit() })]))
        deny("label_overflow", tree: app([copy(bar(), label: String(repeating: "x", count: 4097))]))
        var observations = 0, captures = 0
        let healthy = decide()
        let stale = Model.inspectForNavigation(observe: {
            observations += 1
            return observations == 1 ? healthy : decide(app([bar("other")]))
        }, capture: { captures += 1 })
        check("capture_identity_change_rejected", !stale.canNavigate && observations == 2 && captures == 1)
        observations = 0; captures = 0
        let late = Model.inspectForNavigation(observe: {
            observations += 1
            return observations == 1 ? healthy : decide(app(), [candidate()], context(now: 25))
        }, capture: { captures += 1 })
        check("capture_deadline_expiry_rejected", !late.canNavigate && observations == 2 && captures == 1)
        observations = 0; captures = 0
        let rejected = Model.inspectForNavigation(observe: {
            observations += 1; return decide(app(), [], context())
        }, capture: { captures += 1 })
        check("initial_rejection_does_not_capture_or_reobserve", !rejected.canNavigate && observations == 1 && captures == 0)
        let failed = events.filter { ($0["outcome"] as? String) != "PASS" }
        let result: [String: Any] = ["schema": "cherri-existing-editor-navigation-foundation-controls/1",
            "status": failed.isEmpty ? "PASS" : "FAIL", "events": events,
            "nativeInvocations": 0, "workflowRuns": 0, "uiTapInvocations": 0,
            "actualPriorAXInput": true, "candidateHittabilityIsSyntheticControlInput": true]
        let data = try JSONSerialization.data(withJSONObject: result, options: [.prettyPrinted, .sortedKeys])
        FileHandle.standardOutput.write(data)
        FileHandle.standardOutput.write(Data("\n".utf8))
        if !failed.isEmpty { throw NSError(domain: "ExistingEditorControls", code: 6) }
    }
}
