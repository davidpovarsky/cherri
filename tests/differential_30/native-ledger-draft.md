# Native evidence ledger

**PRE_URL_REPAIR_NATIVE_LEDGER**. Historical native results apply to the exact artifacts recorded below; final compiler binding remains separate.

| Case | Current original | Fork Legacy | Fork V2 |
|---|---|---|---|
| 01 01_unicode_interpolation | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 02 02_case_variants | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 03 03_literal_replace | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 04 04_regex_replace | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 05 05_regex_matches | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 06 06_split_join | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 07 07_text_counts | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 08 08_number_coercion | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 09 09_compound_arithmetic | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 10 10_expression_precedence | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 11 11_rounding | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 12 12_scientific_math | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 13 13_list_endpoints | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 14 14_list_index_range | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 15 15_numeric_list_statistics | FAIL_OUTPUT | PASS (raw digest) | PASS (raw digest) |
| 16 16_typed_dictionary | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 17 17_nested_dictionary_list | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 18 18_json_dictionary | PASS (bounded summary) | PASS (raw digest) | PASS (raw digest) |
| 19 19_dictionary_update | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 20 20_text_branch | PASS (bounded summary) | PASS (raw digest) | PASS (raw digest) |
| 21 21_numeric_conditions | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 22 22_text_predicates | PASS (bounded summary) | PASS (raw digest) | PASS (bounded summary) |
| 23 23_repeat_sum | PASS (raw digest) | PASS (raw digest) | PASS (bounded summary) |
| 24 24_foreach_text | PASS (raw digest) | PASS (raw digest) | PASS (raw digest) |
| 25 25_nested_repeat | PASS (bounded summary) | PASS (raw digest) | PASS (raw digest) |
| 26 26_nested_foreach | PASS (raw digest) | PASS (raw digest) | PASS (raw digest) |
| 27 27_foreach_filter | PASS (raw digest) | PASS (raw digest) | PASS (raw digest) |
| 28 28_if_value | PASS (raw digest) | PASS (raw digest) | PASS (raw digest) |
| 29 29_repeat_results | PASS (raw digest) | PASS (raw digest) | PASS (raw digest) |
| 30 30_fixed_date | PASS (raw digest) | PASS (raw digest) | PASS (raw digest) |
| 31 31_date_adjust_static_qty | COMPILE_BLOCKED | PASS (raw digest) | PASS (raw digest) |
| 32 32_base64_roundtrip | PASS (raw digest) | PASS (raw digest) | PASS (raw digest) |
| 33 33_url_components | PASS (raw digest) | FAIL_OUTPUT | FAIL_OUTPUT |
| 34 34_url_encode_decode | PASS (raw digest) | PASS (raw digest) | PASS (raw digest) |
| 35 35_object_property | PASS (raw digest + image) | PASS (raw digest + image) | PASS (raw digest + image) |
| 36 36_dictionary_coercion | PASS (raw digest) | PASS (raw digest) | PASS (raw digest) |
| 37 37_immutable_capture | PASS (raw digest) | PASS (raw digest) | PASS (raw digest) |
| 38 38_timer_static_qty | STRUCTURAL_ONLY_NOT_EXECUTED | STRUCTURAL_ONLY_NOT_EXECUTED | STRUCTURAL_ONLY_NOT_EXECUTED |
| 39 39_timer_dynamic_qty | STRUCTURAL_ONLY_NOT_EXECUTED | STRUCTURAL_ONLY_NOT_EXECUTED | STRUCTURAL_ONLY_NOT_EXECUTED |
| 40 40_dynamic_quantity_dates | PASS (raw digest + image) | PASS (raw digest + image) | PASS (raw digest + image) |
| 41 Q41_pending_contract | PENDING_NATIVE_PROOF | PENDING_NATIVE_PROOF | PENDING_NATIVE_PROOF |

## Evidence boundaries

- No native operations, signing, compilation, full Go tests, or plist rewriting are performed.
- Every historical expected value and verdict remains unchanged. Case35 old-oracle failures are retained.
- The current view selects the latest semantic observation under the current expected value; environment/import failures remain in history.
- A summary-only PASS is explicitly bounded by the old manifest audit, exact summary, raw byte length, and correlated callback identity. No raw digest is invented.
- Receiver image proof is joined only to runs carrying its exact receipt hash; older runs are not restamped.
- Compiler lineage uses exact unsigned artifact hashes and actual compiler/source hashes, not lane names or timestamps alone.
- Fork native observations do not certify a later compiler revision. Post-URL-repair action/source bindings and fresh changed-case native proof remain required.

Full ledger SHA256: `dd63cc7d4653a63a3eea7c22266f8d2aa915d6abe62b2f06b82c89a76f9f8c1d`.

## Regenerate from preserved proof

Run the script against a new output directory:

```sh
python3 tests/differential_30/native_ledger.py \
  --config tests/differential_30/native-ledger-config.example.json \
  --out /absolute/path/to/a/new/ledger-directory
```

The example configuration points to immutable evidence from this investigation.
For a later run, append a new receipt snapshot and the corresponding compiler
receipts. Add the Q41 contract to supplemental manifests when it is available.
The draft Q41 row has no invented expected output.

Every snapshot must contain a transfer index with the exact copied file hashes.
The ledger verifies every indexed receipt before reading results. A separate
read-only Mac file audit binds the signed paths to their actual bytes. Early
harness versions reserialized the saved runtime manifest; the ledger locates
the original input file by the hash recorded in the run summary and requires
the input and runtime copy to contain exactly the same JSON values. Both byte
hashes remain in the report.

The bounded legacy callback tier uses the pre-existing single-line manifest
audit, correlated run/fixture/name identity, exact summary text, and recorded
raw UTF-8 byte length. It records a null actual digest when none was captured.
It is never upgraded to the later raw-digest or receiver-image tier.

The full historical ledger and raw receipts remain in the evidence directory
named by native-ledger-draft.json. The committed JSON is a compact index, not a
replacement for those receipts. Candidate statistics compiler lineage retains
its base revision, dirty production-file hashes, binary hash, and actual argv;
the candidate receipt has no explicit exit field, which remains null.

Fork revision binding is deliberately pending in this pre-URL-repair draft.
A matching historical program graph is an artifact-binding fact, not a new
native execution. After the repair, changed graphs need their own native proof.
