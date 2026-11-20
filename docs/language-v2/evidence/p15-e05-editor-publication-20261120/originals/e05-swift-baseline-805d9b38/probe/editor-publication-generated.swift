import Foundation
struct CherriCompletionItem: Decodable, Identifiable, Hashable, Sendable {
    var id: String { "\(label)_\(kind)" }
    let label: String
    let kind: Int
    let detail: String?
    let documentation: String?
    let insertText: String?
}
import Foundation

// Controlled backend payloads for exercising the exact extracted production
// publication methods. These are not compiler, SwiftUI or Apple runtime results.
struct CompilationDiagnostic: Error, Sendable {
    let message: String
    let line: Int
    let column: Int
    var endLine: Int? = nil
    var endColumn: Int? = nil
    var length: Int? = nil
}
struct CompiledShortcut: Sendable {
    let name: String
    let plist: Data
    let signedShortcut: Data?
}
struct ProbePosition: Sendable { let line: Int; let character: Int }
struct ProbeRange: Sendable { let start: ProbePosition; let end: ProbePosition }
struct ProbeDiagnostic: Sendable { let message: String; let range: ProbeRange }
struct ProbeAnalysis: Sendable { let valid: Bool; let diagnostics: [ProbeDiagnostic] }
struct ProbeError: Error, Sendable {}

actor ProbeGate {
    private var opened = false
    private var waiting: [CheckedContinuation<Void, Never>] = []
    func wait() async {
        if opened { return }
        await withCheckedContinuation { waiting.append($0) }
    }
    func release() {
        opened = true
        let pending = waiting
        waiting.removeAll()
        for continuation in pending { continuation.resume() }
    }
}

enum ProbeOutcome: Sendable { case success(String), diagnostic(String), generic }
struct ProbeCompilePlan: Sendable {
    let source: String
    let outcome: ProbeOutcome
    let started = ProbeGate()
    let release = ProbeGate()
}
struct ProbeAnalyzePlan: Sendable {
    let source: String
    let message: String
    let started = ProbeGate()
    let release = ProbeGate()
}
struct ProbeCompletionPlan: Sendable {
    let source: String
    let label: String
    let started = ProbeGate()
    let release = ProbeGate()
}

@MainActor enum CherriCompiler {
    static var compilePlans: [ProbeCompilePlan] = []
    static var analysisPlans: [ProbeAnalyzePlan] = []
    static var completionPlans: [ProbeCompletionPlan] = []
    static func compile(source: String, name: String, mode: String, signed: Bool) async throws -> CompiledShortcut {
        precondition(!compilePlans.isEmpty, "unexpected controlled compile")
        let plan = compilePlans.removeFirst()
        precondition(plan.source.utf16.elementsEqual(source.utf16), "wrong actual compile input")
        precondition(mode == "v2" && !signed, "unexpected mode/signing side effect")
        await plan.started.release()
        await plan.release.wait()
        switch plan.outcome {
        case .success(let marker): return CompiledShortcut(name: marker, plist: Data(marker.utf8), signedShortcut: nil)
        case .diagnostic(let message): throw CompilationDiagnostic(message: message, line: 1, column: 1)
        case .generic: throw ProbeError()
        }
    }
    static func analyze(source: String) async throws -> ProbeAnalysis {
        precondition(!analysisPlans.isEmpty, "unexpected controlled analysis")
        let plan = analysisPlans.removeFirst()
        precondition(plan.source.utf16.elementsEqual(source.utf16), "wrong actual analysis input")
        await plan.started.release()
        await plan.release.wait()
        return ProbeAnalysis(valid: false, diagnostics: [ProbeDiagnostic(message: plan.message, range: ProbeRange(start: ProbePosition(line: 0, character: 1), end: ProbePosition(line: 0, character: 3)))])
    }
    static func complete(source: String, line: Int, column: Int) async throws -> [CherriCompletionItem] {
        precondition(!completionPlans.isEmpty, "unexpected controlled completion")
        let plan = completionPlans.removeFirst()
        precondition(plan.source.utf16.elementsEqual(source.utf16), "wrong actual completion input")
        precondition(line > 0 && column > 0, "invalid actual position")
        await plan.started.release()
        await plan.release.wait()
        return [CherriCompletionItem(label: plan.label, kind: 0, detail: nil, documentation: nil, insertText: nil)]
    }
}

@MainActor final class WorkspaceHarness {
    struct Document { var text: String }
    enum Mode: String { case v2 }
    var document = Document(text: "")
    var fileDisplayName = "Controlled"
    var languageMode = Mode.v2
    var isSigning = false
    var isCompiling = false
    var compiled: CompiledShortcut?
    var diagnostic: CompilationDiagnostic?
    var diagnostics: [CompilationDiagnostic] = []
    var signedURL: URL?
    var buildRequestGeneration: UInt64 = 0
    func refreshActionCatalog() async {}
    func writeSignedShortcut(_ data: Data, name: String) throws -> URL {
        preconditionFailure("controlled unsigned route attempted a file write")
    }
    func build(signed: Bool, liveBuild: Bool = false) async {
        let source = document.text
        let requestedName = fileDisplayName

        if signed {
            isSigning = true
        } else {
            isCompiling = true
        }

        defer {
            if signed {
                isSigning = false
            } else {
                isCompiling = false
            }
        }

        do {
            let result = try await CherriCompiler.compile(
                source: source,
                name: requestedName,
                mode: languageMode.rawValue,
                signed: signed
            )

            if liveBuild && source != document.text {
                return
            }

            compiled = result
            diagnostic = nil
            diagnostics = []
            await refreshActionCatalog()

            if let signedData = result.signedShortcut {
                signedURL = try writeSignedShortcut(signedData, name: result.name)
            }
        } catch let compilerError as CompilationDiagnostic {
            if liveBuild && source != document.text {
                return
            }
            diagnostic = compilerError
            if let analysis = try? await CherriCompiler.analyze(source: source), !analysis.valid && !analysis.diagnostics.isEmpty {
                diagnostics = analysis.diagnostics.map {
                    let startLine = $0.range.start.line + 1
                    let startCol = $0.range.start.character + 1
                    let endLine = $0.range.end.line + 1
                    let endCol = $0.range.end.character + 1
                    let len = (endLine == startLine && endCol > startCol) ? (endCol - startCol) : max(1, endCol - startCol)
                    return CompilationDiagnostic(
                        message: $0.message,
                        line: startLine,
                        column: startCol,
                        endLine: endLine,
                        endColumn: endCol,
                        length: len
                    )
                }
            } else {
                diagnostics = [compilerError]
            }
            signedURL = nil
        } catch {
            let diag = CompilationDiagnostic(
                message: error.localizedDescription,
                line: 1,
                column: 1
            )
            diagnostic = diag
            diagnostics = [diag]
            signedURL = nil
        }
    }
}
@MainActor final class CompletionHarness {
    var completionRequest: CherriCompletionRequest
    var contextualCompletions: [CherriCompletionItem] = []
    var completedRequest: CherriCompletionRequest?
    init(_ request: CherriCompletionRequest) { completionRequest = request }
    func refreshContextualCompletions(for request: CherriCompletionRequest) async {
        guard let position = request.position else {
            if request == completionRequest {
                contextualCompletions = []
                completedRequest = request
            }
            return
        }
        let items = (try? await CherriCompiler.complete(
            source: request.source, line: position.line, column: position.column
        )) ?? []
        // Source edits can precede selection notifications. Both invalidate the
        // old request; a cancelled bridge call cannot publish stale candidates.
        guard !Task.isCancelled, request == completionRequest else { return }
        contextualCompletions = items
        completedRequest = request
    }
}

@main @MainActor struct EditorPublicationProbe {
    static var failures = 0
    static var controls = 0
    static func emit(_ name: String, _ passed: Bool, _ observation: [String: Any]) {
        controls += 1
        if !passed { failures += 1 }
        let row: [String: Any] = ["event": "control", "name": name, "result": passed ? "PASS" : "FAIL", "scope": "controlled adapter; actual production method body", "observation": observation]
        let data = try! JSONSerialization.data(withJSONObject: row, options: [.sortedKeys])
        print(String(data: data, encoding: .utf8)!)
    }
    static func observe(_ harness: WorkspaceHarness) -> [String: Any] {
        ["compiled": harness.compiled?.name ?? "", "diagnostic": harness.diagnostic?.message ?? "", "diagnostics": harness.diagnostics.map(\.message), "isCompiling": harness.isCompiling]
    }
    static func reset() {
        precondition(CherriCompiler.compilePlans.isEmpty && CherriCompiler.analysisPlans.isEmpty && CherriCompiler.completionPlans.isEmpty, "unconsumed actual adapter operation")
    }
    static func healthy(_ harness: WorkspaceHarness, source: String, marker: String) async {
        harness.document.text = source
        let plan = ProbeCompilePlan(source: source, outcome: .success(marker))
        await plan.release.release()
        CherriCompiler.compilePlans.append(plan)
        await harness.build(signed: false, liveBuild: true)
    }
    static func workspaceControls() async {
        do {
            let harness = WorkspaceHarness()
            await healthy(harness, source: "current", marker: "CURRENT")
            emit("current_success", harness.compiled?.name == "CURRENT" && harness.diagnostics.isEmpty && !harness.isCompiling, observe(harness))
            reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "old"
            let old = ProbeCompilePlan(source: "old", outcome: .success("OLD"))
            CherriCompiler.compilePlans.append(old)
            let task = Task { await harness.build(signed: false, liveBuild: true) }
            await old.started.wait()
            await healthy(harness, source: "new", marker: "NEW")
            await old.release.release(); await task.value
            emit("late_success_after_newer", harness.compiled?.name == "NEW" && harness.diagnostics.isEmpty, observe(harness)); reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "old"
            let compile = ProbeCompilePlan(source: "old", outcome: .diagnostic("OLD_COMPILE"))
            let analysis = ProbeAnalyzePlan(source: "old", message: "OLD_ANALYSIS")
            await compile.release.release()
            CherriCompiler.compilePlans.append(compile); CherriCompiler.analysisPlans.append(analysis)
            let task = Task { await harness.build(signed: false, liveBuild: true) }
            await analysis.started.wait()
            await healthy(harness, source: "new", marker: "NEW")
            await analysis.release.release(); await task.value
            emit("late_analysis_after_newer", harness.compiled?.name == "NEW" && harness.diagnostic == nil && harness.diagnostics.isEmpty, observe(harness)); reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "old"
            let old = ProbeCompilePlan(source: "old", outcome: .generic)
            CherriCompiler.compilePlans.append(old)
            let task = Task { await harness.build(signed: false, liveBuild: true) }
            await old.started.wait()
            await healthy(harness, source: "new", marker: "NEW")
            await old.release.release(); await task.value
            emit("late_generic_error_after_newer", harness.compiled?.name == "NEW" && harness.diagnostic == nil && harness.diagnostics.isEmpty, observe(harness)); reset()
        }
        do {
            let harness = WorkspaceHarness()
            let oldSource = "\u{00e9}", newSource = "e\u{0301}"
            precondition(oldSource == newSource && !oldSource.utf16.elementsEqual(newSource.utf16))
            harness.document.text = oldSource
            let old = ProbeCompilePlan(source: oldSource, outcome: .success("OLD_UTF16"))
            CherriCompiler.compilePlans.append(old)
            let task = Task { await harness.build(signed: false, liveBuild: true) }
            await old.started.wait()
            await healthy(harness, source: newSource, marker: "NEW_UTF16")
            await old.release.release(); await task.value
            emit("canonical_unicode_is_not_same_snapshot", harness.compiled?.name == "NEW_UTF16", observe(harness)); reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "same"
            harness.compiled = CompiledShortcut(name: "HELD", plist: Data(), signedShortcut: nil)
            let old = ProbeCompilePlan(source: "same", outcome: .success("CANCELLED"))
            CherriCompiler.compilePlans.append(old)
            let task = Task { await harness.build(signed: false, liveBuild: true) }
            await old.started.wait(); task.cancel()
            await old.release.release(); await task.value
            emit("cancelled_same_source_does_not_publish", harness.compiled?.name == "HELD", observe(harness)); reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "old"
            let old = ProbeCompilePlan(source: "old", outcome: .success("OLD"))
            CherriCompiler.compilePlans.append(old)
            let oldTask = Task { await harness.build(signed: false, liveBuild: true) }
            await old.started.wait()
            harness.document.text = "new"
            let newer = ProbeCompilePlan(source: "new", outcome: .success("NEW"))
            CherriCompiler.compilePlans.append(newer)
            let newTask = Task { await harness.build(signed: false, liveBuild: true) }
            await newer.started.wait()
            await old.release.release(); await oldTask.value
            let busyAfterOld = harness.isCompiling
            await newer.release.release(); await newTask.value
            var observation = observe(harness); observation["busyWhileNewerPending"] = busyAfterOld
            emit("old_completion_cannot_clear_new_busy", busyAfterOld && harness.compiled?.name == "NEW" && !harness.isCompiling, observation); reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "invalid"
            let compile = ProbeCompilePlan(source: "invalid", outcome: .diagnostic("CURRENT_COMPILE"))
            let analysis = ProbeAnalyzePlan(source: "invalid", message: "CURRENT_ANALYSIS")
            await compile.release.release(); await analysis.release.release()
            CherriCompiler.compilePlans.append(compile); CherriCompiler.analysisPlans.append(analysis)
            await harness.build(signed: false, liveBuild: true)
            emit("current_error_is_not_suppressed", harness.diagnostic?.message == "CURRENT_COMPILE" && harness.diagnostics.map(\.message) == ["CURRENT_ANALYSIS"], observe(harness)); reset()
        }
    }
    static func completionControls() async {
        let original = CherriCompletionRequest(source: "sho", selection: NSRange(location: 3, length: 0))
        for name in ["current_completion", "different_source", "different_caret", "canonical_unicode", "cancelled_completion"] {
            let requested = name == "canonical_unicode" ? CherriCompletionRequest(source: "\u{00e9}", selection: NSRange(location: 1, length: 0)) : original
            let harness = CompletionHarness(requested)
            let plan = ProbeCompletionPlan(source: requested.source, label: "CONTROLLED_ITEM")
            CherriCompiler.completionPlans.append(plan)
            let task = Task { await harness.refreshContextualCompletions(for: requested) }
            await plan.started.wait()
            switch name {
            case "different_source": harness.completionRequest = CherriCompletionRequest(source: "tex", selection: NSRange(location: 3, length: 0))
            case "different_caret": harness.completionRequest = CherriCompletionRequest(source: "sho", selection: NSRange(location: 2, length: 0))
            case "canonical_unicode": harness.completionRequest = CherriCompletionRequest(source: "e\u{0301}", selection: NSRange(location: 1, length: 0))
            case "cancelled_completion": task.cancel()
            default: break
            }
            await plan.release.release(); await task.value
            let published = harness.contextualCompletions.map(\.label)
            let expected = name == "current_completion" ? ["CONTROLLED_ITEM"] : []
            emit(name, published == expected && (name == "current_completion" ? harness.completedRequest == requested : harness.completedRequest == nil), ["items": published, "completed": harness.completedRequest != nil]); reset()
        }
        let invalid = CherriCompletionRequest(source: "sho", selection: NSRange(location: 1, length: 1))
        let harness = CompletionHarness(invalid)
        await harness.refreshContextualCompletions(for: invalid)
        emit("selection_range_does_not_call_completion", harness.completedRequest == invalid && harness.contextualCompletions.isEmpty, ["completed": harness.completedRequest != nil]); reset()
    }
    static func main() async {
        await workspaceControls()
        await completionControls()
        precondition(controls == 14)
        print("{\"event\":\"summary\",\"controls\":\(controls),\"failed\":\(failures),\"uiExecuted\":false,\"realCompilerExecuted\":false}")
        if failures != 0 { exit(1) }
    }
}
