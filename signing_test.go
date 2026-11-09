package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestP10_A1_RemoteSigningContractAndFailureInjection verifies all positive and negative
// remote signing requirements for Task A1 (1219604454669950):
// 1. Exact plist byte transmission to signing service
// 2. Valid AEA1 binary output verification (both JSON base64 Shortcuty and legacy binary octet-stream)
// 3. Explicit rejection (non-zero error / exit code, no residual unsigned XML output) of:
//    - XML plist returned with HTTP 200 OK (including application/x-plist or application/octet-stream)
//    - HTML error page returned with HTTP 200 OK
//    - Wrong MIME type (text/plain, text/html, application/xml)
//    - Empty response body
//    - HTTP 401/403 authentication failure
//    - HTTP 429/500/503 transient failure
//    - HTTP 400 client error
//    - Network timeout
//    - Corrupted JSON / corrupted base64 / base64 decoding to non-AEA1 bytes
// 4. End-to-end CLI signed compilation with --signing-server verifying exit code != 0 on false-success XML response.
func TestP10_A1_RemoteSigningContractAndFailureInjection(t *testing.T) {
	samplePlist := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>WFWorkflowActions</key>
	<array/>
</dict>
</plist>`)
	validAEA1 := append([]byte("AEA1"), []byte("mock-signed-apple-encrypted-archive-payload-bytes")...)

	t.Run("Positive_ShortcutyJSONBase64_ExactBytesAndAEA1", func(t *testing.T) {
		var receivedFileBytes []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				t.Errorf("failed to parse multipart form: %v", err)
				http.Error(w, "bad multipart", http.StatusBadRequest)
				return
			}
			f, _, err := r.FormFile("file")
			if err != nil {
				t.Errorf("missing form file: %v", err)
				http.Error(w, "missing file", http.StatusBadRequest)
				return
			}
			defer f.Close()
			receivedFileBytes, _ = io.ReadAll(f)

			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"success": true,
				"file": map[string]any{
					"format":         "signed",
					"content_base64": base64.StdEncoding.EncodeToString(validAEA1),
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		svc := &SigningService{
			name: "Mock Shortcuty",
			url:  srv.URL + "/api/v1/sign?response=json",
		}
		signed, err := SignShortcutBytes(svc, "TestShortcut", samplePlist)
		if err != nil {
			t.Fatalf("expected positive signing to succeed, got: %v", err)
		}
		if string(receivedFileBytes) != string(samplePlist) {
			t.Fatalf("signing server did not receive exact plist bytes:\ngot:  %q\nwant: %q", string(receivedFileBytes), string(samplePlist))
		}
		if !looksLikeSignedShortcut(signed) || string(signed) != string(validAEA1) {
			t.Fatalf("unexpected signed bytes: %q", string(signed))
		}
	})

	t.Run("Positive_LegacyBinaryOctetStream_ExactBytesAndAEA1", func(t *testing.T) {
		var receivedShortcut string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			receivedShortcut = payload["shortcut"]
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(validAEA1)
		}))
		defer srv.Close()

		svc := &SigningService{
			name: "Mock Legacy Server",
			url:  srv.URL + "/sign",
		}
		signed, err := SignShortcutBytes(svc, "TestShortcut", samplePlist)
		if err != nil {
			t.Fatalf("expected positive legacy signing to succeed, got: %v", err)
		}
		if receivedShortcut != string(samplePlist) {
			t.Fatalf("legacy signing server did not receive exact plist bytes")
		}
		if !looksLikeSignedShortcut(signed) {
			t.Fatalf("expected valid AEA1 output")
		}
	})

	failureCases := []struct {
		name           string
		urlSuffix      string
		statusCode     int
		contentType    string
		body           []byte
		delay          time.Duration
		expectedErrSub string
	}{
		{
			name:           "FalseSuccess_XMLPlistWithApplicationXPlist_Legacy",
			urlSuffix:      "/sign",
			statusCode:     http.StatusOK,
			contentType:    "application/x-plist",
			body:           samplePlist,
			expectedErrSub: "unsigned XML/plist",
		},
		{
			name:           "FalseSuccess_XMLPlistWithOctetStream_Legacy",
			urlSuffix:      "/sign",
			statusCode:     http.StatusOK,
			contentType:    "application/octet-stream",
			body:           samplePlist,
			expectedErrSub: "unsigned XML/plist",
		},
		{
			name:           "FalseSuccess_XMLPlistInJSONBase64_Shortcuty",
			urlSuffix:      "/api/v1/sign?response=json",
			statusCode:     http.StatusOK,
			contentType:    "application/json",
			body:           []byte(`{"success":true,"file":{"format":"signed","content_base64":"` + base64.StdEncoding.EncodeToString(samplePlist) + `"}}`),
			expectedErrSub: "unsigned XML/plist",
		},
		{
			name:           "FalseSuccess_HTMLErrorPageWith200_Shortcuty",
			urlSuffix:      "/api/v1/sign?response=json",
			statusCode:     http.StatusOK,
			contentType:    "text/html; charset=utf-8",
			body:           []byte(`<!DOCTYPE html><html><body>502 Bad Gateway</body></html>`),
			expectedErrSub: "HTML error page",
		},
		{
			name:           "FalseSuccess_HTMLErrorPageWith200_Legacy",
			urlSuffix:      "/sign",
			statusCode:     http.StatusOK,
			contentType:    "text/html",
			body:           []byte(`<html><body>Error</body></html>`),
			expectedErrSub: "HTML error page",
		},
		{
			name:           "WrongMIME_TextPlain_Legacy",
			urlSuffix:      "/sign",
			statusCode:     http.StatusOK,
			contentType:    "text/plain",
			body:           validAEA1,
			expectedErrSub: "unsupported signing response Content-Type",
		},
		{
			name:           "EmptyResponse_200_Shortcuty",
			urlSuffix:      "/api/v1/sign?response=json",
			statusCode:     http.StatusOK,
			contentType:    "application/octet-stream",
			body:           []byte{},
			expectedErrSub: "empty response",
		},
		{
			name:           "EmptyResponse_200_Legacy",
			urlSuffix:      "/sign",
			statusCode:     http.StatusOK,
			contentType:    "application/octet-stream",
			body:           []byte{},
			expectedErrSub: "empty response",
		},
		{
			name:           "AuthFailure_401_Shortcuty",
			urlSuffix:      "/api/v1/sign?response=json",
			statusCode:     http.StatusUnauthorized,
			contentType:    "application/json",
			body:           []byte(`{"error":"invalid api key"}`),
			expectedErrSub: "authentication failed",
		},
		{
			name:           "AuthFailure_403_Legacy",
			urlSuffix:      "/sign",
			statusCode:     http.StatusForbidden,
			contentType:    "text/plain",
			body:           []byte(`forbidden`),
			expectedErrSub: "authentication failed",
		},
		{
			name:           "TransientFailure_503_Shortcuty",
			urlSuffix:      "/api/v1/sign?response=json",
			statusCode:     http.StatusServiceUnavailable,
			contentType:    "text/plain",
			body:           []byte(`worker overloaded`),
			expectedErrSub: "transient failure",
		},
		{
			name:           "TransientFailure_429_Legacy",
			urlSuffix:      "/sign",
			statusCode:     http.StatusTooManyRequests,
			contentType:    "text/plain",
			body:           []byte(`rate limited`),
			expectedErrSub: "transient failure",
		},
		{
			name:           "HTTPError_400_Shortcuty",
			urlSuffix:      "/api/v1/sign?response=json",
			statusCode:     http.StatusBadRequest,
			contentType:    "application/json",
			body:           []byte(`{"error":"malformed plist"}`),
			expectedErrSub: "HTTP error",
		},
		{
			name:           "CorruptedJSON_200_Shortcuty",
			urlSuffix:      "/api/v1/sign?response=json",
			statusCode:     http.StatusOK,
			contentType:    "application/json",
			body:           []byte(`{"success":true,"file":{"content_base64":`),
			expectedErrSub: "corrupted signing service JSON",
		},
		{
			name:           "CorruptedBase64_200_Shortcuty",
			urlSuffix:      "/api/v1/sign?response=json",
			statusCode:     http.StatusOK,
			contentType:    "application/json",
			body:           []byte(`{"success":true,"file":{"content_base64":"!!!not-valid-base64!!!"}}`),
			expectedErrSub: "corrupted base64",
		},
		{
			name:           "NonAEA1Binary_200_Legacy",
			urlSuffix:      "/sign",
			statusCode:     http.StatusOK,
			contentType:    "application/octet-stream",
			body:           []byte("PK\x03\x04not-an-aea1-archive"),
			expectedErrSub: "missing AEA1 magic",
		},
		{
			name:           "TruncatedAEA1_HeaderOnly4Bytes_Legacy",
			urlSuffix:      "/sign",
			statusCode:     http.StatusOK,
			contentType:    "application/octet-stream",
			body:           []byte("AEA1"),
			expectedErrSub: "truncated AEA1 archive",
		},
		{
			name:           "TruncatedAEA1_HeaderOnly4Bytes_ShortcutyJSON",
			urlSuffix:      "/api/v1/sign?response=json",
			statusCode:     http.StatusOK,
			contentType:    "application/json",
			body:           []byte(`{"success":true,"file":{"format":"signed","content_base64":"` + base64.StdEncoding.EncodeToString([]byte("AEA1")) + `"}}`),
			expectedErrSub: "truncated AEA1 archive",
		},
		{
			name:           "CorruptedAEA1_AllZeroPayloadAfterMagic_Legacy",
			urlSuffix:      "/sign",
			statusCode:     http.StatusOK,
			contentType:    "application/octet-stream",
			body:           append([]byte("AEA1"), make([]byte, 32)...),
			expectedErrSub: "corrupted AEA1 archive",
		},
		{
			name:           "Timeout_Shortcuty",
			urlSuffix:      "/api/v1/sign?response=json",
			statusCode:     http.StatusOK,
			contentType:    "application/octet-stream",
			body:           validAEA1,
			delay:          250 * time.Millisecond,
			expectedErrSub: "signing request timeout",
		},
	}

	for _, tc := range failureCases {
		t.Run(tc.name, func(t *testing.T) {
			origTimeout := signingHTTPTimeout
			if tc.delay > 0 {
				signingHTTPTimeout = 50 * time.Millisecond
			}
			defer func() {
				signingHTTPTimeout = origTimeout
				signingServiceFailed = false
				backoff = 10
			}()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.delay > 0 {
					time.Sleep(tc.delay)
				}
				if tc.contentType != "" {
					w.Header().Set("Content-Type", tc.contentType)
				}
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write(tc.body)
			}))
			defer srv.Close()

			svc := &SigningService{
				name: "Mock Fault Server",
				url:  srv.URL + tc.urlSuffix,
			}

			tmpDir := t.TempDir()
			inFile := filepath.Join(tmpDir, "test_unsigned.shortcut")
			outFile := filepath.Join(tmpDir, "test.shortcut")
			if err := os.WriteFile(inFile, samplePlist, 0644); err != nil {
				t.Fatalf("failed to write input plist: %v", err)
			}
			// Pre-populate outFile with unsigned XML to ensure failure cleans it up rather than leaving a false-success file
			_ = os.WriteFile(outFile, samplePlist, 0644)

			err := useSigningServiceExplicit(svc, inFile, outFile)
			if err == nil {
				t.Fatalf("expected signing to fail for case %s, but it succeeded!", tc.name)
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.expectedErrSub)) {
				t.Fatalf("error %q did not contain expected substring %q", err.Error(), tc.expectedErrSub)
			}
			if _, statErr := os.Stat(outFile); !os.IsNotExist(statErr) {
				t.Fatalf("failed signing left residual file at %s", outFile)
			}
		})
	}

	t.Run("CLI_EndToEnd_SignedMode_RejectsXMLFalseSuccessAndAcceptsAEA1", func(t *testing.T) {
		tmpDir := t.TempDir()
		binPath := filepath.Join(tmpDir, "cherri-test.exe")
		buildCmd := exec.Command("go", "build", "-o", binPath, ".")
		if out, err := buildCmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to build cherri binary: %v\n%s", err, string(out))
		}

		srcPath := filepath.Join(tmpDir, "probe.cherri")
		if err := os.WriteFile(srcPath, []byte("alert(\"Hello Signed Probe\")\n"), 0644); err != nil {
			t.Fatalf("failed to write source: %v", err)
		}

		// 1. Server returns HTTP 200 with unsigned XML plist -> CLI MUST exit non-zero and not leave outPath
		xmlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-plist")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(samplePlist)
		}))
		defer xmlSrv.Close()

		badOut := filepath.Join(tmpDir, "bad.shortcut")
		cmdFail := exec.Command(binPath, srcPath, "--output="+badOut, "--signing-server="+xmlSrv.URL+"/sign", "--no-ansi")
		outFail, errFail := cmdFail.CombinedOutput()
		if errFail == nil {
			t.Fatalf("CLI exited 0 when signing server returned unsigned XML! Output:\n%s", string(outFail))
		}
		if _, statErr := os.Stat(badOut); !os.IsNotExist(statErr) {
			t.Fatalf("CLI left unsigned/corrupt output file at %s after signing failure", badOut)
		}

		// 2. Server returns HTTP 200 with valid AEA1 bytes -> CLI MUST exit 0 and write AEA1 file
		aeaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(validAEA1)
		}))
		defer aeaSrv.Close()

		goodOut := filepath.Join(tmpDir, "good.shortcut")
		cmdOk := exec.Command(binPath, srcPath, "--output="+goodOut, "--signing-server="+aeaSrv.URL+"/sign", "--no-ansi")
		outOk, errOk := cmdOk.CombinedOutput()
		if errOk != nil {
			t.Fatalf("CLI failed when signing server returned valid AEA1: %v\n%s", errOk, string(outOk))
		}
		written, readErr := os.ReadFile(goodOut)
		if readErr != nil {
			t.Fatalf("failed to read signed output: %v", readErr)
		}
		if !looksLikeSignedShortcut(written) {
			t.Fatalf("signed output is not AEA1: %q", string(written))
		}
	})
}

func TestP10_F05_SigningVerificationTiersAndTruncatedAEA1Rejection(t *testing.T) {
	// 1. 4-byte "AEA1" header-only must fail closed
	if looksLikeSignedShortcut([]byte("AEA1")) {
		t.Fatal("4-byte header-only 'AEA1' must NOT pass looksLikeSignedShortcut")
	}
	if _, err := ValidateSignedShortcutBytes([]byte("AEA1")); err == nil || !strings.Contains(err.Error(), "truncated AEA1 archive") {
		t.Fatalf("expected truncated AEA1 archive error for 4-byte header, got: %v", err)
	}

	// 2. 10-byte truncated AEA1 must fail closed
	if _, err := ValidateSignedShortcutBytes([]byte("AEA1\x01\x02\x03\x04\x05\x06")); err == nil || !strings.Contains(err.Error(), "truncated AEA1 archive") {
		t.Fatalf("expected truncated AEA1 archive error for 10-byte payload, got: %v", err)
	}

	// 3. Zero-filled payload after AEA1 magic must fail closed
	zeroPayload := append([]byte("AEA1"), make([]byte, 24)...)
	if _, err := ValidateSignedShortcutBytes(zeroPayload); err == nil || !strings.Contains(err.Error(), "corrupted AEA1 archive") {
		t.Fatalf("expected corrupted AEA1 archive error for zero-filled payload, got: %v", err)
	}

	// 4. Valid non-truncated AEA1 returns TierAEA1HeaderPass and NEVER claims live Apple cryptographic validation or import/execution
	validPayload := append([]byte("AEA1"), []byte("non-zero-archive-header-and-body")...)
	tier, err := ValidateSignedShortcutBytes(validPayload)
	if err != nil {
		t.Fatalf("expected valid AEA1 payload to pass structural check, got: %v", err)
	}
	if tier != TierAEA1HeaderPass {
		t.Fatalf("expected TierAEA1HeaderPass (%q), got %q", TierAEA1HeaderPass, tier)
	}
	if tier == TierAppleSignatureValidated || tier == TierShortcutImported || tier == TierShortcutExecuted {
		t.Fatalf("structural check must never claim live Apple signature/import/execution tier: %q", tier)
	}
}
