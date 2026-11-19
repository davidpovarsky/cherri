# Independent-review UUID integrity correction

Fixture/harness source: 7790f352d6b16167a663a405bf9311af3b806fd7.
Actual immutable compiler: S3 8b3fcf71455ea2c1b4c1cf75f929ccc96f227d46,
binary SHA256439eff31c429874506bff1a18b3d19177f08127ece69f296dcb6a3329a3c6689.

Independent review found that the initial graph gate could treat missing
source/reference UUIDs as equal and did not reject duplicate producer IDs.
Three actual emitted-plist mutations reproduced that defect in the 55de410
gate: remove a producer UUID and all references, set both to empty strings,
or collide the boxes result UUID with the scalar result UUID while updating
its references. The original gate incorrectly accepted all three. The
actual baseline report is preserved here.

The correction validates nonempty unique action UUIDs, requires UUIDs on
named producers, and rejects an edge whose source UUID is missing or empty.
GroupingIdentifier remains separate because control groups correctly repeat
it. The paired mutations verify identity integrity, not merely a dangling
edge to an unchanged producer.

Clean corrected production Phase0 passed in 4.265 seconds:66 attempts,
64 artifacts, two precise original F04 rejections, and the same 53-action
explicit Dictionary feasibility probe. All17 damaged actual graphs were
rejected, including the three reproduced UUID bypasses.

The fixture bytes, authored expected callback, original F04 source hashes
and original F04 failures are unchanged. The earlier32 Python callback/source/
privacy methods are archived separately at55de410; this delta verifies
the real Phase0 graph and17 causal mutations. It does not claim another
execution of that32-method suite.

No signing or Apple runtime execution occurred. This remains an explicit
Dictionary-carrier feasibility diagnostic, with formalF04Acceptance=false.
