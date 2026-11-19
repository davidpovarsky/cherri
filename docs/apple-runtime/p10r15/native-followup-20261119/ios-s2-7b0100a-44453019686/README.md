# Actual iOS S2 evidence

Run 44453019686 completed successfully on exact source
7b0100a2a9ce5991ca8ffe82c805278e5aa7754e at 2026-11-19T04:20:10Z.

https://github.com/davidpovarsky/cherri/actions/runs/44453019686

All 50 observed iOS tests passed; there are zero failed or incomplete
test records and zero UI failures. The source-bound collector emitted
18 formal records: seven UI, ten service, and one build. It retained
57 behavioral UI receipts, 33 startup receipts (11 tests with three
fixed-state stages each), and 197 synthetic input receipts.

The downloaded three-member ios-build-evidence ZIP is preserved together
with its exact original members. Every evidence record points at the
verified sanitized log digest. Replaying the exact-S2 collector against
that log and the published exit code produced identical JSON structures
and identical sanitized log bytes. Windows replay JSON bytes differ due
to serialization; the original CI files were not rewritten.

This verifies this iOS app/core/UI source only. There are zero native
Shortcut runtime events in this artifact. It does not establish signed
Unicode export, binding capture, typed defaults, F04 nested lists, or
physical-device acceptance. The older 4ff1d 49/50 failure and subsequent
ddc6c02 50/50 success remain preserved with their own source identities.

Public evidence contains sanitized synthetic editor state and fixed
startup booleans; raw logs and screenshots were not published.
