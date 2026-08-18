import CodeEditorView
import Foundation
import LanguageSupport
import SwiftUI

struct CherriEditorView: View {
    @Binding var text: String
    let diagnostic: CompilationDiagnostic?
    let actions: [CherriActionInfo]

    @Environment(\.colorScheme) private var colorScheme
    @State private var editPosition = CodeEditor.Position()
    @State private var messages: Set<TextLocated<Message>> = []
    @State private var showActionPalette = false
    @State private var pendingPaletteAction: CherriActionInfo?
    @State private var paletteInsertionRange: NSRange?

    var body: some View {
        VStack(spacing: 0) {
            CodeEditor(
                text: $text,
                position: $editPosition,
                messages: $messages,
                language: .cherri(),
                layout: CodeEditor.LayoutConfiguration(showMinimap: false, wrapText: true)
            )
            .environment(
                \.codeEditorTheme,
                colorScheme == .dark ? Theme.defaultDark : Theme.defaultLight
            )

            Divider()
            completionBar
        }
        .task(id: diagnostic?.id) {
            refreshDiagnostic()
        }
        .sheet(isPresented: $showActionPalette, onDismiss: commitPaletteSelection) {
            ActionPaletteView(actions: actions) { action in
                pendingPaletteAction = action
            }
        }
    }

    private var completionBar: some View {
        HStack(spacing: 8) {
            Button {
                paletteInsertionRange = editPosition.selections.first
                pendingPaletteAction = nil
                showActionPalette = true
            } label: {
                Label("Actions", systemImage: "wand.and.stars")
                    .labelStyle(.titleAndIcon)
            }
            .buttonStyle(.borderless)
            .disabled(actions.isEmpty)

            if !completionSuggestions.isEmpty {
                Divider()
                    .frame(height: 24)

                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 6) {
                        ForEach(completionSuggestions) { action in
                            Button {
                                insert(action: action, replacing: completionContext?.range)
                            } label: {
                                VStack(alignment: .leading, spacing: 1) {
                                    Text(action.name)
                                        .font(.system(.caption, design: .monospaced, weight: .semibold))
                                    if let title = action.title, !title.isEmpty, title != action.name {
                                        Text(title)
                                            .font(.caption2)
                                            .foregroundStyle(.secondary)
                                    }
                                }
                                .padding(.horizontal, 8)
                                .padding(.vertical, 5)
                            }
                            .buttonStyle(.bordered)
                        }
                    }
                }
            }

            Spacer(minLength: 0)
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 6)
        .background(.bar)
    }

    private struct CompletionContext {
        let range: NSRange
        let prefix: String
    }

    private var completionContext: CompletionContext? {
        guard let selection = editPosition.selections.first, selection.length == 0 else { return nil }

        let source = text as NSString
        let cursor = min(max(selection.location, 0), source.length)
        var start = cursor

        while start > 0 && isIdentifierUnit(source.character(at: start - 1)) {
            start -= 1
        }

        guard start < cursor else { return nil }
        if start > 0 {
            let previous = source.character(at: start - 1)
            if previous == 35 || previous == 64 || previous == 34 || previous == 39 { // # @ " '
                return nil
            }
        }

        let range = NSRange(location: start, length: cursor - start)
        return CompletionContext(range: range, prefix: source.substring(with: range))
    }

    private var completionSuggestions: [CherriActionInfo] {
        guard let context = completionContext else { return [] }
        let prefix = context.prefix.lowercased()
        guard !prefix.isEmpty else { return [] }

        return Array(
            actions.lazy
                .filter { $0.name.lowercased().hasPrefix(prefix) }
                .prefix(6)
        )
    }

    private func isIdentifierUnit(_ value: unichar) -> Bool {
        (value >= 65 && value <= 90)
            || (value >= 97 && value <= 122)
            || (value >= 48 && value <= 57)
            || value == 95
    }

    private func commitPaletteSelection() {
        guard let action = pendingPaletteAction else {
            paletteInsertionRange = nil
            return
        }

        let insertionRange = paletteInsertionRange
        pendingPaletteAction = nil
        paletteInsertionRange = nil

        // Wait until the sheet has fully left the hierarchy before changing the
        // CodeEditor binding. This avoids updating its underlying UITextView while
        // UIKit is still completing the presentation transition.
        Task { @MainActor in
            try? await Task.sleep(for: .milliseconds(300))
            insert(action: action, replacing: insertionRange)
        }
    }

    private func insert(action: CherriActionInfo, replacing requestedRange: NSRange?) {
        let source = text as NSString
        let selection = editPosition.selections.first ?? NSRange(location: source.length, length: 0)
        let candidate = requestedRange ?? selection
        let range: NSRange

        if candidate.location >= 0 && candidate.location + candidate.length <= source.length {
            range = candidate
        } else {
            range = NSRange(location: source.length, length: 0)
        }

        // Deliberately update only the text binding. CodeEditorView owns its UIKit
        // selection lifecycle on iOS; programmatically replacing CodeEditor.Position
        // immediately after a text mutation can hand UITextView a range from the old
        // storage and crash the host process. The existing selection remains valid
        // because insertion only grows the document.
        let insertion = "\(action.name)()"
        text = source.replacingCharacters(in: range, with: insertion)
    }

    private func refreshDiagnostic() {
        messages.removeAll()
        guard let diagnostic else { return }

        messages.insert(
            TextLocated(
                location: TextLocation(
                    oneBasedLine: max(diagnostic.line, 1),
                    column: max(diagnostic.column, 1)
                ),
                entity: Message(
                    category: .error,
                    length: 1,
                    summary: "Error",
                    description: AttributedString(diagnostic.message)
                )
            )
        )
    }
}
