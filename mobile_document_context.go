package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/electrikmilk/cherri/internal/language/assets"
	"github.com/electrikmilk/cherri/internal/language/workspace"
)

// mobileDocumentContext belongs to one editor snapshot. It is separate from
// the workflow name and never enters the Apple plist. An empty URI represents
// an unsaved document, which has no local resource authority.
type mobileDocumentContext struct {
	URI           string `json:"uri"`
	DocumentID    string `json:"documentId"`
	Revision      uint64 `json:"revision"`
	AssetRevision uint64 `json:"assetRevision"`
}

type mobileDocumentCompileRequest struct {
	Source       string                 `json:"source"`
	Name         string                 `json:"name"`
	Mode         string                 `json:"mode"`
	BuildProfile BuildProfile           `json:"buildProfile"`
	Signed       bool                   `json:"signed"`
	Document     *mobileDocumentContext `json:"document,omitempty"`
}

func validateMobileDocumentContext(document *mobileDocumentContext) error {
	if document == nil {
		return nil
	}
	id, err := uuid.Parse(document.DocumentID)
	if err != nil || id == uuid.Nil || !strings.EqualFold(id.String(), document.DocumentID) {
		return fmt.Errorf("NATIVE_DOCUMENT_CONTEXT_INVALID: documentId must be a canonical nonzero UUID")
	}
	if document.URI == "" {
		return nil
	}
	uri, err := url.Parse(document.URI)
	if err != nil || !strings.EqualFold(uri.Scheme, "file") || uri.Opaque != "" ||
		(uri.Host != "" && uri.Host != "localhost") || uri.User != nil ||
		uri.RawQuery != "" || uri.ForceQuery || uri.Fragment != "" ||
		!filepath.IsAbs(filepath.FromSlash(uri.Path)) || strings.ContainsAny(uri.Path, "\x00\r\n") {
		return fmt.Errorf("NATIVE_DOCUMENT_CONTEXT_INVALID: uri must be an absolute local file URI")
	}
	return nil
}

func compileDocumentForMobile(requestJSON string) mobileCompileResponse {
	var request mobileDocumentCompileRequest
	decoder := json.NewDecoder(strings.NewReader(requestJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return mobileCompileResponse{OK: false, Error: "NATIVE_DOCUMENT_REQUEST_INVALID: " + err.Error()}
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return mobileCompileResponse{OK: false, Error: "NATIVE_DOCUMENT_REQUEST_INVALID: trailing data"}
	}
	return compileForMobileDocumentWithOptions(request.Source, request.Name, request.Mode, request.Signed,
		CompileOptions{BuildProfile: request.BuildProfile}, request.Document)
}

// The existing bounded cache is shared with language-service analysis.
// Counters are process-wide observations, not attributed per-request deltas.
func nativeAssetCacheSnapshot() assets.Metrics { return assets.Shared().Metrics() }

func invalidateNativeAssetCache() assets.Metrics {
	assets.Shared().Invalidate()
	return assets.Shared().Metrics()
}

func nativeDocumentEntry(document *mobileDocumentContext, name string) string {
	if document == nil {
		return name + ".cherri"
	}
	if document.URI == "" {
		// A source-only draft has identity, but no filesystem authority.
		return "cherri-draft://" + document.DocumentID + "/" + name + ".cherri"
	}
	path, _ := workspace.FilePath(document.URI)
	return path
}
