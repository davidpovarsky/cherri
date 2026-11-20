import SwiftUI

@main
struct CherriApp: App {
    init() {
        CherriStartupLifecycleObserver.shared?.record(.appInitialized)
    }

    var body: some Scene {
        DocumentGroup(newDocument: CherriDocument()) { configuration in
            WorkspaceView(
                document: configuration.$document,
                fileURL: configuration.fileURL
            )
        }
    }
}
