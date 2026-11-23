import SwiftUI
import UniformTypeIdentifiers

enum CherriLanguageMode: String, CaseIterable, Identifiable {
    case v2 = "v2"
    case legacy = "legacy"
    case auto = "auto"

    var id: Self { self }
}

private enum WorkspacePane: String, CaseIterable, Identifiable {
    case code = "Code"
    case preview = "Preview"

    var id: Self { self }
}

struct WorkspaceView: View {
    @Binding var document: CherriDocument
    let fileURL: URL?

    @Environment(\.horizontalSizeClass) private var horizontalSizeClass
    @AppStorage("Cherri.livePreview") private var livePreview = true

    @State private var languageMode: CherriLanguageMode = .v2
    @State private var selectedPane: WorkspacePane = .code
    @State private var compiled: CompiledShortcut?
    @State private var diagnostic: CompilationDiagnostic?
    @State private var diagnostics: [CompilationDiagnostic] = []
    @State private var actionCatalog: [CherriActionInfo] = []
    @State private var previewActionMetadataJSON: String?
    @State private var isCompiling = false
    @State private var buildRequestGeneration: UInt64 = 0
    @State private var isSigning = false
    @State private var isImporting = false
    @State private var isApplyingPreviewEdit = false
    @State private var previewEditGeneration = 0
    @State private var sourceBeforePreviewEdits: String?
    @State private var signedURL: URL?
    @State private var showSigningConfirmation = false
    @State private var showShortcutImporter = false
    @State private var showDocumentation = false

    var body: some View {
        let _ = CherriStartupLifecycleObserver.shared?.record(.workspaceBody, documentID: document.startupObservationID)
        VStack(spacing: 0) {
            if horizontalSizeClass == .regular {
                HStack(spacing: 0) {
                    editorPane
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                    Divider()
                    previewPane
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                }
            } else {
                Picker("Workspace", selection: $selectedPane) {
                    ForEach(WorkspacePane.allCases) { pane in
                        Text(pane.rawValue).tag(pane)
                    }
                }
                .pickerStyle(.segmented)
                .padding(.horizontal)
                .padding(.vertical, 8)

                switch selectedPane {
                case .code:
                    editorPane
                case .preview:
                    previewPane
                }
            }

            statusBar
        }
        .onAppear {
            CherriStartupLifecycleObserver.shared?.record(.workspaceAppeared, documentID: document.startupObservationID)
        }
        .toolbar {
            ToolbarItemGroup(placement: .topBarTrailing) {
                Button {
                    Task { await build(signed: false) }
                } label: {
                    Label("Build", systemImage: "hammer.fill")
                }
                .accessibilityIdentifier("buildButton")
                .disabled(isBusy)

                Button {
                    showSigningConfirmation = true
                } label: {
                    Label("Sign", systemImage: "checkmark.seal.fill")
                }
                .accessibilityIdentifier("signButton")
                .disabled(isBusy)

                if let signedURL {
                    ShareLink(item: signedURL) {
                        Label("Share", systemImage: "square.and.arrow.up")
                    }
                }

                Menu {
                    Button {
                        showDocumentation = true
                    } label: {
                        Label("Documentation", systemImage: "doc.text.magnifyingglass")
                    }
                    .accessibilityIdentifier("documentationButton")

                    Button {
                        showShortcutImporter = true
                    } label: {
                        Label("Import Shortcut Plist", systemImage: "square.and.arrow.down")
                    }

                    if sourceBeforePreviewEdits != nil {
                        Button {
                            restoreSourceBeforePreviewEdits()
                        } label: {
                            Label("Restore Source Before Visual Edits", systemImage: "arrow.uturn.backward")
                        }
                    }

                    Divider()
                    Toggle("Live Preview", isOn: $livePreview)
                } label: {
                    Label("Options", systemImage: "ellipsis.circle")
                }
                .accessibilityIdentifier("workspaceOptionsButton")
            }
        }
        .alert("Sign Shortcut with HubSign?", isPresented: $showSigningConfirmation) {
            Button("Cancel", role: .cancel) {}
            Button("Send and Sign") {
                Task { await build(signed: true) }
            }
        } message: {
            Text("Signing sends the generated Shortcut plist to Cherri's existing HubSign service. Editing, Build, and live Preview stay on this device.")
        }
        .sheet(isPresented: $showDocumentation) {
            DocumentationSearchView()
        }
        .fileImporter(
            isPresented: $showShortcutImporter,
            allowedContentTypes: [.data],
            allowsMultipleSelection: false
        ) { result in
            switch result {
            case .success(let urls):
                guard let url = urls.first else { return }
                Task { await importShortcutPlist(from: url) }
            case .failure(let error):
                diagnostic = CompilationDiagnostic(message: error.localizedDescription, line: 1, column: 1)
            }
        }
        .task(id: CherriLivePreviewRequest(request: currentBuildRequest, isApplyingPreviewEdit: isApplyingPreviewEdit)) {
            guard livePreview, !isImporting, !isApplyingPreviewEdit else { return }
            // An empty document has no preview to render. Keep editor metadata
            // available without starting the compiler or WebKit during opening.
            guard !document.text.isEmpty else {
                compiled = nil
                diagnostic = nil
                diagnostics = []
                signedURL = nil
                if actionCatalog.isEmpty {
                    await refreshActionCatalog()
                }
                return
            }
            // A committed visual edit already rebuilt this exact request.
            // A changed source paused by that edit still resumes through task identity.
            guard compiled?.sourceRequest != currentBuildRequest else { return }
            try? await Task.sleep(for: .milliseconds(650))
            guard !Task.isCancelled else { return }
            await build(signed: false, liveBuild: true)
        }
    }

    private var isBusy: Bool {
        isCompiling || isSigning || isImporting || isApplyingPreviewEdit
    }

    private var editorPane: some View {
        CherriEditorView(
            text: $document.text,
            diagnostic: diagnostic,
            diagnostics: diagnostics,
            actions: actionCatalog,
            startupObservationID: document.startupObservationID
        )
    }

    private var previewPane: some View {
        VStack(spacing: 0) {
            if let compiled {
                ShortcutPreviewView(
                    plist: compiled.plist,
                    name: compiled.name,
                    compilationID: compiled.id,
                    actionMetadataJSON: previewActionMetadataJSON,
                    onEdit: { edit in
                        Task { @MainActor in
                            handlePreviewEdit(edit, rendered: compiled)
                        }
                    }
                )
                .allowsHitTesting(!isBusy)
            } else {
                // A placeholder must not instantiate an invisible WKWebView.
                // The editor remains the same sibling when compiled data arrives.
                ContentUnavailableView(
                    "No Preview Yet",
                    systemImage: "wand.and.stars",
                    description: Text("Build the Cherri source to render the Shortcut preview.")
                )
                .allowsHitTesting(false)
            }

            if compiled != nil {
                HStack(spacing: 8) {
                    Button("Edit Preview Param") {
                        handleShowResultPreviewEdit()
                    }
                    .font(.caption)
                    .buttonStyle(.borderedProminent)
                    .accessibilityIdentifier("previewParamEditButton")

                    Button("Edit Invalid Param") {
                        handlePreviewEdit(.actionParameter(actionIndex: 99, key: "invalid", value: .string("ShouldReject")))
                    }
                    .font(.caption)
                    .buttonStyle(.bordered)
                    .accessibilityIdentifier("previewParamInvalidEditButton")
                }
                .padding(.horizontal, 10)
                .padding(.vertical, 4)
                .background(.bar)
            }
        }
    }

    @ViewBuilder
    private var statusBar: some View {
        HStack(spacing: 8) {
            if isApplyingPreviewEdit {
                ProgressView()
                    .controlSize(.small)
                Text("Syncing visual edit to Cherri…")
            } else if isImporting {
                ProgressView()
                    .controlSize(.small)
                Text("Decompiling Shortcut…")
            } else if isSigning {
                ProgressView()
                    .controlSize(.small)
                Text("Signing Shortcut…")
            } else if isCompiling {
                ProgressView()
                    .controlSize(.small)
                Text("Compiling…")
            } else if let diagnostic {
                Image(systemName: "xmark.circle.fill")
                    .foregroundStyle(.red)
                Text("\(diagnostic.message) (\(diagnostic.locationDescription))")
                    .accessibilityIdentifier("diagnosticStatusText")
                    .lineLimit(2)
            } else if compiled != nil {
                Image(systemName: "checkmark.circle.fill")
                    .foregroundStyle(.green)
                Text("Compiled · \(actionCatalog.count) actions · Preview editable")
                    .accessibilityIdentifier("compiledStatusText")
            } else {
                Text("Ready")
                    .accessibilityIdentifier("readyStatusText")
            }

            Spacer()
        }
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("statusBar")
        .font(.footnote)
        .padding(.horizontal, 12)
        .padding(.vertical, 7)
        .background(.bar)
    }

    private var fileDisplayName: String {
        fileURL?.deletingPathExtension().lastPathComponent ?? "Shortcut"
    }

    private var currentBuildRequest: CherriEditorBuildRequest {
        CherriEditorBuildRequest(source: document.text, mode: languageMode.rawValue, name: fileDisplayName)
    }

    @MainActor
    private func build(signed: Bool, liveBuild: Bool = false) async {
        guard !Task.isCancelled, !isApplyingPreviewEdit else { return }
        buildRequestGeneration += 1
        let generation = buildRequestGeneration
        let request = CherriEditorBuildRequest(
            source: document.text, mode: languageMode.rawValue, name: fileDisplayName
        )

        // Only the current request owns publication and the busy indicator.
        // Manual builds use the same snapshot boundary as live preview.
        isSigning = signed
        isCompiling = !signed
        defer {
            if generation == buildRequestGeneration {
                isSigning = false
                isCompiling = false
            }
        }

        do {
            let result = try await CherriCompiler.compile(
                source: request.source,
                name: request.name,
                mode: request.mode,
                signed: signed
            )

            guard !Task.isCancelled, generation == buildRequestGeneration,
                  request.matches(source: document.text, mode: languageMode.rawValue, name: fileDisplayName)
            else { return }

            compiled = result
            diagnostic = nil
            diagnostics = []
            signedURL = nil
            await refreshActionCatalog()

            guard !Task.isCancelled, generation == buildRequestGeneration,
                  request.matches(source: document.text, mode: languageMode.rawValue, name: fileDisplayName)
            else { return }
            if let signedData = result.signedShortcut {
                signedURL = try writeSignedShortcut(signedData, name: result.name)
            }
        } catch let compilerError as CompilationDiagnostic {
            guard !Task.isCancelled, generation == buildRequestGeneration,
                  request.matches(source: document.text, mode: languageMode.rawValue, name: fileDisplayName)
            else { return }
            let analysis = try? await CherriCompiler.analyze(source: request.source)
            guard !Task.isCancelled, generation == buildRequestGeneration,
                  request.matches(source: document.text, mode: languageMode.rawValue, name: fileDisplayName)
            else { return }

            diagnostic = compilerError
            if let analysis, !analysis.valid && !analysis.diagnostics.isEmpty {
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
            guard !Task.isCancelled, generation == buildRequestGeneration,
                  request.matches(source: document.text, mode: languageMode.rawValue, name: fileDisplayName)
            else { return }
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

    @MainActor
    private func refreshActionCatalog() async {
        if let actions = try? await CherriCompiler.actionCatalog() {
            actionCatalog = actions
            previewActionMetadataJSON = Self.previewMetadataJSON(from: actions)
        }
    }

    // Compact shared metadata for preview-shortcut's generic fallback path:
    // titles plus catalog-derived parameter label maps (plist key -> name)
    // let unknown/new actions render meaningful cards without a second
    // hand-maintained preview database.
    static func previewMetadataJSON(from actions: [CherriActionInfo]) -> String? {
        var metadata: [String: [String: Any]] = [:]
        for action in actions {
            guard let identifier = action.shortcutIdentifier,
                  let title = action.title,
                  !title.isEmpty else { continue }

            var parameterLabels: [String: String] = [:]
            for parameter in action.parameters ?? [] {
                if let key = parameter.key, !key.isEmpty {
                    parameterLabels[key] = parameter.name
                }
            }

            var entry: [String: Any] = ["title": title]
            if !parameterLabels.isEmpty {
                entry["params"] = parameterLabels
            }
            metadata[identifier] = entry
        }
        guard !metadata.isEmpty,
              let data = try? JSONSerialization.data(withJSONObject: metadata) else { return nil }
        return String(data: data, encoding: .utf8)
    }

    @MainActor
    private func handleShowResultPreviewEdit() {
        guard let current = compiled,
              let index = current.previewSourceMap?.uniqueParameterIndex(
                actionIdentifier: "is.workflow.actions.showresult", wireKey: "Text"
              ) else {
            diagnostic = CompilationDiagnostic(message: "This preview has no unique editable Show Result parameter.", line: 1, column: 1)
            return
        }
        handlePreviewEdit(
            .actionParameter(actionIndex: index, key: "Text", value: .string("VisualParamUpdated")),
            rendered: current
        )
    }

    @MainActor
    private func handlePreviewEdit(_ edit: ShortcutPreviewEdit, rendered: CompiledShortcut? = nil) {
        guard !isBusy, let current = compiled else { return }
        let original = rendered ?? current
        guard original.id == current.id, original.sourceRequest == currentBuildRequest else {
            diagnostic = CompilationDiagnostic(message: "The preview is out of date. Build the current source before editing it.", line: 1, column: 1)
            return
        }

        previewEditGeneration += 1
        buildRequestGeneration += 1
        let transaction = CherriPreviewEditTransaction(
            original: original, buildGeneration: buildRequestGeneration, editGeneration: previewEditGeneration
        )
        // Claim the edit before scheduling work. The old source, plist and
        // signed export remain committed while mapping/rebuilding is in flight.
        isApplyingPreviewEdit = true
        Task { @MainActor in
            await syncPreviewEditToSource(edit, transaction: transaction)
        }
    }

    @MainActor
    private func previewEditIsCurrent(_ transaction: CherriPreviewEditTransaction) -> Bool {
        !isImporting && transaction.matches(
            compiled: compiled, request: currentBuildRequest,
            buildGeneration: buildRequestGeneration, editGeneration: previewEditGeneration
        )
    }

    @MainActor
    private func syncPreviewEditToSource(_ edit: ShortcutPreviewEdit, transaction: CherriPreviewEditTransaction) async {
        defer {
            if transaction.editGeneration == previewEditGeneration {
                isApplyingPreviewEdit = false
            }
        }

        do {
            let published = try await transaction.perform(
                edit, operations: .live,
                isCurrent: { previewEditIsCurrent(transaction) },
                publish: { source, rebuilt in
                    // This synchronous MainActor block is the only publication.
                    if sourceBeforePreviewEdits == nil {
                        sourceBeforePreviewEdits = transaction.original.sourceRequest.source
                    }
                    document.text = source
                    compiled = rebuilt
                    diagnostic = nil
                    diagnostics = []
                    signedURL = nil
                }
            )
            if published { await refreshActionCatalog() }
        } catch let compilerError as CompilationDiagnostic {
            guard !Task.isCancelled, previewEditIsCurrent(transaction) else { return }
            diagnostic = compilerError
        } catch {
            guard !Task.isCancelled, previewEditIsCurrent(transaction) else { return }
            diagnostic = CompilationDiagnostic(message: error.localizedDescription, line: 1, column: 1)
        }
    }

    @MainActor
    private func restoreSourceBeforePreviewEdits() {
        guard let source = sourceBeforePreviewEdits else { return }
        previewEditGeneration += 1
        isApplyingPreviewEdit = false
        sourceBeforePreviewEdits = nil
        document.text = source
        compiled = nil
        diagnostic = nil
        signedURL = nil
    }

    @MainActor
    private func importShortcutPlist(from url: URL) async {
        isImporting = true
        defer { isImporting = false }

        let accessed = url.startAccessingSecurityScopedResource()
        defer {
            if accessed {
                url.stopAccessingSecurityScopedResource()
            }
        }

        do {
            let data = try Data(contentsOf: url)
            let name = url.deletingPathExtension().lastPathComponent
            let source = try await CherriCompiler.decompile(plist: data, name: name)

            document.text = source
            compiled = nil
            diagnostic = nil
            signedURL = nil
            sourceBeforePreviewEdits = nil
            selectedPane = .code
            await refreshActionCatalog()
        } catch let compilerError as CompilationDiagnostic {
            diagnostic = compilerError
        } catch {
            diagnostic = CompilationDiagnostic(message: error.localizedDescription, line: 1, column: 1)
        }
    }

    private func writeSignedShortcut(_ data: Data, name: String) throws -> URL {
        let directory = FileManager.default.temporaryDirectory
            .appendingPathComponent("CherriExports", isDirectory: true)
        try FileManager.default.createDirectory(
            at: directory,
            withIntermediateDirectories: true
        )

        let safeName = name
            .replacingOccurrences(of: "/", with: "-")
            .replacingOccurrences(of: ":", with: "-")
        let url = directory.appendingPathComponent(safeName).appendingPathExtension("shortcut")
        try data.write(to: url, options: .atomic)
        return url
    }
}
