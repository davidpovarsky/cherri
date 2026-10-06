package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

var (
	writeFlag = flag.Bool("write", false, "Write migrated content back to input file in-place")
	checkFlag = flag.Bool("check", false, "Verify whether input file can be cleanly migrated and parsed")
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("Usage: cherri-migrate <file.cherri> [--write] [--check]")
		os.Exit(1)
	}

	filePath := args[0]
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", filePath, err)
		os.Exit(1)
	}

	migrated := migrate.MigrateSource(string(content))

	if *checkFlag {
		file := source.NewFile(source.DocumentID(filePath), filePath, 1, migrated)
		parser := syntax.NewParser(file)
		_ = parser.ParseProgram()
		if len(parser.Errors()) > 0 {
			fmt.Fprintf(os.Stderr, "Migration check failed for %s: %v\n", filePath, parser.Errors())
			os.Exit(1)
		}
		fmt.Printf("File %s can be migrated cleanly.\n", filePath)
		return
	}

	if *writeFlag {
		if err := os.WriteFile(filePath, []byte(migrated), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", filePath, err)
			os.Exit(1)
		}
		fmt.Printf("Migrated and updated %s successfully.\n", filePath)
		return
	}

	fmt.Print(migrated)
}
