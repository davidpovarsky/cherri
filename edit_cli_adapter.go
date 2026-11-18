package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

func handleEditVerificationCLI() bool {
	if len(os.Args) < 2 || os.Args[1] != "verify-edit" {
		return false
	}
	var paths []string
	var policy string
	asJSON, seenPolicy := false, false
	invalid := func(message string) {
		fmt.Fprintln(os.Stderr, "[CHERRI_EDIT_USAGE] "+message)
		os.Exit(2)
	}
	for i := 2; i < len(os.Args); i++ {
		argument := os.Args[i]
		switch {
		case argument == "--json":
			if asJSON {
				invalid("duplicate --json")
			}
			asJSON = true
		case argument == "--allowed-delta" || strings.HasPrefix(argument, "--allowed-delta="):
			if seenPolicy {
				invalid("duplicate --allowed-delta")
			}
			seenPolicy = true
			if argument == "--allowed-delta" {
				if i+1 >= len(os.Args) {
					invalid("--allowed-delta requires a policy file")
				}
				i++
				policy = os.Args[i]
			} else {
				policy = strings.TrimPrefix(argument, "--allowed-delta=")
			}
			if policy == "" {
				invalid("--allowed-delta requires a policy file")
			}
		case strings.HasPrefix(argument, "-"):
			invalid("unknown option " + argument)
		default:
			paths = append(paths, argument)
		}
	}
	if len(paths) != 2 {
		invalid("Usage: cherri verify-edit ORIGINAL CANDIDATE [--allowed-delta POLICY.json] [--json]")
	}
	result, err := shortcutcompare.VerifyEditFiles(paths[0], paths[1], policy)
	if err != nil {
		if asJSON {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": false, "code": "CHERRI_EDIT_INVALID", "error": err.Error()})
		}
		fmt.Fprintln(os.Stderr, "[CHERRI_EDIT_INVALID] "+err.Error())
		os.Exit(1)
	}
	if asJSON {
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else if result.OK {
		fmt.Println("Verified exact document preservation and explicit allowed field changes.")
	}
	if !result.OK {
		fmt.Fprintln(os.Stderr, "[CHERRI_EDIT_DELTA_MISMATCH] Candidate differs outside the declared field replacements.")
		os.Exit(1)
	}
	return true
}
