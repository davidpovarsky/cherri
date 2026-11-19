# Eight independently executed local contracts

Verified source: `d4256019b3465ee9f7b3726d4f23f6d449502be0`.

The final clean run executed RP05, RP09, RP10, RP30, RP31, RP32, BRG05, and BRG17 independently from their immutable contracts. The focused public route/native oracle suite passed 57 named checks. The formal producer passed 49 named checks, including 48 deliberate evidence mutations, producing 17 source-bound records for 8 complete local contracts. Both runs had zero failures and zero skips; source was clean before and after. Full audit remains 179 rows, with no inherited passes or release closure.

The negative manifest checks remove required test selections/assertions/tiers and substitute wrong producers, stale source IDs, and unexecuted runtime tiers. Native evidence here covers actual serialized Apple-format data, strict roundtrips, and canonical differentials; it does not establish Apple runtime, automation installation, UI, or device behavior. RP20 remains unverified.

Two initial clean development runs exposed test harness assumptions: required Legacy action includes were missing, and the hybrid-verification helper intentionally omitted authored headers. The fixtures now use explicit raw include syntax and the actual unsigned Legacy CLI for trigger header serialization. Those failed attempts are retained intact. No production behavior was changed for those fixture corrections.

`raw-evidence.zip` preserves the original raw JSONL streams, summaries, actual producer receipt, and execution driver. `archive-index.json` hashes every original stream. The readable receipt is extracted by concatenating Go test JSON Output chunks and parsing the emitted producer JSON; it was not reconstructed from claimed counts.

Archive SHA256: `93bd1a76302390dd10a98ca88197130381f8ea2a2897962604c8e7e76916464d`.
