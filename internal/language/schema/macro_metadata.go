package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// CompilerMacros exposes the existing compiler-only overlay separately from
// the native Apple catalog. Returned descriptors cannot mutate that overlay.
func (r *Registry) CompilerMacros() []*ActionSchema {
	result := make([]*ActionSchema, 0, len(compilerMacroSchemas))
	for _, original := range compilerMacroSchemas {
		copy := cloneCompilerMacro(original)
		result = append(result, copy)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CallableName < result[j].CallableName })
	return result
}

func cloneCompilerMacro(original *ActionSchema) *ActionSchema {
	copy := *original
	copy.Parameters = append([]ParameterSchema(nil), original.Parameters...)
	for i := range copy.Parameters {
		copy.Parameters[i].AcceptedForms = append([]string(nil), original.Parameters[i].AcceptedForms...)
		copy.Parameters[i].EnumValues = append([]string(nil), original.Parameters[i].EnumValues...)
	}
	copy.EmittedKeys = append([]string(nil), original.EmittedKeys...)
	copy.Docs.Keywords = append([]string(nil), original.Docs.Keywords...)
	return &copy
}

// CompilerMacroFingerprint binds every public compiler-macro signature,
// including literal-only parameters and scoped-block requirements. It does not
// change the separately pinned 505-entry native catalog fingerprint.
func (r *Registry) CompilerMacroFingerprint() string {
	return compilerMacroFingerprint(r.CompilerMacros())
}

func compilerMacroFingerprint(macros []*ActionSchema) string {
	descriptors := make([]*ActionSchema, 0, len(macros))
	for _, m := range macros {
		c := cloneCompilerMacro(m)
		// Editorial wording is not an executable compatibility contract.
		c.Docs = ActionDocs{}
		descriptors = append(descriptors, c)
	}
	sort.Slice(descriptors, func(i, j int) bool { return descriptors[i].CallableName < descriptors[j].CallableName })
	encoded, err := json.Marshal(descriptors)
	if err != nil {
		panic("compiler macro metadata is not serializable: " + err.Error())
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
