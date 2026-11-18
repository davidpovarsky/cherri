package main

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

// External service execution is opt-in. A skip supplies no K05 formal evidence.
func TestP15K05LiveSigning(t *testing.T) {
	if os.Getenv("CHERRI_K05_LIVE_SIGNING") != "1" {
		t.Skip("requires explicit CHERRI_K05_LIVE_SIGNING=1; no external signing attempted")
	}
	head := p15InstallGit(t, ".", "rev-parse", "HEAD")
	if p15InstallGit(t, ".", "status", "--porcelain") != "" {
		t.Fatal("live signing receipt requires immutable clean source")
	}
	f := p15NewEditFixture(t)
	info, err := buildinfo.ReadFile(f.binary)
	if err != nil {
		t.Fatal(err)
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	if info.Path != "github.com/electrikmilk/cherri" || settings["vcs.revision"] != head || settings["vcs.modified"] != "false" {
		t.Fatalf("actual compiler is not the exact clean source: module=%s revision=%s modified=%s", info.Path, settings["vcs.revision"], settings["vcs.modified"])
	}
	workspace, source, original, originalBytes := f.prepare(t)
	p15EditReplaceBody(t, source, "P15_K05_ORIGINAL", "P15_K05_EDITED")
	policy := filepath.Join(workspace, "allowed-delta.json")
	p15EditWritePolicy(t, policy, originalBytes)
	capture := f.observe(t, false)
	if server := os.Getenv("CHERRI_SIGNING_SERVER"); server != "" {
		f.env["CHERRI_SIGNING_SERVER"] = server
	}
	signedPath := filepath.Join(workspace, "builds", "P15-K05-Verified-Edit.shortcut")
	out, err := f.run(t, "build", p15EditPath(source), p15EditPath(signedPath), "--signed", "--original="+p15EditPath(original), "--allowed-delta="+p15EditPath(policy))
	if err != nil {
		t.Fatalf("actual configured signing service did not complete: %v\n%s", err, out)
	}
	sourceBytes := p15InstallRead(t, source)
	unsigned := p15InstallRead(t, capture.gate)
	requestInput := p15InstallRead(t, capture.input)
	signed := p15InstallRead(t, signedPath)
	if string(unsigned) != string(requestInput) {
		t.Fatal("live sign input differs from exact gated bytes")
	}
	var gate shortcutcompare.EditVerification
	if err := json.Unmarshal(p15InstallRead(t, capture.receipt), &gate); err != nil || !gate.OK || gate.AllowedChanges != 1 {
		t.Fatalf("missing exact allowed-delta receipt: %+v %v", gate, err)
	}
	var provenance SigningProvenanceManifest
	if err := json.Unmarshal(p15InstallRead(t, capture.manifest), &provenance); err != nil {
		t.Fatal(err)
	}
	digest := func(value []byte) string { return fmt.Sprintf("%x", sha256.Sum256(value)) }
	if gate.OriginalSHA256 != digest(originalBytes) || gate.CandidateSHA256 != digest(unsigned) ||
		provenance.CompilerCommitSHA != head || provenance.SourceSHA256 != digest(sourceBytes) || provenance.UnsignedPlistSHA256 != digest(unsigned) ||
		provenance.SignedArtifactSHA256 != digest(signed) || provenance.NativeActionCount != 2 ||
		provenance.ShortcutName != "P15-K05-Verified-Edit" {
		t.Fatalf("source/gate/signing provenance is detached: gate=%+v signing=%+v", gate, provenance)
	}
	tier, err := ValidateSignedShortcutBytes(signed)
	if err != nil || tier != provenance.VerificationTier {
		t.Fatalf("actual container validation failed: %v %v", tier, err)
	}
	if string(p15InstallRead(t, original)) != string(originalBytes) {
		t.Fatal("live workflow changed original")
	}
	commandLog := string(p15InstallRead(t, capture.log))
	if strings.Count(commandLog, "compile\n") != 1 || strings.Count(commandLog, "sign\n") != 1 ||
		strings.Index(commandLog, "verify-edit\n") > strings.Index(commandLog, "sign\n") {
		t.Fatal("live signing did not follow exactly one compile and verification")
	}
	receipt := map[string]any{
		"source_sha": head, "compiler_binary_sha256": digest(p15InstallRead(t, f.binary)),
		"original_sha256": digest(originalBytes), "edited_source_sha256": digest(sourceBytes),
		"unsigned_sha256": digest(unsigned), "sign_input_sha256": digest(requestInput),
		"signed_sha256": digest(signed), "signed_bytes": len(signed), "service": provenance.ServiceName,
		"signing_method": provenance.SigningMethod, "verification_tier": tier,
		"allowed_changes": gate.AllowedChanges, "native_actions": provenance.NativeActionCount,
		"apple_runtime_verified": false,
		"scope":                  "Actual configured remote signing service and container validation of synthetic data only; no Apple import, validity, installed-consumer or runtime inference.",
	}
	receiptBytes, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if directory := os.Getenv("CHERRI_K05_EVIDENCE_DIR"); directory != "" {
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
		artifacts := map[string][]byte{
			"original.plist": originalBytes, "edited.cherri": sourceBytes, "allowed-delta.json": p15InstallRead(t, policy),
			"verified-unsigned.shortcut": unsigned, "sign-input.shortcut": requestInput, "signed.shortcut": signed,
			"gate-receipt.json": p15InstallRead(t, capture.receipt), "signing-provenance.json": p15InstallRead(t, capture.manifest),
			"live-signing-receipt.json": append(receiptBytes, '\n'),
		}
		for name, data := range artifacts {
			path := filepath.Join(directory, name)
			file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				t.Fatal(err)
			}
			_, writeErr := file.Write(data)
			closeErr := file.Close()
			if writeErr != nil || closeErr != nil {
				t.Fatalf("write %s: %v %v", name, writeErr, closeErr)
			}
		}
	}
	compact, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_K05_LIVE_SIGNING_RECEIPT:" + string(compact))
}
