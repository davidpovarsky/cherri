package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

var (
	writeFlag = flag.Bool("write", false, "Write migrated content back to input file in-place")
	checkFlag = flag.Bool("check", false, "Verify whether input file can be cleanly migrated and parsed")
)

func atomicWriteFile(filePath string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filePath)
	tmpFile, err := os.CreateTemp(dir, "cherri-atomic-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write data: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync data: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("failed to atomically rename %s to %s: %w", tmpPath, filePath, err)
	}
	tmpPath = ""
	return nil
}

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

	migrated, diags, err := migrate.MigrateSourceWithDiagnostics(string(content))
	if err != nil {
		for _, d := range diags {
			fmt.Fprintf(os.Stderr, "Migration error: %s\n", d)
		}
		fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
		os.Exit(1)
	}

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
		if err := atomicWriteFile(filePath, []byte(migrated), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", filePath, err)
			os.Exit(1)
		}
		fmt.Printf("Migrated and updated %s successfully.\n", filePath)
		return
	}

	fmt.Print(migrated)
}
