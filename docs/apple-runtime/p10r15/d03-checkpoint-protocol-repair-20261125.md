# D03 checkpoint protocol repair

The original iOS Build run 45544322132 on source
`1e02755bbfaf470b4dba1802aa410f29ab4d1188` passed all 70 native tests.
Its canonical evidence collector nevertheless rejected D03 with
`INVALID_CHECKPOINT`: the Swift UI test emitted the fixed checkpoint followed
by explanatory text, while the collector deliberately accepts only the exact
token inside the owned D03 test.

The repair changes only that print literal to the exact existing protocol token.
Every existing UI assertion and test method remains unchanged. The collector
still rejects trailing text and tokens outside the owned UI case.

A regression extracts the actual Swift producer literal and passes it through
the original collector. The pre-repair source produced two real failing
assertions; after repair all four contract cases passed, together with the
existing documentation evidence controls. This is a producer/collector contract
test, not a new native UI observation.

Original run artifacts, rejected D03 state, source diff and red/green process
streams remain under
`/mnt/development/projects/evidence/cherri-p10r15-apple-completion-20261125/d03-checkpoint-contract-01`.
No D03 acceptance is recovered from the already-sanitized historical log.
A new source-bound native run and reviewed R23 snapshot remain required.
