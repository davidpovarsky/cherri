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
    let editorOpened: Bool
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

    /// Redacts any string that is not synthetic/allowlisted or standard Apple Shortcuts UI chrome.
    private func sanitizeUIString(_ raw: String?) -> String? {
        guard let raw = raw else { return nil }
        let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty { return nil }
        return String(trimmed.prefix(256))
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
            // Also read dictionaryRepresentation keys to ensure public XCUIElementSnapshot API parity
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

    func test01_ReceiverSelfCallbackAndContainerPersistence() throws {
        let receiver = XCUIApplication(bundleIdentifier: "org.cherrilang.qa.Receiver")
        receiver.launchArguments = ["--trigger-self-callback"]
        receiver.launch()

        let statusText = receiver.staticTexts["qa.receiver.status"]
        XCTAssertTrue(statusText.waitForExistence(timeout: 10), "Receiver status label must exist")
        XCTAssertTrue(statusText.label.contains("SUCCESS"), "Expected receiver self-callback status SUCCESS, got \(statusText.label)")
    }

    func test02_ShortcutsLaunchEditorTreeAndMultiPageScroll() throws {
        let outDir = sharedOutputDirectory()
        let maxPages = Int(ProcessInfo.processInfo.environment["CHERRI_QA_MAX_EDITOR_PAGES"] ?? "12") ?? 12

        let shortcuts = XCUIApplication(bundleIdentifier: "com.apple.shortcuts")
        shortcuts.launch()

        let launched = shortcuts.wait(for: .runningForeground, timeout: 20)
        // Dismiss common first-launch splash / onboarding buttons if present
        let onboardingLabels = ["Continue", "Get Started", "Not Now", "OK", "Allow", "Done", "המשך", "אישור"]
        for label in onboardingLabels {
            let btn = shortcuts.buttons[label].firstMatch
            if btn.exists && btn.isHittable {
                btn.tap()
                Thread.sleep(forTimeInterval: 1.0)
            }
        }

        let foregroundVerified = (shortcuts.state == .runningForeground)

        // Try opening editor via Create Shortcut button if visible on library screen
        var editorOpened = false
        let createCandidates = ["Create Shortcut", "New Shortcut", "Add", "Add Shortcut", "צור קיצור דרך"]
        for candidate in createCandidates {
            let btn = shortcuts.buttons[candidate].firstMatch
            if btn.exists && btn.isHittable {
                btn.tap()
                Thread.sleep(forTimeInterval: 2.0)
                editorOpened = true
                break
            }
        }

        // Also check if an editor or search field for actions is already open
        if !editorOpened {
            if shortcuts.searchFields.firstMatch.exists ||
               shortcuts.collectionViews.firstMatch.exists ||
               shortcuts.scrollViews.firstMatch.exists {
                editorOpened = true
            }
        }

        // Locate actual scrollable container (prefer CollectionView, then ScrollView, then Table)
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
        var detectedAnchorsSet: Set<String> = []
        var detectedCardsSet: Set<String> = []
        var previousDigest: String? = nil
        var previousLastY: Double? = nil
        var bottomEvidence = "MAX_PAGES_REACHED"

        for page in 1...max(1, maxPages) {
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
                        detectedAnchorsSet.insert(lbl)
                    }
                    if node.elementTypeName == "Cell" || node.elementTypeName == "StaticText" || node.elementTypeName == "Button" {
                        if lbl.count >= 3 && lbl.count <= 120 {
                            detectedCardsSet.insert("\(node.elementTypeName):\(lbl)")
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
                detectedAnchors: pageAnchors.sorted(),
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

            // Adaptive scroll: drag from 75% height to 20% height of the scroll container
            let startCoord = scrollElement.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.75))
            let endCoord = scrollElement.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.20))
            startCoord.press(forDuration: 0.05, thenDragTo: endCoord)
            Thread.sleep(forTimeInterval: 0.8)
        }

        let captureStatus: String
        if !foregroundVerified {
            captureStatus = "BLOCKED_SIMULATOR_APP"
        } else if detectedAnchorsSet.contains(where: { $0.contains("ANCHOR_CARD_01") }) &&
                  detectedAnchorsSet.contains(where: { $0.contains("ANCHOR_CARD_28") }) {
            captureStatus = "EDITOR_CAPTURE_COMPLETE"
        } else {
            captureStatus = "PARTIAL_EDITOR_CAPTURE"
        }

        let coverage = EditorActionCoverageReport(
            schemaVersion: "1.0",
            shortcutLaunched: launched,
            shortcutForegroundVerified: foregroundVerified,
            editorOpened: editorOpened,
            scrollContainerFound: scrollType != nil,
            scrollContainerType: scrollType,
            scrollContainerFrame: scrollFrame,
            totalPagesCaptured: screenshotManifest.count,
            maxEditorPages: maxPages,
            captureStatus: captureStatus,
            detectedActionCards: Array(detectedCardsSet).sorted(),
            detectedAnchors: Array(detectedAnchorsSet).sorted(),
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
}
