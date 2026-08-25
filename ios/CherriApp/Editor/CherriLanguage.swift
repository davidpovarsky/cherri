import LanguageSupport

private let cherriReservedIdentifiers = [
    // Control flow and declarations. Kept in sync with Cherri's own token set
    // and the existing cherri-vscode grammar.
    "if", "else", "repeat", "for", "in", "menu", "item", "const", "action",
    "copy", "paste", "default", "enum", "function",

    // # directives are tokenised as '#' plus the identifier so the directive
    // names can still receive keyword highlighting with CodeEditorView.
    "include", "define", "import", "question", "ref",

    // Definitions and no-input behaviour.
    "name", "color", "glyph", "inputs", "outputs", "from", "version", "mac",
    "noinput", "quickactions", "stopwith", "askfor", "getclipboard",

    // Cherri value types and literals.
    "text", "rawtext", "number", "float", "dictionary", "array", "bool",
    "date", "variable", "qty", "true", "false", "nil",

    // Built-in values represented by the existing VS Code grammar.
    "CurrentDate", "Device", "RepeatIndex", "RepeatItem", "ShortcutInput", "Ask"
]

private let cherriReservedOperators = [
    "=", "+=", "-=", "*=", "/=", "==", "!=", ">", ">=", "<", "<=", "<>",
    "+", "-", "*", "/", "%", "&&", "||", "@", "#", ":"
]

extension LanguageConfiguration {
    static func cherri(_ languageService: LanguageService? = nil) -> LanguageConfiguration {
        let stringRegex: Regex<Substring> = /\"(?:\\\"|[^\"])*+\"/
        let numberRegex: Regex<Substring> = /-?[0-9]+(?:\.[0-9]+)?/
        let identifierRegex: Regex<Substring> = /[A-Za-z_][A-Za-z0-9_]*/
        let operatorRegex: Regex<Substring> = /[=+*%!<>&|?:@#\/-]+/

        return LanguageConfiguration(
            name: "Cherri",
            supportsSquareBrackets: true,
            supportsCurlyBrackets: true,
            stringRegex: stringRegex,
            characterRegex: nil,
            numberRegex: numberRegex,
            singleLineComment: "//",
            nestedComment: (open: "/*", close: "*/"),
            identifierRegex: identifierRegex,
            operatorRegex: operatorRegex,
            reservedIdentifiers: cherriReservedIdentifiers,
            reservedOperators: cherriReservedOperators,
            languageService: languageService
        )
    }
}
