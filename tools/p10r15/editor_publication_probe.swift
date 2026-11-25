import Foundation

// Controlled backend payloads for exercising the exact extracted production
// publication methods. These are not compiler, SwiftUI or Apple runtime results.
enum CherriBuildProfile: String, Sendable {
    case debug
    case release
}

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
    var mode = "v2"
    var signed = false
    var signedPayload: Data? = nil
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
    static var compileCalls = 0
    static var receivedBuildRequests: [CherriEditorBuildRequest] = []
    static func compile(request: CherriEditorBuildRequest, signed: Bool) async throws -> CompiledShortcut {
        receivedBuildRequests.append(request)
        return try await compile(source: request.source, name: request.name, mode: request.mode, signed: signed)
    }
    static func compile(source: String, name: String, mode: String, signed: Bool) async throws -> CompiledShortcut {
        precondition(!compilePlans.isEmpty, "unexpected controlled compile")
        let plan = compilePlans.removeFirst()
        compileCalls += 1
        precondition(plan.source.utf16.elementsEqual(source.utf16), "wrong actual compile input")
        precondition(mode == plan.mode && signed == plan.signed, "wrong actual mode/signing input")
        await plan.started.release()
        await plan.release.wait()
        switch plan.outcome {
        case .success(let marker): return CompiledShortcut(name: marker, plist: Data(marker.utf8), signedShortcut: plan.signedPayload)
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


enum ProbePreparationOutcome: Sendable { case ready, diagnostic, cancelled }
struct ProbePreparationPlan: Sendable {
    var outcome: ProbePreparationOutcome = .ready
    var revisionAfter: UInt64? = nil
    let started = ProbeGate()
    let release = ProbeGate()
}
@MainActor final class ProbeProjectAssets {
    var revision: UInt64 = 0
    var plans: [ProbePreparationPlan] = []
    var calls = 0
    func prepare(fileURL: URL?) async throws {
        calls += 1
        guard !plans.isEmpty else { return }
        let plan = plans.removeFirst()
        await plan.started.release()
        await plan.release.wait()
        if let revisionAfter = plan.revisionAfter { revision = revisionAfter }
        switch plan.outcome {
        case .ready: return
        case .diagnostic: throw CompilationDiagnostic(message: "PROJECT_PREPARATION_FAILED", line: 1, column: 1)
        case .cancelled: throw CancellationError()
        }
    }
}

@MainActor final class WorkspaceHarness {
    // Controlled document state only. The build request/context value types and
    // current request getter are production source; no file adapter runs here.
    struct Document {
        var text: String {
            didSet { if !text.utf16.elementsEqual(oldValue.utf16) { revision &+= 1 } }
        }
        var identity = UUID()
        var revision: UInt64 = 0
        func compileContext(fileURL: URL?, assetRevision: UInt64) -> CherriDocumentCompileContext {
            CherriDocumentCompileContext(uri: fileURL?.absoluteString ?? "",
                                         documentID: identity, revision: revision,
                                         assetRevision: assetRevision)
        }
    }
    enum Mode: String { case v2, legacy }
    var document = Document(text: "")
    var fileDisplayName = "Controlled"
    var fileURL: URL? = nil
    let projectAssets = ProbeProjectAssets()
    var lastBuildOutcome = ""
    var publishedBuildRequest: CherriEditorBuildRequest?
    __ACTUAL_BUILD_REQUEST__
    var languageMode = Mode.v2
    var isSigning = false
    var isCompiling = false
    var isApplyingPreviewEdit = false
    var compiled: CompiledShortcut?
    var diagnostic: CompilationDiagnostic?
    var diagnostics: [CompilationDiagnostic] = []
    var signedURL: URL?
    var buildRequestGeneration: UInt64 = 0
    var catalogStarted: ProbeGate?
    var catalogRelease: ProbeGate?
    var signedWrites = 0
    func refreshActionCatalog() async {
        if let started = catalogStarted, let release = catalogRelease {
            catalogStarted = nil; catalogRelease = nil
            await started.release(); await release.wait()
        }
    }
    func writeSignedShortcut(_ data: Data, name: String) throws -> URL {
        signedWrites += 1
        return URL(fileURLWithPath: "/controlled-no-file-write/" + name)
    }
    __ACTUAL_BUILD_METHOD__
}
@MainActor final class CompletionHarness {
    var completionRequest: CherriCompletionRequest
    var contextualCompletions: [CherriCompletionItem] = []
    var completedRequest: CherriCompletionRequest?
    init(_ request: CherriCompletionRequest) { completionRequest = request }
    __ACTUAL_COMPLETION_METHOD__
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
        ["compiled": harness.compiled?.name ?? "", "diagnostic": harness.diagnostic?.message ?? "", "diagnostics": harness.diagnostics.map(\.message), "isCompiling": harness.isCompiling, "isSigning": harness.isSigning, "signedWrites": harness.signedWrites]
    }
    static func reset() {
        precondition(CherriCompiler.compilePlans.isEmpty && CherriCompiler.analysisPlans.isEmpty && CherriCompiler.completionPlans.isEmpty, "unconsumed actual adapter operation")
        CherriCompiler.receivedBuildRequests.removeAll()
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

    static func additionalWorkspaceControls() async {

        do {
            let harness = WorkspaceHarness(); harness.document.text = "\u{00e9}"
            let old = ProbeCompilePlan(source: "\u{00e9}", outcome: .success("STALE_UNICODE"))
            CherriCompiler.compilePlans.append(old)
            let task = Task { await harness.build(signed: false, liveBuild: true) }
            await old.started.wait(); harness.document.text = "e\u{0301}"
            await old.release.release(); await task.value
            emit("unicode_snapshot_without_replacement", harness.compiled == nil, observe(harness)); reset()
        }
        do {
            let harness = WorkspaceHarness()
            harness.signedURL = URL(fileURLWithPath: "/controlled-no-file-write/PREVIOUS_SIGNED")
            await healthy(harness, source: "new", marker: "NEW_UNSIGNED")
            emit("unsigned_success_clears_previous_signed_url", harness.compiled?.name == "NEW_UNSIGNED" && harness.signedURL == nil, observe(harness)); reset()
        }

        do {
            let harness = WorkspaceHarness(); harness.document.text = "old"
            let old = ProbeCompilePlan(source: "old", outcome: .success("OLD_MANUAL"))
            CherriCompiler.compilePlans.append(old)
            let task = Task { await harness.build(signed: false) }
            await old.started.wait(); harness.document.text = "new"
            await old.release.release(); await task.value
            emit("manual_stale_success", harness.compiled == nil, observe(harness)); reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "same"
            let old = ProbeCompilePlan(source: "same", outcome: .success("OLD_SAME"))
            CherriCompiler.compilePlans.append(old)
            let task = Task { await harness.build(signed: false, liveBuild: true) }
            await old.started.wait()
            await healthy(harness, source: "same", marker: "NEW_SAME")
            await old.release.release(); await task.value
            emit("same_source_replacement", harness.compiled?.name == "NEW_SAME", observe(harness)); reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "A"
            let old = ProbeCompilePlan(source: "A", outcome: .success("OLD_A"))
            CherriCompiler.compilePlans.append(old)
            let task = Task { await harness.build(signed: false, liveBuild: true) }
            await old.started.wait()
            await healthy(harness, source: "B", marker: "NEW_B")
            await healthy(harness, source: "A", marker: "NEW_A")
            await old.release.release(); await task.value
            emit("source_aba_replacement", harness.compiled?.name == "NEW_A", observe(harness)); reset()
        }
        for name in ["changed_mode_without_new_request", "changed_name_without_new_request"] {
            let harness = WorkspaceHarness(); harness.document.text = "same"
            let old = ProbeCompilePlan(source: "same", outcome: .success("OLD_CONTEXT"))
            CherriCompiler.compilePlans.append(old)
            let task = Task { await harness.build(signed: false, liveBuild: true) }
            await old.started.wait()
            if name == "changed_mode_without_new_request" { harness.languageMode = .legacy }
            else { harness.fileDisplayName = "Renamed" }
            await old.release.release(); await task.value
            emit(name, harness.compiled == nil, observe(harness)); reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "cancelled"
            let launch = ProbeGate()
            let old = ProbeCompilePlan(source: "cancelled", outcome: .success("CANCELLED_BEFORE_START"))
            await old.release.release()
            CherriCompiler.compilePlans.append(old)
            let before = CherriCompiler.compileCalls
            let task = Task { await launch.wait(); await harness.build(signed: false, liveBuild: true) }
            task.cancel(); await launch.release(); await task.value
            let calls = CherriCompiler.compileCalls - before
            // The queued controlled response is intentionally unused only when
            // the actual production method rejects cancellation before dispatch.
            CherriCompiler.compilePlans.removeAll()
            emit("pre_cancelled_request_does_not_compile", calls == 0 && harness.compiled == nil && !harness.isCompiling, ["actualCompileCalls": calls]); reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "invalid"
            let plan = ProbeCompilePlan(source: "invalid", outcome: .generic)
            await plan.release.release(); CherriCompiler.compilePlans.append(plan)
            await harness.build(signed: false)
            emit("current_generic_error", harness.diagnostic != nil && harness.diagnostics.count == 1 && harness.compiled == nil, observe(harness)); reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "old"
            let started = ProbeGate(), release = ProbeGate()
            harness.catalogStarted = started; harness.catalogRelease = release
            var old = ProbeCompilePlan(source: "old", outcome: .success("OLD_SIGNED"))
            old.signed = true; old.signedPayload = Data([1, 2, 3])
            await old.release.release(); CherriCompiler.compilePlans.append(old)
            let task = Task { await harness.build(signed: true) }
            await started.wait()
            await healthy(harness, source: "new", marker: "NEW_UNSIGNED")
            await release.release(); await task.value
            emit("stale_after_catalog_await_cannot_write", harness.compiled?.name == "NEW_UNSIGNED" && harness.signedURL == nil && harness.signedWrites == 0 && !harness.isSigning && !harness.isCompiling, observe(harness)); reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "invalid"
            let held = CompilationDiagnostic(message: "HELD", line: 1, column: 1)
            harness.diagnostic = held; harness.diagnostics = [held]
            let compile = ProbeCompilePlan(source: "invalid", outcome: .diagnostic("CANCELLED_COMPILE"))
            let analysis = ProbeAnalyzePlan(source: "invalid", message: "CANCELLED_ANALYSIS")
            await compile.release.release()
            CherriCompiler.compilePlans.append(compile); CherriCompiler.analysisPlans.append(analysis)
            let task = Task { await harness.build(signed: false, liveBuild: true) }
            await analysis.started.wait(); task.cancel()
            await analysis.release.release(); await task.value
            emit("cancelled_analysis_does_not_publish", harness.diagnostic?.message == "HELD" && harness.diagnostics.map(\.message) == ["HELD"], observe(harness)); reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "same"
            var plan = ProbeCompilePlan(source: "same", outcome: .success("CURRENT_SIGNED"))
            plan.signed = true; plan.signedPayload = Data([4, 5, 6])
            await plan.release.release(); CherriCompiler.compilePlans.append(plan)
            await harness.build(signed: true)
            emit("current_signed_adapter_publication", harness.compiled?.name == "CURRENT_SIGNED" && harness.signedWrites == 1 && harness.signedURL?.lastPathComponent == "CURRENT_SIGNED" && !harness.isSigning, observe(harness)); reset()
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

    static func documentContextControls() async {
        do {
            let harness = WorkspaceHarness()
            harness.fileURL = URL(fileURLWithPath: "/controlled-no-file-read/project/entry.cherri")
            harness.projectAssets.revision = 7
            harness.document.revision = 9
            let identity = harness.document.identity
            await healthy(harness, source: "context", marker: "CONTEXT")
            let observed = CherriCompiler.receivedBuildRequests.last
            let okay = observed?.document?.uri == harness.fileURL?.absoluteString
                && observed?.document?.documentID == identity
                && observed?.document?.revision == harness.document.revision
                && observed?.document?.assetRevision == 7
                && observed?.source == "context" && observed?.mode == "v2"
                && observed?.buildProfile == .debug
            var observation = observe(harness)
            observation["exactContextForwarded"] = okay
            emit("document_context_forwarded", okay && harness.compiled?.name == "CONTEXT", observation)
            reset()
        }
        for name in ["document_uri_change_drops_result", "document_identity_change_drops_result",
                     "document_revision_aba_drops_result", "asset_revision_change_drops_result"] {
            let harness = WorkspaceHarness()
            harness.fileURL = URL(fileURLWithPath: "/controlled-no-file-read/a/entry.cherri")
            harness.document.text = "same"
            let old = ProbeCompilePlan(source: "same", outcome: .success("STALE_CONTEXT"))
            CherriCompiler.compilePlans.append(old)
            let task = Task { await harness.build(signed: false, liveBuild: true) }
            await old.started.wait()
            switch name {
            case "document_uri_change_drops_result":
                harness.fileURL = URL(fileURLWithPath: "/controlled-no-file-read/b/entry.cherri")
            case "document_identity_change_drops_result":
                harness.document.identity = UUID()
            case "document_revision_aba_drops_result":
                harness.document.text = "other"; harness.document.text = "same"
            default:
                harness.projectAssets.revision &+= 1
            }
            await old.release.release(); await task.value
            emit(name, harness.compiled == nil && harness.signedURL == nil && !harness.isCompiling,
                 observe(harness))
            reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "prepared"
            let preparation = ProbePreparationPlan()
            harness.projectAssets.plans.append(preparation)
            let plan = ProbeCompilePlan(source: "prepared", outcome: .success("PREPARED"))
            await plan.release.release(); CherriCompiler.compilePlans.append(plan)
            let before = CherriCompiler.compileCalls
            let task = Task { await harness.build(signed: false, liveBuild: true) }
            await preparation.started.wait()
            let busy = harness.isCompiling && !harness.isSigning
            let noEarlyCall = CherriCompiler.compileCalls == before
            await preparation.release.release(); await task.value
            var observation = observe(harness)
            observation["busyDuringPreparation"] = busy
            observation["noCompileBeforePreparation"] = noEarlyCall
            emit("preparation_owns_busy_indicator", busy && noEarlyCall
                 && harness.compiled?.name == "PREPARED" && !harness.isCompiling, observation)
            reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "failed"
            harness.compiled = CompiledShortcut(name: "OLD_ARTIFACT", plist: Data(), signedShortcut: Data())
            harness.signedURL = URL(fileURLWithPath: "/controlled-no-file-write/OLD_SIGNED")
            let preparation = ProbePreparationPlan(outcome: .diagnostic)
            await preparation.release.release(); harness.projectAssets.plans.append(preparation)
            let before = CherriCompiler.compileCalls
            await harness.build(signed: true, liveBuild: false)
            var observation = observe(harness)
            observation["compiledCallsDelta"] = CherriCompiler.compileCalls - before
            observation["hasSignedURL"] = harness.signedURL != nil
            observation["lastBuildOutcome"] = harness.lastBuildOutcome
            emit("preparation_failure_clears_old_artifacts",
                 CherriCompiler.compileCalls == before && harness.compiled == nil
                 && harness.signedURL == nil && !harness.isSigning && !harness.isCompiling
                 && harness.diagnostic?.message == "PROJECT_PREPARATION_FAILED"
                 && harness.diagnostics.map(\.message) == ["PROJECT_PREPARATION_FAILED"]
                 && harness.lastBuildOutcome == "Signing failed", observation)
            reset()
        }
        do {
            let harness = WorkspaceHarness(); harness.document.text = "cancelled"
            let preparation = ProbePreparationPlan()
            harness.projectAssets.plans.append(preparation)
            let before = CherriCompiler.compileCalls
            let task = Task { await harness.build(signed: false, liveBuild: true) }
            await preparation.started.wait(); task.cancel()
            await preparation.release.release(); await task.value
            emit("cancelled_preparation_does_not_publish",
                 CherriCompiler.compileCalls == before && harness.compiled == nil
                 && harness.diagnostic == nil && !harness.isCompiling, observe(harness))
            reset()
        }
        for failure in [false, true] {
            let harness = WorkspaceHarness(); harness.document.text = "old"
            let preparation = ProbePreparationPlan(outcome: failure ? .diagnostic : .ready)
            harness.projectAssets.plans.append(preparation)
            let oldTask = Task { await harness.build(signed: false, liveBuild: true) }
            await preparation.started.wait()
            harness.document.text = "new"
            let newer = ProbeCompilePlan(source: "new", outcome: .success("NEW_CONTEXT"))
            CherriCompiler.compilePlans.append(newer)
            let newTask = Task { await harness.build(signed: false, liveBuild: true) }
            await newer.started.wait()
            if failure {
                await newer.release.release(); await newTask.value
                await preparation.release.release(); await oldTask.value
                emit("old_preparation_failure_cannot_replace_new_result",
                     harness.compiled?.name == "NEW_CONTEXT" && harness.diagnostic == nil, observe(harness))
            } else {
                await preparation.release.release(); await oldTask.value
                let busy = harness.isCompiling
                await newer.release.release(); await newTask.value
                var observation = observe(harness); observation["busyWhileNewerPending"] = busy
                emit("old_preparation_cannot_clear_new_busy", busy
                     && harness.compiled?.name == "NEW_CONTEXT" && !harness.isCompiling, observation)
            }
            reset()
        }
        do {
            let harness = WorkspaceHarness()
            let preparation = ProbePreparationPlan(outcome: .cancelled)
            await preparation.release.release(); harness.projectAssets.plans.append(preparation)
            let before = CherriCompiler.compileCalls
            await harness.build(signed: false, liveBuild: false)
            emit("observer_cancellation_does_not_publish_diagnostic",
                 CherriCompiler.compileCalls == before && harness.compiled == nil
                 && harness.diagnostic == nil && harness.diagnostics.isEmpty
                 && !harness.isCompiling && !harness.isSigning, observe(harness))
            reset()
        }
        do {
            let harness = WorkspaceHarness()
            let preparation = ProbePreparationPlan(revisionAfter: 19)
            await preparation.release.release(); harness.projectAssets.plans.append(preparation)
            await healthy(harness, source: "after-observation", marker: "AFTER_OBSERVATION")
            let observed = CherriCompiler.receivedBuildRequests.last
            emit("observer_revision_captured_after_preparation",
                 observed?.document?.assetRevision == 19 && harness.compiled?.name == "AFTER_OBSERVATION",
                 observe(harness))
            reset()
        }
    }

    static func main() async {
        await workspaceControls()
        await additionalWorkspaceControls()
        await documentContextControls()
        await completionControls()
        precondition(controls == 38)
        print("{\"event\":\"summary\",\"controls\":\(controls),\"failed\":\(failures),\"uiExecuted\":false,\"realCompilerExecuted\":false}")
        if failures != 0 { exit(1) }
    }
}
