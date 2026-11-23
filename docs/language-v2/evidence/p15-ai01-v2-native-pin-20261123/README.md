# Reviewed R7 native compiler pin

V2 now pins the actual Linux ARM64 compiler SHA256
`42f6e42e7b6a8ea6f523a752fd2f06b96265e733fa00ad808190e062d1266301`, source
`ca316a3dee252456c4bec751594cf249c709b331`. Independent byte hashing and ELF
machine183 checks matched the supplied pair. The original production package
inspection passed for compiler and migrator, with embedded exact R7 VCS
identity, vcs.modified=false, CGO disabled and a clean canonical clone.
The original build result SHA256 is
`d3bae0c50203bf0237289f8d18b242db1997d95feac5ebcbb2f2beee721fdcfc`.

The migrator hash is
`050e33ad7b078118c0c9dea8ca32a0123926d4a975154244311a5a9f4fe64f91`; it is pair
provenance, not the evaluator executable. No binary was rebuilt by this lane.
The earlier rejected build without VCS fields remains preserved by its owner.

This follow-up changes only the v2 compiler hash and current protocol status.
Every bundle function/class and all test files match the parent exactly.
The existing missing-pin negative explicitly patches the pin to None and
actually passed once after configuration; pure profile lookup also matched
the reviewed pair. The earlier unconfigured6a76d4f proof remains unchanged.

Actual twelve-case calibration is NOT_STARTED at this checkpoint and remains
required before a real evaluation seed or provider call. No live quality
measurement or native Shortcuts execution is claimed.

Original ZIP SHA256: `71da031e97e490f23a97ad4f2045042109cdbb970bf46a81c46e0f30187c291e`.
External manifest SHA256: `37b2df49132bd91c5a16f46dec1cd4ed74fb0780a1e03ff4d534fa753726ee56`.
