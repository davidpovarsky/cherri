import Foundation
import CoreGraphics

/// Source-owned model of one current accessibility snapshot. Query matches are
/// separate observations: UIKit may return hidden matches absent from this tree.
enum QAImportSheetSelection {
    enum Kind: String {
        case application, window, other, sheet, scrollView, button, staticText, navigationBar, unsupported
    }

    struct Node {
        let kind: Kind
        let label: String
        let identifier: String
        let enabled: Bool
        let selected: Bool
        let frame: CGRect
        let children: [Node]
    }

    struct Candidate {
        let queryIndex: Int
        let exists: Bool
        let hittable: Bool
        let snapshot: Node?
    }

    struct Binding {
        let queryIndex: Int
        let containerPath: [Int]
        let titlePath: [Int]
        let buttonPath: [Int]
    }

    struct Decision {
        let binding: Binding?
        let reason: String
        let currentContainerCount: Int
        let matchedCandidateCount: Int
    }

    static func bytesEqual(_ left: String, _ right: String) -> Bool {
        Data(left.utf8) == Data(right.utf8)
    }

    private static func addLabel(_ label: String) -> Bool {
        ["Add Shortcut", "+ Add Shortcut", "Replace"].contains { bytesEqual(label, $0) }
    }

    private static func validFrame(_ frame: CGRect, positive: Bool = false) -> Bool {
        [frame.minX, frame.minY, frame.width, frame.height].allSatisfy { $0.isFinite } &&
        frame.width >= 0 && frame.height >= 0 && (!positive || (frame.width > 0 && frame.height > 0))
    }

    private static func sameNode(_ left: Node, _ right: Node) -> Bool {
        left.kind == right.kind && bytesEqual(left.label, right.label) &&
        bytesEqual(left.identifier, right.identifier) && left.enabled == right.enabled &&
        left.selected == right.selected && left.frame == right.frame &&
        left.children.count == right.children.count &&
        zip(left.children, right.children).allSatisfy { sameNode($0.0, $0.1) }
    }

    /// Only a single-child generic wrapper is transparent. A title inside a
    /// different control or sibling sheet never supplies this container's name.
    private static func branch(_ node: Node, path: [Int]) -> (Node, [Int]) {
        var current = node
        var currentPath = path
        while current.kind == .other && current.children.count == 1 && current.enabled {
            current = current.children[0]
            currentPath.append(0)
        }
        return (current, currentPath)
    }

    static func select(current: Node, candidates: [Candidate], expectedName: String) -> Decision {
        func rejected(_ reason: String, containers: Int = 0, matches: Int = 0) -> Decision {
            Decision(binding: nil, reason: reason, currentContainerCount: containers, matchedCandidateCount: matches)
        }
        guard !expectedName.isEmpty, expectedName.utf8.count <= 4096,
              current.kind == .application, current.enabled, validFrame(current.frame, positive: true),
              candidates.count <= 32, Set(candidates.map(\.queryIndex)).count == candidates.count,
              candidates.allSatisfy({ $0.queryIndex >= 0 && $0.queryIndex < 32 &&
                  (!$0.hittable || $0.exists) && ($0.exists == ($0.snapshot != nil)) }) else {
            return rejected("INVALID_OBSERVATION")
        }
        var count = 0
        func validate(_ node: Node, depth: Int) -> Bool {
            count += 1
            guard count <= 8192, depth <= 64, node.label.utf8.count <= 4096,
                  node.identifier.utf8.count <= 4096, validFrame(node.frame) else { return false }
            return node.children.allSatisfy { validate($0, depth: depth + 1) }
        }
        guard validate(current, depth: 0) else { return rejected("INVALID_OBSERVATION") }
        for candidate in candidates {
            if let snapshot = candidate.snapshot {
                count = 0
                guard validate(snapshot, depth: 0), snapshot.kind == .button,
                      addLabel(snapshot.label) else { return rejected("INVALID_OBSERVATION") }
            }
        }
        var containers: [(path: [Int], title: [Int], button: [Int], snapshot: Node)] = []
        func visit(_ node: Node, path: [Int], ancestorsEnabled: Bool) {
            let enabled = ancestorsEnabled && node.enabled
            if enabled && [.other, .sheet, .scrollView].contains(node.kind) && validFrame(node.frame, positive: true) {
                let branches = node.children.enumerated().map { branch($0.element, path: path + [$0.offset]) }
                let titles = branches.filter { $0.0.kind == .staticText && $0.0.enabled &&
                    bytesEqual($0.0.label, expectedName) && validFrame($0.0.frame, positive: true) &&
                    node.frame.contains($0.0.frame) && current.frame.contains($0.0.frame) }
                let buttons = branches.filter { $0.0.kind == .button && $0.0.enabled && addLabel($0.0.label) &&
                    validFrame($0.0.frame, positive: true) && node.frame.contains($0.0.frame) &&
                    current.frame.contains($0.0.frame) }
                if titles.count == 1 && buttons.count == 1 {
                    containers.append((path, titles[0].1, buttons[0].1, buttons[0].0))
                }
            }
            for (index, child) in node.children.enumerated() {
                visit(child, path: path + [index], ancestorsEnabled: enabled)
            }
        }
        visit(current, path: [], ancestorsEnabled: true)
        guard !containers.isEmpty else { return rejected("NO_CURRENT_NAMED_CONTAINER") }
        var matches: [Binding] = []
        for candidate in candidates where candidate.exists && candidate.hittable {
            guard let observed = candidate.snapshot, observed.enabled else { continue }
            for container in containers where sameNode(container.snapshot, observed) {
                matches.append(Binding(queryIndex: candidate.queryIndex, containerPath: container.path,
                    titlePath: container.title, buttonPath: container.button))
            }
        }
        guard matches.count == 1 else {
            return rejected(matches.isEmpty ? "NO_CURRENT_HITTABLE_MATCH" : "AMBIGUOUS_CURRENT_MATCH",
                            containers: containers.count, matches: matches.count)
        }
        return Decision(binding: matches[0], reason: "UNIQUE_CURRENT_NAMED_CONTAINER",
                        currentContainerCount: containers.count, matchedCandidateCount: matches.count)
    }
}

/// Returning from an already imported editor is navigation only. A matched
/// editor never proves that the actual native library item exists.
enum QAImportedEditorNavigation {
    typealias Node = QAImportSheetSelection.Node
    typealias Candidate = QAImportSheetSelection.Candidate

    struct Context {
        let expectedName: String
        let foreground: Bool
        let dispatchConfirmed: Bool
        let alreadyNavigated: Bool
        let now: Date
        let deadline: Date
    }

    struct Binding {
        let queryIndex: Int
        let navigationBarPath: [Int]
        let buttonPath: [Int]
    }

    struct Decision {
        let binding: Binding?
        let reason: String
        let currentNavigationBarCount: Int
        let matchedCandidateCount: Int
        var canNavigate: Bool { binding != nil }
    }

    static func rejected(_ reason: String) -> Decision {
        Decision(binding: nil, reason: reason, currentNavigationBarCount: 0, matchedCandidateCount: 0)
    }

    private static func same(_ left: Node, _ right: Node) -> Bool {
        left.kind == right.kind && QAImportSheetSelection.bytesEqual(left.label, right.label) &&
        QAImportSheetSelection.bytesEqual(left.identifier, right.identifier) &&
        left.enabled == right.enabled && left.selected == right.selected && left.frame == right.frame &&
        left.children.count == right.children.count &&
        zip(left.children, right.children).allSatisfy { same($0.0, $0.1) }
    }

    private static func frame(_ value: CGRect, positive: Bool = false) -> Bool {
        [value.minX, value.minY, value.width, value.height].allSatisfy { $0.isFinite } &&
        value.width >= 0 && value.height >= 0 && (!positive || (value.width > 0 && value.height > 0))
    }

    static func select(current: Node, candidates: [Candidate], context: Context) -> Decision {
        guard context.foreground else { return rejected("EDITOR_NOT_FOREGROUND") }
        guard context.dispatchConfirmed else { return rejected("IMPORT_DISPATCH_NOT_CONFIRMED") }
        guard !context.alreadyNavigated else { return rejected("EDITOR_BACK_ALREADY_ATTEMPTED") }
        guard context.now.timeIntervalSince1970.isFinite, context.deadline.timeIntervalSince1970.isFinite,
              context.now < context.deadline else { return rejected("IMPORT_LIBRARY_DEADLINE") }
        guard !context.expectedName.isEmpty, context.expectedName.utf8.count <= 4096,
              current.kind == .application, current.enabled, frame(current.frame, positive: true),
              candidates.count <= 32, Set(candidates.map(\.queryIndex)).count == candidates.count,
              candidates.allSatisfy({ $0.queryIndex >= 0 && $0.queryIndex < 32 &&
                  (!$0.hittable || $0.exists) && ($0.exists == ($0.snapshot != nil)) }) else {
            return rejected("INVALID_EDITOR_OBSERVATION")
        }
        var count = 0
        func valid(_ node: Node, depth: Int) -> Bool {
            count += 1
            guard count <= 8192, depth <= 64, node.label.utf8.count <= 4096,
                  node.identifier.utf8.count <= 4096, frame(node.frame) else { return false }
            return node.children.allSatisfy { valid($0, depth: depth + 1) }
        }
        guard valid(current, depth: 0) else { return rejected("INVALID_EDITOR_OBSERVATION") }
        for candidate in candidates {
            if let snapshot = candidate.snapshot {
                count = 0
                guard valid(snapshot, depth: 0), snapshot.kind == .button,
                      QAImportSheetSelection.bytesEqual(snapshot.identifier, "BackButton") else {
                    return rejected("INVALID_EDITOR_OBSERVATION")
                }
            }
        }
        var bars: [(path: [Int], buttonPath: [Int], button: Node)] = []
        func visit(_ node: Node, path: [Int], enabled: Bool) {
            let live = enabled && node.enabled
            if live && node.kind == .navigationBar &&
                QAImportSheetSelection.bytesEqual(node.identifier, context.expectedName) &&
                frame(node.frame, positive: true) && current.frame.contains(node.frame) {
                // Back must be a direct child of this exact named native
                // navigation bar. A global Back or another editor cannot bind it.
                for (index, child) in node.children.enumerated() where child.kind == .button &&
                    child.enabled && QAImportSheetSelection.bytesEqual(child.identifier, "BackButton") &&
                    frame(child.frame, positive: true) && node.frame.contains(child.frame) &&
                    current.frame.contains(child.frame) {
                    bars.append((path, path + [index], child))
                }
            }
            for (index, child) in node.children.enumerated() {
                visit(child, path: path + [index], enabled: live)
            }
        }
        visit(current, path: [], enabled: true)
        guard !bars.isEmpty else { return rejected("NO_CURRENT_REQUESTED_EDITOR_BACK") }
        var matches: [Binding] = []
        for candidate in candidates where candidate.exists && candidate.hittable {
            guard let snapshot = candidate.snapshot, snapshot.enabled else { continue }
            for bar in bars where same(bar.button, snapshot) {
                matches.append(Binding(queryIndex: candidate.queryIndex,
                    navigationBarPath: bar.path, buttonPath: bar.buttonPath))
            }
        }
        guard matches.count == 1 else {
            return Decision(binding: nil,
                reason: matches.isEmpty ? "NO_CURRENT_HITTABLE_EDITOR_BACK" : "AMBIGUOUS_EDITOR_BACK",
                currentNavigationBarCount: bars.count, matchedCandidateCount: matches.count)
        }
        return Decision(binding: matches[0], reason: "UNIQUE_CURRENT_REQUESTED_EDITOR_BACK",
                        currentNavigationBarCount: bars.count, matchedCandidateCount: matches.count)
    }

    static func inspectForNavigation(observe: () throws -> Decision,
                                     capture: () throws -> Void) rethrows -> Decision {
        let before = try observe()
        guard before.canNavigate else { return before }
        try capture()
        // No cached element survives capture. The caller resets its selected
        // native element and rechecks the original clock in every observation.
        return try observe()
    }
}
