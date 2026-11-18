# P15 Stored Content feasibility probes

Status: **source and harness implemented; native execution pending**. This stage does not implement a cache macro or claim persistence across runs.

## Existing contract and exact boundary

Asana cache task [1220544665232085](https://app.asana.com/0/0/1220544665232085) depends on storage task [1219814215189434](https://app.asana.com/0/0/1219814215189434). The requested cache needs scoped keys, TTL and clock behavior, corruption handling, concurrent access, one lazy body evaluation, privacy controls, and typed value preservation. These are separate unfinished implementation and acceptance obligations.

The existing [storage action definitions](../../../actions/storage.cherri) already expose `storeValue`, `getStoredValue`, and `deleteStoredValue` for version 27. The independent [native metadata snapshot](../macos-27/builtin-actions.json) names `WFSetStoredContentAction`, `WFGetStoredContentAction`, and `WFDeleteStoredContentAction`. Store metadata advertises generic multiple `WFContentItem` input/output and passthrough, while its `WFInput` UI parameter is `WFTextInputParameter`. The authored Cherri `storeValue.value` schema is currently **Text with the canonical text codec**. Those facts do not establish arbitrary typed archival. This stage preserves that contract and adds no storage implementation.

The metadata snapshot is from macOS 27. It does not establish execution on the iPadOS 27 simulator or a physical device. Legacy programs use the existing explicit `#include 'actions/storage'`; the V2 registry already exposes these actions.

## Three controlled probes

All source values are synthetic. Every write/delete uses a fixed owned `cherri.qa.p15.storage.*.35a6b60` key. No Lumen/user keys, private corpus data, external save destination, network request, or new backend is used. Separate assertion flags prevent a later failure from overwriting an earlier result.

| Explicit QA selection | What it checks in one invocation | Required exact result |
|---|---|---|
| `qa-storage-text-same-run` | Original Text, Hebrew combining marks, emoji, quotes, backslash and newline; native type comparison; exact readback; own key deletion | `CHERRI_QA_STORAGE_TEXT_RESULT:TYPE_OK:VALUE_OK` |
| `qa-storage-scope` | Different local and global Text values under the same owned key, separate type/value assertions, deletion of both scoped entries | `CHERRI_QA_STORAGE_SCOPE_RESULT:GLOBAL_TYPE_OK:GLOBAL_VALUE_OK:LOCAL_TYPE_OK:LOCAL_VALUE_OK` |
| `qa-storage-delete` | Known value before deletion and zero content items after reading the deleted key | `CHERRI_QA_STORAGE_DELETE_RESULT:BEFORE_OK:NO_VALUE_OK` |

The deletion probe is a diagnostic, not a statement that reading a missing key succeeds. If the action errors, the ordinary runtime gate remains **FAIL**. No catch or retry behavior is inferred.

Each fixture uses the existing before/after trace and `debugExportTrace` envelope. All are Debug fixtures with `REQUIRED_NDJSON`; none receives the dedicated Release-measure exception. Fresh callback correlation, exact source hash, shortcut identity, exact marker, normal trace and the existing publication quarantine remain mandatory. Raw source, unsigned/signed shortcut files, local stores and raster captures are not promoted as public evidence.

The source hashes and declared limits are in [storage-probe-manifest.json](storage-probe-manifest.json). The production selector uses explicit choices and immutable source paths. It does not accept an arbitrary storage key or source path.

## Reproducible source checks

The focused Go suite checks the independent native metadata, actual emitted native graph, original Text producer UUID/name, effect order, Bool scope, readback consumption, cleanup and semantic conditions. Eighteen causal mutations must fail: foreign key, flipped scope, missing cleanup, constant replacement for a read, reversed comparison and added network action, for each of three fixtures. A restored original graph must pass after each mutation. A separate four-configuration check compiles existing Text storage primitives with Legacy/V2 and Debug/Release.

The Python contracts execute the real shell selector through Bash stdin, the actual callback verifier, and exact source hash/UTF-8 checks. Invalid selectors, forged Release status, wrong or partial markers, stale/replayed callbacks, self-tests and missing/wrong traces remain rejected.

After committing a clean source checkout:

~~~text
go build -o dist/storage-probe-compiler.exe .
py -3 -X utf8 tools/p10r15/verify_storage_probes.py dist/storage-probe-compiler.exe
~~~

The producer verifies the compiler's exact VCS revision and clean flag, binds reviewed source bytes to HEAD, runs the focused Go and Python gates, executes the real Phase 0 CLI compilation, and losslessly imports/rebuilds all three storage graphs with exact native action equality. The resulting ignored receipt can be copied into a separate evidence commit; it records source identity, hashes, counts and the explicit native limits. It never signs or simulates storage.

## Work still required

A successful same-run result would establish only that bounded primitive behavior. Two workflows using newly created simulators cannot prove persistence. A cross-run producer must keep the **same installed shortcut and simulator/device state**, finish the write invocation, run an independent read invocation with a fresh callback identity, compare the value and clean up its owned keys. Process restart, device restart and physical-device evidence must be distinguished.

A shared cache facility remains ordinary pending engineering after those primitive boundaries are known: structured lazy body support, typed result/reference ownership, namespacing/versioning, TTL clock semantics and a declared corruption/concurrency policy. Dict/List/Date/File archival cannot be advertised from generic metadata or an apparent Text roundtrip. The original Asana tasks remain open.
