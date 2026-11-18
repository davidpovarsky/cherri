# Remaining macro feasibility audit

Observed 2026-11-18 against the P10R15 integration code and canonical macro metadata at ab9a264ba370890397d74042e00c11a3b4c84ffd. This is a bounded source and native-metadata assessment. It does not replace runtime acceptance or close any Asana child.

## What can be implemented with the existing architecture

| Child | Ordinary implementation work | Separate native prerequisite |
|---|---|---|
| debugLog labels — 1220544718902445 | Extend the existing macro and log path with per-entry label metadata; preserve the original typed raw log and original producer UUID. | Show typed preview/label behavior across branches, repeat flattening, RTL, duplicate labels and six modes on Apple. |
| Local debugExport — 1220544665232213 | Define serialization/redaction/file-result contracts and reuse existing saveFile/saveFilePrompt action definitions. | Actual local file provider, output object, permission failures and opaque native type handling. |
| Shared blocks — 1220544718901241 | Extend the implemented V2 typed measure block through the existing AST/analyzer/IR; define void/early-exit and intended Legacy boundary explicitly. | Native control-flow/type-result behavior for unsupported forms. |
| Persistent cache — 1220544665232085 | Namespace/TTL/invalidation/serialization and collision/clock policy are missing source work. | Stored Content across runs/restart, scope, missing/corrupt data, permission and physical-device persistence/background. |
| Retry — 1220544651247273 | Bounded control policy, idempotence and explicit classification are missing source work. | A recoverable error/status contract has not been demonstrated. Transport/permission/cancel must not be represented as invented catchable values. |
| Pagination — 1220544665232021 | A bounded GET/body-cursor subset can reuse existing loops, typed list/dictionary handling and current HTTP actions. | Actual multi-page response schema, empty/duplicate cursors, status/errors/rate limits/cancellation and signed native execution. |

None of label/export/pagination is blocked solely by access to Apple hardware. Cache and retry have both missing implementation and specific unproven native prerequisites.

## Labels: preserve the current log rather than replacing it

debug_stdlib.go currently accepts the literal withDates/showLogs/showLogsAsText flags and appends the original previous native ActionOutput into the existing execution-local log. The default typed preview uses raw values. Dated typed preview names the presentation item inside a repeat and previews Repeat Results; the raw log is separate. Creation Date capture is tied to the original producer and includes a later-preview backfill route.

A label parameter is absent from actual canonical debugLog metadata. Setting CustomOutputName would only affect the compiled action label; it would not establish stable per-entry runtime log metadata. An implementation must associate labels with each entry even when native Content Graph processing flattens lists, branches skip a call or repeats reuse labels. It must avoid extra work when the label option is absent and keep original downstream references. There is no reason to introduce a second log registry.

## Export: existing file actions are building blocks

actions/documents.cherri provides saveFilePrompt and saveFile using is.workflow.actions.documentpicker.save. saveFile maps WFFileDestinationPath, WFInput, WFSaveFileOverwrite and optional WFFolder and sets WFAskWhereToSave=false. The independent native metadata declares a returned public.data file. An unspecified folder is not evidence that storage is local; silent cloud placement would violate the requested contract.

The missing work includes independently parsed UTF-8 JSON, correct escaping, labels/timestamps/type/error fields, default auth/token/query-secret redaction, zero/one/1000-entry cases, and an explicit destination. Native runtime type introspection alone is insufficient to reconstruct every Bool/Number/List/null distinction: Content Graph may erase singleton collection identity. A bounded first implementation could support source-known serializable values and explicitly mark opaque/unsupported values, with no implicit binary upload or cloud/share action. It must not advertise universal typed serialization. debugExportTrace remains separate internal QA JSONL plumbing.

## Persistent cache: native descriptors do not prove persistence

actions/storage.cherri exposes iOS 27 storeValue/getStoredValue/deleteStoredValue and local/global scope. The current authored storeValue parameter is text. Native setstoredcontent/getstoredcontent metadata allows WFContentItem and Multiple=true, but that extraction establishes parameter/output shape, not durable serialization or cross-run readback.

The historical device acceptance manifest still marks storage and dynamic HTTP fixtures COMPILED_UNSIGNED_PENDING_DEVICE / BLOCKED_DEVICE. The storage fixture has a same-run positive assertion; its previous-run value is only displayed. It does not prove persistence across two runs or restart. Its old lumen namespace also differs from the manifest's claimed synthetic key, so a new proof should use unique cherri.qa keys and explicit cleanup rather than reuse ambiguous storage.

A safe next stage is an independently authored, signed synthetic writer and reader pair through the existing harness. The reader must assert exact typed value and scope after a distinct run; add restart, stale TTL, miss/corruption, delete, permission and global/local isolation. Physical iPad evidence remains necessary for background/persistence claims. An execution-local dictionary is not a replacement for this feature.

## Retry and pagination: establish the actual HTTP/error boundary

actions/web.cherri already maps downloadURL and JSON/form/file requests to is.workflow.actions.downloadurl. Canonical native metadata exposes a public.data Contents of URL result and request URL/method/headers/body parameters. url.getheaders is a separate action returning a dictionary; a second headers request is not the original request's status or failure result.

The bounded extracted built-in inventory search found no catch/retry action among its 434 records. That is not a statement that every Apple API lacks error handling. It means no usable native error-to-value/catch contract is established by current evidence. The runtime owner independently confirms no current signed proof of HTTP response status, recoverable transport error, cancellation cause or stored-content cross-run behavior. Native Alert plus Stop Shortcut is a semantic stop, not a catchable exception.

A narrow retry contract could accept an operation that explicitly returns retryable status as data, restrict side effects to GET/idempotent requests, bound attempts/delay, and let native transport/permission/cancel failures abort. Such a contract needs an explicit scope decision and controlled 500-to-200/404/429/timeout/denied/cancel probes; it cannot silently claim the broader roadmap contract.

A first pagination subset can use GET, explicit same-origin cursor construction, maxPages/maxItems, schema-checked List<Dictionary>, and empty/duplicate-cursor termination. Generic Link headers, response status and automatic retry should remain unsupported until proven. This is implementable source work using existing loops; native synthetic endpoint acceptance is a separate gate.

## Independent Get Type contract finding

The require type witness originally used gettypeaction. Checked-in native metadata independently shows:

| ID | Apple action class/name | Input | Output |
|---|---|---|---|
| is.workflow.actions.gettypeaction | WFGetTypeAction / Get File of Type | WFInput, WFContentItem; optional WFFileType UTI, default public.rtf | File of Type, public.data, Multiple=true |
| is.workflow.actions.getitemtype | WFGetItemTypeAction / Get Type | WFInput, WFContentItem | Type, NSString, Multiple=true |

Core corrected require to the existing canonical typeOf/getitemtype and added metadata-backed causal checks. The older getType alias intentionally includes WFFileType and must keep gettypeaction. Its misleading Get Type doc and text return type require a separate contract correction. Legacy collectType does not accept a file token, so the narrow existing architecture route is a truthful variable return plus the canonical schema generator's existing OutputTypeOverride File facet. Root/core own that correction; this audit does not patch it.

The correction changes the native schema fingerprint and consumers need fresh generated docs/pins. Historical parity against the same incorrect declaration is not independent output-type proof; the old prose parity report even labels getType as getitemtype while actual generated/test payloads use gettypeaction.

## Evidence boundary and next executable work

The new macro coverage producer builds an actual compiler, reads its compilerMacros descriptors, generates its canonical docs, executes controlled require/measure/for-yield/profile cases, verifies native IDs/UUIDs/action counts, imports their expanded output losslessly and recompiles it. It keeps all eleven Asana children OPEN and records missing native/consumer proof separately.

The native specimen manifest currently contains zero independently authored specimens. A compiler-produced shortcut cannot become an independent native exemplar by relabeling it. Final signed simulator receipts, physical proof and installed consumer tuple must be added from their actual owners and exact source revisions.
