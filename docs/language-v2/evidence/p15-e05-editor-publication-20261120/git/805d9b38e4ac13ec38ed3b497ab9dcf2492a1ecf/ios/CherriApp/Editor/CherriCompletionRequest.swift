import Foundation

/// Completion results are valid only for the exact source and caret that
/// produced them. Swift String equality alone normalizes Unicode, whereas
/// editor offsets and compiler diagnostics are measured in UTF-16.
struct CherriCompletionRequest: Equatable {
    let source: String
    let selection: NSRange?

    static func == (lhs: Self, rhs: Self) -> Bool {
        lhs.selection == rhs.selection && lhs.source.utf16.elementsEqual(rhs.source.utf16)
    }

    /// The bridge returns the full contextual set. Filter before the UI limits
    /// the row, preserving parameter/variable/action priority from the service.
    func matchingItems(_ items: [CherriCompletionItem], prefix: String?) -> [CherriCompletionItem] {
        guard let prefix, !prefix.isEmpty else { return items }
        return items.filter { $0.label.lowercased().hasPrefix(prefix.lowercased()) }
    }

    var position: (line: Int, column: Int)? {
        guard let selection, selection.length == 0 else { return nil }
        let units = Array(source.utf16)
        guard selection.location >= 0, selection.location <= units.count else { return nil }
        var line = 1
        var column = 1
        for unit in units.prefix(selection.location) {
            if unit == 10 { line += 1; column = 1 } else { column += 1 }
        }
        return (line, column)
    }
}
