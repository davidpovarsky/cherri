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

    /// The shared language service owns the complete replacement span. This
    /// method translates its zero-based UTF-16 coordinates; it does not lex a
    /// prefix or fall back to catalog insertion when a response is incomplete.
    func applying(_ item: CherriCompletionItem, to current: Self) -> String? {
        guard self == current, let selection, selection.length == 0,
              let edit = item.textEdit else { return nil }
        let units = Array(source.utf16)
        let starts = [0] + units.indices.filter { units[$0] == 10 }.map { $0 + 1 }

        func offset(_ position: CherriWirePosition) -> Int? {
            guard position.line >= 0, position.line < starts.count,
                  position.character >= 0 else { return nil }
            let start = starts[position.line]
            var end = position.line + 1 < starts.count ? starts[position.line + 1] - 1 : units.count
            if end > start && units[end - 1] == 13 { end -= 1 }
            guard position.character <= end - start else { return nil }
            let value = start + position.character
            // UTF-16 offsets may occur between combining marks but must never
            // split a surrogate pair into an invalid string.
            if value > 0 && value < units.count,
               (0xD800...0xDBFF).contains(units[value - 1]),
               (0xDC00...0xDFFF).contains(units[value]) {
                return nil
            }
            return value
        }

        guard let start = offset(edit.range.start), let end = offset(edit.range.end),
              start <= end, selection.location >= start, selection.location <= end
        else { return nil }
        return (source as NSString).replacingCharacters(
            in: NSRange(location: start, length: end - start), with: edit.newText
        )
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
