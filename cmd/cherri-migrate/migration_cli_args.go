package main

import "strings"

// The documented interface places options after the input file. Go's standard
// flag parser stops at the first positional argument, so give that parser the
// option tokens first while preserving their order and its validation. The
// explicit -- delimiter still makes all following tokens literal paths.
func migrationFlagArguments(arguments []string) []string {
	options := make([]string, 0, len(arguments))
	paths := make([]string, 0, 1)
	literal := false
	for _, argument := range arguments {
		if !literal && argument == "--" {
			literal = true
			continue
		}
		if !literal && argument != "-" && strings.HasPrefix(argument, "-") {
			options = append(options, argument)
		} else {
			paths = append(paths, argument)
		}
	}
	return append(append(options, "--"), paths...)
}
