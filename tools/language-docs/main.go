package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

func renderReference(reg *schema.Registry) string {
	actions := reg.AllActions()

	// Group actions by module/category
	modules := make(map[string][]*schema.ActionSchema)
	for _, a := range actions {
		mod := a.Module
		if mod == "" {
			mod = a.Docs.Category
		}
		if mod == "" {
			mod = "general"
		}
		modules[mod] = append(modules[mod], a)
	}

	moduleNames := make([]string, 0, len(modules))
	for m := range modules {
		moduleNames = append(moduleNames, m)
	}
	sort.Strings(moduleNames)

	var b strings.Builder
	b.WriteString("# Cherri Language v2.0 — Action Reference\n\n")
	fmt.Fprintf(&b, "**Language Version:** %s  \n", schema.LanguageVersion)
	fmt.Fprintf(&b, "**Catalog Schema:** %s  \n", schema.CatalogSchemaVersion)
	fmt.Fprintf(&b, "**Schema Fingerprint:** `%s`  \n", reg.Fingerprint())
	fmt.Fprintf(&b, "**Total Actions:** %d  \n\n", len(actions))
	b.WriteString("This documentation is generated automatically from the canonical Cherri v2 ActionSchema registry.\n\n")

	b.WriteString("## Table of Modules\n\n")
	for _, mod := range moduleNames {
		fmt.Fprintf(&b, "- [%s](#module-%s) (%d actions)\n", strings.Title(mod), strings.ToLower(mod), len(modules[mod]))
	}
	b.WriteString("\n---\n\n")

	for _, mod := range moduleNames {
		modActions := modules[mod]
		fmt.Fprintf(&b, "## Module: %s\n\n", strings.Title(mod))

		for _, a := range modActions {
			fmt.Fprintf(&b, "### `%s`\n\n", a.CallableName)
			if a.Docs.Title != "" {
				fmt.Fprintf(&b, "**Title:** %s  \n", a.Docs.Title)
			}
			fmt.Fprintf(&b, "**Apple Identifier:** `%s`  \n", a.AppleIdentifier)
			if a.OutputTypeName != "" && a.OutputTypeName != "Unknown" {
				fmt.Fprintf(&b, "**Returns:** `%s`  \n", a.OutputTypeName)
			}
			if a.Docs.Description != "" {
				fmt.Fprintf(&b, "\n%s\n\n", a.Docs.Description)
			}

			// Signature
			sig := buildSignature(a)
			fmt.Fprintf(&b, "```cherri\n%s\n```\n\n", sig)

			// Parameters table
			if len(a.Parameters) > 0 {
				b.WriteString("| Parameter | Type | Required | Default | Wire Key | Choices |\n")
				b.WriteString("|-----------|------|----------|---------|----------|---------|\n")
				for _, p := range a.Parameters {
					reqStr := "Yes"
					if p.Optional {
						reqStr = "No"
					}
					defStr := "-"
					if p.DefaultValue != nil {
						defStr = fmt.Sprintf("`%s`", *p.DefaultValue)
						if *p.DefaultValue == "" {
							defStr = "`\"\"`"
						}
					}
					choicesStr := "-"
					if len(p.EnumValues) > 0 {
						choicesStr = strings.Join(p.EnumValues, ", ")
					}
					fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s | `%s` | %s |\n", p.Label, p.TypeName, reqStr, defStr, p.WireKey, choicesStr)
				}
				b.WriteString("\n")
			}

			if a.AppIntent != nil {
				fmt.Fprintf(&b, "**App Intent:** `%s` (Bundle: `%s`)\n\n", a.AppIntent.AppIntentIdentifier, a.AppIntent.BundleIdentifier)
			}
			b.WriteString("---\n\n")
		}
	}

	writeCompilerMacros(&b, reg)
	return b.String()
}

func main() {
	check := flag.Bool("check", false, "verify the complete existing reference without writing")
	flag.Parse()
	if flag.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "usage: language-docs [--check] [reference-path]")
		os.Exit(2)
	}
	reg := schema.DefaultRegistry()
	reference := renderReference(reg)
	outPath := filepath.Join("docs", "language-v2", "actions-reference.md")
	if flag.NArg() == 1 {
		outPath = flag.Arg(0)
	}
	if *check {
		data, err := os.ReadFile(outPath)
		if err != nil || string(data) != reference {
			fmt.Fprintln(os.Stderr, "Generated action reference differs from the complete assembled schema:", outPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			os.Exit(1)
		}
		fmt.Printf("Verified complete reference %s (%d native definitions; %d compiler macros).\n", outPath, len(reg.AllActions()), len(reg.CompilerMacros()))
		return
	}
	if err := os.WriteFile(outPath, []byte(reference), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing actions reference: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Generated %s successfully (%d actions).\n", outPath, len(reg.AllActions()))
}

func buildSignature(a *schema.ActionSchema) string {
	var order []int
	primary := -1
	for i, p := range a.Parameters {
		if a.PrimaryParameterID != "" && p.ID == a.PrimaryParameterID {
			primary = i
			order = append(order, i)
			break
		}
	}
	for i := range a.Parameters {
		if i != primary {
			order = append(order, i)
		}
	}
	var parts []string
	for _, index := range order {
		p := a.Parameters[index]
		optional, def := "", ""
		if p.Optional {
			optional = "?"
		}
		if p.DefaultValue != nil {
			def = " = " + formatDefault(p.TypeName, *p.DefaultValue)
		}
		parts = append(parts, fmt.Sprintf("%s%s: %s%s", p.Label, optional, p.TypeName, def))
	}
	ret := ""
	if a.OutputTypeName != "" && a.OutputTypeName != "Void" && a.OutputTypeName != "Unknown" {
		ret = " -> " + a.OutputTypeName
	}
	block := ""
	if a.ScopedBlock {
		block = " { <#body#> }"
	}
	return fmt.Sprintf("%s(%s)%s%s", a.CallableName, strings.Join(parts, ", "), block, ret)
}

// Defaults are registry metadata, not executable snippets. Preserve typed Bool
// and Number literals; Text and opaque symbolic defaults remain quoted.
func formatDefault(kind, value string) string {
	switch strings.ToLower(kind) {
	case "bool", "boolean":
		if value == "true" || value == "false" {
			return value
		}
	case "number":
		decoder := json.NewDecoder(strings.NewReader(value))
		decoder.UseNumber()
		var decoded any
		if json.Valid([]byte(value)) && decoder.Decode(&decoded) == nil {
			if _, ok := decoded.(json.Number); ok {
				return value
			}
		}
	}
	return fmt.Sprintf("%q", value)
}

func writeCompilerMacros(b *strings.Builder, reg *schema.Registry) {
	macros := reg.CompilerMacros()
	b.WriteString("## Compiler macros\n\n")
	fmt.Fprintf(b, "**Compiler Macro Fingerprint:** \x60%s\x60\n\n", reg.CompilerMacroFingerprint())
	fmt.Fprintf(b, "**Compiler Macros:** %d\n\n", len(macros))
	b.WriteString("These compiler expansions are listed separately from the native Apple catalog. Signatures below describe V2 named arguments; Legacy uses its existing positional forms. Each scoped construct describes its supported dialects and body result below. A metadata entry does not certify Apple runtime behavior.\n\n")
	b.WriteString("Build profile \x60--build-profile=debug|release\x60 defaults to debug. Release removes explicitly diagnostic expansions, preserves body effects and semantic guards, and rejects consumed diagnostic outputs.\n\n")
	for _, m := range macros {
		fmt.Fprintf(b, "### \x60%s\x60\n\n%s\n\n", m.CallableName, m.Docs.Description)
		fmt.Fprintf(b, "\x60\x60\x60cherri\n%s\n\x60\x60\x60\n\n", buildSignature(m))
		if len(m.Parameters) > 0 {
			b.WriteString("| Parameter | Type | Optional | Literal required | Default |\n|---|---|---|---|---|\n")
			for _, p := range m.Parameters {
				var def = "-"
				if p.DefaultValue != nil {
					def = *p.DefaultValue
					if def == "" {
						def = "\"\""
					}
				}
				fmt.Fprintf(b, "| \x60%s\x60 | \x60%s\x60 | %t | %t | %s |\n", p.Label, p.TypeName, p.Optional, p.Literal, def)
			}
			b.WriteString("\n")
		}
	}
}
