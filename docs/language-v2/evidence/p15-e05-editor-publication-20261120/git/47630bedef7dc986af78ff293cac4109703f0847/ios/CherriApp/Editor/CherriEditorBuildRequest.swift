import Foundation

/// Immutable source context shared by the live task identity and build
/// publication guard. Editor offsets use UTF-16, so canonically equivalent
/// Swift strings are not interchangeable snapshots.
struct CherriEditorBuildRequest: Equatable {
    let source: String
    let mode: String
    let name: String

    static func == (lhs: Self, rhs: Self) -> Bool {
        lhs.source.utf16.elementsEqual(rhs.source.utf16)
            && lhs.mode == rhs.mode
            && lhs.name.utf16.elementsEqual(rhs.name.utf16)
    }

    func matches(source: String, mode: String, name: String) -> Bool {
        self == Self(source: source, mode: mode, name: name)
    }
}
