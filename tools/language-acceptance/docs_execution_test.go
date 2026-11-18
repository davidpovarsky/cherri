package main

import "testing"

func TestGeneratedDocumentationProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "D01", documentationProductionTests["D01"], nil, (*Runner).runGeneratedDocumentationRequirement,
		"P15_GENERATED_DOCS_RECEIPT:",
		"All505 canonical native definitions and15 separate compiler macros, complete generated signatures/categories, optional primary/default controls and8 real generator CLI drift rejections. This repository's canonical reference; external pinned docs and published-site provenance have separate receipts.")
}
func TestExecutableDocumentationProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "D02", documentationProductionTests["D02"], nil, (*Runner).runExecutableDocumentationRequirement,
		"P15_EXECUTABLE_DOCS_RECEIPT:",
		"Actual public JSON check and unsigned plist compilation for every runnable canonical-guide fence; deliberately invalid source must produce its exact code and no artifact. External37-example docs runner has a separate pinned-compiler receipt. No Apple runtime, site publication or D03 closure.")
}
