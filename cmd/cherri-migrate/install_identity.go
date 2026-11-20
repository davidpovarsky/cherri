package main

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime/debug"

	"github.com/electrikmilk/cherri/internal/language/protocol"
)

// The installer reads embedded build provenance through the shipped migration
// tool, so prebuilt installations do not need a second Go toolchain or Python.
// Commit sidecars remain receipts; they never supply the binary's identity.
type migrationBuildIdentity struct {
	Tool         string `json:"tool"`
	ModulePath   string `json:"modulePath"`
	SourceCommit string `json:"sourceCommit"`
	BuildState   string `json:"buildState"`
	GOOS         string `json:"goos"`
	GOARCH       string `json:"goarch"`
	SHA256       string `json:"sha256,omitempty"`
}

var migrationSourceCommit = regexp.MustCompile(`^[0-9a-f]{40}$`)

func migrationIdentity(info *debug.BuildInfo) (migrationBuildIdentity, error) {
	var result migrationBuildIdentity
	if info == nil {
		return result, fmt.Errorf("[CHERRI_TOOL_IDENTITY_INVALID] embedded Go build identity is missing")
	}
	result.ModulePath = info.Path
	switch info.Path {
	case "github.com/electrikmilk/cherri":
		result.Tool = "cherri"
	case "github.com/electrikmilk/cherri/cmd/cherri-migrate":
		result.Tool = "cherri-migrate"
	default:
		return result, fmt.Errorf("[CHERRI_TOOL_ROLE_MISMATCH] unsupported tool role/main package: %s", info.Path)
	}
	settings := make(map[string]string, len(info.Settings))
	for _, setting := range info.Settings {
		if _, duplicate := settings[setting.Key]; duplicate {
			return result, fmt.Errorf("[CHERRI_TOOL_IDENTITY_INVALID] duplicate build setting %s", setting.Key)
		}
		settings[setting.Key] = setting.Value
	}
	if settings["vcs"] != "git" || settings["vcs.modified"] != "false" || !migrationSourceCommit.MatchString(settings["vcs.revision"]) {
		return result, fmt.Errorf("[CHERRI_TOOL_IDENTITY_INVALID] exact clean Git source revision is required")
	}
	if settings["GOOS"] == "" || settings["GOARCH"] == "" {
		return result, fmt.Errorf("[CHERRI_TOOL_IDENTITY_INVALID] binary platform identity is missing")
	}
	result.SourceCommit, result.BuildState = settings["vcs.revision"], "clean"
	result.GOOS, result.GOARCH = settings["GOOS"], settings["GOARCH"]
	return result, nil
}

func emitMigrationCapabilities(writer io.Writer) error {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fmt.Errorf("[CHERRI_TOOL_IDENTITY_INVALID] running migration build identity is missing")
	}
	identity, err := migrationIdentity(info)
	if err != nil {
		return err
	}
	if identity.Tool != "cherri-migrate" {
		return fmt.Errorf("[CHERRI_TOOL_ROLE_MISMATCH] running tool role is not cherri-migrate")
	}
	caps := protocol.BuildCapabilities(nil)
	response := struct {
		migrationBuildIdentity
		LanguageVersion      string `json:"languageVersion"`
		CatalogSchemaVersion string `json:"catalogSchemaVersion"`
		ProtocolVersion      string `json:"protocolVersion"`
		SchemaFingerprint    string `json:"schemaFingerprint"`
		CodecFingerprint     string `json:"codecFingerprint"`
		MacroFingerprint     string `json:"macroFingerprint"`
	}{identity, caps.LanguageVersion, caps.CatalogSchemaVersion, caps.ProtocolVersion,
		caps.SchemaFingerprint, caps.CodecFingerprint, caps.MacroFingerprint}
	return json.NewEncoder(writer).Encode(response)
}

func emitInspectedBuildIdentity(writer io.Writer, path string) error {
	// Open once: the identity and digest refer to the same file object even if
	// an unrelated process renames a different candidate onto this path.
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("[CHERRI_TOOL_IDENTITY_INVALID] open binary: %w", err)
	}
	defer file.Close()
	info, err := buildinfo.Read(file)
	if err != nil {
		return fmt.Errorf("[CHERRI_TOOL_IDENTITY_INVALID] read embedded Go build identity: %w", err)
	}
	identity, err := migrationIdentity(info)
	if err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	identity.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return json.NewEncoder(writer).Encode(identity)
}
