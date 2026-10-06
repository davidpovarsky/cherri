package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

func main() {
	reg := schema.DefaultRegistry()
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
					if p.DefaultValue != "" {
						defStr = fmt.Sprintf("`%s`", p.DefaultValue)
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

	outPath := filepath.Join("docs", "language-v2", "actions-reference.md")
	if len(os.Args) > 1 {
		outPath = os.Args[1]
	}

	if err := os.WriteFile(outPath, []byte(b.String()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing actions reference: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s successfully (%d actions).\n", outPath, len(actions))
}

func buildSignature(a *schema.ActionSchema) string {
	var parts []string
	if prim, ok := a.PrimaryParameter(); ok {
		parts = append(parts, fmt.Sprintf("%s: %s", prim.Label, prim.TypeName))
	}
	for _, p := range a.Parameters {
		if a.PrimaryParameterID != "" && p.ID == a.PrimaryParameterID {
			continue
		}
		opt := ""
		if p.Optional {
			opt = "?"
		}
		def := ""
		if p.DefaultValue != "" {
			def = fmt.Sprintf(" = %q", p.DefaultValue)
		}
		parts = append(parts, fmt.Sprintf("%s%s: %s%s", p.Label, opt, p.TypeName, def))
	}
	ret := ""
	if a.OutputTypeName != "" && a.OutputTypeName != "Void" && a.OutputTypeName != "Unknown" {
		ret = " -> " + a.OutputTypeName
	}
	return fmt.Sprintf("%s(%s)%s", a.CallableName, strings.Join(parts, ", "), ret)
}
