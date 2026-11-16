import XCTest
import CryptoKit

struct SanitizedFrame: Codable {
    let x: Double
    let y: Double
    let width: Double
    let height: Double
}

struct SanitizedAccessibilityNode: Codable {
    let nodeIndex: Int
    let parentIndex: Int?
    let siblingOrder: Int
    let sourceScreenIndex: Int
    let elementTypeRaw: UInt
    let elementTypeName: String
    let label: String?
    let identifier: String?
    let value: String?
    let frame: SanitizedFrame
    let enabled: Bool
    let visible: Bool
}

struct ScreenshotManifestEntry: Codable {
    let pageIndex: Int
    let fileName: String
    let viewportWidth: Double
    let viewportHeight: Double
    let visibleNodeCount: Int
    let visibleLabelFrameDigest: String
    let firstVisibleCardY: Double?
    let lastVisibleCardY: Double?
    let detectedAnchors: [String]
    let stallDetected: Bool
    let stallSignals: [String]
}

struct EditorActionCoverageReport: Codable {
    let schemaVersion: String
    let shortcutLaunched: Bool
    let shortcutForegroundVerified: Bool
    let expectedShortcutName: String
    let importConsentButtonTapped: Bool
    let importSheetSnapshotCaptured: Bool
    let shortcutExistsInLibrary: Bool
    let libraryShortcutCells: [String]
    let editorOpened: Bool
    let editorIdentityVerified: Bool
    let editorHeaderLabels: [String]
    let scrollContainerFound: Bool
    let scrollContainerType: String?
    let scrollContainerFrame: SanitizedFrame?
    let totalPagesCaptured: Int
    let maxEditorPages: Int
    let captureStatus: String
    let detectedActionCards: [String]
    let detectedAnchors: [String]
    let bottomReachedEvidence: String
    let runtimeTriggerDispatched: Bool
    let runtimeCallbackObservedInUI: Bool
}

final class CherriSimulatorQAUITests: XCTestCase {

    override func setUpWithError() throws {
        continueAfterFailure = true
    }

    private func sharedOutputDirectory() -> URL {
        if let envPath = ProcessInfo.processInfo.environment["CHERRI_QA_SHARED_OUTPUT_DIR"], !envPath.isEmpty {
            let url = URL(fileURLWithPath: envPath)
            try? FileManager.default.createDirectory(at: url, withIntermediateDirectories: true)
            return url
        }
        let tmp = FileManager.default.temporaryDirectory.appendingPathComponent("cherri-simulator-qa-output")
        try? FileManager.default.createDirectory(at: tmp, withIntermediateDirectories: true)
        return tmp
    }

    private func elementTypeName(_ rawType: XCUIElement.ElementType) -> String {
        switch rawType {
        case .application: return "Application"
        case .window: return "Window"
        case .navigationBar: return "NavigationBar"
        case .tabBar: return "TabBar"
        case .toolbar: return "Toolbar"
        case .scrollView: return "ScrollView"
        case .collectionView: return "CollectionView"
        case .table: return "Table"
        case .cell: return "Cell"
        case .button: return "Button"
        case .staticText: return "StaticText"
        case .textField: return "TextField"
        case .textView: return "TextView"
        case .searchField: return "SearchField"
        case .switch: return "Switch"
        case .sheet: return "Sheet"
        case .alert: return "Alert"
        case .other: return "Other"
        default: return "ElementType_\(rawType.rawValue)"
        }
    }

    private static let forbiddenPattern: NSRegularExpression? = {
        let pattern = "(?i)(cherri-shortcut-audit-20261110|\\b[0-3][0-9]\\.plist\\b|BEGIN [A-Z ]*PRIVATE KEY|ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|sk-[A-Za-z0-9]{16,}|Bearer\\s+[A-Za-z0-9._\\-]+|[A-Za-z0-9._%+\\-]+@[A-Za-z0-9.\\-]+\\.[A-Za-z]{2,}|/Users/[A-Za-z0-9._\\-]+|[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12})"
        return try? NSRegularExpression(pattern: pattern)
    }()

    private static let allowlistedExactStrings: Set<String> = [
        "Shortcuts", "All Shortcuts", "Library", "Gallery", "Automation",
        "Search", "Search Actions", "New Shortcut", "Create Shortcut",
        "Add Shortcut", "+ Add Shortcut", "Add Action", "Add", "Done",
        "Cancel", "Select", "More", "Share", "Share Shortcut", "Edit", "Edit Shortcut",
        "Open", "Keep Both", "Set Up Shortcut", "Replace", "Scripting", "Actions",
        "Duplicate", "Delete", "Rename", "Details", "Info", "Play", "Run",
        "Run Shortcut", "Stop", "Show Sidebar", "Hide Sidebar", "editor",
        "New Shortcut, No actions", "New Shortcut, Shortcut Menu",
        "Report a Concern", "Preparing support for\ndescribe a shortcut",
        "Allow", "Always Allow", "Allow Once", "Don't Allow", "OK",
        "Continue", "Get Started", "Not Now", "Close", "Dismiss", "Report",
        "About This Shortcut", "Can Run When Locked", "Shortcut Details",
        "No Shortcuts", "Toolbar", "BackButton", "main.button.newshortcut",
        "all_shortcuts_library", "ellipsis.circle.fill", "plus.circle.fill",
        "magnifyingglass", "Dictate", "Comment", "Text", "Number", "Dictionary",
        "Get Dictionary Value", "Get Value", "Get Value for Key", "Current Date",
        "If", "Otherwise", "End If", "Repeat", "Repeat Times", "End Repeat",
        "Repeat Index", "Repeat Item", "Condition", "Equals", "Is Equal To",
        "Stop and Output", "Stop and output", "Actions Menu", "Get dates from", "Output", "Set Variable", "Add to Variable", "Combine Text",
        "Get Dates from Input", "Quick Look", "Show Alert", "Show Result",
        "Wait", "Nothing", "New Lines", "Text Value", "Key", "Value",
        "המשך", "אישור", "צור קיצור דרך", "הוסף קיצור דרך",
        "בדיקת קיצור דרך סינתטי בעברית - iPadOS 27"
    ]

    private static let allowlistedPrefixes: [String] = [
        "CHERRI_QA_", "ANCHOR_", "qa.", "qa-functional", "qa-scroll-editor",
        "qa-rtl-hebrew", "Cherri QA", "===CHERRI_QA_TRACE_NDJSON_V14===",
        "ElementType_", "Vertical scroll bar", "Horizontal scroll bar",
        "_TtGC7SwiftUI", "More, qa-", "More, New Shortcut", "Edit, New Shortcut",
        "Edit, qa-", "Open, qa-", "Play, qa-", "Preparing support for"
    ]

    private static let libraryChromeLabels: Set<String> = [
        "Shortcuts", "All Shortcuts", "Library", "Gallery", "Automation",
        "Search", "New Shortcut", "Create Shortcut", "Select", "No Shortcuts",
        "Toolbar", "BackButton", "Dictate", "Cancel", "Report", "Add Shortcut",
        "About This Shortcut", "Can Run When Locked"
    ]

    /// Strictly redacts secrets, PII, file paths, and non-allowlisted strings before persistence.
    func sanitizeUIString(_ raw: String?) -> String? {
        guard let raw = raw else { return nil }
        let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty { return nil }

        if let regex = Self.forbiddenPattern {
            let range = NSRange(trimmed.startIndex..<trimmed.endIndex, in: trimmed)
            if regex.firstMatch(in: trimmed, options: [], range: range) != nil {
                return "[REDACTED_SECRET_OR_PII]"
            }
        }

        if Self.allowlistedExactStrings.contains(trimmed) {
            return String(trimmed.prefix(256))
        }
        for prefix in Self.allowlistedPrefixes {
            if trimmed.hasPrefix(prefix) {
                return String(trimmed.prefix(512))
            }
        }
        if trimmed.contains("ANCHOR_") ||
           trimmed.contains("CHERRI_QA_") ||
           trimmed.contains("qa-functional-") ||
           trimmed.contains("qa-scroll-editor-") ||
           trimmed.contains("qa-rtl-hebrew-") {
            return String(trimmed.prefix(512))
        }
        if trimmed.hasSuffix(" actions") || trimmed.hasSuffix("%") {
            return String(trimmed.prefix(128))
        }
        return "[REDACTED_NON_ALLOWLISTED_LEN_\(trimmed.count)]"
    }

    /// Recursively flattens a public XCUIElementSnapshot tree using public APIs only.
    private func flattenSnapshot(
        _ root: XCUIElementSnapshot,
        screenIndex: Int,
        viewport: CGRect
    ) -> [SanitizedAccessibilityNode] {
        var nodes: [SanitizedAccessibilityNode] = []

        func visit(_ snap: XCUIElementSnapshot, parentIdx: Int?, siblingIdx: Int) {
            let currentIdx = nodes.count
            let f = snap.frame
            let isVis = !f.isEmpty && f.width > 0 && f.height > 0 && viewport.intersects(f)
            let rawVal: String?
            if let v = snap.value {
                rawVal = String(describing: v)
            } else {
                rawVal = nil
            }
            // Read dictionaryRepresentation to ensure public XCUIElementSnapshot API parity
            _ = snap.dictionaryRepresentation

            let node = SanitizedAccessibilityNode(
                nodeIndex: currentIdx,
                parentIndex: parentIdx,
                siblingOrder: siblingIdx,
                sourceScreenIndex: screenIndex,
                elementTypeRaw: snap.elementType.rawValue,
                elementTypeName: elementTypeName(snap.elementType),
                label: sanitizeUIString(snap.label),
                identifier: sanitizeUIString(snap.identifier),
                value: sanitizeUIString(rawVal),
                frame: SanitizedFrame(
                    x: (f.origin.x * 10).rounded() / 10,
                    y: (f.origin.y * 10).rounded() / 10,
                    width: (f.size.width * 10).rounded() / 10,
                    height: (f.size.height * 10).rounded() / 10
                ),
                enabled: snap.isEnabled,
                visible: isVis
            )
            nodes.append(node)

            for (idx, child) in snap.children.enumerated() {
                visit(child, parentIdx: currentIdx, siblingIdx: idx)
            }
        }

        visit(root, parentIdx: nil, siblingIdx: 0)
        return nodes
    }

    private func computeDigest(for nodes: [SanitizedAccessibilityNode]) -> String {
        var combined = ""
        for n in nodes where n.visible {
            let lbl = n.label ?? ""
            let id = n.identifier ?? ""
            let val = n.value ?? ""
            combined += "\(n.elementTypeName)|\(lbl)|\(id)|\(val)|\(Int(n.frame.y))|\(Int(n.frame.height));"
        }
        let digest = SHA256.hash(data: Data(combined.utf8))
        return digest.map { String(format: "%02x", $0) }.joined()
    }

    /// Distinguishes the genuine Shortcut action editor from the main All Shortcuts library screen.
    private func evaluateEditorState(
        in shortcuts: XCUIApplication,
        expectedShortcutName: String
    ) -> (editorOpened: Bool, identityVerified: Bool, headerLabels: [String]) {
        var headers: [String] = []
        var sawExpectedInHeader = false
        let navBars = shortcuts.navigationBars
        for i in 0..<min(navBars.count, 5) {
            let nb = navBars.element(boundBy: i)
            if nb.exists {
                if !nb.identifier.isEmpty {
                    headers.append(sanitizeUIString(nb.identifier) ?? "REDACTED")
                    if nb.identifier.localizedCaseInsensitiveContains(expectedShortcutName) {
                        sawExpectedInHeader = true
                    }
                }
                let texts = nb.staticTexts
                for j in 0..<min(texts.count, 4) {
                    let st = texts.element(boundBy: j)
                    if st.exists && !st.label.isEmpty {
                        headers.append(sanitizeUIString(st.label) ?? "REDACTED")
                        if st.label.localizedCaseInsensitiveContains(expectedShortcutName) {
                            sawExpectedInHeader = true
                        }
                    }
                }
                let btns = nb.buttons
                for j in 0..<min(btns.count, 6) {
                    let b = btns.element(boundBy: j)
                    if b.exists && !b.label.isEmpty {
                        headers.append(sanitizeUIString(b.label) ?? "REDACTED")
                        if b.label.localizedCaseInsensitiveContains(expectedShortcutName) {
                            sawExpectedInHeader = true
                        }
                    }
                }
            }
        }

        let namePred = NSPredicate(format: "label == %@ OR label BEGINSWITH %@ OR label CONTAINS[c] %@", expectedShortcutName, expectedShortcutName + ",", expectedShortcutName)
        let hasEditorPane = shortcuts.otherElements["editor"].exists ||
                            shortcuts.navigationBars["New Shortcut"].exists ||
                            shortcuts.navigationBars[expectedShortcutName].exists ||
                            shortcuts.searchFields["Search Actions"].exists ||
                            shortcuts.buttons["Add Action"].exists ||
                            shortcuts.buttons["Done"].exists ||
                            shortcuts.buttons["Info"].exists ||
                            shortcuts.buttons["Shortcut Details"].exists ||
                            shortcuts.buttons["Scripting"].exists

        let libraryStillPrimary = shortcuts.otherElements["all_shortcuts_library"].exists &&
                                  shortcuts.navigationBars["All Shortcuts"].exists &&
                                  !hasEditorPane

        let editorOpened = hasEditorPane || (!libraryStillPrimary && !shortcuts.otherElements["all_shortcuts_library"].exists)
        let identityVerified = editorOpened && (
            sawExpectedInHeader ||
            shortcuts.buttons.matching(namePred).firstMatch.exists ||
            shortcuts.staticTexts.matching(namePred).firstMatch.exists ||
            shortcuts.navigationBars[expectedShortcutName].exists
        )
        return (editorOpened, identityVerified, headers)
    }

    private func waitForHandshakeFile(_ url: URL, timeout: TimeInterval) -> Bool {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            if FileManager.default.fileExists(atPath: url.path) {
                return true
            }
            Thread.sleep(forTimeInterval: 0.25)
        }
        return false
    }

    private func dismissOnboardingAndStaleAlerts(shortcuts: XCUIApplication, springboard: XCUIApplication) {
        let dismissOrder = ["OK", "Continue", "Get Started", "Not Now", "Dismiss", "המשך", "אישור"]
        for _ in 0..<3 {
            var dismissedAny = false
            for targetApp in [shortcuts, springboard] {
                for label in dismissOrder {
                    let btn = targetApp.buttons[label].firstMatch
                    if btn.exists && btn.isHittable {
                        btn.tap()
                        dismissedAny = true
                        Thread.sleep(forTimeInterval: 0.8)
                    }
                }
            }
            if !dismissedAny {
                break
            }
        }
    }

    func test01_ReceiverSelfCallbackAndContainerPersistence() throws {
        let receiver = XCUIApplication(bundleIdentifier: "org.cherrilang.qa.Receiver")
        receiver.launchArguments = ["--reset-callback-store", "--trigger-self-callback"]
        receiver.launch()

        let statusText = receiver.staticTexts["qa.receiver.status"]
        XCTAssertTrue(statusText.waitForExistence(timeout: 10), "Receiver status label must exist")
        XCTAssertTrue(statusText.label.contains("SUCCESS"), "Expected receiver self-callback status SUCCESS, got \(statusText.label)")
    }

    func test02_ShortcutsImportLibraryAndActionEditorCoverage() throws {
        let outDir = sharedOutputDirectory()
        let maxPages = min(20, max(1, Int(ProcessInfo.processInfo.environment["CHERRI_QA_MAX_EDITOR_PAGES"] ?? "12") ?? 12))
        let expectedShortcutName = ProcessInfo.processInfo.environment["CHERRI_QA_EXPECTED_SHORTCUT_NAME"] ?? "qa-functional-v2"
        let expectSignedImport = (ProcessInfo.processInfo.environment["CHERRI_QA_EXPECT_SIGNED_IMPORT"] == "true")
        let expectRuntimeRun = (ProcessInfo.processInfo.environment["CHERRI_QA_EXPECT_RUNTIME_RUN"] == "true")
        let execRunNonce = ProcessInfo.processInfo.environment["CHERRI_QA_EXEC_RUN_NONCE"] ?? ""

        let shortcuts = XCUIApplication(bundleIdentifier: "com.apple.shortcuts")
        let springboard = XCUIApplication(bundleIdentifier: "com.apple.springboard")

        // Ensure Shortcuts is foregrounded and clear any first-launch / prior-probe modals before importing
        shortcuts.activate()
        var launched = shortcuts.wait(for: .runningForeground, timeout: 10)
        if !launched {
            shortcuts.launch()
            launched = shortcuts.wait(for: .runningForeground, timeout: 15)
        }
        dismissOnboardingAndStaleAlerts(shortcuts: shortcuts, springboard: springboard)

        // Step 1: Coordinated host -> simulator signed .shortcut import handshake (if AEA1 signed artifact is ready)
        var importConsentTapped = false
        var importSheetCaptured = false
        if expectSignedImport {
            let reqURL = outDir.appendingPathComponent("request-signed-import.txt")
            let retryReqURL = outDir.appendingPathComponent("request-signed-import-retry.txt")
            let ackURL = outDir.appendingPathComponent("ack-signed-import.txt")
            let retryAckURL = outDir.appendingPathComponent("ack-signed-import-retry.txt")
            try? FileManager.default.removeItem(at: ackURL)
            try? FileManager.default.removeItem(at: retryAckURL)
            try? FileManager.default.removeItem(at: retryReqURL)
            try? expectedShortcutName.data(using: .utf8)?.write(to: reqURL, options: .atomic)

            // Wait purely on filesystem for host simctl openurl so AX queries never starve SpringBoard/Shortcuts IPC
            _ = waitForHandshakeFile(ackURL, timeout: 65.0)

            let sheetPredicate = NSPredicate(format: "label == 'Add Shortcut' OR label == '+ Add Shortcut' OR label == 'Set Up Shortcut' OR label == 'Replace' OR label == 'Keep Both' OR label == 'הוסף קיצור דרך' OR label CONTAINS[c] 'Add Shortcut' OR label CONTAINS[c] 'Set Up Shortcut' OR label CONTAINS[c] 'Keep Both'")
            func pollImportSheetButton(timeout: TimeInterval) -> XCUIElement? {
                let end = Date().addingTimeInterval(timeout)
                while Date() < end {
                    for targetApp in [shortcuts, springboard] {
                        let predBtn = targetApp.buttons.matching(sheetPredicate).firstMatch
                        if predBtn.exists && predBtn.isHittable {
                            return predBtn
                        }
                    }
                    Thread.sleep(forTimeInterval: 1.5)
                }
                return nil
            }

            var foundSheetBtn = pollImportSheetButton(timeout: 18.0)
            if foundSheetBtn == nil {
                try? expectedShortcutName.data(using: .utf8)?.write(to: retryReqURL, options: .atomic)
                _ = waitForHandshakeFile(retryAckURL, timeout: 65.0)
                foundSheetBtn = pollImportSheetButton(timeout: 20.0)
            }

            if let btn = foundSheetBtn {
                let sheetShot = XCUIScreen.main.screenshot()
                try? sheetShot.pngRepresentation.write(to: outDir.appendingPathComponent("import-consent-sheet.png"), options: .atomic)
                if let sheetSnap = try? shortcuts.snapshot() {
                    let appF = shortcuts.frame.isEmpty ? CGRect(x: 0, y: 0, width: 1024, height: 1366) : shortcuts.frame
                    let sheetNodes = flattenSnapshot(sheetSnap, screenIndex: 0, viewport: appF)
                    let enc = JSONEncoder()
                    enc.outputFormatting = [.prettyPrinted, .sortedKeys]
                    if let d = try? enc.encode(sheetNodes) {
                        try? d.write(to: outDir.appendingPathComponent("import-sheet-tree.json"), options: .atomic)
                    }
                }
                importSheetCaptured = true
                btn.tap()
                importConsentTapped = true
                let gone = NSPredicate(format: "exists == false")
                _ = XCTWaiter.wait(for: [XCTNSPredicateExpectation(predicate: gone, object: btn)], timeout: 12)
                Thread.sleep(forTimeInterval: 1.5)

                // Handle secondary confirmation if Shortcuts asks Replace / Keep Both / Done
                let followUpLabels = ["Replace", "Keep Both", "Add Shortcut", "Done"]
                let followDeadline = Date().addingTimeInterval(5.0)
                while Date() < followDeadline {
                    var tappedFollowUp = false
                    for label in followUpLabels {
                        let b2 = shortcuts.buttons[label].firstMatch
                        if b2.exists && b2.isHittable {
                            b2.tap()
                            tappedFollowUp = true
                            Thread.sleep(forTimeInterval: 1.2)
                            break
                        }
                    }
                    if !tappedFollowUp {
                        break
                    }
                }
            } else {
                let diagShot = XCUIScreen.main.screenshot()
                try? diagShot.pngRepresentation.write(to: outDir.appendingPathComponent("import-sheet-timeout.png"), options: .atomic)
                if let timeoutSnap = try? shortcuts.snapshot() {
                    let appF = shortcuts.frame.isEmpty ? CGRect(x: 0, y: 0, width: 1024, height: 1366) : shortcuts.frame
                    let timeoutNodes = flattenSnapshot(timeoutSnap, screenIndex: 0, viewport: appF)
                    let enc = JSONEncoder()
                    enc.outputFormatting = [.prettyPrinted, .sortedKeys]
                    if let d = try? enc.encode(timeoutNodes) {
                        try? d.write(to: outDir.appendingPathComponent("import-sheet-timeout-tree.json"), options: .atomic)
                    }
                }
            }
        } else {
            // Even without host handshake signal, check if an import sheet is already visible
            let importButtons = ["Add Shortcut", "+ Add Shortcut", "Set Up Shortcut", "Replace", "הוסף קיצור דרך"]
            for candidate in importButtons {
                let btn = shortcuts.buttons[candidate].firstMatch
                if btn.waitForExistence(timeout: 1) && btn.isHittable {
                    importConsentTapped = true
                    btn.tap()
                    Thread.sleep(forTimeInterval: 1.5)
                    break
                }
            }
        }

        dismissOnboardingAndStaleAlerts(shortcuts: shortcuts, springboard: springboard)
        let foregroundVerified = (shortcuts.state == .runningForeground)

        // Step 2: Inspect All Shortcuts library for expectedShortcutName (with bounded wait after import)
        var shortcutExistsInLibrary = false
        var libraryCells: [String] = []
        let libDeadline = Date().addingTimeInterval(expectSignedImport ? 15.0 : 2.0)
        while Date() < libDeadline {
            libraryCells.removeAll()
            let cellsQuery = shortcuts.collectionViews.cells
            let cellCount = min(cellsQuery.count, 30)
            for i in 0..<cellCount {
                let cell = cellsQuery.element(boundBy: i)
                if cell.exists {
                    let rawLabel = cell.label
                    if let sanitized = sanitizeUIString(rawLabel) {
                        libraryCells.append(sanitized)
                    }
                    if rawLabel == expectedShortcutName ||
                       rawLabel.hasPrefix("\(expectedShortcutName),") ||
                       rawLabel.localizedCaseInsensitiveContains(expectedShortcutName) ||
                       cell.buttons[expectedShortcutName].exists ||
                       cell.staticTexts[expectedShortcutName].exists {
                        shortcutExistsInLibrary = true
                    }
                }
            }
            if !shortcutExistsInLibrary {
                let namePred = NSPredicate(format: "label == %@ OR label BEGINSWITH %@ OR label CONTAINS[c] %@", expectedShortcutName, expectedShortcutName + ",", expectedShortcutName)
                if shortcuts.buttons.matching(namePred).firstMatch.exists ||
                   shortcuts.staticTexts.matching(namePred).firstMatch.exists ||
                   shortcuts.navigationBars[expectedShortcutName].exists {
                    shortcutExistsInLibrary = true
                }
            }
            if shortcutExistsInLibrary || !expectSignedImport {
                break
            }
            Thread.sleep(forTimeInterval: 0.5)
        }

        // Step 3: Open the genuine action editor for expectedShortcutName (or New Shortcut if blocked)
        if shortcutExistsInLibrary {
            let libShot = XCUIScreen.main.screenshot()
            try? libShot.pngRepresentation.write(to: outDir.appendingPathComponent("library-after-import.png"), options: .atomic)

            // If not already showing the expected shortcut inside Other["editor"], open it WITHOUT triggering tile execution!
            // On iPadOS grid tiles, tapping the card body runs the Shortcut; use Edit / More (...) button or context menu -> Edit.
            var editorMatchesExpected = shortcuts.otherElements["editor"].exists && (
                shortcuts.navigationBars[expectedShortcutName].exists ||
                shortcuts.navigationBars.buttons.matching(NSPredicate(format: "label CONTAINS[c] %@", expectedShortcutName)).firstMatch.exists
            )
            if !editorMatchesExpected {
                let editByLabel = shortcuts.buttons["Edit, \(expectedShortcutName)"].firstMatch
                let openByLabel = shortcuts.buttons["Open, \(expectedShortcutName)"].firstMatch
                let moreByLabel = shortcuts.buttons["More, \(expectedShortcutName)"].firstMatch
                if editByLabel.exists && editByLabel.isHittable {
                    editByLabel.tap()
                    Thread.sleep(forTimeInterval: 1.5)
                } else if moreByLabel.exists && moreByLabel.isHittable {
                    moreByLabel.tap()
                    Thread.sleep(forTimeInterval: 1.5)
                } else if openByLabel.exists && openByLabel.isHittable {
                    openByLabel.tap()
                    Thread.sleep(forTimeInterval: 1.5)
                } else {
                    let cellsQuery = shortcuts.collectionViews.cells
                    let cellCount = min(cellsQuery.count, 30)
                    for i in 0..<cellCount {
                        let cell = cellsQuery.element(boundBy: i)
                        if cell.exists && (
                            cell.label == expectedShortcutName ||
                            cell.label.hasPrefix("\(expectedShortcutName),") ||
                            cell.label.localizedCaseInsensitiveContains(expectedShortcutName) ||
                            cell.staticTexts[expectedShortcutName].exists
                        ) {
                            let cellActionBtn = cell.buttons.matching(NSPredicate(format: "label CONTAINS[c] 'More' OR label CONTAINS[c] 'Edit' OR identifier CONTAINS[c] 'ellipsis'")).firstMatch
                            if cellActionBtn.exists && cellActionBtn.isHittable {
                                cellActionBtn.tap()
                                Thread.sleep(forTimeInterval: 1.5)
                            } else {
                                cell.press(forDuration: 0.9)
                                Thread.sleep(forTimeInterval: 1.2)
                            }
                            break
                        }
                    }
                }

                // If tapping More or long-pressing opened a context menu with Edit / Edit Shortcut / Details, tap Edit
                if !shortcuts.otherElements["editor"].exists {
                    let menuEditLabels = ["Edit", "Edit Shortcut"]
                    var tappedMenuEdit = false
                    for lbl in menuEditLabels {
                        for appCandidate in [shortcuts, springboard] {
                            let mBtn = appCandidate.buttons[lbl].firstMatch
                            if mBtn.exists && mBtn.isHittable {
                                mBtn.tap()
                                tappedMenuEdit = true
                                Thread.sleep(forTimeInterval: 1.5)
                                break
                            }
                            let mText = appCandidate.staticTexts[lbl].firstMatch
                            if mText.exists && mText.isHittable {
                                mText.tap()
                                tappedMenuEdit = true
                                Thread.sleep(forTimeInterval: 1.5)
                                break
                            }
                        }
                        if tappedMenuEdit {
                            break
                        }
                    }
                }
            }

            // On iPadOS 27 split preview, tap the top-right "Edit" button if not already in full action editor mode
            let alreadyFullEditor = shortcuts.buttons["Scripting"].exists ||
                                    shortcuts.buttons["Actions"].exists ||
                                    shortcuts.buttons["Done"].exists ||
                                    shortcuts.searchFields["Search Actions"].exists ||
                                    !shortcuts.otherElements["all_shortcuts_library"].exists
            if !alreadyFullEditor {
                let editPred = NSPredicate(format: "label == 'Edit' OR label == 'Edit Shortcut' OR label BEGINSWITH 'Edit'")
                let matchingEditBtns = shortcuts.buttons.matching(editPred)
                var chosenEditBtn: XCUIElement? = nil
                var bestEditX: CGFloat = 250
                for idx in 0..<min(matchingEditBtns.count, 8) {
                    let b = matchingEditBtns.element(boundBy: idx)
                    if b.exists && b.isHittable && b.frame.origin.x > bestEditX {
                        bestEditX = b.frame.origin.x
                        chosenEditBtn = b
                    }
                }
                if let editBtn = chosenEditBtn {
                    editBtn.tap()
                    Thread.sleep(forTimeInterval: 1.8)
                } else {
                    // Fallback: check rightmost button in top bar above y=100, excluding Share Shortcut & Play
                    let navButtons = shortcuts.navigationBars.buttons
                    var candidateBtn: XCUIElement? = nil
                    var bestX: CGFloat = 0
                    for i in 0..<min(navButtons.count, 10) {
                        let b = navButtons.element(boundBy: i)
                        if b.exists && b.isHittable && b.frame.origin.x > 580 && b.frame.origin.y < 110 && b.frame.size.width >= 38 {
                            let lbl = b.label
                            if lbl != "Share Shortcut" && lbl != "Share" && lbl != "Play" && lbl != "Run" {
                                if b.frame.origin.x > bestX {
                                    bestX = b.frame.origin.x
                                    candidateBtn = b
                                }
                            }
                        }
                    }
                    if let btn = candidateBtn {
                        btn.tap()
                        Thread.sleep(forTimeInterval: 1.8)
                    }
                }
            }
        } else {
            // Try tapping New Shortcut button by identifier or label so we test opening the genuine editor
            let newShortcutById = shortcuts.buttons["main.button.newshortcut"].firstMatch
            if newShortcutById.exists && newShortcutById.isHittable {
                newShortcutById.tap()
                Thread.sleep(forTimeInterval: 2.0)
            } else {
                let createCandidates = ["New Shortcut", "Create Shortcut", "צור קיצור דרך"]
                for candidate in createCandidates {
                    let btn = shortcuts.buttons[candidate].firstMatch
                    if btn.exists && btn.isHittable {
                        btn.tap()
                        Thread.sleep(forTimeInterval: 2.0)
                        break
                    }
                }
            }
        }

        let (editorOpened, editorIdentityVerified, headerLabels) = evaluateEditorState(
            in: shortcuts,
            expectedShortcutName: expectedShortcutName
        )

        // Step 4: Locate scrollable canvas inside Other["editor"] (avoiding all_shortcuts_library and action picker drawer)
        var scrollElement: XCUIElement = shortcuts
        var scrollType: String? = nil
        var scrollFrame: SanitizedFrame? = nil

        let editorRoot = shortcuts.otherElements["editor"].firstMatch
        let searchRoot: XCUIElement = editorRoot.exists ? editorRoot : shortcuts
        let libraryRoot = shortcuts.otherElements["all_shortcuts_library"].firstMatch
        let libraryFrame = libraryRoot.exists ? libraryRoot.frame : CGRect.zero

        let candidates: [(String, XCUIElementQuery)] = [
            ("ScrollView", searchRoot.scrollViews),
            ("CollectionView", searchRoot.collectionViews),
            ("Table", searchRoot.tables)
        ]
        var bestScore: CGFloat = -1
        for (typeName, query) in candidates {
            let count = min(query.count, 8)
            for i in 0..<count {
                let el = query.element(boundBy: i)
                if el.exists {
                    let f = el.frame
                    if f.height <= 120 || f.width <= 150 { continue }
                    if libraryRoot.exists && abs(f.origin.x - libraryFrame.origin.x) < 10 && f.size.width <= libraryFrame.size.width + 10 {
                        continue
                    }
                    var score = f.width * f.height
                    // In iPadOS 27 full split editor, action canvas is on the left (minX < 350) while picker drawer is on the right (minX >= 400)
                    if editorRoot.exists && f.origin.x < 350 {
                        score += 2_000_000
                    }
                    if typeName == "ScrollView" {
                        score += 500_000
                    }
                    if score > bestScore {
                        bestScore = score
                        scrollElement = el
                        scrollType = typeName
                        scrollFrame = SanitizedFrame(x: f.origin.x, y: f.origin.y, width: f.width, height: f.height)
                    }
                }
            }
        }

        let appFrame = shortcuts.frame.isEmpty ? CGRect(x: 0, y: 0, width: 1024, height: 1366) : shortcuts.frame
        var allMergedNodes: [SanitizedAccessibilityNode] = []
        var screenshotManifest: [ScreenshotManifestEntry] = []
        var orderedAnchors: [String] = []
        var seenAnchors: Set<String> = []
        var orderedActionCards: [String] = []
        var seenActionCards: Set<String> = []
        var previousDigest: String? = nil
        var previousLastY: Double? = nil
        var bottomEvidence = "MAX_PAGES_REACHED"

        for page in 1...maxPages {
            let screenshot = XCUIScreen.main.screenshot()
            let fileName = String(format: "page-%04d.png", page)
            let fileURL = outDir.appendingPathComponent(fileName)
            try? screenshot.pngRepresentation.write(to: fileURL, options: .atomic)

            let attachment = XCTAttachment(screenshot: screenshot)
            attachment.name = fileName
            attachment.lifetime = .keepAlways
            add(attachment)

            let snap = try shortcuts.snapshot()
            let pageNodes = flattenSnapshot(snap, screenIndex: page, viewport: appFrame)
            allMergedNodes.append(contentsOf: pageNodes)

            let digest = computeDigest(for: pageNodes)
            var pageAnchors: [String] = []
            var firstCardY: Double? = nil
            var lastCardY: Double? = nil

            for node in pageNodes where node.visible {
                for candidateText in [node.label, node.value].compactMap({ $0 }) {
                    if candidateText.contains("ANCHOR_") {
                        pageAnchors.append(candidateText)
                        if !seenAnchors.contains(candidateText) {
                            seenAnchors.insert(candidateText)
                            orderedAnchors.append(candidateText)
                        }
                    }
                    if (node.elementTypeName == "Cell" || node.elementTypeName == "StaticText" || node.elementTypeName == "Button" || node.elementTypeName == "TextField" || node.elementTypeName == "TextView") &&
                       !Self.libraryChromeLabels.contains(candidateText) &&
                       !candidateText.hasPrefix("[REDACTED_") {
                        let cardEntry = "\(node.elementTypeName):\(candidateText)"
                        if !seenActionCards.contains(cardEntry) {
                            seenActionCards.insert(cardEntry)
                            orderedActionCards.append(cardEntry)
                        }
                        if firstCardY == nil || node.frame.y < firstCardY! {
                            firstCardY = node.frame.y
                        }
                        if lastCardY == nil || node.frame.y > lastCardY! {
                            lastCardY = node.frame.y
                        }
                    }
                }
            }

            var stallSignals: [String] = []
            if let prevD = previousDigest, prevD == digest {
                stallSignals.append("IDENTICAL_VISIBLE_LABEL_FRAME_DIGEST")
            }
            if let prevY = previousLastY, let currY = lastCardY, abs(currY - prevY) < 2.0 {
                stallSignals.append("ZERO_LAST_CARD_Y_DELTA")
            }
            if seenAnchors.contains(where: { $0.contains("ANCHOR_CARD_28") }) {
                stallSignals.append("BOTTOM_ANCHOR_28_VISIBLE")
            }
            let stalled = (stallSignals.count >= 2) || (page > 1 && stallSignals.contains("IDENTICAL_VISIBLE_LABEL_FRAME_DIGEST"))

            screenshotManifest.append(ScreenshotManifestEntry(
                pageIndex: page,
                fileName: fileName,
                viewportWidth: Double(appFrame.width),
                viewportHeight: Double(appFrame.height),
                visibleNodeCount: pageNodes.filter { $0.visible }.count,
                visibleLabelFrameDigest: digest,
                firstVisibleCardY: firstCardY,
                lastVisibleCardY: lastCardY,
                detectedAnchors: pageAnchors,
                stallDetected: stalled,
                stallSignals: stallSignals
            ))

            if stalled {
                bottomEvidence = "MULTI_SIGNAL_STALL_AT_PAGE_\(page):" + stallSignals.joined(separator: "+")
                break
            }
            if scrollType == nil {
                bottomEvidence = "SINGLE_PAGE_NO_SCROLL_CONTAINER"
                break
            }

            previousDigest = digest
            previousLastY = lastCardY

            let startCoord = scrollElement.coordinate(withNormalizedOffset: CGVector(dx: 0.45, dy: 0.78))
            let endCoord = scrollElement.coordinate(withNormalizedOffset: CGVector(dx: 0.45, dy: 0.22))
            startCoord.press(forDuration: 0.05, thenDragTo: endCoord)
            Thread.sleep(forTimeInterval: 0.9)
        }

        // Step 5: Coordinated live runtime x-callback-url execution while UI test can confirm any Open/Allow dialog
        var runtimeTriggerDispatched = false
        var runtimeCallbackObservedInUI = false
        if expectRuntimeRun && (shortcutExistsInLibrary || importConsentTapped) {
            let doneBtn = shortcuts.buttons["Done"].firstMatch
            if doneBtn.exists && doneBtn.isHittable {
                doneBtn.tap()
                Thread.sleep(forTimeInterval: 1.0)
            } else if shortcuts.otherElements["editor"].exists {
                var closedEditor = false
                let backBtn = shortcuts.buttons["BackButton"].firstMatch
                if backBtn.exists && backBtn.isHittable {
                    backBtn.tap()
                    closedEditor = true
                    Thread.sleep(forTimeInterval: 1.2)
                }
                if !closedEditor {
                    let navButtons = shortcuts.navigationBars.buttons
                    for i in 0..<min(navButtons.count, 5) {
                        let b = navButtons.element(boundBy: i)
                        if b.exists && b.isHittable && b.frame.origin.x < 110 && b.frame.origin.y < 120 {
                            b.tap()
                            closedEditor = true
                            Thread.sleep(forTimeInterval: 1.2)
                            break
                        }
                    }
                }
                if !closedEditor {
                    let allBtns = shortcuts.buttons
                    for i in 0..<min(allBtns.count, 16) {
                        let b = allBtns.element(boundBy: i)
                        if b.exists && b.isHittable && b.frame.origin.x >= 0 && b.frame.origin.x < 100 && b.frame.origin.y >= 0 && b.frame.origin.y < 100 && b.frame.size.width < 100 {
                            b.tap()
                            closedEditor = true
                            Thread.sleep(forTimeInterval: 1.2)
                            break
                        }
                    }
                }
                if shortcuts.otherElements["editor"].exists {
                    shortcuts.terminate()
                    Thread.sleep(forTimeInterval: 1.0)
                    shortcuts.activate()
                    _ = shortcuts.wait(for: .runningForeground, timeout: 10)
                    Thread.sleep(forTimeInterval: 2.0)
                }
            }
            if shortcuts.state != .runningForeground {
                shortcuts.activate()
                _ = shortcuts.wait(for: .runningForeground, timeout: 10)
                Thread.sleep(forTimeInterval: 1.5)
            }
            let preRunShot = XCUIScreen.main.screenshot()
            try? preRunShot.pngRepresentation.write(to: outDir.appendingPathComponent("before-runtime-trigger.png"), options: .atomic)

            let runReqURL = outDir.appendingPathComponent("request-shortcut-run.txt")
            let runAckURL = outDir.appendingPathComponent("ack-shortcut-run.txt")
            try? FileManager.default.removeItem(at: runAckURL)
            try? execRunNonce.data(using: .utf8)?.write(to: runReqURL, options: .atomic)
            // Avoid flooding SpringBoard/Shortcuts with AX queries while simctl openurl dispatches RUN_URL
            runtimeTriggerDispatched = waitForHandshakeFile(runAckURL, timeout: 55.0)

            let receiver = XCUIApplication(bundleIdentifier: "org.cherrilang.qa.Receiver")
            let confirmPredicate = NSPredicate(format: "label IN {'Always Allow', 'Allow', 'Open', 'Allow Once', 'Run', 'Continue', 'OK'}")
            var capturedMidway = false

            func pollRuntimeConfirmAndCallback(until deadline: Date) {
                while Date() < deadline {
                    if !runtimeTriggerDispatched && FileManager.default.fileExists(atPath: runAckURL.path) {
                        runtimeTriggerDispatched = true
                    }
                    for targetApp in [springboard, shortcuts] {
                        let btn = targetApp.buttons.matching(confirmPredicate).firstMatch
                        if btn.exists && btn.isHittable {
                            btn.tap()
                            Thread.sleep(forTimeInterval: 0.8)
                        }
                    }
                    if receiver.state == .runningForeground {
                        let runIdLabel = receiver.staticTexts["qa.receiver.lastRunId"]
                        let statusLabel = receiver.staticTexts["qa.receiver.status"]
                        let matchesNonce = execRunNonce.isEmpty || (runIdLabel.exists && runIdLabel.label.contains(execRunNonce))
                        let completedStatus = statusLabel.exists && (statusLabel.label.contains("SUCCESS") || statusLabel.label.contains("ERROR") || statusLabel.label.contains("CANCELLED"))
                        if matchesNonce && completedStatus {
                            runtimeTriggerDispatched = runtimeTriggerDispatched || FileManager.default.fileExists(atPath: runAckURL.path)
                            runtimeCallbackObservedInUI = true
                            let rtShot = XCUIScreen.main.screenshot()
                            try? rtShot.pngRepresentation.write(to: outDir.appendingPathComponent("runtime-callback.png"), options: .atomic)
                            return
                        }
                    }
                    Thread.sleep(forTimeInterval: 1.5)
                }
            }

            pollRuntimeConfirmAndCallback(until: Date().addingTimeInterval(15.0))
            if !runtimeCallbackObservedInUI && !capturedMidway {
                capturedMidway = true
                let midShot = XCUIScreen.main.screenshot()
                try? midShot.pngRepresentation.write(to: outDir.appendingPathComponent("runtime-midway.png"), options: .atomic)
            }
            if !runtimeCallbackObservedInUI {
                let toShot = XCUIScreen.main.screenshot()
                try? toShot.pngRepresentation.write(to: outDir.appendingPathComponent("runtime-timeout.png"), options: .atomic)
                var timeoutNodes: [SanitizedAccessibilityNode] = []
                if let scSnap = try? shortcuts.snapshot() {
                    timeoutNodes.append(contentsOf: flattenSnapshot(scSnap, screenIndex: 90, viewport: appFrame))
                }
                if let sbSnap = try? springboard.snapshot() {
                    timeoutNodes.append(contentsOf: flattenSnapshot(sbSnap, screenIndex: 91, viewport: appFrame))
                }
                let enc = JSONEncoder()
                enc.outputFormatting = [.prettyPrinted, .sortedKeys]
                if let d = try? enc.encode(timeoutNodes) {
                    try? d.write(to: outDir.appendingPathComponent("runtime-timeout-tree.json"), options: .atomic)
                }
            }
        }

        let hasTopAnchor = orderedAnchors.contains(where: { $0.contains("ANCHOR_CARD_01") || $0.contains("ANCHOR_RTL_01") })
        let hasBottomAnchor = orderedAnchors.contains(where: { $0.contains("ANCHOR_CARD_28") || $0.contains("ANCHOR_RTL_01") })

        let captureStatus: String
        if !foregroundVerified {
            captureStatus = "BLOCKED_SIMULATOR_APP"
        } else if editorOpened && shortcutExistsInLibrary && editorIdentityVerified && hasTopAnchor && hasBottomAnchor && !orderedActionCards.isEmpty {
            captureStatus = "EDITOR_CAPTURE_COMPLETE"
        } else {
            captureStatus = "PARTIAL_EDITOR_CAPTURE"
        }

        let coverage = EditorActionCoverageReport(
            schemaVersion: "1.4",
            shortcutLaunched: launched,
            shortcutForegroundVerified: foregroundVerified,
            expectedShortcutName: expectedShortcutName,
            importConsentButtonTapped: importConsentTapped,
            importSheetSnapshotCaptured: importSheetCaptured,
            shortcutExistsInLibrary: shortcutExistsInLibrary,
            libraryShortcutCells: libraryCells,
            editorOpened: editorOpened,
            editorIdentityVerified: editorIdentityVerified,
            editorHeaderLabels: headerLabels,
            scrollContainerFound: scrollType != nil,
            scrollContainerType: scrollType,
            scrollContainerFrame: scrollFrame,
            totalPagesCaptured: screenshotManifest.count,
            maxEditorPages: maxPages,
            captureStatus: captureStatus,
            detectedActionCards: orderedActionCards,
            detectedAnchors: orderedAnchors,
            bottomReachedEvidence: bottomEvidence,
            runtimeTriggerDispatched: runtimeTriggerDispatched,
            runtimeCallbackObservedInUI: runtimeCallbackObservedInUI
        )

        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]

        if let treeData = try? encoder.encode(allMergedNodes) {
            try? treeData.write(to: outDir.appendingPathComponent("editor-tree.json"), options: .atomic)
        }
        if let manifestData = try? encoder.encode(screenshotManifest) {
            try? manifestData.write(to: outDir.appendingPathComponent("screenshots-manifest.json"), options: .atomic)
        }
        if let covData = try? encoder.encode(coverage) {
            try? covData.write(to: outDir.appendingPathComponent("editor-action-coverage.json"), options: .atomic)
        }

        XCTAssertTrue(launched && foregroundVerified, "Apple Shortcuts app (com.apple.shortcuts) must launch in foreground on Simulator")
    }

    func test03_SanitizerRedactsSecretsAndNonAllowlistedStrings() throws {
        XCTAssertEqual(sanitizeUIString("ghp_FAKE_SECRET_TOKEN_12345"), "[REDACTED_SECRET_OR_PII]")
        XCTAssertEqual(sanitizeUIString("-----BEGIN PRIVATE KEY-----"), "[REDACTED_SECRET_OR_PII]")
        XCTAssertEqual(sanitizeUIString("cherri-shortcut-audit-20261110/035.plist"), "[REDACTED_SECRET_OR_PII]")
        XCTAssertEqual(sanitizeUIString("david@example.com"), "[REDACTED_SECRET_OR_PII]")
        XCTAssertEqual(sanitizeUIString("D9A9954E-1234-5678-9ABC-DEF012345678"), "[REDACTED_SECRET_OR_PII]")

        let customTitle = sanitizeUIString("Private User Note Title")
        XCTAssertNotNil(customTitle)
        XCTAssertTrue(customTitle!.hasPrefix("[REDACTED_NON_ALLOWLISTED_LEN_"), "Expected non-allowlisted redaction, got \(customTitle!)")

        XCTAssertEqual(sanitizeUIString("ANCHOR_CARD_01_TOP_HEADER"), "ANCHOR_CARD_01_TOP_HEADER")
        XCTAssertEqual(sanitizeUIString("CHERRI_QA_RESULT:ok:count=2:mode=v2"), "CHERRI_QA_RESULT:ok:count=2:mode=v2")
        XCTAssertEqual(sanitizeUIString("All Shortcuts"), "All Shortcuts")
    }

    func test04ConfirmPendingRuntimeDialog() throws {
        guard let outputDir = ProcessInfo.processInfo.environment["CHERRI_QA_SHARED_OUTPUT_DIR"], !outputDir.isEmpty else { return }
        let outDir = URL(fileURLWithPath: outputDir, isDirectory: true)
        let sentinel = outDir.appendingPathComponent("confirm-dialog-mode.txt")
        guard ProcessInfo.processInfo.environment["CHERRI_QA_DIALOG_CONFIRM_ONLY"] == "1" || FileManager.default.fileExists(atPath: sentinel.path) else { return }
        let shortcuts = XCUIApplication(bundleIdentifier: "com.apple.shortcuts")
        let springboard = XCUIApplication(bundleIdentifier: "com.apple.springboard")
        let receiver = XCUIApplication(bundleIdentifier: "org.cherrilang.qa.Receiver")
        let execRunNonce = ProcessInfo.processInfo.environment["CHERRI_QA_EXEC_RUN_NONCE"] ?? ""
        let confirmPredicate = NSPredicate(format: "label IN {'Always Allow', 'Allow', 'Open', 'Allow Once', 'Run', 'Continue', 'OK'}")
        let deadline = Date().addingTimeInterval(22.0)

        while Date() < deadline {
            for targetApp in [springboard, shortcuts] {
                let btn = targetApp.buttons.matching(confirmPredicate).firstMatch
                if btn.exists && btn.isHittable {
                    btn.tap()
                    Thread.sleep(forTimeInterval: 1.5)
                }
            }
            if receiver.state == .runningForeground {
                let runIdLabel = receiver.staticTexts["qa.receiver.lastRunId"]
                let statusLabel = receiver.staticTexts["qa.receiver.status"]
                let matchesNonce = execRunNonce.isEmpty || (runIdLabel.exists && runIdLabel.label.contains(execRunNonce))
                let completedStatus = statusLabel.exists && (statusLabel.label.contains("SUCCESS") || statusLabel.label.contains("ERROR") || statusLabel.label.contains("CANCELLED"))
                if matchesNonce && completedStatus {
                    let rtShot = XCUIScreen.main.screenshot()
                    try? rtShot.pngRepresentation.write(to: outDir.appendingPathComponent("runtime-callback.png"), options: .atomic)
                    return
                }
            }
            Thread.sleep(forTimeInterval: 1.5)
        }
        let afterShot = XCUIScreen.main.screenshot()
        try? afterShot.pngRepresentation.write(to: outDir.appendingPathComponent("phase4-after-confirm.png"), options: .atomic)
    }
}
