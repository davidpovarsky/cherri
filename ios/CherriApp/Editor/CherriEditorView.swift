import CodeEditorView
import Foundation
import LanguageSupport
import SwiftUI

struct CherriEditorView: View {
    @Binding var text: String
    let diagnostic: CompilationDiagnostic?
    var diagnostics: [CompilationDiagnostic] = []
    let actions: [CherriActionInfo]

    @Environment(\.colorScheme) private var colorScheme
    @State private var editPosition = CodeEditor.Position()
    @State private var messages: Set<TextLocated<Message>> = []
    @State private var showActionPalette = false
    @State private var pendingPaletteAction: CherriActionInfo?
    @State private var paletteInsertionRange: NSRange?
    @State private var contextualCompletions: [CherriCompletionItem] = []
    @State private var hoverText: String?
    @State private var signatureText: String?
    @State private var definitionText: String?
    @State private var showRenameAlert = false
    @State private var renameNewName = ""
    @State private var renameTargetPosition: (line: Int, col: Int)?

    var body: some View {
        VStack(spacing: 0) {
            if !diagnostics.isEmpty {
                diagnosticsBanner
            }
            if let hover = hoverText {
                HStack {
                    Text(hover)
                        .font(.caption)
                        .accessibilityIdentifier("hoverContent")
                    Spacer()
                    Button("Dismiss") { hoverText = nil }
                        .font(.caption2)
                        .accessibilityIdentifier("dismissHoverButton")
                }
                .padding(.horizontal, 10)
                .padding(.vertical, 4)
                .background(Color.blue.opacity(0.12))
            }
            if let sig = signatureText {
                HStack {
                    Text(sig)
                        .font(.caption)
                        .accessibilityIdentifier("signatureContent")
                    Spacer()
                    Button("Dismiss") { signatureText = nil }
                        .font(.caption2)
                        .accessibilityIdentifier("dismissSignatureButton")
                }
                .padding(.horizontal, 10)
                .padding(.vertical, 4)
                .background(Color.purple.opacity(0.12))
            }
            if let def = definitionText {
                HStack {
                    Text(def)
                        .font(.caption)
                        .accessibilityIdentifier("definitionContent")
                    Spacer()
                    Button("Dismiss") { definitionText = nil }
                        .font(.caption2)
                        .accessibilityIdentifier("dismissDefinitionButton")
                }
                .padding(.horizontal, 10)
                .padding(.vertical, 4)
                .background(Color.green.opacity(0.12))
            }

            CodeEditor(
                text: $text,
                position: $editPosition,
                messages: $messages,
                language: .cherri(),
                layout: CodeEditor.LayoutConfiguration(showMinimap: false, wrapText: true)
            )
            .accessibilityIdentifier("cherriCodeEditor")
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
        .task(id: diagnostics.map(\.id)) {
            refreshDiagnostic()
        }
        .task(id: editPosition.selections.first) {
            await refreshContextualCompletions()
        }
        .sheet(isPresented: $showActionPalette, onDismiss: commitPaletteSelection) {
            ActionPaletteView(actions: actions) { action in
                pendingPaletteAction = action
            }
        }
        .alert("Rename Symbol", isPresented: $showRenameAlert) {
            TextField("New Name", text: $renameNewName)
                .accessibilityIdentifier("renameTextField")
            Button("Rename") {
                if let pos = renameTargetPosition, !renameNewName.isEmpty {
                    Task {
                        if let resp = try? await CherriCompiler.rename(source: text, line: pos.line, column: pos.col, newName: renameNewName), resp.ok, let newSource = resp.source {
                            text = newSource
                        }
                    }
                }
            }
            .accessibilityIdentifier("confirmRenameButton")
            Button("Cancel", role: .cancel) {}
        }
    }

    private var diagnosticsBanner: some View {
        VStack(alignment: .leading, spacing: 2) {
            ForEach(Array(diagnostics.prefix(4).enumerated()), id: \.offset) { idx, diag in
                HStack {
                    Image(systemName: "exclamationmark.triangle.fill")
                        .foregroundStyle(.red)
                        .font(.caption2)
                    Text("L\(diag.line):C\(diag.column) (len:\(diag.length ?? 1)): \(diag.message)")
                        .font(.caption2)
                        .lineLimit(1)
                        .accessibilityIdentifier("diagnosticItem_\(idx)")
                }
            }
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 4)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Color.red.opacity(0.1))
        .accessibilityIdentifier("diagnosticsBanner")
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
            .accessibilityIdentifier("actionsButton")
            .buttonStyle(.borderless)
            .disabled(actions.isEmpty)

            Divider()
                .frame(height: 20)

            Button {
                let sel = editPosition.selections.first ?? NSRange(location: 0, length: 0)
                let (line, col) = lineAndColumn(at: sel.location)
                Task {
                    if let resp = try? await CherriCompiler.hover(source: text, line: line, column: col), let contents = resp.contents {
                        hoverText = contents
                    } else {
                        hoverText = "No hover info at line \(line):\(col)"
                    }
                }
            } label: {
                Image(systemName: "info.circle")
            }
            .accessibilityIdentifier("hoverButton")
            .buttonStyle(.borderless)

            Button {
                let sel = editPosition.selections.first ?? NSRange(location: 0, length: 0)
                let (line, col) = lineAndColumn(at: sel.location)
                Task {
                    if let resp = try? await CherriCompiler.signatureHelp(source: text, line: line, column: col), let label = resp.label {
                        signatureText = label
                    } else {
                        signatureText = "No signature at line \(line):\(col)"
                    }
                }
            } label: {
                Image(systemName: "function")
            }
            .accessibilityIdentifier("signatureButton")
            .buttonStyle(.borderless)

            Button {
                let sel = editPosition.selections.first ?? NSRange(location: 0, length: 0)
                let (line, col) = lineAndColumn(at: sel.location)
                Task {
                    if let resp = try? await CherriCompiler.definition(source: text, line: line, column: col), let range = resp.range {
                        definitionText = "Definition at \(range.start.line + 1):\(range.start.character + 1)"
                    } else {
                        definitionText = "No definition found"
                    }
                }
            } label: {
                Image(systemName: "arrow.up.forward.app")
            }
            .accessibilityIdentifier("definitionButton")
            .buttonStyle(.borderless)

            Button {
                let sel = editPosition.selections.first ?? NSRange(location: 0, length: 0)
                renameTargetPosition = lineAndColumn(at: sel.location)
                renameNewName = ""
                showRenameAlert = true
            } label: {
                Image(systemName: "pencil.and.outline")
            }
            .accessibilityIdentifier("renameButton")
            .buttonStyle(.borderless)

            if !contextualCompletions.isEmpty {
                Divider()
                    .frame(height: 24)

                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 6) {
                        ForEach(contextualCompletions.prefix(8)) { item in
                            Button {
                                insertItem(item, replacing: completionContext?.range)
                            } label: {
                                VStack(alignment: .leading, spacing: 1) {
                                    Text(item.label)
                                        .font(.system(.caption, design: .monospaced, weight: .semibold))
                                    if let detail = item.detail, !detail.isEmpty {
                                        Text(detail)
                                            .font(.caption2)
                                            .foregroundStyle(.secondary)
                                    }
                                }
                                .padding(.horizontal, 8)
                                .padding(.vertical, 5)
                            }
                            .accessibilityIdentifier("completionItem_\(item.label)")
                            .buttonStyle(.bordered)
                        }
                    }
                }
                .accessibilityIdentifier("contextualCompletionsList")
            } else if !completionSuggestions.isEmpty {
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
                            .accessibilityIdentifier("suggestionItem_\(action.name)")
                            .buttonStyle(.bordered)
                        }
                    }
                }
                .accessibilityIdentifier("suggestionsList")
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
        guard let scalar = UnicodeScalar(value) else { return false }
        return scalar == "_" || CharacterSet.letters.contains(scalar) || CharacterSet.decimalDigits.contains(scalar)
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
        text = source.replacingCharacters(in: range, with: action.insertionSnippet)
    }

    private func insertItem(_ item: CherriCompletionItem, replacing requestedRange: NSRange?) {
        let source = text as NSString
        let selection = editPosition.selections.first ?? NSRange(location: source.length, length: 0)
        let candidate = requestedRange ?? selection
        let range: NSRange

        if candidate.location >= 0 && candidate.location + candidate.length <= source.length {
            range = candidate
        } else {
            range = NSRange(location: source.length, length: 0)
        }

        let replacement = item.insertText ?? item.label
        text = source.replacingCharacters(in: range, with: replacement)
    }

    private func lineAndColumn(at offset: Int) -> (line: Int, column: Int) {
        let ns = text as NSString
        let bounded = min(max(offset, 0), ns.length)
        var line = 1
        var col = 1
        for i in 0..<bounded {
            if ns.character(at: i) == 10 { // newline
                line += 1
                col = 1
            } else {
                col += 1
            }
        }
        return (line, col)
    }

    private func refreshContextualCompletions() async {
        guard let selection = editPosition.selections.first else {
            contextualCompletions = []
            return
        }
        let (line, col) = lineAndColumn(at: selection.location)
        if let items = try? await CherriCompiler.complete(source: text, line: line, column: col), !items.isEmpty {
            contextualCompletions = items
        } else {
            contextualCompletions = []
        }
    }

    private func refreshDiagnostic() {
        messages.removeAll()
        let allDiags = diagnostics.isEmpty ? (diagnostic.map { [$0] } ?? []) : diagnostics
        for diag in allDiags {
            let msgLength = max(1, diag.length ?? 1)
            messages.insert(
                TextLocated(
                    location: TextLocation(
                        oneBasedLine: max(diag.line, 1),
                        column: max(diag.column, 1)
                    ),
                    entity: Message(
                        category: .error,
                        length: msgLength,
                        summary: "Error",
                        description: AttributedString(diag.message)
                    )
                )
            )
        }
    }
}
