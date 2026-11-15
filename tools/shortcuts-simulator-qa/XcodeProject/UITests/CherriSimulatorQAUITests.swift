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
        "Cancel", "Select", "More", "Share", "Share Shortcut", "Edit",
        "Duplicate", "Delete", "Rename", "Details", "Info", "Play", "Run",
        "Run Shortcut", "Stop", "Show Sidebar", "Hide Sidebar", "editor",
        "New Shortcut, No actions", "New Shortcut, Shortcut Menu",
        "Report a Concern", "Preparing support for\ndescribe a shortcut",
        "Allow", "Always Allow", "Allow Once", "Don't Allow", "OK",
        "Continue", "Get Started", "Not Now", "Close", "Dismiss", "Report",
        "Set Up Shortcut", "Replace", "About This Shortcut", "Can Run When Locked",
        "No Shortcuts", "Toolbar", "BackButton", "main.button.newshortcut",
        "all_shortcuts_library", "ellipsis.circle.fill", "plus.circle.fill",
        "magnifyingglass", "Dictate", "Comment", "Text", "Number", "Dictionary",
        "Get Dictionary Value", "If", "Otherwise", "End If", "Repeat", "End Repeat",
        "Stop and Output", "Set Variable", "Add to Variable", "Combine Text",
        "Get Dates from Input", "Quick Look", "Show Alert", "Show Result",
        "Wait", "Nothing", "המשך", "אישור", "צור קיצור דרך", "הוסף קיצור דרך",
        "בדיקת קיצור דרך סינתטי בעברית - iPadOS 27"
    ]

    private static let allowlistedPrefixes: [String] = [
        "CHERRI_QA_", "ANCHOR_", "qa.", "qa-functional", "qa-scroll-editor",
        "qa-rtl-hebrew", "Cherri QA", "===CHERRI_QA_TRACE_NDJSON_V14===",
        "ElementType_", "Vertical scroll bar", "Horizontal scroll bar",
        "_TtGC7SwiftUI", "More, qa-", "More, New Shortcut", "Edit, New Shortcut",
        "Preparing support for"
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
                return String(trimmed.prefix(256))
            }
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
        let navBars = shortcuts.navigationBars
        for i in 0..<min(navBars.count, 5) {
            let nb = navBars.element(boundBy: i)
            if nb.exists {
                if !nb.identifier.isEmpty {
                    headers.append( sanitizeUIString(nb.identifier) ?? "REDACTED" )
                }
                let texts = nb.staticTexts
                for j in 0..<min(texts.count, 4) {
                    let st = texts.element(boundBy: j)
                    if st.exists && !st.label.isEmpty {
                        headers.append( sanitizeUIString(st.label) ?? "REDACTED" )
                    }
                }
            }
        }

        let hasEditorPane = shortcuts.otherElements["editor"].exists ||
                            shortcuts.navigationBars["New Shortcut"].exists ||
                            shortcuts.navigationBars[expectedShortcutName].exists ||
                            shortcuts.searchFields["Search Actions"].exists ||
                            shortcuts.buttons["Add Action"].exists ||
                            shortcuts.buttons["Done"].exists ||
                            shortcuts.buttons["Info"].exists ||
                            shortcuts.buttons["Shortcut Details"].exists

        let libraryStillPrimary = shortcuts.otherElements["all_shortcuts_library"].exists &&
                                  shortcuts.navigationBars["All Shortcuts"].exists &&
                                  !hasEditorPane

        let editorOpened = hasEditorPane || (!libraryStillPrimary && !shortcuts.otherElements["all_shortcuts_library"].exists)
        let identityVerified = editorOpened && (
            shortcuts.buttons[expectedShortcutName].exists ||
            shortcuts.staticTexts[expectedShortcutName].exists ||
            shortcuts.navigationBars[expectedShortcutName].exists
        )
        return (editorOpened, identityVerified, headers)
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

        let shortcuts = XCUIApplication(bundleIdentifier: "com.apple.shortcuts")
        let springboard = XCUIApplication(bundleIdentifier: "com.apple.springboard")

        // Prefer activate() so any modal import sheet opened by simctl openurl is preserved
        shortcuts.activate()
        var launched = shortcuts.wait(for: .runningForeground, timeout: 10)
        if !launched {
            shortcuts.launch()
            launched = shortcuts.wait(for: .runningForeground, timeout: 15)
        }

        // Step 1: Check if a signed .shortcut import consent sheet is present; if so, tap Add Shortcut
        var importConsentTapped = false
        let importButtons = ["Add Shortcut", "+ Add Shortcut", "Set Up Shortcut", "Replace", "הוסף קיצור דרך"]
        for candidate in importButtons {
            let btn = shortcuts.buttons[candidate].firstMatch
            if btn.waitForExistence(timeout: 2) && btn.isHittable {
                btn.tap()
                importConsentTapped = true
                let gone = NSPredicate(format: "exists == false")
                _ = XCTWaiter.wait(for: [XCTNSPredicateExpectation(predicate: gone, object: btn)], timeout: 15)
                Thread.sleep(forTimeInterval: 1.5)
                break
            }
        }

        // Step 2: Dismiss first-launch splash / onboarding or SpringBoard permission alerts if present
        let onboardingLabels = ["Continue", "Get Started", "Not Now", "OK", "Allow", "Always Allow", "המשך", "אישור"]
        for targetApp in [shortcuts, springboard] {
            for label in onboardingLabels {
                let btn = targetApp.buttons[label].firstMatch
                if btn.exists && btn.isHittable {
                    btn.tap()
                    Thread.sleep(forTimeInterval: 1.0)
                }
            }
        }

        let foregroundVerified = (shortcuts.state == .runningForeground)

        // Step 3: Inspect All Shortcuts library for expectedShortcutName
        var shortcutExistsInLibrary = false
        var libraryCells: [String] = []
        let cellsQuery = shortcuts.collectionViews.cells
        let cellCount = min(cellsQuery.count, 25)
        for i in 0..<cellCount {
            let cell = cellsQuery.element(boundBy: i)
            if cell.exists {
                let rawLabel = cell.label
                if let sanitized = sanitizeUIString(rawLabel) {
                    libraryCells.append(sanitized)
                }
                if rawLabel == expectedShortcutName ||
                   rawLabel.hasPrefix("\(expectedShortcutName),") ||
                   cell.buttons[expectedShortcutName].exists ||
                   cell.staticTexts[expectedShortcutName].exists {
                    shortcutExistsInLibrary = true
                }
            }
        }
        if !shortcutExistsInLibrary && shortcuts.buttons[expectedShortcutName].exists {
            shortcutExistsInLibrary = true
        }

        // Step 4: Open editor for the imported shortcut (if present in library), or open New Shortcut editor
        if shortcutExistsInLibrary {
            let moreBtn = shortcuts.buttons["More, \(expectedShortcutName)"].firstMatch
            if moreBtn.exists && moreBtn.isHittable {
                moreBtn.tap()
                Thread.sleep(forTimeInterval: 2.0)
            } else {
                // Locate the shortcut cell and tap its secondary (ellipsis) button
                for i in 0..<cellCount {
                    let cell = cellsQuery.element(boundBy: i)
                    if cell.exists && (cell.label == expectedShortcutName || cell.label.hasPrefix("\(expectedShortcutName),")) {
                        if cell.buttons.count >= 2 {
                            let ellipsis = cell.buttons.element(boundBy: 1)
                            if ellipsis.exists && ellipsis.isHittable {
                                ellipsis.tap()
                                Thread.sleep(forTimeInterval: 2.0)
                                break
                            }
                        }
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

        // Step 5: Locate scrollable container inside the active view (excluding all_shortcuts_library when editor is open)
        var scrollElement: XCUIElement = shortcuts
        var scrollType: String? = nil
        var scrollFrame: SanitizedFrame? = nil

        let candidates: [(String, XCUIElementQuery)] = [
            ("CollectionView", shortcuts.collectionViews),
            ("ScrollView", shortcuts.scrollViews),
            ("Table", shortcuts.tables)
        ]
        var bestArea: CGFloat = 0
        for (typeName, query) in candidates {
            let count = min(query.count, 6)
            for i in 0..<count {
                let el = query.element(boundBy: i)
                if el.exists {
                    let f = el.frame
                    let area = f.width * f.height
                    if area > bestArea && f.height > 120 {
                        bestArea = area
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
                if let lbl = node.label {
                    if lbl.contains("ANCHOR_") {
                        pageAnchors.append(lbl)
                        if !seenAnchors.contains(lbl) {
                            seenAnchors.insert(lbl)
                            orderedAnchors.append(lbl)
                        }
                    }
                    if (node.elementTypeName == "Cell" || node.elementTypeName == "StaticText" || node.elementTypeName == "Button") &&
                       !Self.libraryChromeLabels.contains(lbl) &&
                       !lbl.hasPrefix("[REDACTED_") {
                        let cardEntry = "\(node.elementTypeName):\(lbl)"
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

            let startCoord = scrollElement.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.75))
            let endCoord = scrollElement.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.20))
            startCoord.press(forDuration: 0.05, thenDragTo: endCoord)
            Thread.sleep(forTimeInterval: 0.8)
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
            bottomReachedEvidence: bottomEvidence
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
}
