import CodeEditorView
import Foundation
import LanguageSupport
import SwiftUI

struct CherriEditorView: View {
    @Binding var text: String
    let diagnostic: CompilationDiagnostic?

    @Environment(\.colorScheme) private var colorScheme
    @State private var editPosition = CodeEditor.Position()
    @State private var messages: Set<TextLocated<Message>> = []

    var body: some View {
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
        .task(id: diagnostic?.id) {
            refreshDiagnostic()
        }
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
                    description: NSAttributedString(string: diagnostic.message)
                )
            )
        )
    }
}
