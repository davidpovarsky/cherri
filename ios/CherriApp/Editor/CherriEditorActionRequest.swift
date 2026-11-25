import Foundation

/// A response belongs to one exact document revision and one issued action.
/// Language analysis and edits remain in the shared compiler.
struct CherriEditorActionRequest: Equatable, Sendable {
    let documentID: UUID
    let revision: UInt64
    let sequence: UInt64
    let source: String

    static func == (lhs: Self, rhs: Self) -> Bool {
        lhs.documentID == rhs.documentID
            && lhs.revision == rhs.revision
            && lhs.sequence == rhs.sequence
            && lhs.source.utf16.elementsEqual(rhs.source.utf16)
    }
}
