# Cherri Language v2.0 — Language Guide

**Version:** 2.0  
**Status:** Canonical Active Language  

Cherri Language v2.0 is a modern, statically typed language that compiles directly to Apple Shortcuts plists (`.shortcut`). It eliminates legacy DSL ambiguities, replaces ad-hoc preprocessing with a true AST and semantic IR, and provides deterministic tooling for CLI, LSP, and iOS applications.

---

## 1. Quick Overview & What's New in v2.0

| Feature | Legacy Cherri | Cherri v2.0 |
|---------|---------------|-------------|
| **Variable Declaration** | `@var = value` or `const var = value` | `let var = value` (immutable), `var var = value` (mutable) |
| **String Interpolation** | `"Hello {var}"` (ad-hoc braces) | `f"Hello {var}"` (true parsed expressions inside `{}`) |
| **Raw Strings** | Not supported | `r"\d+\s+"` (no escape parsing) |
| **Multiline Strings** | Not supported | `"""line 1\nline 2"""` |
| **Function Declarations** | Header dispatch hack | `function name(params) -> ReturnType { ... }` (AST-first) |
| **Call Arguments** | Unlabeled positional only | Primary argument + named labels: `resizeImage(photo, width: 800)` |
| **List/Collection Indexing** | 1-based indexing | **0-based indexing** (`list[0]`) |
| **Block Expressions** | Incomplete | `if`, `repeat`, `for`, `menu` yield values via `yield` |
| **Native Escapes** | Raw action syntax | `native.action`, `native.plist`, `native.ref`, `native.workflow` |
| **Tooling** | Compiler only | `cherri check --json`, `cherri format --write`, `cherri lsp`, `--capabilities-json` |

---

## 2. Variables and Bindings

### Immutable Bindings (`let`)
Variables defined with `let` are immutable. They cannot be reassigned.
```cherri
let name = "Alice"
let count = 42
// name = "Bob"  // Compile Error: E_ASSIGN_IMMUTABLE
```

### Mutable Bindings (`var`)
Variables defined with `var` can be reassigned with `=` or compound assignment operators (`+=`, `-=`, `*=`, `/=`):
```cherri
var counter = 0
counter += 1
counter = 10
```

### Type Annotations
Types are inferred automatically, but optional explicit type annotations can be supplied:
```cherri
let message: Text = "Welcome"
var total: Number = 0
var photos: List<Image> = []
```

---

## 3. Strings & Text

### Plain Strings
Enclosed in double quotes. Braces `{}` in plain strings are literal and are NOT interpolated.
```cherri
let greeting = "Hello, world!"
let jsonTemplate = "{\"key\": \"value\"}" // braces are literal
```

### Interpolated Strings (`f"..."`)
Prefix with `f`. Expressions inside `{}` are parsed and evaluated. Literal braces are escaped as `{{` and `}}`:
```cherri
let name = "David"
let count = 5
let message = f"User {name} has {count} items. Literal brace: {{example}}"
```

### Raw Strings (`r"..."`)
Prefix with `r`. Backslashes are treated as literal characters:
```cherri
let regex = r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b"
```

### Multiline Strings (`"""..."""`)
Enclosed in triple quotes:
```cherri
let sql = """SELECT id, name, email
FROM users
WHERE active = true"""
```

---

## 4. Types and Collections

### Primitive Types
- `Text`
- `Number` (both integers and floating-point values)
- `Bool` (`true`, `false`)

### Content Types
Native Apple Shortcuts content items:
- `Image`
- `File`
- `URL`
- `Date`
- `Contact`
- `CalendarEvent`
- `AnyContent` (arbitrary native Shortcut content)

### Collection Types
- **List:** `List<T>`, literal `[item1, item2, item3]`. Indexed starting at **0** (`items[0]`).
- **Map:** `Map<Text, V>`, literal `{"key": value}`.
- **Record:** Anonymous structural records `{ name: Text, age: Number }`.
- **Optional:** `T?`, e.g., `Text?` or `Number?`. Represented by `none` when absent.
- **Union:** Small type unions `Text | Number`.

---

## 5. Functions & Actions

### Calling Actions
Actions in Cherri v2 accept an optional primary argument without label, followed by named arguments:
```cherri
// Primary argument + named arguments
resizeImage(photo, width: 800, height: 600)

// Named arguments only
alert(message: "Operation completed!", title: "Success")

// Unlabeled primary argument
show("Hello from Cherri v2!")
```

### User-Defined Functions
```cherri
function calculateDiscount(price: Number, percentage: Number = 10) -> Number {
    let discount = price * (percentage / 100)
    return price - discount
}

let finalPrice = calculateDiscount(100, percentage: 15)
```

---

## 6. Control Flow

### Conditionals (`if` / `else`)
```cherri
if count > 10 {
    show("More than 10 items")
} else if count > 0 {
    show("Some items")
} else {
    show("No items")
}
```

### Value-Producing `if`
```cherri
let statusText = if count > 0 {
    yield f"{count} items remaining"
} else {
    yield "Out of stock"
}
```

### Counting Loops (`repeat`)
```cherri
repeat 3 as i {
    show(f"Iteration: {i}")
}
```

### Iteration Loops (`for ... in`)
```cherri
let names = ["Alice", "Bob", "Charlie"]

// Item only
for item in names {
    show(item)
}

// Index (0-based) and item
for (index, item) in names {
    show(f"#{index}: {item}")
}
```

### Menus (`menu`)
```cherri
menu("Select an action") {
    case "Option 1" {
        show("Selected 1")
    }
    case "Option 2" {
        show("Selected 2")
    }
}
```

---

## 7. Metadata and Shortcut Headers

Declare shortcut properties using the `shortcut` declaration:
```cherri
shortcut "Morning Routine" {
    icon: { glyph: 59789, color: 4282601983 }
    trigger time(event: .sunrise)
}
```

### Setup Questions
```cherri
setup apiKey: Text {
    prompt: "Enter your API Key"
    default: "sk-demo"
}
```

---

## 8. Native Escape Hatches

When exact plist keys or undocumented actions are required, use `native`:
```cherri
let result = native.action(
    identifier: "com.apple.custom.action",
    parameters: {
        "CustomKey": "CustomValue",
        "Threshold": 100
    }
)
```

---

## 9. CLI Tools

### Check Code
Analyzes syntax and types without compiling:
```bash
cherri check my_shortcut.cherri
cherri check my_shortcut.cherri --json
```

### Format Code
Formats code using canonical Cherri style:
```bash
cherri format my_shortcut.cherri
cherri format my_shortcut.cherri --write
```

### Compiler Capabilities & Schema Metadata
Inspect supported features and action catalog fingerprint:
```bash
cherri --capabilities-json
```

### Language Server (LSP)
Starts a standard JSON-RPC Language Server on stdin/stdout:
```bash
cherri lsp
```

---

## 10. Migration from Legacy Cherri

Use the official `cherri-migrate` tool to update legacy files:
```bash
cherri-migrate legacy.cherri --write
```

The migration tool:
1. Converts `@variable = ...` to `let` or `var` based on reassignment analysis
2. Strips legacy `#include` statements
3. Converts string interpolation `"Count {count}"` to `f"Count {count}"`
4. Formats code canonically.
