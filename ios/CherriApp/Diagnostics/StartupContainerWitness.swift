import SwiftUI
import UIKit

/// Records only this owned zero-size container's attachment. It does not inspect
/// CodeEditorView internals or imply that its text view exists or is interactive.
struct CherriStartupContainerWitness: UIViewRepresentable {
    let documentID: String?

    func makeUIView(context: Context) -> WitnessView {
        CherriStartupLifecycleObserver.shared?.record(.editorContainerCreated, documentID: documentID)
        return WitnessView(documentID: documentID)
    }

    func updateUIView(_ uiView: WitnessView, context: Context) {
        uiView.updateDocumentID(documentID)
    }

    final class WitnessView: UIView {
        private var association: CherriStartupDocumentAssociation

        init(documentID: String?) {
            association = CherriStartupDocumentAssociation(documentID: documentID)
            super.init(frame: .zero)
            isUserInteractionEnabled = false
            isAccessibilityElement = false
            accessibilityElementsHidden = true
        }

        required init?(coder: NSCoder) { nil }

        func updateDocumentID(_ next: String?) {
            if association.update(next) {
                CherriStartupLifecycleObserver.shared?.record(.editorContainerDocumentChanged,
                    documentID: association.documentID, once: false)
            }
        }

        override func didMoveToWindow() {
            super.didMoveToWindow()
            CherriStartupLifecycleObserver.shared?.record(
                window == nil ? .editorContainerDetached : .editorContainerAttached,
                documentID: association.documentID, once: false)
        }
    }
}
