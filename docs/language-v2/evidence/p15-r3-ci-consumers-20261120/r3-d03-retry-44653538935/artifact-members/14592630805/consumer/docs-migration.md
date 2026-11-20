---
title: Migration Guide
parent: Language v2.0
layout: default
nav_order: 3
permalink: /fork-language/migration
---

# Migrating from Cherri v1 to v2.0
{: .no_toc }

This guide explains how to migrate existing Cherri codebases to Cherri Language v2.0.

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## 1. Automated Migration with `cherri-migrate`

Cherri v2 includes an AST-aware migration CLI tool: `cherri-migrate`.

### Usage

```bash
# Preview migrated output to stdout
cherri-migrate my_shortcut.cherri

# Apply migration changes directly to the file
cherri-migrate my_shortcut.cherri --write
```

The migration tool automatically:
1. Replaces `@var` declarations with `let` or `var` based on reassignment analysis.
2. Removes legacy `#include` directives.
3. Upgrades interpolation strings `"Hello {name}"` to `f"Hello {name}"`.
4. Canonicalizes formatting.

---

## 2. Before and After Examples

### Variable Declarations

**Cherri v1 (Legacy reference):**
```cherri-legacy
@name = "Alice"
const greeting = "Hello"
@count = 0
@count = @count + 1
```

**Cherri v2.0 (Modern):**
```cherri
let name = "Alice"
let greeting = "Hello"
var count = 0
count = count + 1
```

---

### String Interpolation

**Cherri v1 (Legacy reference):**
```cherri-legacy
@city = "Tel Aviv"
show("Welcome to {city}!")
```

**Cherri v2.0 (Modern):**
```cherri
let city = "Tel Aviv"
show(f"Welcome to {city}!")
```

Plain strings `"Welcome to {city}!"` now preserve literal braces. To interpolate, prepend `f` to the string literal.

---

### Action Calls & Named Arguments

**Cherri v1 (Legacy reference):**
```cherri-legacy
resizeImage(@photo, 800, 600)
```

**Cherri v2.0 (Modern):**
```cherri
let photo = selectPhotos()
resizeImage(photo, width: 800, height: 600)
```

In Cherri v2, the primary argument is passed positionally and subsequent arguments use parameter labels for clarity and deterministic binding.

---

### Indexing

**Cherri v1 (Legacy reference):**
```cherri-legacy
@first = @list[1] // 1-based indexing
```

**Cherri v2.0 (Modern):**
```cherri
let values = ["first", "second"]
let first = values[0] // 0-based indexing
```

Collections and lists now consistently use standard **0-based indexing**.

---

## 3. Legacy Syntax Diagnostic Codes

If you attempt to compile Cherri v1 code with `cherri` v2.0, the compiler will emit explicit diagnostic errors:

| Legacy Syntax | Diagnostic Code | Remediation |
|---------------|-----------------|-------------|
| `@var` | `E_LEGACY_SYNTAX` | Replace with `let var = ...` or `var var = ...` |
| `const var` | `E_LEGACY_SYNTAX` | Replace with `let var = ...` |
| `#include "..."` | `E_LEGACY_SYNTAX` | Remove `#include` |

Run `cherri check <file>` to view all diagnostics with file locations and remediation hints.

## Executable migration diagnostic

The following example intentionally uses historical syntax on the default V2
path. It is tested for the exact diagnostic below. Use the explicitly selected
`--lang=legacy` path only when you intend to compile a Legacy program.

<!-- cherri-test: diagnostic E_LEGACY_SYNTAX -->
```cherri
@value = "historical"
```
