# Frozen compiler documentation verification

Compiler S: `b34ae47ba79d15c08e8a8bdec9a761a7a4b722bb`.
Documentation D: `9fb6256a4549d93414e9b65318d4794ae40fbee7`.

The new isolated docs checkout pins the exact frozen compiler and explains the observed nested List result boundary. Its diagnostic example is executed against S: `E_TARGET_VALUE_SHAPE`, failed compilation, and no unsigned artifact. Unknown `AnyContent` nesting remains explicitly unverified.

## Actual verification

| Scope | Observed result |
| --- | --- |
| Compiler provenance | Ordinary clean Git clone at S; real Windows binary has exact VCS revision and `vcs.modified=false`; SHA-256 `89b7de6d44d852e88d13f6191c0e9403f19b37c3b0e85d1c2b79d93d530498df` |
| Complete canonical reference | S generator output equals the entire committed actions body, SHA-256 `96aeab4c5fb5ba3be4f7e9a7e7f31335630a169eb2016d5f6d9a9eb4f9032324`; frontmatter preserved |
| Windows docs examples | 38/38 passed: 35 runnable examples and three exact negative diagnostics |
| Real compiler cache controls | 12/12 passed, zero failures/skips; the wrong-revision control used the actual previously verified S71225 Windows compiler |
| Independent Linux CI | Both snippet/cache and Jekyll site jobs succeeded on exact D |
| Rendered artifact | 520 reference signatures, 505 native definitions, 415 search entries and 21 action search sections; downloaded artifacts rechecked locally |
| F04 rendered guide | Actual generated HTML contains the diagnostic, observed cardinality limit and explicit AnyContent uncertainty |

The 524 reference-only blocks are generated signatures or explicitly identified historical fragments. They are not reported as executed examples. CI independently executed the same 38 examples; its Linux compiler binary has a different platform-specific hash from Windows.

CI: https://github.com/davidpovarsky/cherrilang.org/actions/runs/44443573124

The Windows compiler build and canonical generator ran at below-normal priority with one Go worker, a 120-second process deadline and a memory guard. The complete local preparation and validation took 57.906 seconds and left both the docs checkout and normal compiler source clone clean. The previous D7f fixture used by the separate ongoing K4ff suite was not modified.

The committed LF Markdown and Windows CRLF checkout have different raw file hashes; `committed-canonical-reference.json` verifies that newline normalization is the only difference and that the committed body is byte-identical to the canonical generated body. Both raw hashes are retained rather than conflated.

Actual S capabilities:

- Schema: `7bfff2b8c3fddf3892e943ef45b79bbbc4cc984ad051a15099a5da0fc6fccc0b`
- Codec: `3e7e3555eefe1e030d254df0a22c62fdc3a139f92e39da34f6d841f8bfa1689e`
- Macro: `1fe7f7993e2473aec591341712f42c521f913974b8e84091a5c2950232230dec`

These are documentation, source, cache and rendered-site checks. They do not certify Apple runtime behavior, publication of the live website, or full 179-requirement closure.

Original receipt and log bytes are preserved in `raw-evidence.zip`, with their digests in `raw-file-sha256.json`. The readable JSON copies normalize only CRLF line endings to LF; values and counts are unchanged. The original Go build-info line includes its native trailing tab and is retained inside the ZIP.
