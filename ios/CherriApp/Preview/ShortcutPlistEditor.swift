import Foundation

enum ShortcutPreviewValue: Equatable {
    case string(String)
    case integer(Int)
    case real(Double)
    case bool(Bool)

    var propertyListValue: Any {
        switch self {
        case .string(let value): value
        case .integer(let value): value
        case .real(let value): value
        case .bool(let value): value
        }
    }
}

enum ShortcutPreviewEdit: Equatable {
    case workflowType(value: String, enabled: Bool)
    case quickActionSurface(value: String, enabled: Bool)
    case actionParameter(actionIndex: Int, key: String, value: ShortcutPreviewValue)
}

enum ShortcutPlistEditor {
    enum EditError: LocalizedError {
        case invalidShortcut
        case invalidActionIndex(Int)

        var errorDescription: String? {
            switch self {
            case .invalidShortcut:
                "The compiled Shortcut plist could not be edited."
            case .invalidActionIndex(let index):
                "The preview referenced an invalid Shortcut action index (\(index))."
            }
        }
    }

    static func applying(_ edit: ShortcutPreviewEdit, to data: Data) throws -> Data {
        let object = try PropertyListSerialization.propertyList(from: data, options: [], format: nil)
        guard var shortcut = object as? [String: Any] else {
            throw EditError.invalidShortcut
        }

        switch edit {
        case .workflowType(let value, let enabled):
            var values = stringArray(shortcut["WFWorkflowTypes"])
            setMembership(value, enabled: enabled, in: &values)
            shortcut["WFWorkflowTypes"] = values

        case .quickActionSurface(let value, let enabled):
            var surfaces = stringArray(shortcut["WFQuickActionSurfaces"])
            setMembership(value, enabled: enabled, in: &surfaces)
            shortcut["WFQuickActionSurfaces"] = surfaces

            // A selected Finder/Services surface has no effect unless the
            // Shortcut is also enabled as a Quick Action. Match Shortcuts' UI by
            // enabling that workflow type when a surface is explicitly enabled.
            if enabled {
                var workflowTypes = stringArray(shortcut["WFWorkflowTypes"])
                setMembership("QuickActions", enabled: true, in: &workflowTypes)
                shortcut["WFWorkflowTypes"] = workflowTypes
            }

        case .actionParameter(let actionIndex, let key, let value):
            guard var actions = shortcut["WFWorkflowActions"] as? [[String: Any]],
                  actions.indices.contains(actionIndex) else {
                throw EditError.invalidActionIndex(actionIndex)
            }

            var action = actions[actionIndex]
            var parameters = action["WFWorkflowActionParameters"] as? [String: Any] ?? [:]
            parameters[key] = value.propertyListValue
            action["WFWorkflowActionParameters"] = parameters
            actions[actionIndex] = action
            shortcut["WFWorkflowActions"] = actions
        }

        return try PropertyListSerialization.data(
            fromPropertyList: shortcut,
            format: .xml,
            options: 0
        )
    }

    private static func stringArray(_ value: Any?) -> [String] {
        if let strings = value as? [String] {
            return strings
        }
        return (value as? [Any])?.compactMap { $0 as? String } ?? []
    }

    private static func setMembership(_ value: String, enabled: Bool, in values: inout [String]) {
        values.removeAll { $0 == value }
        if enabled {
            values.append(value)
        }
    }
}
