import SwiftUI

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
    @State private var isCompiling = false
    @State private var isSigning = false
    @State private var signedURL: URL?

    var body: some View {
        NavigationStack {
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
            .navigationTitle(fileDisplayName)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItemGroup(placement: .topBarTrailing) {
                    Button {
                        Task { await build(signed: false) }
                    } label: {
                        Label("Build", systemImage: "hammer.fill")
                    }
                    .disabled(isCompiling || isSigning)

                    Button {
                        Task { await build(signed: true) }
                    } label: {
                        Label("Sign", systemImage: "checkmark.seal.fill")
                    }
                    .disabled(isCompiling || isSigning)

                    if let signedURL {
                        ShareLink(item: signedURL) {
                            Label("Share", systemImage: "square.and.arrow.up")
                        }
                    }

                    Menu {
                        Toggle("Live Preview", isOn: $livePreview)
                    } label: {
                        Label("Options", systemImage: "ellipsis.circle")
                    }
                }
            }
        }
        .task(id: document.text) {
            guard livePreview else { return }
            try? await Task.sleep(for: .milliseconds(650))
            guard !Task.isCancelled else { return }
            await build(signed: false, liveBuild: true)
        }
    }

    private var editorPane: some View {
        CherriEditorView(text: $document.text, diagnostic: diagnostic)
    }

    private var previewPane: some View {
        ShortcutPreviewView(
            plist: compiled?.plist,
            name: compiled?.name ?? fileDisplayName
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
            if isSigning {
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
                Text("Compiled")
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
