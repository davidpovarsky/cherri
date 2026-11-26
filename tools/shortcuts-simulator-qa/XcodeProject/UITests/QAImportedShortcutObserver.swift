import XCTest

struct QAImportCommand: Decodable {
    let id: String
    let operation: String
    let shortcutName: String?
    let requiresAbsentLibraryRecord: Bool?
}

final class QAImportedShortcutObserver {
    private let runDirectory: URL
    private let shortcuts: XCUIApplication
    private let captureAction: (String) -> Void
    private let deliveryBudget: QADifferentialImportTiming.DeliveryBudget

    init(runDirectory: URL, shortcuts: XCUIApplication,
         deliveryBudget: QADifferentialImportTiming.DeliveryBudget = .differential,
         capture: @escaping (String) -> Void) {
        self.runDirectory = runDirectory
        self.shortcuts = shortcuts
        self.captureAction = capture
        self.deliveryBudget = deliveryBudget
    }

    private func capture(_ name: String) { captureAction(name) }

    private func importSelectionNode(_ snapshot: XCUIElementSnapshot, depth: Int = 0,
                                     count: inout Int) throws -> QAImportSheetSelection.Node {
        count += 1
        guard count <= 8192, depth <= 64,
              snapshot.label.utf8.count <= 4096, snapshot.identifier.utf8.count <= 4096 else {
            throw NSError(domain: "CherriQAImportSelection", code: 1,
                          userInfo: [NSLocalizedDescriptionKey: "IMPORT_SNAPSHOT_BOUNDS"])
        }
        let kind: QAImportSheetSelection.Kind
        switch snapshot.elementType {
        case .application: kind = .application
        case .window: kind = .window
        case .other: kind = .other
        case .sheet: kind = .sheet
        case .scrollView: kind = .scrollView
        case .navigationBar: kind = .navigationBar
        case .cell: kind = .cell
        case .collectionView: kind = .collectionView
        case .button: kind = .button
        case .staticText: kind = .staticText
        default: kind = .unsupported
        }
        let children = try snapshot.children.map {
            try importSelectionNode($0, depth: depth + 1, count: &count)
        }
        return QAImportSheetSelection.Node(kind: kind, label: snapshot.label,
            identifier: snapshot.identifier, enabled: snapshot.isEnabled, selected: snapshot.isSelected,
            frame: snapshot.frame, children: children)
    }

    func importShortcut(_ command: QAImportCommand) throws -> [String: Any] {
        let name = try XCTUnwrap(command.shortcutName)
        if shortcuts.state != .runningForeground {
            shortcuts.activate()
            _ = shortcuts.wait(for: .runningForeground, timeout: 8)
        }
        var timing = QADifferentialImportTiming(searchStartedAt: Date(), deliveryBudget: deliveryBudget)
        let dispatchPath = runDirectory.appendingPathComponent("dispatch-" + command.id + ".json")
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        var timingReceipt: [String: Any] = [
            "contract": "dispatch-exit-anchored-add-v1",
            "dispatchReceipt": dispatchPath.lastPathComponent,
            "addSearchStartedUTC": formatter.string(from: timing.searchStartedAt)
        ]
        func observeDispatch() -> QADifferentialImportTiming.DispatchState {
            let data = try? Data(contentsOf: dispatchPath)
            let now = Date()
            let state = timing.observeDispatch(data, now: now)
            timingReceipt["lastDispatchCheckUTC"] = formatter.string(from: now)
            switch state {
            case .pending(let deadline):
                timingReceipt["dispatchState"] = "PENDING"
                timingReceipt["dispatchDeadlineUTC"] = formatter.string(from: deadline)
            case .exited(let exit):
                timingReceipt["dispatchState"] = "EXITED"
                timingReceipt["dispatchExitObservedUTC"] = formatter.string(from: exit)
            case .rejected(let reason):
                timingReceipt["dispatchState"] = "REJECTED"
                timingReceipt["dispatchFailure"] = reason
            }
            return state
        }
        func checkAdd() -> QADifferentialImportTiming.AddDecision {
            let dispatch = observeDispatch()
            let now = Date()
            let decision = timing.addDecision(after: dispatch, now: now)
            timingReceipt["lastAddCheckUTC"] = formatter.string(from: now)
            if let deadline = decision.deadline {
                timingReceipt["currentSearchDeadlineUTC"] = formatter.string(from: deadline)
            }
            return decision
        }
        func receipt(_ fields: [String: Any]) -> [String: Any] {
            var fields = fields
            fields["importTiming"] = timingReceipt
            return fields
        }
        func stop(_ outcome: String) -> [String: Any] {
            capture(command.id + "-import")
            XCTFail(outcome + " for " + name)
            return receipt(["outcome": outcome, "name": name])
        }

        func checkSheet(found: Bool, hittable: Bool,
                        identity: Bool) -> QADifferentialImportTiming.SheetDecision {
            let dispatch = observeDispatch()
            let now = Date()
            let decision = timing.sheetDecision(after: dispatch, now: now,
                buttonExists: found, buttonHittable: hittable, identityMatches: identity)
            timingReceipt["sheetContract"] = "requested-identity-and-add-v1"
            timingReceipt["lastSheetCheckUTC"] = formatter.string(from: now)
            timingReceipt["sheetButtonExists"] = found
            timingReceipt["sheetButtonHittable"] = hittable
            timingReceipt["sheetIdentityMatched"] = identity
            timingReceipt["sheetCanTap"] = decision.canTap
            if let deadline = decision.deadline {
                timingReceipt["currentSearchDeadlineUTC"] = formatter.string(from: deadline)
            }
            return decision
        }
        // AX calls can outlive their nominal wait. Read the actual process
        // receipt and the clock again after each call, before using its result.
        // Every invocation reads fresh values, including after capture().
        var identity = false
        var selectedButton: XCUIElement?
        func observeSheet() throws -> QADifferentialImportTiming.SheetDecision {
            selectedButton = nil
            identity = false
            if let failure = checkAdd().failure { return .rejected(failure) }
            let buttons = shortcuts.buttons.matching(NSPredicate(
                format: "label == 'Add Shortcut' OR label == '+ Add Shortcut' OR label == 'Replace'")).allElementsBoundByIndex
            if let failure = checkAdd().failure { return .rejected(failure) }
            guard buttons.count <= 32 else { return .rejected("IMPORT_QUERY_BOUNDS") }
            var candidates: [QAImportSheetSelection.Candidate] = []
            for (index, button) in buttons.enumerated() {
                let exists = button.exists
                if let failure = checkAdd().failure { return .rejected(failure) }
                var hittable = false
                var observed: QAImportSheetSelection.Node?
                if exists {
                    hittable = button.isHittable
                    if let failure = checkAdd().failure { return .rejected(failure) }
                    let snapshot = try button.snapshot()
                    if let failure = checkAdd().failure { return .rejected(failure) }
                    var count = 0
                    observed = try importSelectionNode(snapshot, count: &count)
                }
                candidates.append(QAImportSheetSelection.Candidate(queryIndex: index, exists: exists,
                                                                   hittable: hittable, snapshot: observed))
            }
            let snapshot = try shortcuts.snapshot()
            if let failure = checkAdd().failure { return .rejected(failure) }
            var count = 0
            let current = try importSelectionNode(snapshot, count: &count)
            let selected = QAImportSheetSelection.select(current: current, candidates: candidates, expectedName: name)
            timingReceipt["sheetSelectionContract"] = "current-container-bound-add-v1"
            timingReceipt["sheetSelectionReason"] = selected.reason
            timingReceipt["sheetQueryCandidateCount"] = candidates.count
            timingReceipt["sheetCurrentContainerCount"] = selected.currentContainerCount
            timingReceipt["sheetMatchedCandidateCount"] = selected.matchedCandidateCount
            for key in ["sheetSelectedQueryIndex", "sheetSelectedContainerPath", "sheetSelectedTitlePath", "sheetSelectedButtonPath"] {
                timingReceipt.removeValue(forKey: key)
            }
            if let binding = selected.binding {
                guard QAImportConsentPolicy.permits(
                    buttonLabel: candidates[binding.queryIndex].snapshot?.label,
                    requiresAbsentLibraryRecord: command.requiresAbsentLibraryRecord == true) else {
                    return .rejected("IMPORT_REPLACE_FORBIDDEN")
                }
                selectedButton = buttons[binding.queryIndex]
                identity = true
                timingReceipt["sheetSelectedQueryIndex"] = binding.queryIndex
                timingReceipt["sheetSelectedContainerPath"] = binding.containerPath
                timingReceipt["sheetSelectedTitlePath"] = binding.titlePath
                timingReceipt["sheetSelectedButtonPath"] = binding.buttonPath
            }
            return checkSheet(found: candidates.contains { $0.exists },
                              hittable: candidates.contains { $0.exists && $0.hittable }, identity: identity)
        }
        while true {
            let decision = try QADifferentialImportTiming.inspectSheetForConsent(
                observe: observeSheet,
                capture: {
                    timingReceipt["addObservedUTC"] = formatter.string(from: Date())
                    self.capture(command.id + "-import")
                })
            if let failure = decision.failure { return stop(failure) }
            if decision.canTap {
                timingReceipt["sheetMatchedAfterCaptureUTC"] = formatter.string(from: Date())
                break
            }
            let remaining = decision.deadline!.timeIntervalSinceNow
            Thread.sleep(forTimeInterval: max(0, min(0.2, remaining)))
        }
        XCTAssertTrue(identity, "Signed import sheet must identify " + name)
        guard identity, let button = selectedButton else {
            return receipt(["outcome": "IMPORT_IDENTITY_MISMATCH", "name": name])
        }
        button.tap()
        if let failure = checkAdd().failure { return stop(failure) }
        let addTappedAt = Date()
        timingReceipt["addTappedUTC"] = formatter.string(from: addTappedAt)
        timingReceipt["libraryContract"] = "dispatch-exit-anchored-library-v1"
        func checkLibrary() -> QADifferentialImportTiming.LibraryDecision {
            let dispatch = observeDispatch()
            let now = Date()
            let decision = timing.libraryDecision(after: dispatch, addTappedAt: addTappedAt, now: now)
            timingReceipt["lastLibraryCheckUTC"] = formatter.string(from: now)
            if let deadline = decision.deadline {
                timingReceipt["libraryDeadlineUTC"] = formatter.string(from: deadline)
            }
            timingReceipt["libraryCanConfirm"] = decision.canConfirm
            return decision
        }
        var libraryObservationOrdinal = 0
        func observeLibrary() throws -> QAImportedLibrarySelection.Decision {
            func boundary() -> String? {
                let decision = checkLibrary()
                if let failure = decision.failure { return failure }
                guard let deadline = decision.deadline, Date() < deadline else { return "IMPORT_LIBRARY_DEADLINE" }
                return nil
            }
            if let failure = boundary() { return QAImportedLibrarySelection.rejected(failure) }
            let foreground = shortcuts.state == .runningForeground
            if let failure = boundary() { return QAImportedLibrarySelection.rejected(failure) }
            guard foreground else { return QAImportedLibrarySelection.rejected("LIBRARY_NOT_FOREGROUND") }
            let cells = shortcuts.cells.matching(NSPredicate(
                format: "label == %@ OR label BEGINSWITH %@", name, name + ", ")).allElementsBoundByIndex
            if let failure = boundary() { return QAImportedLibrarySelection.rejected(failure) }
            guard cells.count <= 32 else { return QAImportedLibrarySelection.rejected("INVALID_LIBRARY_OBSERVATION") }
            var candidates: [QAImportSheetSelection.Candidate] = []
            for (index, cell) in cells.enumerated() {
                let exists = cell.exists
                if let failure = boundary() { return QAImportedLibrarySelection.rejected(failure) }
                var hittable = false
                var observed: QAImportSheetSelection.Node?
                if exists {
                    hittable = cell.isHittable
                    if let failure = boundary() { return QAImportedLibrarySelection.rejected(failure) }
                    let snapshot = try cell.snapshot()
                    if let failure = boundary() { return QAImportedLibrarySelection.rejected(failure) }
                    var count = 0
                    observed = try importSelectionNode(snapshot, count: &count)
                }
                candidates.append(.init(queryIndex: index, exists: exists, hittable: hittable, snapshot: observed))
            }
            let snapshot = try shortcuts.snapshot()
            if let failure = boundary() { return QAImportedLibrarySelection.rejected(failure) }
            var count = 0
            let current = try importSelectionNode(snapshot, count: &count)
            let stillForeground = shortcuts.state == .runningForeground
            let clock = checkLibrary()
            if let failure = clock.failure { return QAImportedLibrarySelection.rejected(failure) }
            guard let deadline = clock.deadline else { return QAImportedLibrarySelection.rejected("IMPORT_LIBRARY_DEADLINE") }
            let observedAt = Date()
            let selected = QAImportedLibrarySelection.select(current: current, candidates: candidates,
                context: .init(expectedName: name, foreground: stillForeground,
                               dispatchConfirmed: clock.canConfirm, now: observedAt, deadline: deadline))
            timingReceipt["librarySelectionContract"] = "unique-current-library-cell-v1"
            timingReceipt["librarySelectionReason"] = selected.reason
            timingReceipt["libraryCurrentNamedCellCount"] = selected.currentNamedCellCount
            timingReceipt["libraryQueryCandidateCount"] = selected.queryCandidateCount
            timingReceipt["librarySelectionObservedUTC"] = formatter.string(from: observedAt)
            for key in ["librarySelectedQueryIndex", "librarySelectedCellPath", "librarySelectedCellLabel",
                        "librarySelectedCellIdentifier"] { timingReceipt.removeValue(forKey: key) }
            if let binding = selected.binding {
                timingReceipt["librarySelectedQueryIndex"] = binding.queryIndex
                timingReceipt["librarySelectedCellPath"] = binding.cellPath
                timingReceipt["librarySelectedCellLabel"] = binding.label
                timingReceipt["librarySelectedCellIdentifier"] = binding.identifier
            }
            func serialized(_ node: QAImportSheetSelection.Node) -> [String: Any] {
                ["kind": node.kind.rawValue, "label": node.label, "identifier": node.identifier,
                 "enabled": node.enabled, "selected": node.selected,
                 "frame": ["x": Double(node.frame.minX), "y": Double(node.frame.minY),
                           "width": Double(node.frame.width), "height": Double(node.frame.height)],
                 "children": node.children.map { serialized($0) }]
            }
            libraryObservationOrdinal += 1
            let raw: [String: Any] = [
                "schema": "cherri-current-library-selection-observation/1", "phase": command.id,
                "ordinal": libraryObservationOrdinal, "observedUTC": formatter.string(from: observedAt),
                "deadlineUTC": formatter.string(from: deadline), "expectedName": name,
                "foreground": stillForeground, "dispatchConfirmed": clock.canConfirm,
                "current": serialized(current), "candidates": candidates.map { candidate -> [String: Any] in
                    ["queryIndex": candidate.queryIndex, "exists": candidate.exists,
                     "hittable": candidate.hittable, "snapshot": candidate.snapshot.map { serialized($0) as Any } ?? NSNull()]
                }, "decision": selected.reason, "canConfirm": selected.canConfirm,
                "currentNamedCellCount": selected.currentNamedCellCount,
                "queryCandidateCount": selected.queryCandidateCount]
            let bytes = try JSONSerialization.data(withJSONObject: raw, options: [.sortedKeys])
            guard bytes.count <= 2 * 1024 * 1024 else {
                throw NSError(domain: "CherriQALibrarySelection", code: 1,
                              userInfo: [NSLocalizedDescriptionKey: "LIBRARY_OBSERVATION_BYTES"])
            }
            let filename = command.id + "-library-observation-" + String(format: "%04d", libraryObservationOrdinal) + ".json"
            let url = runDirectory.appendingPathComponent(filename)
            try bytes.write(to: url, options: .withoutOverwriting)
            try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: url.path)
            if let failure = boundary() { return QAImportedLibrarySelection.rejected(failure) }
            return selected
        }
        var editorBackTapped = false
        var selectedEditorBack: XCUIElement?
        func editorNavigationBoundary() -> String? {
            let decision = checkLibrary()
            if let failure = decision.failure { return failure }
            guard let deadline = decision.deadline, Date() < deadline else {
                return "IMPORT_LIBRARY_DEADLINE"
            }
            return nil
        }
        func observeEditorBack() throws -> QAImportedEditorNavigation.Decision {
            selectedEditorBack = nil
            if let failure = editorNavigationBoundary() { return QAImportedEditorNavigation.rejected(failure) }
            let foreground = shortcuts.state == .runningForeground
            if let failure = editorNavigationBoundary() { return QAImportedEditorNavigation.rejected(failure) }
            guard foreground else { return QAImportedEditorNavigation.rejected("EDITOR_NOT_FOREGROUND") }
            let namedBars = shortcuts.navigationBars.matching(identifier: name).allElementsBoundByIndex
            if let failure = editorNavigationBoundary() { return QAImportedEditorNavigation.rejected(failure) }
            guard namedBars.count == 1 else { return QAImportedEditorNavigation.rejected("NO_UNIQUE_CURRENT_REQUESTED_EDITOR") }
            let buttons = namedBars[0].buttons.matching(identifier: "BackButton").allElementsBoundByIndex
            if let failure = editorNavigationBoundary() { return QAImportedEditorNavigation.rejected(failure) }
            guard buttons.count <= 32 else { return QAImportedEditorNavigation.rejected("INVALID_EDITOR_OBSERVATION") }
            var candidates: [QAImportSheetSelection.Candidate] = []
            for (index, button) in buttons.enumerated() {
                let exists = button.exists
                if let failure = editorNavigationBoundary() { return QAImportedEditorNavigation.rejected(failure) }
                var hittable = false
                var observed: QAImportSheetSelection.Node?
                if exists {
                    hittable = button.isHittable
                    if let failure = editorNavigationBoundary() { return QAImportedEditorNavigation.rejected(failure) }
                    let snapshot = try button.snapshot()
                    if let failure = editorNavigationBoundary() { return QAImportedEditorNavigation.rejected(failure) }
                    var count = 0
                    observed = try importSelectionNode(snapshot, count: &count)
                }
                candidates.append(QAImportSheetSelection.Candidate(queryIndex: index, exists: exists,
                                                                   hittable: hittable, snapshot: observed))
            }
            let snapshot = try shortcuts.snapshot()
            if let failure = editorNavigationBoundary() { return QAImportedEditorNavigation.rejected(failure) }
            var count = 0
            let current = try importSelectionNode(snapshot, count: &count)
            let stillForeground = shortcuts.state == .runningForeground
            let decision = checkLibrary()
            if let failure = decision.failure { return QAImportedEditorNavigation.rejected(failure) }
            guard let deadline = decision.deadline else { return QAImportedEditorNavigation.rejected("IMPORT_LIBRARY_DEADLINE") }
            let selected = QAImportedEditorNavigation.select(current: current, candidates: candidates,
                context: .init(expectedName: name, foreground: stillForeground,
                               dispatchConfirmed: decision.canConfirm, alreadyNavigated: editorBackTapped,
                               now: Date(), deadline: deadline))
            timingReceipt["editorNavigationContract"] = "requested-editor-back-within-library-clock-v1"
            timingReceipt["editorNavigationReason"] = selected.reason
            timingReceipt["editorNavigationQueryCount"] = candidates.count
            timingReceipt["editorNavigationBarCount"] = selected.currentNavigationBarCount
            timingReceipt["editorNavigationMatchCount"] = selected.matchedCandidateCount
            for key in ["editorBackQueryIndex", "editorBackBarPath", "editorBackButtonPath"] {
                timingReceipt.removeValue(forKey: key)
            }
            if let binding = selected.binding {
                selectedEditorBack = buttons[binding.queryIndex]
                timingReceipt["editorBackQueryIndex"] = binding.queryIndex
                timingReceipt["editorBackBarPath"] = binding.navigationBarPath
                timingReceipt["editorBackButtonPath"] = binding.buttonPath
            }
            return selected
        }
        var replaceTapped = false
        while true {
            var decision = checkLibrary()
            if let failure = decision.failure { return stop(failure) }
            if Date() >= decision.deadline! { break }
            let library = try QAImportedLibrarySelection.inspectForConfirmation(
                observe: observeLibrary, capture: { self.capture(command.id + "-library") })
            decision = checkLibrary()
            if let failure = decision.failure { return stop(failure) }
            if Date() >= decision.deadline! { break }
            if library.reason.hasPrefix("AMBIGUOUS_") ||
                library.reason == "LIBRARY_IDENTITY_CHANGED_DURING_CAPTURE" ||
                library.reason == "INVALID_LIBRARY_OBSERVATION" {
                return stop(library.reason)
            }
            if library.canConfirm && decision.canConfirm {
                timingReceipt["libraryMatchedAfterCaptureUTC"] = formatter.string(from: Date())
                return receipt(["outcome": "IMPORTED", "name": name, "identityVerified": identity,
                                "libraryIdentityVerified": true, "libraryUniqueCurrentCellVerified": true,
                                "requiresAbsentLibraryRecord": command.requiresAbsentLibraryRecord == true,
                                "replaceTapped": replaceTapped])
            }
            if !editorBackTapped {
                let navigation = try QAImportedEditorNavigation.inspectForNavigation(
                    observe: observeEditorBack,
                    capture: { self.capture(command.id + "-imported-editor") })
                decision = checkLibrary()
                if let failure = decision.failure { return stop(failure) }
                if Date() >= decision.deadline! { break }
                if navigation.canNavigate, let back = selectedEditorBack, decision.canConfirm {
                    // Claim before the native tap; never retry Back. The editor
                    // title grants navigation only, never library confirmation.
                    editorBackTapped = true
                    timingReceipt["editorBackAttemptCount"] = 1
                    timingReceipt["editorBackIntentUTC"] = formatter.string(from: Date())
                    back.tap()
                    timingReceipt["editorBackReturnedUTC"] = formatter.string(from: Date())
                    decision = checkLibrary()
                    if let failure = decision.failure { return stop(failure) }
                    if Date() >= decision.deadline! { break }
                    continue
                }
            }
            let replacement = shortcuts.buttons["Replace"].firstMatch
            let replacementExists = replacement.exists
            decision = checkLibrary()
            if let failure = decision.failure { return stop(failure) }
            if Date() >= decision.deadline! { break }
            if replacementExists {
                let hittable = replacement.isHittable
                decision = checkLibrary()
                if let failure = decision.failure { return stop(failure) }
                if Date() >= decision.deadline! { break }
                if hittable {
                    if command.requiresAbsentLibraryRecord == true {
                        return stop("UNEXPECTED_REPLACEMENT_FOR_FRESH_IMPORT")
                    }
                    capture(command.id + "-replace")
                    decision = checkLibrary()
                    if let failure = decision.failure { return stop(failure) }
                    if Date() >= decision.deadline! { break }
                    replacement.tap()
                    replaceTapped = true
                    decision = checkLibrary()
                    if let failure = decision.failure { return stop(failure) }
                    if Date() >= decision.deadline! { break }
                }
            }
            Thread.sleep(forTimeInterval: max(0, min(0.5, decision.deadline!.timeIntervalSinceNow)))
        }
        capture(command.id + "-library-timeout")
        let debugState = shortcuts.debugDescription
        let debugPath = runDirectory.appendingPathComponent(command.id + "-library-timeout.txt")
        try? debugState.write(to: debugPath, atomically: true, encoding: String.Encoding.utf8)
        XCTFail("No accessible native library item after import for " + name)
        return receipt(["outcome": "LIBRARY_IDENTITY_NOT_OBSERVED", "name": name,
                        "identityVerified": identity, "replaceTapped": replaceTapped])
    }

}
