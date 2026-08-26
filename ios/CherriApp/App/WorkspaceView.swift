import SwiftUI
import UniformTypeIdentifiers

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

    @State private var selectedPane: WorkspacePane = .code
    @State private var compiled: CompiledShortcut?
    @State private var diagnostic: CompilationDiagnostic?
    @State private var actionCatalog: [CherriActionInfo] = []
    @State private var previewActionMetadataJSON: String?
    @State private var isCompiling = false
    @State private var isSigning = false
    @State private var isImporting = false
    @State private var isApplyingPreviewEdit = false
    @State private var previewEditGeneration = 0
    @State private var sourceBeforePreviewEdits: String?
    @State private var signedURL: URL?
    @State private var showSigningConfirmation = false
    @State private var showShortcutImporter = false

    var body: some View {
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
        .toolbar {
            ToolbarItemGroup(placement: .topBarTrailing) {
                Button {
                    Task { await build(signed: false) }
                } label: {
                    Label("Build", systemImage: "hammer.fill")
                }
                .disabled(isBusy)

                Button {
                    showSigningConfirmation = true
                } label: {
                    Label("Sign", systemImage: "checkmark.seal.fill")
                }
                .disabled(isBusy)

                if let signedURL {
                    ShareLink(item: signedURL) {
                        Label("Share", systemImage: "square.and.arrow.up")
                    }
                }

                Menu {
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
        .task(id: document.text) {
            guard livePreview, !isImporting, !isApplyingPreviewEdit else { return }
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
            actions: actionCatalog
        )
    }

    private var previewPane: some View {
        ShortcutPreviewView(
            plist: compiled?.plist,
            name: compiled?.name ?? fileDisplayName,
            actionMetadataJSON: previewActionMetadataJSON,
            onEdit: { edit in
                Task { @MainActor in
                    handlePreviewEdit(edit)
                }
            }
        )
        .overlay {
            if compiled == nil && diagnostic == nil && !isCompiling {
                ContentUnavailableView(
                    "No Preview Yet",
                    systemImage: "wand.and.stars",
                    description: Text("Build the Cherri source to render the Shortcut preview.")
                )
                .allowsHitTesting(false)
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
                    .lineLimit(2)
            } else if compiled != nil {
                Image(systemName: "checkmark.circle.fill")
                    .foregroundStyle(.green)
                Text("Compiled · \(actionCatalog.count) actions · Preview editable")
            } else {
                Text("Ready")
            }

            Spacer()
        }
        .font(.footnote)
        .padding(.horizontal, 12)
        .padding(.vertical, 7)
        .background(.bar)
    }

    private var fileDisplayName: String {
        fileURL?.deletingPathExtension().lastPathComponent ?? "Shortcut"
    }

    @MainActor
    private func build(signed: Bool, liveBuild: Bool = false) async {
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
                signed: signed
            )

            if liveBuild && source != document.text {
                return
            }

            compiled = result
            diagnostic = nil
            await refreshActionCatalog()

            if let signedData = result.signedShortcut {
                signedURL = try writeSignedShortcut(signedData, name: result.name)
            }
        } catch let compilerError as CompilationDiagnostic {
            if liveBuild && source != document.text {
                return
            }
            diagnostic = compilerError
            signedURL = nil
        } catch {
            diagnostic = CompilationDiagnostic(
                message: error.localizedDescription,
                line: 1,
                column: 1
            )
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
    private func handlePreviewEdit(_ edit: ShortcutPreviewEdit) {
        guard let current = compiled else { return }

        do {
            let editedPlist = try ShortcutPlistEditor.applying(edit, to: current.plist)
            if sourceBeforePreviewEdits == nil {
                sourceBeforePreviewEdits = document.text
            }

            previewEditGeneration += 1
            let generation = previewEditGeneration
            let name = current.name

            // Update the visible preview immediately. Decompile and recompile in
            // the background to canonicalize the corresponding Cherri source.
            compiled = CompiledShortcut(name: name, plist: editedPlist, signedShortcut: nil)
            signedURL = nil

            Task { @MainActor in
                await syncPreviewEditToSource(plist: editedPlist, name: name, generation: generation)
            }
        } catch {
            diagnostic = CompilationDiagnostic(message: error.localizedDescription, line: 1, column: 1)
        }
    }

    @MainActor
    private func syncPreviewEditToSource(plist: Data, name: String, generation: Int) async {
        isApplyingPreviewEdit = true
        defer {
            if generation == previewEditGeneration {
                isApplyingPreviewEdit = false
            }
        }

        do {
            let source = try await CherriCompiler.decompile(plist: plist, name: name)
            let rebuilt = try await CherriCompiler.compile(source: source, name: name)
            guard generation == previewEditGeneration else { return }

            document.text = source
            compiled = rebuilt
            diagnostic = nil
            signedURL = nil
            await refreshActionCatalog()
        } catch let compilerError as CompilationDiagnostic {
            guard generation == previewEditGeneration else { return }
            diagnostic = compilerError
        } catch {
            guard generation == previewEditGeneration else { return }
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
