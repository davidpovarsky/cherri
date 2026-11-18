import Foundation
import CoreGraphics
import CoreText
import ImageIO
import Vision

// Local-only recognizer. The Python caller captures this raw JSON privately.
// Revision, level and language are explicit; unsupported locales are not guessed.
func emit(_ object: [String: Any], code: Int32 = 0) -> Never {
    let data = try! JSONSerialization.data(withJSONObject: object, options: [.sortedKeys])
    print(String(decoding: data, as: UTF8.self))
    exit(code)
}

let selfTest = CommandLine.arguments.count == 2 && CommandLine.arguments[1] == "--self-test"
guard selfTest || CommandLine.arguments.count == 3 else {
    emit(["status": "INVALID_ARGUMENTS"], code: 2)
}
let requested = selfTest ? "en-US" : CommandLine.arguments[2]
let request = VNRecognizeTextRequest()
request.revision = VNRecognizeTextRequestRevision3
request.recognitionLevel = .accurate
request.usesLanguageCorrection = false
request.automaticallyDetectsLanguage = false
do {
    let supported = try request.supportedRecognitionLanguages().sorted()
    let exact = supported.first { $0.caseInsensitiveCompare(requested) == .orderedSame }
    let language = exact ?? supported.first { $0.split(separator: "-").first == requested.split(separator: "-").first }
    guard let language else {
        emit(["status": "LANGUAGE_UNSUPPORTED", "supportedLanguages": supported,
              "engine": "APPLE_VISION", "revision": request.revision,
              "recognitionLevel": "accurate"], code: 3)
    }
    request.recognitionLanguages = [language]
    let image: CGImage
    if selfTest {
        guard let context = CGContext(data: nil, width: 1000, height: 240, bitsPerComponent: 8,
                                      bytesPerRow: 0, space: CGColorSpaceCreateDeviceRGB(),
                                      bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue) else {
            emit(["status": "SELF_TEST_IMAGE_FAILED"], code: 2)
        }
        context.setFillColor(CGColor(gray: 1, alpha: 1))
        context.fill(CGRect(x: 0, y: 0, width: 1000, height: 240))
        let text = NSAttributedString(string: "CHERRI QA 12345", attributes: [
            NSAttributedString.Key(kCTFontAttributeName as String): CTFontCreateWithName("Helvetica" as CFString, 60, nil),
            NSAttributedString.Key(kCTForegroundColorAttributeName as String): CGColor(gray: 0, alpha: 1)])
        context.textPosition = CGPoint(x: 50, y: 100)
        CTLineDraw(CTLineCreateWithAttributedString(text as CFAttributedString), context)
        guard let generated = context.makeImage() else { emit(["status": "SELF_TEST_IMAGE_FAILED"], code: 2) }
        image = generated
    } else {
        let url = URL(fileURLWithPath: CommandLine.arguments[1])
        guard let source = CGImageSourceCreateWithURL(url as CFURL, nil),
              let decoded = CGImageSourceCreateImageAtIndex(source, 0, nil) else {
            emit(["status": "IMAGE_UNREADABLE"], code: 2)
        }
        image = decoded
    }
    guard image.width > 0, image.height > 0, image.width <= 16384, image.height <= 16384 else {
        emit(["status": "IMAGE_UNREADABLE"], code: 2)
    }
    try VNImageRequestHandler(cgImage: image, options: [:]).perform([request])
    let results = request.results ?? []
    guard results.count <= 1000 else { emit(["status": "SEGMENT_LIMIT"], code: 2) }
    var segments: [[String: Any]] = []
    for result in results {
        guard let candidate = result.topCandidates(1).first else { continue }
        let box = result.boundingBox
        guard candidate.string.count <= 2048,
              [box.minX, box.minY, box.width, box.height].allSatisfy({ $0.isFinite }),
              candidate.confidence.isFinite else {
            emit(["status": "INVALID_OBSERVATION"], code: 2)
        }
        segments.append(["text": candidate.string, "confidence": Double(candidate.confidence),
                         "language": language, "source": "APPLE_VISION",
                         "frame": ["x": Double(box.minX) * Double(image.width),
                                   "y": (1 - Double(box.maxY)) * Double(image.height),
                                   "width": Double(box.width) * Double(image.width),
                                   "height": Double(box.height) * Double(image.height)]])
    }
    if selfTest {
        let exact = segments.count == 1 && segments.first?["text"] as? String == "CHERRI QA 12345"
        emit(["status": exact ? "PASS" : "FAIL", "engine": "APPLE_VISION", "revision": request.revision,
              "recognitionLevel": "accurate", "exactGeneratedTextMatched": exact,
              "proofTier": "GENERATED_TEXT_OCR_ONLY", "nativeEditorProof": false,
              "supportedLanguages": supported, "chosenLanguage": language,
              "observationCount": segments.count, "confidence": segments.compactMap { $0["confidence"] as? Double }],
             code: exact ? 0 : 2)
    }
    emit(["status": "OBSERVED", "engine": "APPLE_VISION", "revision": request.revision,
          "recognitionLevel": "accurate", "languageCorrection": false,
          "supportedLanguages": supported, "chosenLanguage": language,
          "imageWidth": image.width, "imageHeight": image.height,
          "osVersion": ProcessInfo.processInfo.operatingSystemVersionString,
          "segments": segments])
} catch {
    // Localized errors may contain image paths or recognized text.
    emit(["status": "VISION_REQUEST_FAILED", "errorCode": (error as NSError).code], code: 2)
}
