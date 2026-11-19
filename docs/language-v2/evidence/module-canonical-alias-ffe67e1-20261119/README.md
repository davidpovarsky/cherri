# Built-in module aliases: canonical identity

This archive records the public causal baseline and fixed tests for source commit ffe67e1cc7c2363c813d9a32a1865b655597708f. It is a bounded compiler proof, not a final release, Apple runtime, or complete module acceptance claim.

The original source replaced stable action schema IDs with source alias spellings. The canonical backend correctly rejected those unknown definition IDs. The repair changes only imported CallableName, preserves the existing stable schema ID and Variant metadata, and passes the schema ID into the canonical resolved call. The canonical adapter lookup and strict native comparator are unchanged.

The new tests compare actual unqualified native output to lowercase and uppercase imported aliases for three real catalog actions and independently inspect 13 real exported schemas across four built-in modules. They also check that import aliases do not mutate the shared registry. No new interpretation of Variant as VariantID is introduced.

| Execution | Result | Seconds | Source state |
| --- | --- | ---: | --- |
| Causal baseline | 3 pass, 8 fail, 0 skip | 29.625 | Existing production at 6fd0476 plus the new test |
| Working repair | 343 pass, 0 fail, 0 skip | 45.922 | Two production-line repair plus the unchanged test |
| Exact committed repair | 343 pass, 0 fail, 0 skip | 41.750 | Clean ffe67e1 before and after |

The eight baseline failure events are six alias compilation failures and their parity parent, plus the schema identity test. The three unqualified controls passed. The 343-case gate also covers existing module, call, enum, value-flow and control-type regressions; exact commands and source hashes are in the receipts.

The earlier runner-construction syntax error occurred before any test launch and supplied no evidence. This archive retains the successfully executed baseline and both executed repair gates. The archive contains public test source provenance, text logs, JSON receipts and runners only. No private corpus, native artifact or compiler executable is included.
