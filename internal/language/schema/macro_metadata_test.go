package schema

import "testing"

func TestCompilerMacroMetadataIsSeparateAndImmutable(t *testing.T) {
	reg := DefaultRegistry()
	nativeBefore := reg.Fingerprint()
	if len(reg.AllActions()) != 505 {
		t.Fatal("native catalog denominator changed")
	}
	macros := reg.CompilerMacros()
	seen := map[string]bool{}
	for i, m := range macros {
		if seen[m.CallableName] {
			t.Fatal("duplicate macro name")
		}
		seen[m.CallableName] = true
		if i > 0 && macros[i-1].CallableName >= m.CallableName {
			t.Fatal("macro order is not stable")
		}
	}
	for _, name := range []string{"debugLog", "debugExportTrace", "require", "measure"} {
		if !seen[name] {
			t.Fatalf("missing existing macro %s", name)
		}
	}
	for _, a := range reg.AllActions() {
		if a.CallableName == "require" || a.CallableName == "measure" {
			t.Fatal("compiler macro counted as native Apple definition")
		}
	}
	fingerprint := reg.CompilerMacroFingerprint()
	if len(fingerprint) != 64 {
		t.Fatal("missing macro fingerprint")
	}
	macros[0].CallableName = "MUTATED"
	if len(macros[0].Parameters) > 0 {
		macros[0].Parameters[0].Label = "MUTATED"
	}
	if reg.CompilerMacroFingerprint() != fingerprint || reg.Fingerprint() != nativeBefore {
		t.Fatal("caller mutated shared registry")
	}
}

func TestCompilerMacroFingerprintRejectsSignatureMutations(t *testing.T) {
	reg := DefaultRegistry()
	original := reg.CompilerMacros()
	baseline := compilerMacroFingerprint(original)
	reversed := reg.CompilerMacros()
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	if compilerMacroFingerprint(reversed) != baseline {
		t.Fatal("map/order altered fingerprint")
	}
	mutations := []func([]*ActionSchema){
		func(ms []*ActionSchema) { ms[0].OutputTypeName = "MUTATED" },
		func(ms []*ActionSchema) { ms[0].ScopedBlock = !ms[0].ScopedBlock },
		func(ms []*ActionSchema) {
			for _, m := range ms {
				if len(m.Parameters) > 0 {
					m.Parameters[0].Literal = !m.Parameters[0].Literal
					return
				}
			}
		},
		func(ms []*ActionSchema) {
			for _, m := range ms {
				if len(m.Parameters) > 0 {
					m.Parameters[0].WireKey = "MUTATED"
					return
				}
			}
		},
	}
	for i, mutate := range mutations {
		m := reg.CompilerMacros()
		mutate(m)
		if compilerMacroFingerprint(m) == baseline {
			t.Fatalf("semantic mutation%d not detected", i)
		}
	}
	edited := reg.CompilerMacros()
	edited[0].Docs.Description = "editorial update"
	if compilerMacroFingerprint(edited) != baseline {
		t.Fatal("editorial text changed executable fingerprint")
	}
}
