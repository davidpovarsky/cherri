#if DEBUG
import CryptoKit
import Foundation
import SwiftUI
import UIKit

/// Explicit UI-test observation only. It delays delivery AFTER the real
/// compiler returns; it never supplies, edits or re-labels an analysis result.
@MainActor
final class CherriEditorDeliveryObservation {
    struct Configuration: Decodable {
        let schema: String
        let nonce: String
        let olderSourceSHA256: String
        let currentSourceSHA256: String
        let caretTargets: [String: Int]
    }

    struct Identity: Equatable {
        let documentID: UUID
        let revision: UInt64
        let generation: UInt64
        let sourceSHA256: String
        let liveBuild: Bool

        var fields: [String: Any] {
            ["documentID": documentID.uuidString, "revision": revision,
             "generation": generation, "sourceSHA256": sourceSHA256, "liveBuild": liveBuild]
        }
    }

    static let shared: CherriEditorDeliveryObservation? = {
        let environment = ProcessInfo.processInfo.environment
        guard ProcessInfo.processInfo.arguments.contains("--uitesting"),
              let raw = environment["CHERRI_RP39_UI_OBSERVATION"],
              raw.utf8.count <= 2048,
              let data = raw.data(using: .utf8),
              let configuration = try? JSONDecoder().decode(Configuration.self, from: data),
              configuration.schema == "cherri-rp39-ui-observation/1",
              UUID(uuidString: configuration.nonce) != nil,
              isDigest(configuration.olderSourceSHA256),
              isDigest(configuration.currentSourceSHA256),
              configuration.olderSourceSHA256 != configuration.currentSourceSHA256,
              configuration.caretTargets.count == 4,
              configuration.caretTargets.allSatisfy({
                  isDigest($0.key) && (0..<16384).contains($0.value)
              }) else { return nil }
        return CherriEditorDeliveryObservation(configuration)
    }()

    let configuration: Configuration
    private var phase = "configured"
    private var older: Identity?
    private var current: Identity?
    private var olderDiagnostics: [[String: Any]] = []
    private var currentDiagnostics: [[String: Any]] = []
    private var currentCaret: [String: Any]?
    private var discardedCaret: [String: Any]?
    private var held: CheckedContinuation<Void, Never>?
    private var expiry: Task<Void, Never>?
    private var openedAt: TimeInterval?
    private var releasedAt: TimeInterval?
    var readNativeCaret: (() -> [String: Any])?

    init(_ configuration: Configuration) { self.configuration = configuration }

    static func digest(_ text: String) -> String {
        SHA256.hash(data: Data(text.utf8)).map { String(format: "%02x", $0) }.joined()
    }

    private static func isDigest(_ value: String) -> Bool {
        value.count == 64 && value.utf8.allSatisfy { (48...57).contains($0) || (97...102).contains($0) }
    }

    private func identity(_ request: CherriEditorBuildRequest, _ generation: UInt64, _ liveBuild: Bool) -> Identity? {
        guard let document = request.document else { return nil }
        return Identity(documentID: document.documentID, revision: document.revision,
                        generation: generation, sourceSHA256: Self.digest(request.source), liveBuild: liveBuild)
    }

    private func diagnosticFields(_ analysis: CherriAnalyzeResponse) -> [[String: Any]] {
        analysis.diagnostics.prefix(8).map {
            ["code": $0.code, "severity": $0.severity,
             "startLine": $0.range.start.line, "startCharacter": $0.range.start.character,
             "endLine": $0.range.end.line, "endCharacter": $0.range.end.character]
        }
    }

    func afterRealAnalysis(_ analysis: CherriAnalyzeResponse?,
                           request: CherriEditorBuildRequest, generation: UInt64, liveBuild: Bool) async {
        guard !Task.isCancelled, !liveBuild, phase == "configured",
              let value = identity(request, generation, liveBuild),
              value.sourceSHA256 == configuration.olderSourceSHA256 else { return }
        guard let analysis, !analysis.valid, !analysis.diagnostics.isEmpty,
              analysis.diagnostics.count <= 8 else {
            phase = "older-analysis-invalid"
            return
        }
        older = value
        olderDiagnostics = diagnosticFields(analysis)
        openedAt = ProcessInfo.processInfo.systemUptime
        phase = "older-real-analysis-held"
        await withTaskCancellationHandler {
            await withCheckedContinuation { continuation in
                held = continuation
                // The existing Build observation allowance is eight seconds.
                // Expiry is a failure observation, never a longer UI deadline.
                expiry = Task { @MainActor [weak self] in
                    try? await Task.sleep(for: .seconds(8))
                    guard !Task.isCancelled else { return }
                    self?.finish("delivery-expired")
                }
            }
        } onCancel: { [weak self] in
            Task { @MainActor in self?.finish("delivery-cancelled") }
        }
    }

    /// Called only after the production assignment of the real diagnostics.
    func didPublish(_ analysis: CherriAnalyzeResponse?,
                    request: CherriEditorBuildRequest, generation: UInt64, liveBuild: Bool) {
        guard let value = identity(request, generation, liveBuild), let older else { return }
        if value == older {
            // The guard-removal counterfactual must remain observable as wrong.
            phase = "older-analysis-published"
            finish(phase)
            return
        }
        guard phase == "older-real-analysis-held", liveBuild,
              value.sourceSHA256 == configuration.currentSourceSHA256,
              value.documentID == older.documentID,
              value.revision > older.revision, value.generation > older.generation,
              let analysis, !analysis.valid, !analysis.diagnostics.isEmpty,
              analysis.diagnostics.count <= 8 else { return }
        current = value
        currentDiagnostics = diagnosticFields(analysis)
        currentCaret = readNativeCaret?()
        finish("current-analysis-published")
    }

    /// Called by the unchanged post-await stale/cancellation guard.
    func didDiscard(request: CherriEditorBuildRequest, generation: UInt64, liveBuild: Bool) {
        guard let value = identity(request, generation, liveBuild), value == older else { return }
        discardedCaret = readNativeCaret?()
        phase = phase == "current-analysis-published" && current != nil
            ? "older-analysis-discarded" : "older-discarded-before-current"
    }

    private func finish(_ reason: String) {
        guard let continuation = held else { return }
        phase = reason
        releasedAt = ProcessInfo.processInfo.systemUptime
        held = nil
        expiry?.cancel()
        expiry = nil
        continuation.resume()
    }

    var snapshot: [String: Any] {
        ["schema": configuration.schema, "nonce": configuration.nonce, "phase": phase,
         "older": older?.fields as Any? ?? NSNull(),
         "current": current?.fields as Any? ?? NSNull(),
         "olderDiagnostics": olderDiagnostics, "currentDiagnostics": currentDiagnostics,
         "currentCaret": currentCaret as Any? ?? NSNull(),
         "discardedCaret": discardedCaret as Any? ?? NSNull(),
         "elapsedMilliseconds": openedAt.map {
             Int(((releasedAt ?? ProcessInfo.processInfo.systemUptime) - $0) * 1000)
         } as Any? ?? NSNull()]
    }
}

/// An absent supplier is not an observed false setting. Only the owning
/// Workspace injects its actual AppStorage value into this DEBUG observation.
private struct CherriRP39LivePreviewKey: EnvironmentKey {
    static let defaultValue: Bool? = nil
}

extension EnvironmentValues {
    var rp39LivePreview: Bool? {
        get { self[CherriRP39LivePreviewKey.self] }
        set { self[CherriRP39LivePreviewKey.self] = newValue }
    }
}

/// The witness is read-only: no delegate replacement, caret assignment, focus,
/// keyboard event or source mutation. XCTest still uses the actual UI controls.
@MainActor
struct CherriRP39NativeEditorWitness: UIViewRepresentable {
    let source: String
    @Environment(\.rp39LivePreview) private var livePreview

    func makeUIView(context: Context) -> Witness {
        let view = Witness()
        view.isUserInteractionEnabled = false
        view.isAccessibilityElement = true
        view.accessibilityIdentifier = "rp39NativeEditorObservation"
        view.accessibilityLabel = "RP39 native editor observation"
        view.source = source
        view.livePreview = livePreview
        CherriEditorDeliveryObservation.shared?.readNativeCaret = { [weak view] in
            view?.nativeSnapshot() ?? ["status": "witness-released"]
        }
        return view
    }

    func updateUIView(_ view: Witness, context: Context) {
        view.source = source
        view.livePreview = livePreview
    }

    final class Witness: UIView {
        var source = ""
        var livePreview: Bool?

        override var accessibilityValue: String? {
            get {
                guard let observation = CherriEditorDeliveryObservation.shared else { return nil }
                let value: [String: Any] = ["native": nativeSnapshot(), "delivery": observation.snapshot]
                guard let raw = try? JSONSerialization.data(withJSONObject: value, options: [.sortedKeys]),
                      raw.count <= 8192 else { return nil }
                return String(decoding: raw, as: UTF8.self)
            }
            set { }
        }

        func nativeSnapshot() -> [String: Any] {
            guard let window else { return ["status": "no-window"] }
            var pending: [UIView] = [window]
            var editors: [UITextView] = []
            var visited = 0
            while let next = pending.popLast() {
                visited += 1
                guard visited <= 512 else { return ["status": "scan-limit"] }
                if let editor = next as? UITextView,
                   editor.accessibilityIdentifier == "cherriCodeEditor" {
                    editors.append(editor)
                }
                pending.append(contentsOf: next.subviews)
            }
            guard editors.count == 1, let editor = editors.first,
                  editor.window === window,
                  (editor.text ?? "").utf16.elementsEqual(source.utf16) else {
                return ["status": "editor-identity", "editorCount": editors.count]
            }
            let text = editor.text ?? ""
            let length = text.utf16.count
            let selected = editor.selectedRange
            guard length <= 16384, selected.location != NSNotFound,
                  selected.location <= length, selected.length <= length - selected.location else {
                return ["status": "selection-invalid"]
            }
            let digest = CherriEditorDeliveryObservation.digest(text)
            var value: [String: Any] = [
                "status": "observed", "editorCount": 1, "sourceSHA256": digest,
                "utf16Length": length, "selectionLocation": selected.location,
                "selectionLength": selected.length, "firstResponder": editor.isFirstResponder,
            ]
            if let livePreview {
                value["livePreviewEnabled"] = livePreview
            }
            // Observe the current collapsed caret so XCTest can begin the real
            // press-and-drag at that caret. This never sets a text selection.
            if selected.length == 0,
               let currentPosition = editor.position(from: editor.beginningOfDocument, offset: selected.location) {
                let localCaret = editor.caretRect(for: currentPosition)
                let rect = editor.convert(localCaret, to: window)
                let frame = editor.convert(editor.bounds, to: window)
                let dx = (rect.midX - frame.minX) / frame.width
                let dy = (rect.midY - frame.minY) / frame.height
                let geometry = [localCaret.midX, localCaret.midY, rect.midX, rect.midY,
                                frame.minX, frame.minY, frame.width, frame.height, dx, dy]
                if frame.width > 0, frame.height > 0,
                   geometry.allSatisfy({ $0.isFinite }),
                   (0...1).contains(dx), (0...1).contains(dy) {
                    value["currentCaretOffset"] = selected.location
                    value["currentCaretDX"] = Double(dx)
                    value["currentCaretDY"] = Double(dy)
                    value["currentCaretEditorPoint"] = [Double(localCaret.midX), Double(localCaret.midY)]
                    value["currentCaretWindowPoint"] = [Double(rect.midX), Double(rect.midY)]
                    // Read-only directional predictions admit at most one real
                    // XCTest arrow key; they never assign or prove a selection.
                    for (field, direction) in [
                        ("currentCaretLeftUTF16Offset", UITextLayoutDirection.left),
                        ("currentCaretRightUTF16Offset", UITextLayoutDirection.right),
                    ] {
                        if let next = editor.position(from: currentPosition, in: direction, offset: 1) {
                            let nextOffset = editor.offset(from: editor.beginningOfDocument, to: next)
                            if (0...length).contains(nextOffset), nextOffset != selected.location {
                                value[field] = nextOffset
                            }
                        }
                    }
                }
            }
            if let target = CherriEditorDeliveryObservation.shared?.configuration.caretTargets[digest],
               target <= length,
               let position = editor.position(from: editor.beginningOfDocument, offset: target) {
                let localCaret = editor.caretRect(for: position)
                let rect = editor.convert(localCaret, to: window)
                let frame = editor.convert(editor.bounds, to: window)
                let dx = (rect.midX - frame.minX) / frame.width
                let dy = (rect.midY - frame.minY) / frame.height
                if frame.width > 0, frame.height > 0, dx.isFinite, dy.isFinite,
                   (0...1).contains(dx), (0...1).contains(dy) {
                    value["targetOffset"] = target
                    value["targetDX"] = Double(dx)
                    value["targetDY"] = Double(dy)
                    // Diagnostic geometry only: no focus, selection or event.
                    // The native closest position does not prove where XCTest
                    // delivered its separately retained synthesized pointer.
                    let geometry = [localCaret.midX, localCaret.midY, rect.midX, rect.midY,
                                    frame.minX, frame.minY, frame.width, frame.height]
                    if geometry.allSatisfy({ $0.isFinite }) {
                        value["targetEditorPoint"] = [Double(localCaret.midX), Double(localCaret.midY)]
                        value["targetWindowPoint"] = [Double(rect.midX), Double(rect.midY)]
                        value["editorWindowFrame"] = [Double(frame.minX), Double(frame.minY),
                                                       Double(frame.width), Double(frame.height)]
                        if let closest = editor.closestPosition(to: CGPoint(x: localCaret.midX, y: localCaret.midY)) {
                            let closestOffset = editor.offset(from: editor.beginningOfDocument, to: closest)
                            if (0...length).contains(closestOffset) {
                                value["targetClosestUTF16Offset"] = closestOffset
                            }
                        }
                    }
                }
            }
            return value
        }
    }
}
#endif
