import Foundation
import Combine

/// Observes the selected document's existing project directory. Notifications
/// invalidate the shared Go cache and the editor snapshot together. The entry
/// file is excluded: editing it must not reread an unchanged large asset.
@MainActor
final class CherriProjectAssetObserver: ObservableObject {
    @Published private(set) var revision: UInt64 = 0
    @Published private(set) var entryNotifications: UInt64 = 0
    @Published private(set) var assetNotifications: UInt64 = 0
    private(set) var isReady = false
    private var selectedURI: String?
    private var generation = UUID()
    private var presenter: Presenter?
    private var registration: Task<Void, Never>?
    private var invalidation: Task<Void, Never>?
    private var failure: String?

    func activate(fileURL: URL?) {
        let uri = fileURL?.absoluteString ?? ""
        if let selectedURI, selectedURI.utf16.elementsEqual(uri.utf16) { return }
        stop()
        selectedURI = uri
        failure = nil
        guard let fileURL else {
            isReady = true
            return
        }
        guard fileURL.isFileURL else {
            failure = "The selected document must be a local file URL."
            return
        }
        let token = generation
        let presenter = Presenter(entryURL: fileURL) { [weak self] isEntryChange in
            Task { @MainActor [weak self] in
                guard let self, self.generation == token else { return }
                if isEntryChange {
                    self.entryNotifications &+= 1
                } else {
                    self.assetNotifications &+= 1
                    self.invalidate()
                }
            }
        }
        self.presenter = presenter
        registration = Task { @MainActor [weak self] in
            do {
                try await Task.detached(priority: .utility) {
                    var error: NSError?
                    NSFileCoordinator(filePresenter: nil).coordinate(readingItemAt: presenter.rootURL, options: [],
                                                   error: &error) { _ in
                        // Register while the coordinated read is active so an
                        // already-running writer cannot be missed at startup.
                        NSFileCoordinator.addFilePresenter(presenter)
                    }
                    if let error { throw error }
                }.value
                guard let self, self.generation == token, !Task.isCancelled else {
                    presenter.finish()
                    return
                }
                self.invalidate()
                await self.invalidation?.value
                guard self.generation == token, !Task.isCancelled else { return }
                self.isReady = self.failure == nil
            } catch {
                presenter.finish()
                guard let self, self.generation == token else { return }
                self.failure = error.localizedDescription
                self.isReady = false
            }
        }
    }

    /// A compile waits for actual registration/cache invalidation, then captures
    /// its request. Every later revision still rejects stale publication.
    func prepare(fileURL: URL?) async throws {
        activate(fileURL: fileURL)
        let token = generation
        await registration?.value
        while true {
            let observedRevision = revision
            await invalidation?.value
            try Task.checkCancellation()
            if observedRevision == revision { break }
        }
        try Task.checkCancellation()
        guard token == generation else { throw CancellationError() }
        if let failure {
            throw CompilationDiagnostic(message: "Project observation failed: \(failure)", line: 1, column: 1)
        }
        guard isReady else { throw CancellationError() }
    }

    func stop() {
        generation = UUID()
        revision &+= 1
        selectedURI = nil
        isReady = false
        registration?.cancel()
        registration = nil
        presenter?.finish()
        presenter = nil
    }

    private func invalidate() {
        revision &+= 1
        let prior = invalidation
        let token = generation
        invalidation = Task { @MainActor [weak self] in
            await prior?.value
            do {
                _ = try await CherriCompiler.invalidateAssetCache()
            } catch {
                guard let self, self.generation == token else { return }
                self.failure = error.localizedDescription
                self.isReady = false
            }
        }
    }

    deinit {
        registration?.cancel()
        presenter?.finish()
    }

    /// NSFilePresenter callbacks are delivered on this serial queue. Only the
    /// MainActor owner mutates revision or UI state. Security scope is balanced
    /// even if registration completes after the editor has disappeared.
    private final class Presenter: NSObject, NSFilePresenter, @unchecked Sendable {
        let rootURL: URL
        private let entryURL: URL
        private let securityURL: URL
        private let changed: @Sendable (Bool) -> Void
        private let accessLock = NSLock()
        private var accessed: Bool
        var presentedItemURL: URL? { rootURL }
        let presentedItemOperationQueue: OperationQueue = {
            let queue = OperationQueue()
            queue.name = "org.cherrilang.project-assets"
            queue.qualityOfService = .utility
            queue.maxConcurrentOperationCount = 1
            return queue
        }()

        init(entryURL: URL, changed: @escaping @Sendable (Bool) -> Void) {
            securityURL = entryURL
            self.entryURL = entryURL.standardizedFileURL
            rootURL = entryURL.deletingLastPathComponent().standardizedFileURL
            self.changed = changed
            accessed = entryURL.startAccessingSecurityScopedResource()
            super.init()
        }

        func finish() {
            NSFileCoordinator.removeFilePresenter(self)
            accessLock.lock()
            let release = accessed
            accessed = false
            accessLock.unlock()
            if release { securityURL.stopAccessingSecurityScopedResource() }
        }

        private func subitemChanged(_ url: URL) {
            let isEntry = url.standardizedFileURL.absoluteString.utf16.elementsEqual(entryURL.absoluteString.utf16)
            changed(isEntry)
        }

        func presentedItemDidChange() { changed(false) }
        func presentedItemDidMove(to newURL: URL) { changed(false) }
        func accommodatePresentedItemDeletion(completionHandler: @escaping (Error?) -> Void) {
            changed(false)
            completionHandler(nil)
        }
        func presentedSubitemDidAppear(at url: URL) { subitemChanged(url) }
        func presentedSubitemDidChange(at url: URL) { subitemChanged(url) }
        func presentedSubitem(at oldURL: URL, didMoveTo newURL: URL) {
            subitemChanged(oldURL)
            subitemChanged(newURL)
        }
        func accommodatePresentedSubitemDeletion(at url: URL, completionHandler: @escaping (Error?) -> Void) {
            subitemChanged(url)
            completionHandler(nil)
        }
    }
}
