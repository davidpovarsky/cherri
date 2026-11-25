import SwiftUI
import UniformTypeIdentifiers

extension UTType {
    static let cherriSource = UTType(exportedAs: "org.cherrilang.cherri.file", conformingTo: .plainText)
}

struct CherriDocument: FileDocument {
    static var readableContentTypes: [UTType] { [.cherriSource] }
    static var writableContentTypes: [UTType] { [.cherriSource] }

    var text: String {
        didSet {
            if !text.utf16.elementsEqual(oldValue.utf16) {
                sourceRevision &+= 1
            }
        }
    }
    private let editorDocumentID = UUID()
    private var sourceRevision: UInt64 = 0
    private var editorActionSequence: UInt64 = 0
    let startupObservationID: String?

    mutating func beginEditorAction() -> CherriEditorActionRequest {
        editorActionSequence &+= 1
        return CherriEditorActionRequest(
            documentID: editorDocumentID, revision: sourceRevision,
            sequence: editorActionSequence, source: text
        )
    }

    func acceptsEditorAction(_ request: CherriEditorActionRequest) -> Bool {
        request.documentID == editorDocumentID
            && request.revision == sourceRevision
            && request.sequence == editorActionSequence
            && request.source.utf16.elementsEqual(text.utf16)
    }

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
