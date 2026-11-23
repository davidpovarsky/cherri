import SwiftUI

struct DocumentationSearchView: View {
    @Environment(\.dismiss) private var dismiss
    @State private var query = ""
    @State private var includeMigration = false
    @State private var index: DocumentationIndex?
    @State private var loadError: String?

    private var results: [DocumentationSection] {
        index?.search(query, includeMigration: includeMigration) ?? []
    }

    var body: some View {
        NavigationStack {
            Group {
                if let index {
                    VStack(spacing: 0) {
                        HStack {
                            Text("Include migration reference")
                            Spacer()
                            // Keep the native switch's interaction area separate from the row label.
                            Toggle("Include migration reference", isOn: $includeMigration)
                                .labelsHidden()
                                .toggleStyle(.switch)
                                .fixedSize()
                                .accessibilityIdentifier("docsMigrationToggle")
                        }
                        .font(.subheadline)
                        .padding(.horizontal)
                        .padding(.vertical, 8)
                        List {
                            Section {
                                ForEach(results) { section in
                                    NavigationLink {
                                        DocumentationSectionView(section: section)
                                    } label: {
                                        VStack(alignment: .leading, spacing: 4) {
                                            Text(section.title)
                                            Text(section.documentTitle)
                                                .font(.caption)
                                                .foregroundStyle(.secondary)
                                        }
                                    }
                                    .accessibilityIdentifier("docsResult_" + section.id)
                                }
                            } footer: {
                                Text("Cherri \(index.languageVersion) documentation · Available offline")
                            }
                        }
                        .overlay {
                            if results.isEmpty {
                                ContentUnavailableView.search(text: query)
                                    .accessibilityIdentifier("docsNoResults")
                            }
                        }
                    }
                } else if let loadError {
                    ContentUnavailableView("Documentation Unavailable",
                                           systemImage: "doc.text.magnifyingglass",
                                           description: Text(loadError))
                        .accessibilityIdentifier("docsLoadError")
                } else {
                    ProgressView("Loading documentation…")
                }
            }
            .navigationTitle("Documentation")
            .navigationBarTitleDisplayMode(.inline)
            .searchable(text: $query, placement: .navigationBarDrawer(displayMode: .always),
                        prompt: "Search current documentation")
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { dismiss() }
                        .accessibilityIdentifier("docsDoneButton")
                }
            }
            // Keep the sheet's dismissal action available while search is active.
            .modifier(DocumentationSearchToolbarVisibility())
            .task {
                guard index == nil && loadError == nil else { return }
                do {
                    let loaded = try await Task.detached(priority: .userInitiated) {
                        try DocumentationIndex.load()
                    }.value
                    try Task.checkCancellation()
                    index = loaded
                } catch is CancellationError {
                    return
                } catch {
                    loadError = error.localizedDescription
                }
            }
        }
    }
}

private struct DocumentationSearchToolbarVisibility: ViewModifier {
    @ViewBuilder
    func body(content: Content) -> some View {
        if #available(iOS 17.1, *) {
            content.searchPresentationToolbarBehavior(.avoidHidingContent)
        } else {
            content
        }
    }
}

private struct DocumentationSectionView: View {
    let section: DocumentationSection

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                Text(section.documentTitle)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                Text(section.title)
                    .font(.title2.bold())
                    .accessibilityIdentifier("docsSectionTitle")
                if section.migrationOnly {
                    Label("Migration reference", systemImage: "arrow.triangle.branch")
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                }
                ForEach(Array(section.blocks.enumerated()), id: \.offset) { _, block in
                    if block.kind == "code" {
                        VStack(alignment: .leading, spacing: 6) {
                            if block.language == "cherri-legacy" {
                                Text("Legacy example · Migration only")
                                    .font(.caption.bold())
                            }
                            ScrollView(.horizontal) {
                                Text(verbatim: block.text)
                                    .font(.system(.body, design: .monospaced))
                                    .fixedSize(horizontal: true, vertical: false)
                                    .textSelection(.enabled)
                                    .padding(12)
                            }
                            .background(.quaternary, in: RoundedRectangle(cornerRadius: 10))
                        }
                    } else {
                        Text(prose(block.text))
                            .textSelection(.enabled)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }
                }
            }
            .padding()
        }
        .navigationTitle(section.title)
        .navigationBarTitleDisplayMode(.inline)
        .accessibilityIdentifier("docsSectionReader")
    }

    private func prose(_ text: String) -> AttributedString {
        (try? AttributedString(markdown: text,
                               options: .init(interpretedSyntax: .inlineOnlyPreservingWhitespace)))
            ?? AttributedString(text)
    }
}
