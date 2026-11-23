import Foundation

/// An edit belongs to the DOM render that produced it, not merely the latest
/// SwiftUI parent. A queued message cannot acquire a newer compilation context.
struct CherriPreviewRenderState {
    private var generation: UInt64 = 0
    private var pendingID: UUID?
    private var renderedID: UUID?

    func needsRender(_ id: UUID) -> Bool {
        id != pendingID && id != renderedID
    }

    mutating func begin(_ id: UUID) -> UInt64 {
        generation += 1
        pendingID = id
        renderedID = nil
        return generation
    }

    func isPending(_ id: UUID, generation: UInt64) -> Bool {
        self.generation == generation && pendingID == id
    }

    mutating func complete(_ id: UUID, generation: UInt64) {
        guard isPending(id, generation: generation) else { return }
        pendingID = nil
        renderedID = id
    }

    func accepts(_ messageID: UUID?, committedID: UUID) -> Bool {
        guard let messageID else { return false }
        return pendingID == nil && renderedID == messageID && messageID == committedID
    }

    mutating func invalidate() {
        generation += 1
        pendingID = nil
        renderedID = nil
    }
}
