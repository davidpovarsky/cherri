import Foundation

/// Reads only the receipt for the current command. UI consent may be needed
/// before openurl exits, so pending delivery permits inspection, never success.
struct QADifferentialImportTiming {
    enum DispatchState: Equatable {
        case pending(deadline: Date)
        case exited(at: Date)
        case rejected(String)

        var failure: String? {
            if case .rejected(let reason) = self { return reason }
            return nil
        }

        var succeeded: Bool {
            if case .exited = self { return true }
            return false
        }
    }

    enum AddDecision: Equatable {
        case search(deadline: Date)
        case rejected(String)

        var failure: String? {
            if case .rejected(let reason) = self { return reason }
            return nil
        }

        var deadline: Date? {
            if case .search(let deadline) = self { return deadline }
            return nil
        }
    }



    enum SheetDecision: Equatable {
        case inspect(deadline: Date)
        case consent(deadline: Date)
        case rejected(String)

        var failure: String? {
            if case .rejected(let reason) = self { return reason }
            return nil
        }

        var deadline: Date? {
            switch self {
            case .inspect(let deadline), .consent(let deadline): return deadline
            case .rejected: return nil
            }
        }

        var canTap: Bool {
            if case .consent = self { return true }
            return false
        }
    }

    enum LibraryDecision: Equatable {
        case inspect(deadline: Date, canConfirm: Bool)
        case rejected(String)

        var failure: String? {
            if case .rejected(let reason) = self { return reason }
            return nil
        }

        var deadline: Date? {
            if case .inspect(let deadline, _) = self { return deadline }
            return nil
        }

        var canConfirm: Bool {
            if case .inspect(_, let canConfirm) = self { return canConfirm }
            return false
        }
    }

    private struct Receipt: Decodable, Equatable {
        let schemaVersion: Int
        let command: [String]
        let timeoutSeconds: TimeInterval
        let startedUTC: String
        let pid: Int
        let outcome: String
        let timedOut: Bool
        let exitCode: Int?
        let phasePublishedUTC: String?
        let exitObservedUTC: String?
        let completedUTC: String?
        let elapsedSeconds: TimeInterval?
        let processReturncode: Int?
        let parentReaped: Bool?
    }

    private struct Launch: Equatable {
        let command: [String]
        let startedUTC: String
        let pid: Int
    }

    let searchStartedAt: Date
    private var launch: Launch?
    private var completed: Receipt?
    private var failure: String?

    init(searchStartedAt: Date) {
        self.searchStartedAt = searchStartedAt
    }

    private static func timestamp(_ text: String) -> Date? {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        if let value = formatter.date(from: text) { return value }
        formatter.formatOptions = [.withInternetDateTime]
        return formatter.date(from: text)
    }

    private mutating func reject(_ reason: String) -> DispatchState {
        failure = reason
        return .rejected(reason)
    }

    mutating func observeDispatch(_ data: Data?, now: Date) -> DispatchState {
        if let failure = failure { return .rejected(failure) }
        guard let data = data else { return reject("IMPORT_DISPATCH_RECEIPT_MISSING") }
        guard let receipt = try? JSONDecoder().decode(Receipt.self, from: data) else {
            return reject("IMPORT_DISPATCH_RECEIPT_MALFORMED")
        }
        guard now.timeIntervalSince1970.isFinite,
              searchStartedAt.timeIntervalSince1970.isFinite,
              now >= searchStartedAt,
              receipt.schemaVersion == 1, receipt.timeoutSeconds == 60,
              receipt.pid > 0, receipt.command.count == 5,
              Array(receipt.command.prefix(3)) == ["xcrun", "simctl", "openurl"],
              !receipt.command[3].isEmpty,
              let target = URL(string: receipt.command[4]), target.isFileURL,
              !target.path.isEmpty,
              let started = Self.timestamp(receipt.startedUTC),
              started <= searchStartedAt else {
            return reject("IMPORT_DISPATCH_RECEIPT_MALFORMED")
        }
        if let publishedText = receipt.phasePublishedUTC {
            guard let published = Self.timestamp(publishedText),
                  published >= started, published <= now else {
                return reject("IMPORT_DISPATCH_RECEIPT_MALFORMED")
            }
        }
        let currentLaunch = Launch(command: receipt.command, startedUTC: receipt.startedUTC, pid: receipt.pid)
        if let launch = launch, launch != currentLaunch {
            return reject("IMPORT_DISPATCH_RECEIPT_CHANGED")
        }
        launch = currentLaunch
        if let completed = completed, completed != receipt {
            return reject("IMPORT_DISPATCH_RECEIPT_CHANGED")
        }
        if receipt.timedOut { return reject("IMPORT_DISPATCH_TIMED_OUT") }
        let dispatchDeadline = started.addingTimeInterval(60)
        switch receipt.outcome {
        case "LAUNCHED":
            guard receipt.exitCode == nil, receipt.exitObservedUTC == nil,
                  receipt.completedUTC == nil, receipt.elapsedSeconds == nil,
                  receipt.processReturncode == nil, receipt.parentReaped == nil else {
                return reject("IMPORT_DISPATCH_RECEIPT_MALFORMED")
            }
            guard now < dispatchDeadline else {
                return reject("IMPORT_DISPATCH_DEADLINE_EXCEEDED")
            }
            return .pending(deadline: dispatchDeadline)
        case "EXITED":
            guard receipt.exitCode == 0, receipt.processReturncode == 0,
                  receipt.parentReaped == true,
                  let publishedText = receipt.phasePublishedUTC, let published = Self.timestamp(publishedText),
                  let exitText = receipt.exitObservedUTC, let exit = Self.timestamp(exitText),
                  let completedText = receipt.completedUTC, let ended = Self.timestamp(completedText),
                  let elapsed = receipt.elapsedSeconds, elapsed.isFinite, elapsed >= 0,
                  exit >= published, ended >= exit, ended <= now else {
                return reject("IMPORT_DISPATCH_RECEIPT_MALFORMED")
            }
            guard exit <= dispatchDeadline, elapsed <= 60 else {
                return reject("IMPORT_DISPATCH_LATE_EXIT")
            }
            completed = receipt
            return .exited(at: exit)
        case "EXIT_NONZERO": return reject("IMPORT_DISPATCH_EXIT_NONZERO")
        case "TIMED_OUT": return reject("IMPORT_DISPATCH_TIMED_OUT")
        case "FAILED": return reject("IMPORT_DISPATCH_FAILED")
        default: return reject("IMPORT_DISPATCH_RECEIPT_MALFORMED")
        }
    }

    func addDecision(after dispatch: DispatchState, now: Date) -> AddDecision {
        let deadline: Date
        switch dispatch {
        case .pending(let dispatchDeadline):
            guard now < dispatchDeadline else {
                return .rejected("IMPORT_DISPATCH_DEADLINE_EXCEEDED")
            }
            deadline = dispatchDeadline
        case .exited(let exit):
            deadline = max(searchStartedAt, exit).addingTimeInterval(15)
        case .rejected(let reason): return .rejected(reason)
        }
        guard now < deadline else { return .rejected("IMPORT_BUTTON_MISSING") }
        return .search(deadline: deadline)
    }



    static func inspectSheetForConsent(observe: () throws -> SheetDecision,
                                       capture: () throws -> Void) rethrows -> SheetDecision {
        let before = try observe()
        guard before.canTap else { return before }
        try capture()
        // A screenshot can block while the import surface transitions. Consent
        // depends on a fresh AX observation and fresh clock after it returns.
        return try observe()
    }

    func sheetDecision(after dispatch: DispatchState, now: Date,
                       buttonExists: Bool, buttonHittable: Bool,
                       identityMatches: Bool) -> SheetDecision {
        // A prior sheet can expose Add while delivery of this command is still
        // pending. Reuse the original clock; readiness never renews its deadline.
        switch addDecision(after: dispatch, now: now) {
        case .rejected(let reason): return .rejected(reason)
        case .search(let deadline):
            guard buttonExists && buttonHittable && identityMatches else {
                return .inspect(deadline: deadline)
            }
            // Identified consent may be necessary before openurl can exit.
            // Import success still requires the independent completed dispatch
            // and exact native library item in libraryDecision.
            return .consent(deadline: deadline)
        }
    }

    func libraryDecision(after dispatch: DispatchState, addTappedAt: Date,
                         now: Date) -> LibraryDecision {
        guard addTappedAt.timeIntervalSince1970.isFinite,
              now.timeIntervalSince1970.isFinite,
              addTappedAt >= searchStartedAt, now >= addTappedAt else {
            return .rejected("IMPORT_LIBRARY_CLOCK_INVALID")
        }
        let deadline: Date
        switch dispatch {
        case .pending(let dispatchDeadline):
            // Consent may precede completion of the original60s URL delivery.
            // Pending delivery permits observation, never import confirmation.
            guard now < dispatchDeadline else {
                return .rejected("IMPORT_DISPATCH_DEADLINE_EXCEEDED")
            }
            deadline = dispatchDeadline
        case .exited(let exit):
            // Preserve the full30s library window after both Add and delivery.
            // Neither repeated AX queries nor receipt reads reset this anchor.
            deadline = max(addTappedAt, exit).addingTimeInterval(30)
        case .rejected(let reason): return .rejected(reason)
        }
        return .inspect(deadline: deadline,
                        canConfirm: dispatch.succeeded && now < deadline)
    }
}
