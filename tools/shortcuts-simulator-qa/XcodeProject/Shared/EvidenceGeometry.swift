import Foundation
import CoreGraphics

/// Serialization must remain finite even for an off-screen AX sentinel frame.
/// Multiplying a finite CGFloat by ten can itself overflow before rounding.
struct SanitizedFrame: Codable {
    let x: Double
    let y: Double
    let width: Double
    let height: Double

    init(frame: CGRect) {
        x = QAEvidenceGeometry.coordinate(frame.origin.x)
        y = QAEvidenceGeometry.coordinate(frame.origin.y)
        width = QAEvidenceGeometry.coordinate(frame.size.width)
        height = QAEvidenceGeometry.coordinate(frame.size.height)
    }
}

enum QAEvidenceGeometry {
    static func coordinate(_ value: CGFloat) -> Double {
        guard value.isFinite else { return 0 }
        let scaled = value * 10
        // At this magnitude CGFloat already has no subpixel precision. Keep
        // the original finite value instead of manufacturing infinity.
        let rounded = scaled.isFinite ? scaled.rounded() / 10 : value
        let result = Double(rounded)
        return result.isFinite ? result : 0
    }

    static func isValid(_ frame: CGRect) -> Bool {
        !frame.isNull && !frame.isInfinite &&
        [frame.origin.x, frame.origin.y, frame.width, frame.height,
         frame.minX, frame.maxX, frame.minY, frame.maxY].allSatisfy { $0.isFinite }
    }
}
