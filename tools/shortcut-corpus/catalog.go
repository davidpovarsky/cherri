/*
 * Copyright (c) Cherri
 */

package main

import (
	"fmt"
	"os"
	"os/exec"
)

// obtainCatalog loads Cherri's machine-readable action catalog, preferring an
// explicit file. The analyzer compares only against this published interface,
// never against compiler internals.
func obtainCatalog(config analyzeConfig) (*actionCatalog, error) {
	if config.catalogPath != "" {
		return loadCatalogFromFile(config.catalogPath)
	}
	if config.cherriBin == "" {
		return &actionCatalog{
			byIdentifier: map[string][]*catalogEntry{},
			knownKeys:    map[string]map[string]bool{},
			parameters:   map[string]map[string][]catalogParameter{},
			source:       "none",
		}, nil
	}

	output, err := exec.Command(config.cherriBin, "--actions-json").Output()
	if err != nil {
		return nil, fmt.Errorf(
			"unable to run %q --actions-json (use -catalog or -cherri): %w",
			config.cherriBin, err,
		)
	}
	return parseCatalog(output, config.cherriBin+" --actions-json")
}

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file does not exist")
		}
		return nil, err
	}
	return data, nil
}
