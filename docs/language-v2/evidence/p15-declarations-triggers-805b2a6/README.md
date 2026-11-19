# Declaration and trigger execution evidence

Verified source: `805b2a69d9a4d0c651c15c5fc37ebf082741a342`.

The actual producer independently executed the immutable AC03 and M05 scenarios. AC03 exercised unknown-codec and invalid-label declarations, healthy declarations, duplicate enum/call-label diagnostics, and the actual tracked-source generation rehearsal that rejects stale facets and restores the healthy generated registry. M05 executed all seven existing trigger families through migration, actual Legacy CLI serialization, V2 serialization, and XML/binary import/recompile, including payload/order/identity deletion mutations.

The clean formal gate passed 13 named checks, including 12 deliberately damaged manifest controls, in 43.854 seconds: 2 complete local contracts, 3 source-bound tier records, zero failures/skips. Source status was clean before/after, and the full audit stayed at 179 requirements with no inherited result. AC02 remains unverified by these tests. Automation installation, Apple runtime and release closure were not claimed.

`raw-evidence.zip` retains the exact JSONL output, summary, emitted receipt and execution driver. A first host-only driver failed Python parsing before any Go command because a JavaScript replacement expanded a special replacement sequence; that invalid driver is retained for provenance. The corrected driver executed the test once and produced the hashed successful log.

Raw Go JSONL SHA256: `e87643ea680b1351ff2254015617416f2337f8fb435b0a3a15eee97e48856c8f`.
Archive SHA256: `0ed21e35d4ca4a47cdd32683c95891a8702e483ce905a8467c2ff9c563617b47`.
