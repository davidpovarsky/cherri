/*
 * Copyright (c) Cherri
 */

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/electrikmilk/args-parser"
	"howett.net/plist"
)

var signFailed = false
var signingServiceFailed = false
var backoff = 10

// SignShortcut signs the unsigned shortcut at inPath and writes the signed file to outPath.
func SignShortcut(inPath, outPath, mode string) error {
	if mode == "" {
		mode = "people-who-know-me"
		if args.Using("share") && args.Value("share") == "anyone" {
			mode = "anyone"
		}
	}

	inputPath = inPath
	outputPath = outPath

	if darwin && !args.Using("hubsign") && !args.Using("signing-server") {
		if args.Using("debug") {
			fmt.Printf("Signing %s to %s...", inPath, outPath)
		}
		var signCmd = exec.Command(
			"shortcuts",
			"sign",
			"-i", inPath,
			"-o", outPath,
			"-m", mode,
		)
		var stdErr bytes.Buffer
		signCmd.Stderr = &stdErr
		var signErr = signCmd.Run()
		if signErr == nil {
			data, readErr := os.ReadFile(outPath)
			if readErr == nil && looksLikeSignedShortcut(data) {
				if args.Using("debug") {
					fmt.Println(ansi("Done.", green))
				}
				return nil
			}
			_ = os.Remove(outPath)
			return fmt.Errorf("macOS shortcuts sign did not produce a valid AEA1 signed Shortcut at %s", outPath)
		}

		signFailed = true
		_ = os.Remove(outPath)
		if args.Using("debug") {
			fmt.Print(ansi("Failed!\n", red))
		}
		errMsg := strings.TrimSpace(stdErr.String())
		if errMsg == "" {
			errMsg = signErr.Error()
		}
		fmt.Printf("%s\n%s\n", ansi("Failed to sign Shortcut using macOS :(", orange, bold), ansi(errMsg, orange))
		return fmt.Errorf("macOS shortcuts sign failed: %s", errMsg)
	}

	if args.Using("signing-server") {
		var custom = SigningService{
			name: "Custom Signing Server URL",
			url:  args.Value("signing-server"),
		}
		return useSigningServiceExplicit(&custom, inPath, outPath)
	}

	return signPathWithHubSignCandidates(inPath, outPath)
}

func signPathWithHubSignCandidates(inPath, outPath string) error {
	var hubErrors []string
	candidates := hubSignCandidatesFunc()
	for idx, svc := range candidates {
		svcCopy := svc
		if err := useSigningServiceExplicit(&svcCopy, inPath, outPath); err == nil {
			return nil
		} else {
			hubErrors = append(hubErrors, fmt.Sprintf("%s: %v", svcCopy.name, err))
			if idx+1 < len(candidates) && !args.Using("no-ansi") {
				fmt.Printf("%s %s failed (%v); trying next HubSign service...\n", ansi("Warning:", orange, bold), svcCopy.name, err)
			}
		}
	}
	return fmt.Errorf("%s", strings.Join(hubErrors, " | "))
}

// sign runs the shortcuts sign command on the unsigned shortcut file.
func sign() {
	var signingMode = "people-who-know-me"
	if args.Using("share") && args.Value("share") == "anyone" {
		signingMode = "anyone"
	}
	if err := SignShortcut(inputPath, outputPath, signingMode); err != nil {
		exit(err.Error())
	}
}

func useHubSign() {
	if err := signPathWithHubSignCandidates(inputPath, outputPath); err != nil {
		exit(err.Error())
	}
}

type SigningService struct {
	name string
	url  string
	info func() string
}

// Sign the Shortcut using a signing service.
func useSigningService(service *SigningService) {
	if err := useSigningServiceExplicit(service, inputPath, outputPath); err != nil {
		exit(err.Error())
	}
}

func useSigningServiceExplicit(service *SigningService, inPath, outPath string) error {
	handleBackoff(service)

	if !args.Using("no-ansi") {
		fmt.Println(ansi(fmt.Sprintf("Signing using %s service...", service.name), green))
		if service.info != nil {
			fmt.Println(service.info())
		}
	}

	var signedShortcut, err = requestSignedShortcutExplicit(service, inPath, outPath)
	if err != nil {
		if outPath != "" && outPath != inPath {
			_ = os.Remove(outPath)
		}
		return err
	}
	if len(signedShortcut) == 0 {
		if outPath != "" && outPath != inPath {
			_ = os.Remove(outPath)
		}
		return fmt.Errorf("signing service returned empty response")
	}

	if !looksLikeSignedShortcut(signedShortcut) {
		if outPath != "" && outPath != inPath {
			_ = os.Remove(outPath)
		}
		return fmt.Errorf("signing server response does not look like a signed Shortcut (missing AEA1 magic)")
	}

	var writeErr = os.WriteFile(outPath, signedShortcut, 0644)
	if writeErr != nil {
		return fmt.Errorf("failed to write signed shortcut to %s: %w", outPath, writeErr)
	}

	if args.Using("debug") {
		fmt.Println(ansi("Done.", green))
	}
	return nil
}

func handleBackoff(service *SigningService) {
	if signingServiceFailed {
		fmt.Println(ansi(fmt.Sprintf("Backing off from %s", service.name), red))
		for i := 5; i > 0; i-- {
			fmt.Printf("%d seconds...\r", i)
			time.Sleep(1 * time.Second)
		}
		fmt.Print("\n\n")
	}
}

func requestSignedShortcut(service *SigningService) []byte {
	bytes, err := requestSignedShortcutExplicit(service, inputPath, outputPath)
	if err != nil {
		exit(err.Error())
	}
	return bytes
}

func requestSignedShortcutExplicit(service *SigningService, inPath, outPath string) ([]byte, error) {
	var xmlData []byte
	var marshalErr error

	if inPath != "" {
		raw, readErr := os.ReadFile(inPath)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read unsigned shortcut %s: %w", inPath, readErr)
		}
		xmlData = raw
	} else if len(shortcut.WFWorkflowActions) > 0 {
		xmlData, marshalErr = plist.Marshal(shortcut, plist.XMLFormat)
		if marshalErr != nil {
			return nil, fmt.Errorf("failed to marshal shortcut plist: %w", marshalErr)
		}
	} else {
		return nil, fmt.Errorf("no shortcut data or input path available to sign")
	}

	name := basename
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(outPath), ".shortcut")
	}
	if name == "" {
		name = "Shortcut"
	}

	return SignShortcutBytes(service, name, xmlData)
}

var signingHTTPTimeout = 45 * time.Second

func classifySigningNetworkError(err error) error {
	var netErr net.Error
	if err != nil {
		if ok := strings.Contains(strings.ToLower(err.Error()), "timeout") || strings.Contains(strings.ToLower(err.Error()), "deadline exceeded"); ok {
			return fmt.Errorf("signing request timeout: %w", err)
		}
		if e, ok := err.(net.Error); ok && e.Timeout() {
			netErr = e
			return fmt.Errorf("signing request timeout: %w", netErr)
		}
	}
	return fmt.Errorf("signing request error: %w", err)
}

func classifySigningHTTPError(statusCode int, status string, body []byte) error {
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 256 {
		snippet = snippet[:256] + "..."
	}
	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return fmt.Errorf("signing authentication failed (%s): %s", status, snippet)
	case statusCode == http.StatusTooManyRequests || statusCode >= 500:
		return fmt.Errorf("signing service transient failure (%s): %s", status, snippet)
	default:
		return fmt.Errorf("signing service HTTP error (%s): %s", status, snippet)
	}
}

func classifyNonAEA1Payload(contentType string, body []byte) error {
	trimmed := strings.TrimSpace(string(body))
	lowerCT := strings.ToLower(contentType)
	lowerTrimmed := strings.ToLower(trimmed)
	if len(body) == 0 {
		return fmt.Errorf("signing service returned empty response")
	}
	if len(body) >= 4 && string(body[:4]) == "AEA1" {
		if _, err := ValidateSignedShortcutBytes(body); err != nil {
			return fmt.Errorf("signing service returned invalid AEA1 payload (%v, Content-Type: %s)", err, contentType)
		}
	}
	if strings.Contains(lowerCT, "text/html") || strings.HasPrefix(lowerTrimmed, "<!doctype html") || strings.HasPrefix(lowerTrimmed, "<html") {
		return fmt.Errorf("signing service returned HTML error page instead of AEA1 signed Shortcut (Content-Type: %s)", contentType)
	}
	if strings.Contains(lowerCT, "xml") || strings.Contains(lowerCT, "plist") || strings.HasPrefix(trimmed, "<?xml") || strings.HasPrefix(trimmed, "<plist") || strings.HasPrefix(trimmed, "bplist00") {
		return fmt.Errorf("signing service returned unsigned XML/plist data instead of AEA1 signed Shortcut (Content-Type: %s)", contentType)
	}
	return fmt.Errorf("signing service response does not look like a signed Shortcut (missing AEA1 magic, Content-Type: %s)", contentType)
}

// doSigningPOST executes a signing POST request with automatic IPv4 fallback when CI runners lack IPv6 routing.
func doSigningPOST(targetURL, contentType string, bodyBytes []byte) (*http.Response, []byte, error) {
	runOnce := func(transport http.RoundTripper) (*http.Response, []byte, error) {
		request, httpErr := http.NewRequest("POST", targetURL, bytes.NewReader(bodyBytes))
		if httpErr != nil {
			return nil, nil, fmt.Errorf("failed to create http request: %w", httpErr)
		}
		request.Header.Set("Content-Type", contentType)
		request.Header.Set("User-Agent", fmt.Sprintf("Shortcuts/5037.111 cherri/%s", version))
		client := &http.Client{
			Timeout:   signingHTTPTimeout,
			Transport: transport,
		}
		response, resErr := client.Do(request)
		if resErr != nil {
			return nil, nil, resErr
		}
		defer response.Body.Close()
		body, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", readErr)
		}
		return response, body, nil
	}

	response, body, err := runOnce(nil)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "no route to host") || strings.Contains(msg, "network is unreachable") {
			if baseTransport, ok := http.DefaultTransport.(*http.Transport); ok {
				v4Transport := baseTransport.Clone()
				dialer := &net.Dialer{Timeout: 20 * time.Second, KeepAlive: 30 * time.Second}
				v4Transport.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) {
					return dialer.DialContext(ctx, "tcp4", addr)
				}
				response, body, err = runOnce(v4Transport)
			}
		}
	}
	if err != nil {
		if strings.Contains(err.Error(), "failed to read response body") || strings.Contains(err.Error(), "failed to create http request") {
			return nil, nil, err
		}
		return nil, nil, classifySigningNetworkError(err)
	}
	return response, body, nil
}

// / SignShortcutBytes signs explicit plist XML bytes using a remote signing service.
func SignShortcutBytes(service *SigningService, name string, xmlData []byte) ([]byte, error) {
	if service == nil || strings.TrimSpace(service.url) == "" {
		return nil, fmt.Errorf("signing service URL is empty")
	}
	if len(xmlData) == 0 {
		return nil, fmt.Errorf("cannot sign empty shortcut plist payload")
	}
	if name == "" {
		name = "Shortcut"
	}

	// 1. If service is Shortcuty Sign or supports multipart file upload:
	if strings.Contains(service.url, "shortcuty.app") || strings.Contains(service.url, "sign?response=json") {
		var b bytes.Buffer
		w := multipart.NewWriter(&b)
		part, err := w.CreateFormFile("file", name+".shortcut")
		if err != nil {
			return nil, fmt.Errorf("failed to create multipart form file: %w", err)
		}
		if _, err := part.Write(xmlData); err != nil {
			return nil, fmt.Errorf("failed to write multipart payload: %w", err)
		}
		if err := w.Close(); err != nil {
			return nil, fmt.Errorf("failed to close multipart writer: %w", err)
		}

		response, body, postErr := doSigningPOST(service.url, w.FormDataContentType(), b.Bytes())
		if postErr != nil {
			return nil, postErr
		}

		if response.StatusCode != http.StatusOK {
			signingServiceFailed = true
			backoff += 10
			return nil, classifySigningHTTPError(response.StatusCode, response.Status, body)
		}

		signingServiceFailed = false
		if backoff > 10 {
			backoff -= 10
		} else {
			backoff = 10
		}

		if len(body) == 0 {
			return nil, fmt.Errorf("signing service returned empty response")
		}

		contentType := response.Header.Get("Content-Type")
		if strings.Contains(strings.ToLower(contentType), "application/json") {
			var jsonRes struct {
				Success bool `json:"success"`
				File    struct {
					Format        string `json:"format"`
					ContentBase64 string `json:"content_base64"`
					SignedSHA256  string `json:"signed_sha256"`
				} `json:"file"`
				Error struct {
					Message string `json:"message"`
					Code    string `json:"code"`
				} `json:"error"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(body, &jsonRes); err != nil {
				return nil, fmt.Errorf("corrupted signing service JSON response: %w", err)
			}
			if jsonRes.Success && jsonRes.File.ContentBase64 != "" {
				decoded, err := base64.StdEncoding.DecodeString(jsonRes.File.ContentBase64)
				if err != nil {
					return nil, fmt.Errorf("corrupted base64 signed payload: %w", err)
				}
				if !looksLikeSignedShortcut(decoded) {
					return nil, classifyNonAEA1Payload("application/octet-stream (decoded base64)", decoded)
				}
				if expectedSHA := strings.TrimSpace(jsonRes.File.SignedSHA256); expectedSHA != "" {
					actualSHA := fmt.Sprintf("%x", sha256.Sum256(decoded))
					if !strings.EqualFold(expectedSHA, actualSHA) {
						return nil, fmt.Errorf("signing service signed_sha256 mismatch: expected %s, got %s", expectedSHA, actualSHA)
					}
				}
				return decoded, nil
			}
			if jsonRes.Error.Message != "" {
				return nil, fmt.Errorf("signing server error: %s (%s)", jsonRes.Error.Message, jsonRes.Error.Code)
			}
			if jsonRes.Message != "" {
				return nil, fmt.Errorf("signing server error: %s", jsonRes.Message)
			}
			return nil, fmt.Errorf("signing server returned JSON with success=false and no signed payload")
		}

		if looksLikeSignedShortcut(body) {
			return body, nil
		}

		return nil, classifyNonAEA1Payload(contentType, body)
	}

	// 2. Legacy scaxyz/shortcut-signing-server JSON payload:
	var payload = map[string]string{
		"shortcutName": name,
		"shortcut":     string(xmlData),
	}
	var jsonPayload, jsonErr = json.Marshal(payload)
	if jsonErr != nil {
		return nil, fmt.Errorf("failed to marshal json payload: %w", jsonErr)
	}

	response, body, postErr := doSigningPOST(service.url, "application/json", jsonPayload)
	if postErr != nil {
		return nil, postErr
	}

	if response.StatusCode != http.StatusOK {
		signingServiceFailed = true
		backoff += 10
		return nil, classifySigningHTTPError(response.StatusCode, response.Status, body)
	}

	signingServiceFailed = false
	if backoff > 10 {
		backoff -= 10
	} else {
		backoff = 10
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("signing service returned empty response")
	}

	var responseContentType = response.Header.Get("Content-Type")
	mediaType := strings.TrimSpace(strings.Split(responseContentType, ";")[0])
	// Do NOT allow application/x-plist or XML/HTML content types for signed shortcuts
	var allowedContentTypes = []string{"application/octet-stream", "application/x-apple-shortcut"}
	if !slices.Contains(allowedContentTypes, mediaType) {
		if strings.HasPrefix(strings.TrimSpace(string(body)), "<?xml") || strings.HasPrefix(strings.TrimSpace(string(body)), "<plist") || strings.Contains(strings.ToLower(mediaType), "plist") || strings.Contains(strings.ToLower(mediaType), "xml") || strings.Contains(strings.ToLower(mediaType), "html") {
			return nil, classifyNonAEA1Payload(responseContentType, body)
		}
		return nil, fmt.Errorf("unsupported signing response Content-Type: %s", responseContentType)
	}

	if !looksLikeSignedShortcut(body) {
		return nil, classifyNonAEA1Payload(responseContentType, body)
	}

	return body, nil
}

// SigningVerificationTier distinguishes local/mock protocol checks from live Apple cryptographic signing and device execution.
type SigningVerificationTier string

const (
	TierSigningProtocolContractPass SigningVerificationTier = "SIGNING_PROTOCOL_CONTRACT_PASS"
	TierAEA1HeaderPass              SigningVerificationTier = "AEA1_HEADER_PASS"
	TierAppleSignatureValidated     SigningVerificationTier = "APPLE_SIGNATURE_VALIDATED"
	TierShortcutImported            SigningVerificationTier = "SHORTCUT_IMPORTED"
	TierShortcutExecuted            SigningVerificationTier = "SHORTCUT_EXECUTED"
)

// MinAEA1ArchiveLength is the minimum byte length for a non-truncated AEA1 archive
// (4-byte "AEA1" magic + 4-byte profile/scrypt field + 4-byte root header length + payload bytes).
// A 4-byte "AEA1" header-only response is a truncated archive and must fail closed.
const MinAEA1ArchiveLength = 16

// ValidateSignedShortcutBytes verifies that buffer has a non-truncated AEA1 header and non-zero archive body.
// It returns TierAEA1HeaderPass (structural container check only; never claims live Apple cryptographic validation).
func ValidateSignedShortcutBytes(buffer []byte) (SigningVerificationTier, error) {
	if len(buffer) == 0 {
		return "", fmt.Errorf("empty signed shortcut buffer")
	}
	if len(buffer) < 4 || string(buffer[:4]) != "AEA1" {
		return "", fmt.Errorf("missing AEA1 magic header")
	}
	if len(buffer) < MinAEA1ArchiveLength {
		return "", fmt.Errorf("truncated AEA1 archive: got %d bytes, minimum required is %d bytes", len(buffer), MinAEA1ArchiveLength)
	}
	hasNonZeroPayload := false
	for _, b := range buffer[4:] {
		if b != 0 {
			hasNonZeroPayload = true
			break
		}
	}
	if !hasNonZeroPayload {
		return "", fmt.Errorf("corrupted AEA1 archive: payload bytes after AEA1 magic are all zero")
	}
	return TierAEA1HeaderPass, nil
}

// looksLikeSignedShortcut performs structural checks to make sure response is a non-truncated AEA1 signed Shortcut.
func looksLikeSignedShortcut(buffer []byte) bool {
	_, err := ValidateSignedShortcutBytes(buffer)
	return err == nil
}

func removeUnsigned() {
	var _, signedStatErr = os.Stat(fmt.Sprintf("%s%s.shortcut", relativePath, workflowName))
	if os.IsNotExist(signedStatErr) {
		return
	}
	var _, unsignedStatErr = os.Stat(fmt.Sprintf("%s%s%s", relativePath, workflowName, unsignedEnd))
	if os.IsNotExist(unsignedStatErr) {
		return
	}

	if args.Using("debug") {
		fmt.Printf("Removing %s%s...", workflowName, unsignedEnd)
	}

	var removeErr = os.Remove(inputPath)
	handle(removeErr)

	if args.Using("debug") {
		fmt.Println(ansi("Done.", green))
	}
}

// Sign the Shortcut using the primary Shortcuty HubSign remote signing service.
func hubSign() SigningService {
	return SigningService{
		name: "Shortcuty Sign",
		url:  "https://sign.shortcuty.app/api/v1/sign?response=json",
		info: func() string {
			return fmt.Sprintf("Shortcut Signing Powered By %s", ansi("Shortcuty Sign", red))
		},
	}
}

// routineHubSign returns upstream Cherri's RoutineHub HubSign service endpoint.
func routineHubSign() SigningService {
	return SigningService{
		name: "RoutineHub HubSign",
		url:  "https://hubsign.routinehub.services/sign",
		info: func() string {
			return fmt.Sprintf("Shortcut Signing Powered By %s", ansi("RoutineHub", red))
		},
	}
}

var hubSignCandidatesFunc = func() []SigningService {
	return []SigningService{hubSign(), routineHubSign()}
}

func isBuiltInHubSignURL(rawURL string) bool {
	return strings.Contains(rawURL, "shortcuty.app") || strings.Contains(rawURL, "hubsign.routinehub.services")
}

// SigningProvenanceManifest records exact-file signing hashes and verification metadata without exposing URLs or secrets.
type SigningProvenanceManifest struct {
	SchemaVersion        string                  `json:"schemaVersion"`
	CompilerCommitSHA    string                  `json:"compilerCommitSha,omitempty"`
	SigningMethod        string                  `json:"signingMethod"`
	ServiceName          string                  `json:"serviceName"`
	ShortcutName         string                  `json:"shortcutName"`
	SourceFile           string                  `json:"sourceFile,omitempty"`
	SourceSHA256         string                  `json:"sourceSha256,omitempty"`
	UnsignedFile         string                  `json:"unsignedFile"`
	UnsignedPlistSHA256  string                  `json:"unsignedPlistSha256"`
	UnsignedByteSize     int                     `json:"unsignedByteSize"`
	NativeActionCount    int                     `json:"nativeActionCount"`
	SignedFile           string                  `json:"signedFile"`
	SignedArtifactSHA256 string                  `json:"signedArtifactSha256"`
	SignedByteSize       int                     `json:"signedByteSize"`
	HeaderMagic          string                  `json:"headerMagic"`
	VerificationTier     SigningVerificationTier `json:"verificationTier"`
}

var signingPrivacyDenyPattern = regexp.MustCompile(`(?i)(cherri-shortcut-audit-20261110|\b[0-3][0-9]\.plist\b|BEGIN [A-Z ]*PRIVATE KEY|ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|sk-[A-Za-z0-9]{16,})`)

// SignUnsignedShortcutFileWithProvenance signs an exact pre-compiled unsigned .shortcut plist file on disk
// without recompiling it, enforces synthetic-only privacy rules, verifies AEA1 + SHA256 integrity, and returns provenance metadata.
func SignUnsignedShortcutFileWithProvenance(
	unsignedPath string,
	signedOutPath string,
	sourcePath string,
	compilerCommitSHA string,
	service *SigningService,
) (*SigningProvenanceManifest, error) {
	if strings.TrimSpace(unsignedPath) == "" {
		return nil, fmt.Errorf("unsigned shortcut path is empty")
	}
	if strings.TrimSpace(signedOutPath) == "" {
		return nil, fmt.Errorf("signed output shortcut path is empty")
	}
	if signingPrivacyDenyPattern.MatchString(unsignedPath) || signingPrivacyDenyPattern.MatchString(sourcePath) {
		return nil, fmt.Errorf("privacy guard blocked signing of forbidden/private corpus path")
	}

	unsignedBytes, err := os.ReadFile(unsignedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read unsigned shortcut file %s: %w", filepath.Base(unsignedPath), err)
	}
	if len(unsignedBytes) == 0 {
		return nil, fmt.Errorf("cannot sign empty unsigned shortcut file %s", filepath.Base(unsignedPath))
	}
	if signingPrivacyDenyPattern.Match(unsignedBytes) {
		return nil, fmt.Errorf("privacy guard blocked signing of payload containing forbidden/private corpus markers")
	}
	if len(unsignedBytes) >= 4 && string(unsignedBytes[:4]) == "AEA1" {
		return nil, fmt.Errorf("input file %s is already an AEA1 signed archive; expected unsigned plist", filepath.Base(unsignedPath))
	}

	var plistDoc map[string]any
	if _, err := plist.Unmarshal(unsignedBytes, &plistDoc); err != nil {
		return nil, fmt.Errorf("input file %s is not a valid plist: %w", filepath.Base(unsignedPath), err)
	}
	rawActions, hasActions := plistDoc["WFWorkflowActions"]
	if !hasActions {
		return nil, fmt.Errorf("input plist %s is missing WFWorkflowActions", filepath.Base(unsignedPath))
	}
	nativeActionCount := 0
	if slice, ok := rawActions.([]any); ok {
		nativeActionCount = len(slice)
	}

	var sourceSHA256 string
	var sourceBase string
	if strings.TrimSpace(sourcePath) != "" {
		srcBytes, srcErr := os.ReadFile(sourcePath)
		if srcErr != nil {
			return nil, fmt.Errorf("failed to read source file %s: %w", filepath.Base(sourcePath), srcErr)
		}
		if signingPrivacyDenyPattern.Match(srcBytes) {
			return nil, fmt.Errorf("privacy guard blocked source file containing forbidden/private corpus markers")
		}
		sourceSHA256 = fmt.Sprintf("%x", sha256.Sum256(srcBytes))
		sourceBase = filepath.Base(sourcePath)
	}

	shortcutName := strings.TrimSuffix(filepath.Base(signedOutPath), ".shortcut")
	shortcutName = strings.TrimSuffix(shortcutName, "_unsigned")
	shortcutName = strings.TrimSuffix(shortcutName, "-signed")
	if shortcutName == "" {
		shortcutName = "Shortcut"
	}

	var (
		signedBytes   []byte
		signErr       error
		activeService = service
		signingMethod = "HUBSIGN_REMOTE_SERVICE"
	)

	if activeService != nil {
		if !isBuiltInHubSignURL(activeService.url) {
			signingMethod = "CUSTOM_REMOTE_SIGNING_SERVICE"
		}
		signedBytes, signErr = SignShortcutBytes(activeService, shortcutName, unsignedBytes)
	} else {
		var hubErrors []string
		for _, candidate := range hubSignCandidatesFunc() {
			svcCopy := candidate
			b, err := SignShortcutBytes(&svcCopy, shortcutName, unsignedBytes)
			if err == nil {
				activeService = &svcCopy
				signedBytes = b
				signErr = nil
				break
			}
			hubErrors = append(hubErrors, fmt.Sprintf("%s: %v", svcCopy.name, err))
		}
		if len(signedBytes) == 0 {
			signErr = fmt.Errorf("%s", strings.Join(hubErrors, " | "))
		}
	}

	if signErr != nil {
		if unsignedPath != signedOutPath {
			_ = os.Remove(signedOutPath)
		}
		return nil, signErr
	}

	tier, valErr := ValidateSignedShortcutBytes(signedBytes)
	if valErr != nil {
		if unsignedPath != signedOutPath {
			_ = os.Remove(signedOutPath)
		}
		return nil, fmt.Errorf("signed artifact validation failed: %w", valErr)
	}

	if err := os.MkdirAll(filepath.Dir(signedOutPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.WriteFile(signedOutPath, signedBytes, 0600); err != nil {
		return nil, fmt.Errorf("failed to write signed shortcut to %s: %w", filepath.Base(signedOutPath), err)
	}

	unsignedSHA256 := fmt.Sprintf("%x", sha256.Sum256(unsignedBytes))
	signedSHA256 := fmt.Sprintf("%x", sha256.Sum256(signedBytes))

	return &SigningProvenanceManifest{
		SchemaVersion:        "1.4",
		CompilerCommitSHA:    strings.TrimSpace(compilerCommitSHA),
		SigningMethod:        signingMethod,
		ServiceName:          activeService.name,
		ShortcutName:         shortcutName,
		SourceFile:           sourceBase,
		SourceSHA256:         sourceSHA256,
		UnsignedFile:         filepath.Base(unsignedPath),
		UnsignedPlistSHA256:  unsignedSHA256,
		UnsignedByteSize:     len(unsignedBytes),
		NativeActionCount:    nativeActionCount,
		SignedFile:           filepath.Base(signedOutPath),
		SignedArtifactSHA256: signedSHA256,
		SignedByteSize:       len(signedBytes),
		HeaderMagic:          string(signedBytes[:4]),
		VerificationTier:     tier,
	}, nil
}
