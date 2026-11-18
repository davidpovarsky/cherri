# Native nil binding and clear: exact-source runtime evidence

Run https://github.com/davidpovarsky/cherri/actions/runs/44401936674 completed on a04c6feede79358ee642a1ae27f0a0da8f995ad7 with one fresh correlated compiled SUCCESS callback:

    CHERRI_QA_NIL_CLEAR:binding=0:assignment=0:control=0:present=1

The strict immutable-source oracle verified both run and required trace gates. The 28-action synthetic workflow produced two inline and two collector records. All 20 published payload hashes were independently rechecked; all eight causal local oracle mutations were rejected. GitHub's artifact digest matches the downloaded ZIP. HubSign published AEA1_HEADER_PASS signing metadata; this receipt does not republish signed bytes.

Fixture SHA256: 5acf0db40267630c067d21a61c297b653e5c58e31387fdc96ad60f721263f8d5.
Unsigned plist SHA256: b3ce8686b45002fc26d54b579adb49aadfad2ee2edda59693378932e02f43cbb.
Signed artifact SHA256 from provenance: 67ae90a548b32f151e87c7e854718a83ce4cdf73e232870a0235f4f62deffc36.
Original audit SHA256: 032666b66e1f754519580c43502a64f5718e2ae3b92cd8738dc90ef003ff3590.

This verifies these existing nil binding/assignment/control clearing paths and the present-value control on this exact Simulator run. It does not prove every Optional/null/container or explicit yield-none path, the later integrated compiler, physical-device behavior, or complete Inspector identity mapping. Inspector remains NEEDS_REVIEW. Raw/private/raster artifacts are excluded; archive-manifest.json binds original and normalized text bytes separately.
