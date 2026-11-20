import XCTest

/// Observe only the fixed public label cases while the native action is paused
/// in Quick Look. This never constructs a successful callback or changes values.
final class QADebugLabelPreviewObserver {
    private let output: URL
    private let start: TimeInterval
    private var policy: QADebugLabelPreviewPolicy
    private var finished = false

    init?(outputDirectory: URL, startedUptime: TimeInterval) {
        let environment = ProcessInfo.processInfo.environment
        let contextURL = outputDirectory.appendingPathComponent("debug-label-preview-context.json")
        let output = outputDirectory.appendingPathComponent("debug-label-preview.json")
        guard startedUptime.isFinite, startedUptime >= 0,
              startedUptime <= ProcessInfo.processInfo.systemUptime,
              !FileManager.default.fileExists(atPath: output.path),
              let data = try? Data(contentsOf: contextURL), data.count <= 4096,
              let context = try? JSONDecoder().decode(QADebugLabelPreviewContext.self, from: data),
              context.fixtureId == environment["CHERRI_QA_EXPECTED_SHORTCUT_NAME"],
              context.runNonce == environment["CHERRI_QA_EXEC_RUN_NONCE"],
              context.compilerCommitSha == environment["CHERRI_QA_SOURCE_SHA"],
              context.runId == environment["CHERRI_QA_WORKFLOW_RUN_ID"],
              context.runAttempt == Int(environment["CHERRI_QA_WORKFLOW_RUN_ATTEMPT"] ?? ""),
              let policy = QADebugLabelPreviewPolicy(context: context) else { return nil }
        self.output = output
        self.policy = policy
        self.start = startedUptime
    }

    private var elapsedMs: Int { max(0, Int((ProcessInfo.processInfo.systemUptime - start) * 1000)) }

    private func available(_ app: XCUIApplication) -> Bool {
        guard elapsedMs < QADebugLabelPreviewPolicy.budgetMs else { return false }
        let foreground = app.state == .runningForeground
        // app.state can itself wait for an unresponsive process. The returned
        // state never grants a new action after the monotonic budget expired.
        return foreground && elapsedMs < QADebugLabelPreviewPolicy.budgetMs
    }

    func step(in app: XCUIApplication) {
        guard !finished else { return }
        let before = elapsedMs
        var visible = Array(repeating: false, count: 5)
        var dated = Array(repeating: false, count: 5)
        var done: XCUIElement?
        var title = ""
        var foreground = available(app)
        if foreground {
            let bars = app.navigationBars
            let barCount = min(4, bars.count)
            for index in 0..<barCount {
                guard available(app) else { foreground = false; break }
                let bar = bars.element(boundBy: index)
                let candidate = bar.buttons["Done"].firstMatch
                if candidate.exists && candidate.isHittable {
                    guard available(app) else { foreground = false; break }
                    done = candidate
                    title = bar.identifier
                    break
                }
            }
        }
        if foreground && done != nil {
            for (index, label) in QADebugLabelPreviewPolicy.labels.enumerated() {
                guard available(app) else { foreground = false; break }
                let candidates = app.staticTexts.matching(NSPredicate(format: "label CONTAINS %@", label))
                let count = min(4, candidates.count)
                for candidateIndex in 0..<count {
                    guard available(app) else { foreground = false; break }
                    let candidate = candidates.element(boundBy: candidateIndex)
                    if candidate.exists && candidate.isHittable {
                        guard available(app) else { foreground = false; break }
                        let value = candidate.label
                        // A source card containing the label is not a preview
                        // title/body: a reviewed row starts with its exact label.
                        if QADebugLabelPreviewPolicy.labelVisible(value, label: label, context: policy.context) {
                            visible[index] = true
                            dated[index] = dated[index] || QADebugLabelPreviewPolicy.dateSuffixVisible(value, label: label)
                        }
                    }
                }
                if QADebugLabelPreviewPolicy.labelVisible(title, label: label, context: policy.context) {
                    visible[index] = true
                    dated[index] = dated[index] || QADebugLabelPreviewPolicy.dateSuffixVisible(title, label: label)
                }
            }
        }
        let reviewedTitle = QADebugLabelPreviewPolicy.reviewedPreviewTitle(title,
            context: policy.context, visibleLabel: visible.contains(true), previouslyOwned: policy.hasOwnedPreview)
        foreground = foreground && available(app)
        let container = foreground && done != nil && reviewedTitle
        let doneExists = foreground && done?.exists == true
        let doneHittable = doneExists && done?.isHittable == true
        foreground = foreground && available(app)
        let decision = policy.observe(startedMs: before, observedMs: elapsedMs,
            foreground: foreground, previewContainer: container, doneExists: doneExists,
            doneHittable: doneHittable, labels: visible, dateSuffixes: dated)
        switch decision {
        case .swipe:
            guard available(app) else { finish(); return }
            app.swipeLeft()
            _ = policy.completed(.swipe, at: elapsedMs)
        case .dismiss:
            guard available(app), let button = done, button.exists, button.isHittable,
                  available(app) else { finish(); return }
            button.tap()
            _ = policy.completed(.dismiss, at: elapsedMs)
            finish()
        case .stop:
            finish()
        case .wait:
            break
        }
    }

    func finish() {
        guard !finished else { return }
        finished = true
        let receipt = policy.receipt(elapsedMs: elapsedMs)
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        guard let data = try? encoder.encode(receipt), data.count <= 32768 else { return }
        // One receipt per execution nonce. A later confirm pass cannot replace
        // an observed failure with a different UI state.
        guard !FileManager.default.fileExists(atPath: output.path) else { return }
        try? data.write(to: output, options: .withoutOverwriting)
    }
}
