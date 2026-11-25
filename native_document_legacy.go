package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/electrikmilk/cherri/internal/language/assets"
	"github.com/electrikmilk/cherri/internal/language/workspace"
)

// Only the document-aware mobile call installs this state, while owning both
// compiler mutexes. It never changes cwd, legacy CLI roots or global permissions.
type nativeLegacyDocumentState struct {
	document         mobileDocumentContext
	entry            string
	cache            *assets.Cache
	root             *workspace.Root
	scope            *assets.Scope
	lineOrigins      [][]string
	copies           map[string][]string
	generatedOrigins map[int]string
}

var nativeLegacyDocument *nativeLegacyDocumentState

const nativeGeneratedDocument = "\x00generated-no-file-authority"

func (state *nativeLegacyDocumentState) close() {
	if state.scope != nil {
		_ = state.scope.Close()
	}
	if state.root != nil {
		_ = state.root.Close()
	}
}

func nativeLegacyInitializeLines() {
	if nativeLegacyDocument == nil {
		return
	}
	state := nativeLegacyDocument
	origin := nativeGeneratedDocument
	if state.entry != "" {
		origin = filepath.ToSlash(filepath.Base(state.entry))
	}
	state.lineOrigins = make([][]string, len(lines))
	for i := range lines {
		state.lineOrigins[i] = []string{origin}
	}
	state.copies = make(map[string][]string)
}

func nativeLegacyNormalizeLines() {
	if nativeLegacyDocument == nil {
		return
	}
	state := nativeLegacyDocument
	if len(state.lineOrigins) != len(lines) {
		parserError("E_WORKSPACE_ACCESS: native source origin shape changed without provenance")
	}
	flattened := make([][]string, 0, len(lines))
	for i, line := range lines {
		normalized := strings.ReplaceAll(strings.ReplaceAll(line, "\r\n", "\n"), "\r", "\n")
		count := strings.Count(normalized, "\n") + 1
		origins := state.lineOrigins[i]
		if len(origins) != count {
			parserError("E_WORKSPACE_ACCESS: expanded native source has ambiguous origins")
		}
		for _, origin := range origins {
			flattened = append(flattened, []string{origin})
		}
	}
	state.lineOrigins = flattened
}

func nativeLegacyPrependLines(chunks []string, origins [][]string) {
	if nativeLegacyDocument == nil {
		return
	}
	if origins == nil {
		origins = make([][]string, len(chunks))
		for i, chunk := range chunks {
			origins[i] = make([]string, strings.Count(chunk, "\n")+1)
			for j := range origins[i] {
				origins[i][j] = nativeGeneratedDocument
			}
		}
	}
	if len(origins) != len(chunks) {
		parserError("E_WORKSPACE_ACCESS: generated prefix lost source origins")
	}
	nativeLegacyDocument.lineOrigins = append(origins, nativeLegacyDocument.lineOrigins...)
}

func nativeLegacyOriginAt(line int) string {
	if nativeLegacyDocument == nil || line < 0 || line >= len(nativeLegacyDocument.lineOrigins) {
		return nativeGeneratedDocument
	}
	origin := nativeLegacyDocument.lineOrigins[line]
	if len(origin) != 1 {
		return nativeGeneratedDocument
	}
	return origin[0]
}

func nativeLegacyReplaceLineOrigins(line int, origins []string) {
	if nativeLegacyDocument == nil {
		return
	}
	if line < 0 || line >= len(nativeLegacyDocument.lineOrigins) {
		parserError("E_WORKSPACE_ACCESS: source replacement lost its origin")
	}
	nativeLegacyDocument.lineOrigins[line] = append([]string(nil), origins...)
}

func nativeLegacyRepeatedOrigin(origin string, count int) []string {
	result := make([]string, count)
	for i := range result {
		result[i] = origin
	}
	return result
}

func nativeLegacyCaptureBodyOrigins(raw string, startLine int) []string {
	if nativeLegacyDocument == nil {
		return nil
	}
	trimmedLeft := strings.TrimLeftFunc(raw, unicode.IsSpace)
	prefix := raw[:len(raw)-len(trimmedLeft)]
	first := startLine + strings.Count(prefix, "\n")
	count := strings.Count(strings.TrimSpace(raw), "\n") + 1
	result := make([]string, count)
	for i := range result {
		result[i] = nativeLegacyOriginAt(first + i)
	}
	return result
}

func nativeLegacyBeginGeneratedHeader() {
	if nativeLegacyDocument != nil {
		nativeLegacyDocument.generatedOrigins = make(map[int]string)
	}
}

func nativeLegacyRememberGeneratedBodyLine(offset int, origins []string, index int) {
	if nativeLegacyDocument == nil {
		return
	}
	if index < 0 || index >= len(origins) {
		parserError("E_WORKSPACE_ACCESS: function body lost source origin")
	}
	nativeLegacyDocument.generatedOrigins[offset] = origins[index]
}

func nativeLegacyGeneratedHeaderOrigins(header string) [][]string {
	if nativeLegacyDocument == nil {
		return nil
	}
	origins := make([]string, 0, strings.Count(header, "\n")+1)
	offset := 0
	for _, line := range strings.Split(header, "\n") {
		origin, found := nativeLegacyDocument.generatedOrigins[offset]
		if !found {
			origin = nativeGeneratedDocument
		}
		origins = append(origins, origin)
		offset += len(line) + 1
	}
	return [][]string{origins}
}

func (state *nativeLegacyDocumentState) readInclude(from, requested string) ([]byte, string, error) {
	if state.entry == "" || from == nativeGeneratedDocument {
		return nil, "", workspace.ErrAccess
	}
	if state.root == nil {
		root, err := workspace.Open(state.entry)
		if err != nil {
			return nil, "", err
		}
		state.root = root
	}
	resource, err := state.root.Open(from, requested)
	if err != nil {
		return nil, "", err
	}
	defer resource.Close()
	data, err := io.ReadAll(resource.File)
	if err != nil {
		return nil, "", err
	}
	after, err := resource.File.Stat()
	if err != nil || !os.SameFile(resource.Info, after) || resource.Info.Size() != after.Size() ||
		!resource.Info.ModTime().Equal(after.ModTime()) || int64(len(data)) != after.Size() {
		return nil, "", fmt.Errorf("included source changed while reading")
	}
	return data, resource.RelativePath, nil
}

func nativeLegacyCurrentAssetSnapshot() *assets.Snapshot {
	if nativeLegacyDocument == nil {
		return nil
	}
	return currentAction.embeddedAsset
}

func nativeLegacyPrepareEmbeddedAsset(requested string) error {
	state := nativeLegacyDocument
	if state == nil {
		return nil
	}
	if state.entry == "" || currentAction.sourceDocument == nativeGeneratedDocument {
		return workspace.ErrAccess
	}
	if state.scope == nil {
		scope, err := assets.OpenScope(state.entry, state.cache)
		if err != nil {
			return err
		}
		state.scope = scope
	}
	snapshot, err := state.scope.Read(currentAction.sourceDocument, requested)
	if err != nil {
		return err
	}
	currentAction.embeddedAsset = &snapshot
	return nil
}

func loadPackageForLegacyEntry() (*cherriPackage, bool) {
	if nativeLegacyDocument != nil {
		return nil, false
	}
	return loadPackage("info.plist")
}
