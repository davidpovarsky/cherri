# Signed pagination probes

**Native status: NOT_EXECUTED.** This stage registers five controlled V2
fixtures in the existing signed Simulator pipeline. It does not close Asana
1220544665232021 or infer HTTP error handling from a successful callback.

The data commit is
`38c7098772f4b822d6a3555137c0c7cf8e5b1e5e`.
All six JSON pages reside under the existing repository's
`docs/apple-runtime/p10r15/paginate-pages/` directory. The fixture manifest
records every exact raw GitHub URL, data hash and fixture source hash. The
compiler commit and data commit are separate identities.

| Workflow selection | Exact fixture | Required semantic result |
| --- | --- | --- |
| `qa-paginate-ordered` | `qa-paginate-ordered-v2` | Two ordered Dictionary items across three pages, including an empty middle page; native types, Number/Bool/zero, Hebrew/emoji and nested values checked |
| `qa-paginate-cycle` | `qa-paginate-cycle-v2` | Two ordered items from a two-page cycle with maxPages 5 and maxItems 10 |
| `qa-paginate-page-cap` | `qa-paginate-page-cap-v2` | First Dictionary only with maxPages 1 |
| `qa-paginate-item-cap` | `qa-paginate-item-cap-v2` | First Dictionary only with maxItems 1 |
| `qa-paginate-empty` | `qa-paginate-empty-v2` | Empty page plus zero-page and zero-item budgets produce empty results |

The first two requested pilots are **ordered** and **cycle**. The remaining
three isolate cap/empty behavior after initial native evidence is inspected.
A nested/null cardinality mismatch is a failing observation, not a reason to
weaken the probe. The first fixture compares independent native type witnesses
and exact Unicode source bytes.

Each fixture returns only
`CHERRI_QA_PAGINATE_PROBE_RESULT:P15_PAGINATE_<SCENARIO>_OK`
after its internal assertions, followed by the existing normal trace envelope.
Every new target requires Debug + `REQUIRED_NDJSON`, fresh nonce, exact source,
shortcut identity, callback marker and actual trace. The Release no-trace
exception remains exclusively `qa-measure-release-v2`.

## Preflight and evidence boundaries

The actual orchestrator invokes
`tools/p10r15/verify_paginate_probe_data.py` for these five explicit selections
before simulator/signing work. The verifier checks:

- Full immutable data SHA, exact reviewed page and fixture paths, checkout
  hashes and source URLs. Every page, fixture, manifest and the verifier must
  exist byte-exact in HEAD; ignored, merely staged or modified files are rejected.
- Six real HTTP 200 responses with exact URLs, no redirect, bounded bytes and
  matching SHA256. No credentials or private content are sent.
- Source-byte binding, including the literal Unicode expected by the ordered
  fixture.

Any verification failure stops the pipeline before native work. Existing
unrelated selections do not acquire this network preflight. The runtime
data-verification receipt stays quarantined with other nonallowlisted raw
artifacts; the committed compact local receipt supplies reviewable data-byte
evidence. No response body is stored in a public receipt or callback.

Publication source trust adds only these five exact nested relative filenames.
It does not trust the whole directory or a matching basename elsewhere. Raw
sources, unsigned/signed shortcuts, raster files and archives retain the
existing quarantine policy.

## Executed source/gate verification

```sh
python3 -m unittest discover -s tools/shortcuts-simulator-qa -p 'test_*.py' -v
python3 tools/p10r15/verify_paginate_probe_data.py --report dist/paginate-public-data-verification.json
python3 tools/shortcuts-simulator-qa/validate_macro_fixtures.py . dist/paginate-harness-compiler
```

The last command runs the actual production Phase 0 using the real local CLI
and applies causal Release/publication faults. It never signs or runs a
shortcut. Callback tests reject wrong markers, ERROR/CANCELLED, stale nonce,
wrong source/shortcut, replay, missing/wrong trace and a forged Release
exception for every new target.

The public-data tests reject mutable SHA, wrong URL/path/hash, duplicate
fixture identity, changed source URL even after its hash is updated, HTTP 404,
redirect, changed/oversized bytes and timeout. The actual shell preflight is
executed with a controlled verifier dependency: nonzero verifier status must
prevent the next stage, while unrelated fixture selection remains independent.

### Retained Windows test-runner failure

The expanded selector is 9,741 characters. Git Bash on this Windows host
truncated the selector when the tests supplied it through `bash -c`,
producing an unexpected EOF at line 177. The identical 213-line selector
passed full shell syntax checking and executed correctly through
`bash -s` stdin. Selector tests now use stdin, including the collection
tests; assertions and production selector behavior are preserved. The failed
runs and corrected run hashes/counts are retained in the local handoff receipt.

Native runtime, native HTTP response status/error/cancellation behavior,
redirect handling, actual request observations and physical iPad acceptance
remain separate requirements. A local data download or fixture compilation
does not satisfy them.

### Retained ignored-source staging failure

The first local harness commit omitted five new `.cherri` fixtures because the
repository ignore rules excluded them from ordinary directory staging. This was
caught before handoff or dispatch. The follow-up adds each reviewed source
explicitly and verifies the committed bytes before any public-data request.
A causal temporary Git repository test retains the ignored/missing, staged-only
and modified/staged-again failures plus restored committed controls.
