import SwiftUI
import UniformTypeIdentifiers

extension UTType {
    static let cherriSource = UTType(exportedAs: "org.cherrilang.cherri.file", conformingTo: .plainText)
}

struct CherriDocument: FileDocument {
    static var readableContentTypes: [UTType] { [.cherriSource] }
    static var writableContentTypes: [UTType] { [.cherriSource] }

    var text: String
    let startupObservationID: String?

    init(text: String = CommandLine.arguments.contains("--uitesting") ? "" : "show(\"Hello from Cherri\")\n") {
        self.text = text
        startupObservationID = CherriStartupLifecycleObserver.shared == nil ? nil : CherriStartupLifecycleObserver.newDocumentID()
        CherriStartupLifecycleObserver.shared?.record(.documentInitialized, documentID: startupObservationID)
    }

    init(configuration: ReadConfiguration) throws {
        let identity = CherriStartupLifecycleObserver.shared == nil ? nil : CherriStartupLifecycleObserver.newDocumentID()
        startupObservationID = identity
        CherriStartupLifecycleObserver.shared?.record(.documentReadStarted, documentID: identity)
        guard let data = configuration.file.regularFileContents,
              let text = String(data: data, encoding: .utf8) else {
            throw CocoaError(.fileReadCorruptFile)
        }
        self.text = text
        CherriStartupLifecycleObserver.shared?.record(.documentReadFinished, documentID: identity)
    }

    func fileWrapper(configuration: WriteConfiguration) throws -> FileWrapper {
        CherriStartupLifecycleObserver.shared?.record(.documentWriteStarted, documentID: startupObservationID)
        let wrapper = FileWrapper(regularFileWithContents: Data(text.utf8))
        CherriStartupLifecycleObserver.shared?.record(.documentWriteFinished, documentID: startupObservationID)
        return wrapper
    }
}
