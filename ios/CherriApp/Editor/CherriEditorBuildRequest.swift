import Foundation

/// Immutable source context shared by the live task identity and build
/// publication guard. Editor offsets use UTF-16, so canonically equivalent
/// Swift strings are not interchangeable snapshots.
struct CherriEditorBuildRequest: Equatable, Sendable {
    let source: String
    let mode: String
    let name: String
    let buildProfile: CherriBuildProfile

    init(source: String, mode: String, name: String, buildProfile: CherriBuildProfile = .debug) {
        self.source = source
        self.mode = mode
        self.name = name
        self.buildProfile = buildProfile
    }

    static func == (lhs: Self, rhs: Self) -> Bool {
        lhs.source.utf16.elementsEqual(rhs.source.utf16)
            && lhs.mode == rhs.mode
            && lhs.name.utf16.elementsEqual(rhs.name.utf16)
            && lhs.buildProfile == rhs.buildProfile
    }

    func matches(source: String, mode: String, name: String, buildProfile: CherriBuildProfile = .debug) -> Bool {
        self == Self(source: source, mode: mode, name: name, buildProfile: buildProfile)
    }
}

/// Resuming an edit must restart a source-triggered live task that paused while
/// the mapping/rebuild was in flight. The build request still owns staleness.
struct CherriLivePreviewRequest: Equatable {
    let request: CherriEditorBuildRequest
    let isApplyingPreviewEdit: Bool
}
