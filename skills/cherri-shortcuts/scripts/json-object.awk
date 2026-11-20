# POSIX awk JSON parser for an object root. LC_ALL=C makes UTF-8 validation
# independent of the host locale. Only an exact top-level string is extracted.
BEGIN {
    for (byte = 1; byte < 256; byte++) ordinal[sprintf("%c", byte)] = byte
    nul = sprintf("%c", 0)
    position = 1
}
{
    if (length(document) + length($0) + 1 > 8388608) { oversized = 1; exit 1 }
    document = document $0 "\n"
}
END {
    if (oversized) exit 1
    skip_space()
    if (substr(document, position, 1) != "{" || !parse_object(0)) exit 1
    skip_space()
    if (position <= length(document)) exit 1
    if (mode == "exists") exit !selected_found
    if (wanted != "" && selected_found && selected_kind == "string") {
        # Shell command substitution removes trailing newlines. Reject control
        # characters in selected contract fields instead of silently changing
        # their contents. Ordinary nested documentation strings remain valid.
        for (i = 1; i <= length(selected); i++) {
            c = substr(selected, i, 1)
            if (c == nul || ordinal[c] < 32) exit 1
        }
        printf "%s", selected
    }
}
function skip_space(    c) {
    while (position <= length(document)) {
        c = substr(document, position, 1)
        if (c != " " && c != "\t" && c != "\r" && c != "\n") break
        position++
    }
}
function hex4(    text, n, i, c, digit) {
    text = substr(document, position, 4)
    if (length(text) != 4 || text !~ /^[0-9a-fA-F]+$/) return -1
    n = 0
    for (i = 1; i <= 4; i++) {
        c = tolower(substr(text, i, 1))
        digit = index("0123456789abcdef", c) - 1
        n = n * 16 + digit
    }
    position += 4
    return n
}
function utf8(code) {
    if (code < 128) return sprintf("%c", code)
    if (code < 2048) return sprintf("%c%c", 192 + int(code / 64), 128 + code % 64)
    if (code < 65536) return sprintf("%c%c%c", 224 + int(code / 4096), 128 + int(code / 64) % 64, 128 + code % 64)
    return sprintf("%c%c%c%c", 240 + int(code / 262144), 128 + int(code / 4096) % 64, 128 + int(code / 64) % 64, 128 + code % 64)
}
function parse_string(    result, c, escape, code, low, count, n, j, next_byte, raw) {
    if (substr(document, position, 1) != "\"") return 0
    position++
    result = ""
    while (position <= length(document)) {
        c = substr(document, position++, 1)
        if (c == "\"") { scalar = result; return 1 }
        if (c == "\\") {
            escape = substr(document, position++, 1)
            if (escape == "\"" || escape == "\\" || escape == "/") result = result escape
            else if (escape == "b") result = result sprintf("%c", 8)
            else if (escape == "f") result = result sprintf("%c", 12)
            else if (escape == "n") result = result "\n"
            else if (escape == "r") result = result "\r"
            else if (escape == "t") result = result "\t"
            else if (escape == "u") {
                code = hex4()
                if (code < 0) return 0
                if (code >= 55296 && code <= 56319) {
                    if (substr(document, position, 2) != "\\u") return 0
                    position += 2
                    low = hex4()
                    if (low < 56320 || low > 57343) return 0
                    code = 65536 + (code - 55296) * 1024 + low - 56320
                } else if (code >= 56320 && code <= 57343) return 0
                result = result utf8(code)
            } else return 0
        } else {
            n = ordinal[c]
            if (c == nul || n < 32) return 0
            if (n < 128) result = result c
            else {
                if (n >= 194 && n <= 223) count = 1
                else if (n >= 224 && n <= 239) count = 2
                else if (n >= 240 && n <= 244) count = 3
                else return 0
                raw = c
                for (j = 1; j <= count; j++) {
                    c = substr(document, position++, 1)
                    next_byte = ordinal[c]
                    if (next_byte < 128 || next_byte > 191) return 0
                    if (j == 1 && ((n == 224 && next_byte < 160) || (n == 237 && next_byte >= 160) || (n == 240 && next_byte < 144) || (n == 244 && next_byte >= 144))) return 0
                    raw = raw c
                }
                result = result raw
            }
        }
    }
    return 0
}
function parse_object(depth,    object_id, key, c) {
    if (depth > 64 || substr(document, position++, 1) != "{") return 0
    object_id = ++object_count
    skip_space()
    if (substr(document, position, 1) == "}") { position++; kind = "object"; return 1 }
    while (position <= length(document)) {
        if (!parse_string()) return 0
        key = scalar
        if ((object_id SUBSEP key) in seen) return 0
        seen[object_id SUBSEP key] = 1
        skip_space()
        if (substr(document, position++, 1) != ":") return 0
        skip_space()
        if (!parse_value(depth + 1)) return 0
        if (depth == 0 && key == wanted) { selected_found = 1; selected_kind = kind; selected = scalar }
        skip_space()
        c = substr(document, position++, 1)
        if (c == "}") { kind = "object"; return 1 }
        if (c != ",") return 0
        skip_space()
    }
    return 0
}
function parse_array(depth,    c) {
    if (depth > 64 || substr(document, position++, 1) != "[") return 0
    skip_space()
    if (substr(document, position, 1) == "]") { position++; kind = "array"; return 1 }
    while (position <= length(document)) {
        if (!parse_value(depth + 1)) return 0
        skip_space()
        c = substr(document, position++, 1)
        if (c == "]") { kind = "array"; return 1 }
        if (c != ",") return 0
        skip_space()
    }
    return 0
}
function parse_value(depth,    c, rest) {
    if (depth > 64) return 0
    c = substr(document, position, 1)
    if (c == "{") return parse_object(depth)
    if (c == "[") return parse_array(depth)
    if (c == "\"") { if (!parse_string()) return 0; kind = "string"; return 1 }
    rest = substr(document, position)
    if (substr(rest, 1, 4) == "true" || substr(rest, 1, 4) == "null") { position += 4; kind = "scalar"; return 1 }
    if (substr(rest, 1, 5) == "false") { position += 5; kind = "scalar"; return 1 }
    if (match(rest, /^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?/)) { position += RLENGTH; kind = "scalar"; return 1 }
    return 0
}
