# P10R15 bounded GET pagination

Asana task: 1220544665232021 (parent 1220544599390563). **OPEN.**
The task's proposed trailing-block API is not an existing language guarantee.
This stage implements a smaller, explicit body-next GET contract through the
existing compiler macro registry, native actions, shared text codec and shared
control builders. It does not add a network client or storage infrastructure.

## Implemented source contract

Both Legacy and V2 accept the same positional arguments. V2 also accepts named
arguments through the ordinary binder:

```cherri
let rows = paginate(
    "https://example.invalid/pages/one.json",
    itemsKey: "items",
    nextKey: "next",
    maxPages: 20,
    maxItems: 1000
)
for row in rows {
    getValue(row, "id")
}
```

The result is statically `List<Map<Text,AnyContent>>`. Items are appended by
their original native Dictionary reference. There is no JSON/Text round-trip or
wrapper Dictionary around each item. Nested values retain their original
references in the emitted graph.

| Option | Implemented boundary |
| --- | --- |
| `startURL` | Required literal absolute HTTP(S) URL; no userinfo, fragment, raw whitespace or backslash |
| `itemsKey` | Literal simple key, default `items`; 1–64 ASCII characters, starting with a letter or underscore |
| `nextKey` | Same key restrictions, default `next`; must differ from `itemsKey` |
| `maxPages` | Literal integer 0–100, default 20; bounds native GET action invocations |
| `maxItems` | Literal integer 0–10,000, default 1,000; returns the ordered bounded prefix |

Either zero cap produces only an empty native List, Set Variable and Get
Variable: no GET action is emitted. Analysis-only consumers and both compiler
frontends share `schema.ResolvePaginateOptions`. Invalid literal options fail
before producing an artifact, including in Release. Pagination is semantic
source behavior and is not removed by the diagnostic-stripping pass.

A response body must convert to one native Dictionary and contain the exact
configured items key. A nonempty items collection is traversed in order;
every appended item must match a native Dictionary type witness. The page
state and output list use unique per-call variable names. Native Get Type is
`is.workflow.actions.getitemtype`, compared with an actual Dictionary/Text
witness rather than an English type label.

A missing, null/empty or empty-Text next value stops traversal. A nonempty next
value must be one native Text. It may be an absolute URL beginning with the
configured origin and slash, or a simple sibling filename. A sibling filename
cannot contain colon, slash, backslash, whitespace, query, fragment or percent
encoding, and cannot be `.` or `..`. It resolves against the **initial URL's
directory**. Absolute URL paths and queries are allowed within the same origin;
a redirect is outside the guarantee described below.

Before a GET, the resolved URL's SHA256 is compared against prior URL hashes.
This detects a repeat of the exact resolved URL spelling while keeping
case-distinct paths distinct even if native Text equality folds case. Key
comparisons also use SHA256. The public claim is not canonical URL
equivalence: different encodings, redirect targets or different queries may
refer to the same server resource.

Fixed semantic diagnostics have the form `PAGINATE_*`. They do not include
URLs, headers, cursors, tokens or response bodies. They emit native Alert and
Stop, following the existing guarded-macro convention. No catch/retry API or
runtime ERROR classification is implied.

## Executed local verification

Reproduce the source checks with:

```sh
go test -count=1 -timeout=5m -run '^TestP15Paginate' .
python3 tools/p10r15/verify_paginate.py --report dist/paginate-validation.json
```

The producer builds the current compiler, records its hash, captures actual
capabilities, fingerprints all compiler/action sources and its test
dependencies, and rejects source drift during execution. It uses real CLI
compiles and imports/recompiles every resulting native action graph exactly.
It never signs a shortcut or claims a model execution is Apple runtime.

A local HTTP server supplies only synthetic JSON pages. The evaluator consumes
the actual emitted plist graph and reuses the existing native text/reference
decoder from `verify_debug_export.py`. The tests cover:

- Both frontends and Debug/Release, custom selectors, lower/upper caps and
  zero-request paths.
- Three pages with a middle empty page; no-next, empty and singleton inputs;
  1,000 ordered Dictionary items with no extra GET after the cap.
- Repeated URL cycles and case-distinct paths, maxPages and maxItems truncation.
- Unicode, escaped Text, Number, Bool, null, nested Dictionary/List values and
  original object identity under the explicitly stated model assumptions.
- Missing/wrong-case keys, non-Dictionary pages/items, invalid next types,
  cross-origin/escaping next markers, whitespace and fragments.
- Real loopback HTTP 404 and 429 bodies, malformed JSON and no retry.
- Real invalid CLI invocations with nonzero exit and no output artifact.
- Causal changes to the emitted graph's page cap, item cap, cycle predicate,
  original item reference, type guard, HTTP method and full-cap stop. Each
  faulty graph must fail; the restored graph must pass.

The model deliberately exposes a limit: **a 404 response containing an otherwise
valid page body passes the body-only model**. The native action does not expose
a verified response-status contract in this implementation. This case is
recorded as an unverified HTTP-status boundary, not HTTP success.

The existing OpenMinis workflow runs these source checks and retains the compact
receipt even when a subsequent step fails. The checked-in receipt identifies
its exact source commit, source fingerprint, dirty state and native execution
count. Raw HTTP bodies and generated unsigned artifacts stay in ignored
`dist/`.

## Independent native metadata and remaining evidence

The focused Go tests compare native identifiers and input keys against the
checked-in macOS 27 builtin-action metadata, including Get Contents of URL,
Dictionary conversion, Dictionary Value, SHA256, Count, Repeat, list range and
Append Variable. That metadata is independent of the new expansion and does
not prove iPad runtime behavior.

The following remain required before closing the Asana task:

| Requirement | Current boundary |
| --- | --- |
| Signed iPad execution and exact trace | Not executed by this stage |
| Content Graph cardinality | Count Items, null/empty/singleton collapse and collection identity require native proof |
| Typed Dictionary/nested identity | Emitted reference identity and model checks pass; native conversion/Append behavior remains unverified |
| HTTP non-2xx, cancellation and transport errors | No status output, catch, retry/backoff, timeout or causal error classification is implemented/proven |
| Native redirects | `maxPages` bounds GET invocations, not redirect hops or wall-clock duration |
| Strict external JSON array shape | Not promised; scalar Dictionary/null may behave as one/zero items in Content Graph |
| Link headers, page/offset expressions, trailing block, general cursor selectors | Ordinary implementation gaps outside this bounded API |
| Retry/cache dependencies | Separate tasks remain open; this stage does not infer their native feasibility |

A semantic guard prevents the model from returning its partial local result,
but native Alert/Stop output and callback behavior must be tested. Do not label
that behavior a catchable exception or a proven atomic native failure.

## Controlled public pages for the signed phase

`docs/apple-runtime/p10r15/paginate-pages/` contains only reviewed synthetic
pages: one/two/three (middle page empty), empty and a two-page cycle. Their next
values are simple sibling filenames. This allows an immutable raw GitHub
commit URL without embedding a self-referential commit hash inside the JSON.

After the source commit is integrated and pushed, use the published **full
40-character data commit**:

```text
https://raw.githubusercontent.com/davidpovarsky/cherri/<DATA_COMMIT>/docs/apple-runtime/p10r15/paginate-pages/one.json
```

Verify the fetched bytes against the same commit's checked-in files before
dispatch. Pin the data commit separately from the compiler commit. Register
every native fixture explicitly in the existing signed-QA selector/publication
allowlist with a normal trace and exact callback marker; no Release exception
or arbitrary nested fixture directory is authorized by this document.
