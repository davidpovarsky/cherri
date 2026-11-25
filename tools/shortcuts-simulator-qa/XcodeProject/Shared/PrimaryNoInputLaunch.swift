import Foundation
import CryptoKit

enum QAPrimaryNoInputFailure: String, Error {
    case contractSize = "CONTRACT_SIZE"
    case contractJSON = "CONTRACT_JSON"
    case contractFields = "CONTRACT_FIELDS"
    case contractCanonical = "CONTRACT_NONCANONICAL"
    case contractIdentity = "CONTRACT_IDENTITY"
    case nonce = "CONTRACT_NONCE"
    case primaryURL = "PRIMARY_URL"
    case callbackURL = "CALLBACK_URL"
    case requestedURL = "REQUESTED_URL_MISMATCH"
    case source = "TRIGGER_SOURCE"
    case consumed = "ATTEMPT_ALREADY_CONSUMED"
    case evidence = "ORIGINAL_EVIDENCE_WRITE"
}

struct QAPrimaryNoInputContract {
    static let schema = "cherri-function-envelope-no-input-launch/1"
    static let mode = "primary-no-input"
    static let fixture = "qa-function-envelope-presence-v2"
    static let workflow = "CherriFunctionEnvelopePresenceProbe"
    let original: Data
    let sha256: String
    let runNonce: String
    let primaryURL: String

    static func byteEqual(_ left: String?, _ right: String) -> Bool {
        guard let left = left else { return false }
        return Data(left.utf8) == Data(right.utf8)
    }

    static func digest(_ data: Data) -> String {
        SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
    }

    private static func query(_ raw: String, scheme: String, host: String,
                              path: String, keys: Set<String>) throws -> [String: String] {
        let bytes = Array(raw.utf8)
        guard !bytes.isEmpty, bytes.count <= 8192,
              bytes.allSatisfy({ $0 >= 0x21 && $0 <= 0x7e }) else {
            throw QAPrimaryNoInputFailure.primaryURL
        }
        func hex(_ b: UInt8) -> Bool {
            (48...57).contains(b) || (65...70).contains(b) || (97...102).contains(b)
        }
        var i = 0
        while i < bytes.count {
            if bytes[i] == 37 {
                guard i + 2 < bytes.count, hex(bytes[i + 1]), hex(bytes[i + 2]) else {
                    throw QAPrimaryNoInputFailure.primaryURL
                }
                i += 3
            } else {
                i += 1
            }
        }
        guard raw.hasPrefix(scheme + "://" + host + path + "?"),
              let parts = URLComponents(string: raw),
              parts.scheme == scheme, parts.host == host, parts.path == path,
              parts.user == nil, parts.password == nil, parts.port == nil,
              parts.fragment == nil, parts.url != nil,
              let items = parts.queryItems, items.count == keys.count else {
            throw QAPrimaryNoInputFailure.primaryURL
        }
        var values: [String: String] = [:]
        for item in items {
            guard keys.contains(item.name), values[item.name] == nil,
                  let value = item.value else {
                throw QAPrimaryNoInputFailure.primaryURL
            }
            values[item.name] = value
        }
        guard Set(values.keys) == keys else { throw QAPrimaryNoInputFailure.primaryURL }
        return values
    }

    init(_ raw: String) throws {
        let data = Data(raw.utf8)
        guard !data.isEmpty, data.count <= 65536 else {
            throw QAPrimaryNoInputFailure.contractSize
        }
        guard let values = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] else {
            throw QAPrimaryNoInputFailure.contractJSON
        }
        let keys: Set<String> = ["schema", "mode", "fixtureId", "workflowName", "runNonce", "primaryURL"]
        guard Set(values.keys) == keys,
              values.values.allSatisfy({ $0 is String }) else {
            throw QAPrimaryNoInputFailure.contractFields
        }
        guard let encoded = try? JSONSerialization.data(withJSONObject: values,
                        options: [.sortedKeys, .withoutEscapingSlashes]), encoded == data else {
            // A duplicate key, alternate escaping or extra whitespace cannot
            // be hidden by Foundation's object decoder.
            throw QAPrimaryNoInputFailure.contractCanonical
        }
        guard values["schema"] as? String == Self.schema,
              values["mode"] as? String == Self.mode,
              values["fixtureId"] as? String == Self.fixture,
              values["workflowName"] as? String == Self.workflow,
              let nonce = values["runNonce"] as? String,
              let url = values["primaryURL"] as? String else {
            throw QAPrimaryNoInputFailure.contractIdentity
        }
        let nonceBytes = Array(nonce.utf8)
        guard (1...128).contains(nonceBytes.count),
              nonceBytes.allSatisfy({ (48...57).contains($0) || (65...90).contains($0)
                  || (97...122).contains($0) || $0 == 45 || $0 == 95 }) else {
            throw QAPrimaryNoInputFailure.nonce
        }
        let outer = try Self.query(url, scheme: "shortcuts", host: "x-callback-url",
                    path: "/run-shortcut",
                    keys: ["name", "x-source", "x-success", "x-error", "x-cancel"])
        guard Self.byteEqual(outer["name"], Self.workflow),
              Self.byteEqual(outer["x-source"], "CherriSimulatorQAReceiver") else {
            throw QAPrimaryNoInputFailure.primaryURL
        }
        for (key, host) in [("x-success", "success"), ("x-error", "error"), ("x-cancel", "cancel")] {
            let callback: [String: String]
            do {
                callback = try Self.query(outer[key]!, scheme: "cherriqa", host: host, path: "",
                              keys: ["runId", "fixtureId", "shortcutName", "source"])
            } catch {
                throw QAPrimaryNoInputFailure.callbackURL
            }
            guard Self.byteEqual(callback["runId"], nonce),
                  Self.byteEqual(callback["fixtureId"], Self.fixture),
                  Self.byteEqual(callback["shortcutName"], Self.workflow),
                  Self.byteEqual(callback["source"], "shortcuts-x-callback") else {
                throw QAPrimaryNoInputFailure.callbackURL
            }
        }
        original = data
        sha256 = Self.digest(data)
        runNonce = nonce
        primaryURL = url
    }
}

struct QAPrimaryNoInputPermit {
    let allowed: Bool
    let allowsFallback: Bool
    let reason: String?
    fileprivate let token: UUID?
    fileprivate let source: String
    fileprivate let requestedURL: String
}

/// One instance lives in CallbackEventStore.shared for the whole app process.
/// Its original files also prevent the same contract being replayed after a restart.
final class QAPrimaryNoInputLaunchGate {
    static let environmentKey = "CHERRI_QA_PRIMARY_NO_INPUT_CONTRACT_JSON"
    private let lock = NSLock()
    private let constrained: Bool
    private let contract: QAPrimaryNoInputContract?
    private let contractFailure: QAPrimaryNoInputFailure?
    private let contractSHA256: String?
    private let contractByteCount: Int
    private let contractOriginal: Data?
    private let evidenceRoot: URL
    private var consumed = false
    private var sequence = 0
    private var attemptOrdinal = 0
    private var pending: UUID?
    private var directory: URL?

    init(rawContract: String?, evidenceRoot: URL) {
        self.evidenceRoot = evidenceRoot.resolvingSymlinksInPath().standardizedFileURL
        constrained = rawContract != nil
        contractByteCount = rawContract?.utf8.count ?? 0
        contractSHA256 = rawContract.map { QAPrimaryNoInputContract.digest(Data($0.utf8)) }
        contractOriginal = rawContract.flatMap { $0.utf8.count <= 65536 ? Data($0.utf8) : nil }
        if let raw = rawContract {
            do {
                contract = try QAPrimaryNoInputContract(raw)
                contractFailure = nil
            } catch {
                contract = nil
                contractFailure = (error as? QAPrimaryNoInputFailure) ?? .contractJSON
            }
        } else {
            contract = nil
            contractFailure = nil
        }
    }

    private func approvedDirectory() throws -> URL {
        let fm = FileManager.default
        func directoryOnly(_ url: URL, privateOnly: Bool = false) throws {
            let attrs = try fm.attributesOfItem(atPath: url.path)
            guard attrs[.type] as? FileAttributeType == .typeDirectory,
                  !privateOnly || (((attrs[.posixPermissions] as? NSNumber)?.intValue ?? -1) & 0o077) == 0 else {
                throw QAPrimaryNoInputFailure.evidence
            }
        }
        if let existing = directory {
            try directoryOnly(existing.deletingLastPathComponent(), privateOnly: true)
            try directoryOnly(existing, privateOnly: true)
            return existing
        }
        try directoryOnly(evidenceRoot)
        let base = evidenceRoot.appendingPathComponent("primary-no-input-launch", isDirectory: true)
        if !fm.fileExists(atPath: base.path) {
            try fm.createDirectory(at: base, withIntermediateDirectories: false,
                                   attributes: [.posixPermissions: 0o700])
        }
        try directoryOnly(base, privateOnly: true)
        let leaf = contractSHA256 ?? "rejected-oversized-contract"
        let target = base.appendingPathComponent(leaf, isDirectory: true)
        // An old directory, including an empty one, is never reused.
        guard !fm.fileExists(atPath: target.path) else { throw QAPrimaryNoInputFailure.evidence }
        try fm.createDirectory(at: target, withIntermediateDirectories: false,
                               attributes: [.posixPermissions: 0o700])
        try directoryOnly(target, privateOnly: true)
        let owner: [String: Any] = [
            "schema": "cherri-function-envelope-trigger-owner/1",
            "processID": Int(ProcessInfo.processInfo.processIdentifier),
            "contractByteCount": contractByteCount,
            "contractSHA256": contractSHA256 as Any? ?? NSNull(),
            "contractOriginalRetained": contractOriginal != nil
        ]
        let ownerData = try JSONSerialization.data(withJSONObject: owner, options: [.sortedKeys])
        try writeOriginal(ownerData, to: target.appendingPathComponent("owner.original.json"))
        if let original = contractOriginal {
            try writeOriginal(original, to: target.appendingPathComponent("contract.original.json"))
        }
        directory = target
        return target
    }

    private func writeOriginal(_ data: Data, to url: URL) throws {
        guard data.count <= 262144 else { throw QAPrimaryNoInputFailure.evidence }
        try data.write(to: url, options: .withoutOverwriting)
        try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: url.path)
        guard try Data(contentsOf: url) == data else { throw QAPrimaryNoInputFailure.evidence }
    }

    private func retain(event: String, source: String, requestedURL: String,
                        reason: String? = nil, success: Bool? = nil, token: UUID? = nil) throws {
        guard sequence < 32 else { throw QAPrimaryNoInputFailure.evidence }
        let target = try approvedDirectory()
        sequence += 1
        let raw = Data(requestedURL.utf8)
        let retentionFailure = raw.count > 65536 || contractOriginal == nil
        var row: [String: Any] = [
            "schema": "cherri-function-envelope-trigger-receipt/1",
            "mode": QAPrimaryNoInputContract.mode,
            "sequence": sequence, "event": event, "attemptOrdinal": attemptOrdinal,
            "processID": Int(ProcessInfo.processInfo.processIdentifier),
            "timestampISO8601": ISO8601DateFormatter().string(from: Date()),
            "contractSHA256": contractSHA256 as Any? ?? NSNull(),
            "contractByteCount": contractByteCount, "source": source,
            "attemptConsumed": consumed,
            "requestedURLUTF8Bytes": raw.count,
            "requestedURLSHA256": QAPrimaryNoInputContract.digest(raw),
            "requestedURLOriginalRetained": raw.count <= 65536,
            "contractOriginalRetained": contractOriginal != nil,
            "retentionFailure": retentionFailure
        ]
        if raw.count <= 65536 {
            row["requestedURL"] = requestedURL
            row["requestedURLUTF8Base64"] = raw.base64EncodedString()
        }
        if let plan = contract {
            let primary = Data(plan.primaryURL.utf8)
            row["fixtureId"] = QAPrimaryNoInputContract.fixture
            row["workflowName"] = QAPrimaryNoInputContract.workflow
            row["runNonce"] = plan.runNonce
            row["primaryURL"] = plan.primaryURL
            row["primaryURLUTF8Base64"] = primary.base64EncodedString()
            row["primaryURLUTF8Bytes"] = primary.count
            row["primaryURLSHA256"] = QAPrimaryNoInputContract.digest(primary)
        }
        if let reason = reason { row["reason"] = reason }
        if retentionFailure { row["reason"] = QAPrimaryNoInputFailure.evidence.rawValue }
        if let success = success { row["success"] = success }
        if let token = token { row["invocationToken"] = token.uuidString }
        let encoded = try JSONSerialization.data(withJSONObject: row,
                          options: [.sortedKeys, .withoutEscapingSlashes])
        let name = String(format: "%03d-%@.original.json", sequence, event)
        try writeOriginal(encoded, to: target.appendingPathComponent(name))
        guard !retentionFailure else { throw QAPrimaryNoInputFailure.evidence }
    }

    private func rejected(_ failure: QAPrimaryNoInputFailure,
                          source: String, requestedURL: String) -> QAPrimaryNoInputPermit {
        let code: String
        do {
            try retain(event: "rejection", source: source, requestedURL: requestedURL,
                       reason: failure.rawValue)
            code = failure.rawValue
        } catch {
            code = QAPrimaryNoInputFailure.evidence.rawValue
        }
        return QAPrimaryNoInputPermit(allowed: false, allowsFallback: false, reason: code,
                                     token: nil, source: source, requestedURL: requestedURL)
    }

    func begin(rawURL: String, source: String) -> QAPrimaryNoInputPermit {
        lock.lock()
        defer { lock.unlock() }
        guard constrained else {
            return QAPrimaryNoInputPermit(allowed: true, allowsFallback: true, reason: nil,
                                         token: nil, source: source, requestedURL: rawURL)
        }
        guard !consumed else { return rejected(.consumed, source: source, requestedURL: rawURL) }
        // No second semantic request can be substituted after any first request,
        // including an invalid URL, malformed contract or original-write failure.
        consumed = true
        guard let plan = contract else {
            return rejected(contractFailure ?? .contractJSON, source: source, requestedURL: rawURL)
        }
        guard ["env", "argv", "onOpenURL"].contains(source) else {
            return rejected(.source, source: source, requestedURL: rawURL)
        }
        guard QAPrimaryNoInputContract.byteEqual(rawURL, plan.primaryURL) else {
            return rejected(.requestedURL, source: source, requestedURL: rawURL)
        }
        attemptOrdinal = 1
        let token = UUID()
        do {
            try retain(event: "intent", source: source, requestedURL: rawURL, token: token)
        } catch {
            return QAPrimaryNoInputPermit(allowed: false, allowsFallback: false,
                reason: QAPrimaryNoInputFailure.evidence.rawValue, token: nil,
                source: source, requestedURL: rawURL)
        }
        pending = token
        return QAPrimaryNoInputPermit(allowed: true, allowsFallback: false, reason: nil,
                                     token: token, source: source, requestedURL: rawURL)
    }

    @discardableResult
    func complete(_ permit: QAPrimaryNoInputPermit, success: Bool) -> Bool {
        lock.lock()
        defer { lock.unlock() }
        guard constrained else { return true }
        guard permit.allowed, let token = permit.token, pending == token else { return false }
        pending = nil
        do {
            try retain(event: "completion", source: permit.source, requestedURL: permit.requestedURL,
                       success: success, token: token)
            return true
        } catch {
            // Completion has happened already. Never retry a semantic invocation
            // to replace a missing receipt.
            return false
        }
    }
}
