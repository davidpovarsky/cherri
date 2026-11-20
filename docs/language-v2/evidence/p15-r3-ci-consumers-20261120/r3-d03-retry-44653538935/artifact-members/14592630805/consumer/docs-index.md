---
title: Language v2.0
layout: default
nav_order: 5
has_children: true
permalink: /fork-language/
---

# Cherri Language v2.0 Guide

Welcome to the Cherri Language v2.0 active authoring guide.

The default public authoring path is V2. Existing Legacy programs remain available through the explicit --lang=legacy route; this section documents V2.

Cherri v2.0 is an expressive, safe, strongly-typed language for authoring native Apple Shortcuts. It replaces legacy ad-hoc syntax with formal lexical and grammar rules, first-class interpolation, block-scoped variables, static typing, and direct native plist preservation.

## Highlights of Language v2.0

- **Clean Modern Syntax**: Declarative `let` (immutable) and `var` (mutable) bindings instead of `@variables`.
- **First-Class Interpolation**: Clean `f"Hello, {name}!"` string interpolation preserving Apple's `WFTextTokenAttachment` natively.
- **Strong Static Typing**: Type checking with clear, actionable diagnostics (`E_TYPE_MISMATCH`, `E_UNDEFINED_SYMBOL`, `E_ARG_UNKNOWN`, etc.).
- **Deterministic Action Binding**: Positional, named, and primary parameters resolved deterministically against the Action Registry.
- **Rich Control Flow**: `if` / `else if` / `else`, `repeat(n)`, `for item in list`, and `menu` statements.
- **Safety First**: Deprecated legacy constructs (`#include`, `@var`, `const`) produce explicit migration errors pointing to modern replacements.

## Key Sections

- [Language Guide]({{ '/fork-language/guide' | relative_url }}) - Comprehensive language reference, syntax, types, control flow, functions, and diagnostics.
- [Actions Reference]({{ '/fork-language/actions' | relative_url }}) - Generated reference for 505 canonical definitions: 497 callable entries and 8 compiler constructs. The 15 compiler macros are listed separately; catalog membership does not certify Apple runtime behavior.
- [Migration Guide]({{ '/fork-language/migration' | relative_url }}) - Guide for migrating existing Cherri v1 shortcuts to v2.0.

{: .note }
For historical reference, the legacy upstream Cherri v1 documentation remains archived under [Legacy Language Docs]({{ '/language/' | relative_url }}).
