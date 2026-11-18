# Frozen native pilot audit contracts

Source commit: 36fa81b3543d468710468aad2252a9e6899b3e4a. Twelve Python protocol and frozen-source contract tests pass from a clean committed tree. The auditor now recognizes the existing empty split, collect counts, nil clear, file name binding and explicit numeric coercion pilots, deriving fixture bytes and expected output from the reviewed Git source rather than downloaded artifact claims.

The real run 44401898941 remains BLOCKED_IMPORT. Its strict audit exits 3 with FAIL_EVIDENCE_VALIDATION: source provenance disagreement. Published assertions contain UNKNOWN source/build provenance on the blocked import path. This is an observed failure, not native runtime acceptance. No Apple behavior, formal runtime ID, signing-byte verification, or release gate is closed by these twelve synthetic protocol checks.

The original four audited pilot contracts and nine tests remain in scope. Added tests cover exact current selectors, rejected source-contract changes, and numeric-coercion receipt acceptance/rejection. See manifest.json for original host byte hashes and explicit normalized archive hashes.
