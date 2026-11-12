package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const p10r10CorpusDir = `C:\Users\DAVID\Code\cherri-shortcut-audit-20261110\normalized`

// TestP10R10CorpusMeasure measures natural coverage and whole-workflow fidelity on the private
// corpus. It is gated by CHERRI_P10R10_CORPUS=1 and never writes private content into the repo.
func TestP10R10CorpusMeasure(t *testing.T) {
	if os.Getenv("CHERRI_P10R10_CORPUS") != "1" {
		t.Skip("set CHERRI_P10R10_CORPUS=1 to run")
	}
	entries, err := os.ReadDir(p10r10CorpusDir)
	if err != nil {
		t.Skipf("corpus not present: %v", err)
	}
	var files []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".plist") {
			files = append(files, filepath.Join(p10r10CorpusDir, e.Name()))
		}
	}
	sort.Strings(files)
	total, natural := 0, 0
	reasons := map[string]int{}
	fallbackIDs := map[string]int{}
	for _, f := range files {
		id := strings.TrimSuffix(filepath.Base(f), ".plist")
		raw, _ := os.ReadFile(f)
		doc, err := DecodeNativeShortcutDocument(raw)
		if err != nil {
			t.Errorf("%s decode: %v", id, err)
			continue
		}
		t0 := time.Now()
		plan, err := PlanHybridImport(doc)
		if err != nil {
			fmt.Printf("%s PLAN ERR: %v (%.0fms)\n", id, err, float64(time.Since(t0).Milliseconds()))
			t.Errorf("%s plan: %v", id, err)
			continue
		}
		_, errL := RenderHybridLegacySource(doc, plan)
		_, errV := RenderHybridV2Source(doc, plan)
		status := "OK"
		if errL != nil || errV != nil {
			status = fmt.Sprintf("FAIL legacy=%v v2=%v", errL, errV)
			t.Errorf("%s %s", id, status)
		}
		total += plan.TotalActions
		natural += plan.NaturalVerifiedCount
		for k, v := range plan.ReasonCounts {
			reasons[k] += v
		}
		for _, d := range plan.Decisions {
			if d.Classification != "NATURAL_VERIFIED" && d.Classification != "SOURCE_INVALID" {
				fallbackIDs[d.AppleIdentifier]++
			}
		}
		fmt.Printf("%s actions=%d natural=%d (%.1f%%) %s (%.0fms)\n", id, plan.TotalActions, plan.NaturalVerifiedCount, 100*float64(plan.NaturalVerifiedCount)/float64(max(plan.TotalActions, 1)), status, float64(time.Since(t0).Milliseconds()))
	}
	fmt.Printf("TOTAL actions=%d natural=%d (%.2f%%) reasons=%v\n", total, natural, 100*float64(natural)/float64(max(total, 1)), reasons)
	type kv struct {
		id string
		n  int
	}
	var top []kv
	for k, v := range fallbackIDs {
		top = append(top, kv{k, v})
	}
	sort.Slice(top, func(i, j int) bool { return top[i].n > top[j].n })
	fmt.Println("TOP FALLBACK IDENTIFIERS:")
	for i := 0; i < len(top) && i < 30; i++ {
		fmt.Printf("  %5d  %s\n", top[i].n, top[i].id)
	}
}
