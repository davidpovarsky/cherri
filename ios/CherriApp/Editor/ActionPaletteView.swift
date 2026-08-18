import SwiftUI

struct ActionPaletteView: View {
    let actions: [CherriActionInfo]
    let onSelect: (CherriActionInfo) -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var query = ""

    var body: some View {
        NavigationStack {
            List(filteredActions) { action in
                Button {
                    onSelect(action)
                    dismiss()
                } label: {
                    VStack(alignment: .leading, spacing: 4) {
                        HStack(alignment: .firstTextBaseline) {
                            Text(action.title?.isEmpty == false ? action.title! : action.name)
                                .font(.headline)
                                .foregroundStyle(.primary)
                            Spacer()
                            if action.macOnly == true {
                                Text("macOS")
                                    .font(.caption2)
                                    .foregroundStyle(.secondary)
                            }
                        }

                        Text(action.signature)
                            .font(.system(.caption, design: .monospaced))
                            .foregroundStyle(.secondary)
                            .lineLimit(2)

                        if let description = action.description, !description.isEmpty {
                            Text(description)
                                .font(.caption)
                                .foregroundStyle(.secondary)
                                .lineLimit(2)
                        }

                        if let category = action.category, !category.isEmpty {
                            Text(categoryLabel(action, category: category))
                                .font(.caption2)
                                .foregroundStyle(.tertiary)
                        }
                    }
                    .contentShape(Rectangle())
                    .padding(.vertical, 3)
                }
                .buttonStyle(.plain)
            }
            .overlay {
                if filteredActions.isEmpty {
                    ContentUnavailableView.search(text: query)
                }
            }
            .navigationTitle("Cherri Actions")
            .navigationBarTitleDisplayMode(.inline)
            .searchable(text: $query, prompt: "Action name or description")
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button("Done") { dismiss() }
                }
            }
        }
    }

    private var filteredActions: [CherriActionInfo] {
        let trimmed = query.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return actions }
        let needle = trimmed.lowercased()

        return actions.filter { action in
            action.name.lowercased().contains(needle)
                || action.title?.lowercased().contains(needle) == true
                || action.description?.lowercased().contains(needle) == true
                || action.category?.lowercased().contains(needle) == true
                || action.subcategory?.lowercased().contains(needle) == true
        }
    }

    private func categoryLabel(_ action: CherriActionInfo, category: String) -> String {
        if let subcategory = action.subcategory, !subcategory.isEmpty {
            return "\(category) · \(subcategory)"
        }
        return category
    }
}
