import SwiftUI
import UniformTypeIdentifiers

extension UTType {
    static let cherriSource = UTType(exportedAs: "org.cherrilang.cherri.file", conformingTo: .plainText)
}

struct CherriDocument: FileDocument {
    static var readableContentTypes: [UTType] { [.cherriSource] }
    static var writableContentTypes: [UTType] { [.cherriSource] }

    var text: String

    init(text: String = "show(\"Hello from Cherri\")\n") {
        self.text = text
    }

    init(configuration: ReadConfiguration) throws {
        guard let data = configuration.file.regularFileContents,
              let text = String(data: data, encoding: .utf8) else {
            throw CocoaError(.fileReadCorruptFile)
        }
        self.text = text
    }

    func fileWrapper(configuration: WriteConfiguration) throws -> FileWrapper {
        FileWrapper(regularFileWithContents: Data(text.utf8))
    }
}
