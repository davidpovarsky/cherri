# Native collect counts: exact-source runtime evidence

Run https://github.com/davidpovarsky/cherri/actions/runs/44401917924 completed successfully on source a04c6feede79358ee642a1ae27f0a0da8f995ad7. The independently replayed source oracle verified one fresh correlated compiled callback, SUCCESS, with the exact value:

    CHERRI_QA_COLLECT_COUNTS:empty=0:single=1:one:many=100:0:99

Run and required NDJSON trace gates passed. The trace has two inline and two collector records (collector_begin, before, after, collector_end). Twenty published payload hashes were independently rechecked; all eight causal oracle corruptions were rejected. The 136-action synthetic workflow was signed using HUBSIGN_REMOTE_SERVICE, with published AEA1_HEADER_PASS metadata. The signed bytes themselves are not copied into this receipt.

Source SHA256: 86e612043d5a914af787733ffc3209d3e51f9352e1d5a5fb423856c7ee996ee8.
Unsigned plist SHA256: 8ab14ddda68bb8ef9df9c1538a14321b4cc5ec2aba3ed4dd352b9541c77a12b0.
Signed artifact SHA256 from the published provenance: ea3fb7d626b5ac091e9d391a23c680f68b3673b10fa527966b3946a3233a0c74.

The downloaded ZIP's SHA256 matches GitHub's artifact digest. archive-manifest.json separately binds the original byte hashes and normalized, semantically identical text derivatives committed here. All original bounded artifacts remain intact on the evidence host. Large editor trees, raster material, raw logs, private inputs and executable artifacts are not republished.

This proves scalar-yield collect cardinality for empty/one/100 inputs and the recorded endpoint order on this exact Simulator run. It does not prove list-yield nesting, every intermediate element, the later integrated compiler, physical-device execution or complete Inspector identity mapping. The Inspector status remains NEEDS_REVIEW. The original empty-split, numeric-zero and other failed native runs remain unchanged and are not superseded by this distinct fixture.
