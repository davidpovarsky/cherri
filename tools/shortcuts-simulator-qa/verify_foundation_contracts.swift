import Foundation
import CoreGraphics

@main
enum QAFoundationContracts {
    static func main() throws {
        func require(_ condition: @autoclosure () -> Bool, _ message: String) throws {
            if !condition() { throw NSError(domain: "QAFoundationContracts", code: 1,
                                             userInfo: [NSLocalizedDescriptionKey: message]) }
        }
        let values: [CGFloat] = [0, 1.24, -1.26, .greatestFiniteMagnitude,
                                -.greatestFiniteMagnitude, .leastNonzeroMagnitude,
                                .infinity, -.infinity, .nan]
        try require(!(CGFloat.greatestFiniteMagnitude * 10).isFinite,
                    "Control must reproduce overflow in the original rounding expression")
        for value in values {
            try require(QAEvidenceGeometry.coordinate(value).isFinite, "Coordinate must remain JSON finite")
        }
        try require(QAEvidenceGeometry.coordinate(1.24) == 1.2, "Normal positive rounding")
        try require(QAEvidenceGeometry.coordinate(-1.26) == -1.3, "Normal negative rounding")
        try require(QAEvidenceGeometry.coordinate(.greatestFiniteMagnitude) == Double.greatestFiniteMagnitude,
                    "A finite extreme value must not be replaced with infinity")
        let frames: [CGRect] = [.infinite, .null, .zero, CGRect(x: 1.24, y: -1.26, width: 100, height: 20),
                                CGRect(x: CGFloat.greatestFiniteMagnitude, y: 0, width: 1, height: 1)]
        let bytes = try JSONEncoder().encode(frames.map { SanitizedFrame(frame: $0) })
        _ = try JSONDecoder().decode([SanitizedFrame].self, from: bytes)
        try require(!QAEvidenceGeometry.isValid(.infinite), "Infinite AX rectangle must be marked invalid")
        try require(!QAEvidenceGeometry.isValid(.null), "Null AX rectangle must be marked invalid")
        try require(QAEvidenceGeometry.isValid(frames[3]), "Normal AX rectangle must remain valid")
        var strictEncoderRejected = false
        do { _ = try JSONEncoder().encode([Double.infinity]) }
        catch { strictEncoderRejected = true }
        try require(strictEncoderRejected, "Negative control: JSON encoder must reject actual infinity")

        for (input, expected) in [("0", "0"), ("-123", "-123"), ("00042", "42")] {
            try require(QACallbackDiagnostics.numericErrorCode(input) == expected, "Numeric error code")
        }
        for input in ["secret=123", "1.2", "1234567890123", "+1", " 1", "?", "https://example.invalid"] {
            try require(QACallbackDiagnostics.numericErrorCode(input) == nil, "Arbitrary error code must be rejected")
        }
        try require(QACallbackDiagnostics.numericErrorCode(nil) == nil, "Missing error code")
        let shape = CallbackPayloadShape("CHERRI_QA_RESULT:ok\n{\"kind\":\"COLLECTOR_EVENT\"}\n{\"kind\":\"INLINE_DEBUG_ACTION\"}\n")
        try require(shape.nonemptyLineCount == 3 && shape.collectorMarkerCount == 1 &&
                    shape.inlineMarkerCount == 1 && shape.sentinelLineCount == 0, "Payload shape counts")
        let shapeBytes = try JSONEncoder().encode(shape)
        let encoded = String(decoding: shapeBytes, as: UTF8.self)
        try require(!encoded.contains("CHERRI_QA_RESULT") && !encoded.contains("COLLECTOR_EVENT"),
                    "Shape diagnostic must not include raw payload")
        print("{\"schemaVersion\":\"1.0\",\"geometryFinite\":true,\"oldRoundingOverflowReproduced\":true,\"strictJSONNegativeControl\":true,\"numericErrorsOnly\":true,\"payloadShapeOnly\":true}")
    }
}
