# Rejected iOS xcresult preparation: bounded observation

The original hosted run 45869676536 on source 5bf7c5e04d45bedbf55f92701a25f25aa03d0ffa stopped before XCTest with only a generic preparation rejection. Its internal cause remains unobserved. The later, separate source-after Git timeout does not establish that earlier cause.

This component adds a fixed marker before the unchanged prepare command and closed rejection-only metadata: stage, exception category, Git operation timing and byte counts, and a source-file ordinal. It emits no raw paths, argv values, streams or exception messages. The observation is bounded to eight command records and 8192 bytes. Original source checks, exit 70, 15-second prepare and 60-second collection envelopes, encrypted collection, test selection and application bytes remain unchanged.

Actual focused controls on Linux: the old collector plus one new context control failed because the observation line was absent; the candidate passed all four new controls, with zero errors/skips. All 22 original method ASTs are unchanged and were not rerun for this component. Source bytes matched before/after and both qualification parents were reaped with owned process groups absent. Synthetic boundary injection does not establish an Apple or Git runtime outcome.

The archive retains 37 exact source, command, stream, event and process originals. Its SHA-256 is b33a8c5bdc18ac988a001a794580f2e2f2f296e9b77b5b4015c12537b5368d59; the adjacent manifest lists every member byte count, mode and digest. Qualification receipt: a27d24dd24571d8bcc87985b378d67320c4cafd56cefac3416903e522b73e87d. Independent source/privacy/default-compatibility review: 93995d772b7724b33ccd370c6122057fc600da3f6cc02f94bc5a13e86872b7ae.

No build, native invocation, workflow dispatch or formal acceptance is claimed by this checkpoint.
