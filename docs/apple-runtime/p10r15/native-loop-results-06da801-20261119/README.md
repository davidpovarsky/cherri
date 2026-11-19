# Native loop output and name-omission repair

Production source commits:

- 651ce489ad4bbd39139008c843cc3c8cb18327fd preserves an explicitly absent native ActionOutput display name across the IR/backend adapter. Ordinary generated backend references retain their historical producer-ID default.
- 06da8017157e5a52290a884e232ec4bcaf01be12 imports native Repeat Results as a statement loop followed by an explicit native output reference. It does not assert one nested List per iteration, add wrapper cardinality, or relax the F04 authoring diagnostic.

## Causal public controls

The new native-loop tests, run against b34ae47 production before the fixes, produced 17 named failure events and one passing authored-rejection control. Eight failure messages identify V2 compile demotion of nested native control regions; missing-name controls also exposed strict wire mismatches. These are synthetic public fixtures.

The direct codec baseline retained the original b34ae47 backend, lowerer and shared-codec bytes. Its absent and explicitly empty native output names incorrectly became the producer UUID; the named token and ordinary generated-reference default controls passed. The loop importer working changes do not participate in these direct codec cases.

On exact clean committed source 06da801, the focused gate discovered and passed 128 named tests/subtests, with zero failures or skips. It includes the new native each/count loop combinations, named/absent output references, List and empty-List boundaries, all P12 control-region cases, F04 rejection and supported-boundary controls, BRG07 reference recovery, and existing repeat controls. The complete public baseline and final logs and SHA256 receipts are retained here.

## Targeted private confirmation

A temporary, uncommitted host-only test selected the two affected original specimens identified by the earlier private diagnostic observer. Both passed exact Legacy/V2 wire identity and pre-overlay semantic parity for every natural action. Their combined natural action count increased from 2,293 to 2,297, recovering all four actions, with zero remaining Stage-3 demotions.

The temporary observer was removed after execution; production source was clean at the same commit before and after. The complete 38-original and 76-total-file identity/content snapshots remained unchanged. Only aggregate receipts and the private raw-log hash are archived here; no private inputs, identities, generated source or raw logs are included.

These focused results do not replace the five required private tests on the final combined release source. The unchanged 23,340 natural-action floor and zero-Stage3 requirement remain in force. Native F04 flattening evidence and the authored nested-List rejection remain unchanged.
