package main

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
)

// Observation-only external harness: immutable CI bytes, exact S2 production intake.
func TestActualWorkflowKitS2EvidenceIntake(t *testing.T) {
    root := os.Getenv("CHERRI_ACTUAL_ENCODING_ARTIFACT_DIR")
    if root == "" { t.Fatal("actual CI artifact directory is required") }
    data, err := os.ReadFile(filepath.Join(root, "required-encoding-evidence.json"))
    if err != nil { t.Fatal(err) }
    var doc struct { Records []EvidenceRecord }
    if err := json.Unmarshal(data, &doc); err != nil { t.Fatal(err) }
    if len(doc.Records) != 2 { t.Fatalf("expected two actual CI records, got %d", len(doc.Records)) }
    for i, id := range []string{"CI01", "RP47"} {
        record := doc.Records[i]
        t.Run(id, func(t *testing.T) {
            if record.ImplementationSHA != "7b0100a2a9ce5991ca8ffe82c805278e5aa7754e" ||
                record.TestID != nativeEncodingTestID(id) || record.Result != "passed" || record.ExitCode != 0 {
                t.Fatal("actual CI identity/result mismatch")
            }
            for j := range record.Artifacts {
                artifact := &record.Artifacts[j]
                if artifact.RunID != "44452839062" { t.Fatal("actual run mismatch") }
                // Transport relocation only; original producer JSON remains unchanged.
                artifact.Path = filepath.Join(root, filepath.Base(artifact.Path))
                b, err := os.ReadFile(artifact.Path)
                if err != nil { t.Fatal(err) }
                sum := sha256.Sum256(b)
                if artifact.SHA256 != hex.EncodeToString(sum[:]) { t.Fatalf("digest mismatch: %s", artifact.Path) }
            }
            if err := validateNativeEncodingRecord(id, record); err != nil { t.Fatal(err) }
            t.Log("actual positive/corrupt/restored reports and process receipts accepted at exact S2")
            stale := record
            stale.ImplementationSHA = "0000000000000000000000000000000000000000"
            if err := validateNativeEncodingRecord(id, stale); err == nil {
                t.Fatal("actual S2 evidence incorrectly accepted for a different source")
            }
        })
    }
}
