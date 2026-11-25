# Strict Go test event compatibility — 2026-11-25

## Failure and change

The frozen differential gate at ca26086ba81ed7247b7b009aff2f59d5943dc9d6 rejected successful Go 1.27.4 production subprocesses because their JSON output events contain the documented optional OutputType field. The shared acceptance decoder used DisallowUnknownFields but did not declare this field. RP50 harness intake and editor/exhaustive-node producers therefore failed before their existing event inventories could be checked.

The shared validator now declares OutputType and validates its exact field spelling, string type, event placement, and documented classification. Omitted or blank metadata remains compatible with prior toolchains. Framing output is accepted. Error and error-continuation output explicitly fail a successful-evidence intake, including a contradictory synthetic stream that later claims pass. Unrecognized fields or classifications still fail.

Package/test identity, required inventory, run/pass order, parent-child ownership, elapsed budgets, timestamps, package completion, duplicate JSON keys, UTF-8 validation, and DisallowUnknownFields remain enforced. No acceptance tier or language/runtime expected result changed.

## Primary source

The actual installed toolchain reports go1.27.4 linux/arm64. Its source was inspected read-only:

- /mnt/development/toolchains/go1.27.4/go/src/cmd/test2json/main.go, lines 34–46 and 89–95: the command's event schema and OutputType contract. SHA256 98613882be8e9dcfed687f594b253b2a0474771981099174406ad74bb377b5c7.
- /mnt/development/toolchains/go1.27.4/go/src/cmd/internal/test2json/test2json.go, event definition and writeOutputEvent: actual production of framing/error classifications. SHA256 81b811cc3af1cc49da5faec1a93aabf12b769a69a5dbc60505c125edb19d288b.

The source paths, hashes and bounded excerpts are retained in the external evidence receipt toolchain-source.json. The change follows this installed producer's contract and does not accept future fields without an explicit schema update.

## Focused evidence

Evidence directory: /mnt/development/projects/evidence/cherri-differential-30-20261125/compatibility/go-events.

- RED: output-type-red.log, SHA256 ce54061d8f63002346f2e872607986ad98a0a456e99b0d4c38d0b48fbb2845fe. Both explicit blank/framing controls and an actual go test -json producer failed with unknown field OutputType. The omitted-field control already passed.
- GREEN: output-type-green.log, SHA256 f8168e51b600429aa1f5d2120c035d9d1315b5765aa4b80317bcfddf4ed670d0. Three new focused top-level tests plus the existing strict editor and exhaustive-node parser controls passed.
- The actual producer was a temporary Go module running one real test, not a synthetic success receipt. Go 1.27.4 emitted three framing events. The accepted stream had 1,402 bytes and SHA256 6b7e26493245862e79ea01404ff6db18fdb1065277643fbfc13d31444f314147.
- Nineteen new negative controls reject wrong types, null, wrong placement, unknown values/fields, duplicate OutputType keys, error classifications followed by claimed pass, foreign identities, failed/skipped status, and missing elapsed time.
- git diff --check passed. The full suite remains root-owned; these controls establish parser compatibility only.

BRG33's separate T04 date-quantity diagnostic masking failure is not attributed to OutputType. Its permissive event collector was already ignoring this field; the isolated production failure has a distinct receipt and repair.
