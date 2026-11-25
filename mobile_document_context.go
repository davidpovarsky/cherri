package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
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

// This forwarding entry establishes the additive request shape for the causal
// controls. The implementation is replaced only after preserving their RED
// outcomes against the existing context-free mobile route.
func compileForMobileDocumentWithOptions(src, name, mode string, sign bool, options CompileOptions, document *mobileDocumentContext) mobileCompileResponse {
	if err := validateMobileDocumentContext(document); err != nil {
		return mobileCompileResponse{OK: false, Error: err.Error(), Mode: mode}
	}
	return compileForMobileModeWithOptions(src, name, mode, sign, options)
}
