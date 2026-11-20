# R4 evidence-policy fixture repair

## Scope

Frozen R3 K 8e66254d56ea3e4ac70f6cda6ef9d31217e0937c failed RP02 and BRG04 in actual full179 run 44651827685. Both selected the stale synthetic FN01 fixture. The strict validator correctly rejected the retired generic runFunctions producer. No production validator or immutable acceptance contract changed.

The test-only repair binds the synthetic positive model to the registered FN01 producer, command, complete required-test list and scope. The negative model requires exactly the missing-iOS validation detail. Comments explicitly distinguish policy-fixture data from actual FN01/Apple execution evidence.

## Actual clean replay

- Original clean K baseline: selected test failed, exit 1, raw SHA256 6264f6a03ab9cb9dd9365a3b3c848c47ff7977f6f7732c754ba90826d3bd367f.
- First clean repair 33da522799fe5a18f5bc2eb1aca6a48627ab8afa: 34 controls passed; the new exact-error assertion omitted the aggregate error prefix and failed. Original bytes remain retained.
- Final clean source 78d293c9c4d8965d445757ad9e30f23b9777f0c5 compares the structured validation detail: 78 named tests / 42 parents passed, zero fail/skip, 31.031 seconds. Includes every registered RP02 and BRG04 producer test plus all meta and anti-false-positive controls.
- Final raw SHA256 e8e3cff2d433dd355b3221749bff0facd1969bdeab17f3aefaf4cdfffebf92b0. Source was clean before and after execution.
- Independent peer review found no weakened production oracle, tier rule or contract.

## Boundaries

This component does not retroactively alter R3's 95 PASS / 2 FAIL / 82 pending-or-unexecuted results. Integration and a fresh full179 run remain required. No FN01, iOS, external consumer or final closure is granted by synthetic fixture controls.

All six process files under evidence/p15-policy-fixture-r4-20261120 are copied byte-for-byte from the actual runs, including the first repaired failure.

The three original process receipt JSON files retain their Windows CRLF bytes. Only those exact files are excluded from whitespace lint after staged-blob byte verification; source and authored prose pass normal whitespace checks.
