/*
 * Copyright (c) Cherri
 * Structural comparator delegated to shared internal/shortcutcompare
 */

package main

import (
	"encoding/json"
	"fmt"

	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

type Difference = shortcutcompare.Difference
type ComparisonResult = shortcutcompare.ComparisonResult

func CompareShortcuts(pathA, pathB string) (*ComparisonResult, error) {
	return shortcutcompare.CompareFiles(pathA, pathB)
}

func printComparison(result *ComparisonResult, asJSON bool) {
	if asJSON {
		encoded, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(encoded))
		return
	}
	if result.Equal {
		fmt.Println("Structurally equal.")
		return
	}
	fmt.Printf("Structural differences (%d):\n", len(result.Differences))
	for _, difference := range result.Differences {
		fmt.Printf("  %-12s %s | a=%v b=%v\n", difference.Kind, difference.Path, difference.A, difference.B)
	}
	if result.Truncated {
		fmt.Println("  ... additional differences truncated")
	}
}
