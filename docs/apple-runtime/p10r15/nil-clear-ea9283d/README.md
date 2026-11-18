# Explicit nil diagnostic clean local receipt

Source: ea9283d0f81b338a19699bf07ba169e7875442e0.

A clean normal Git clone built the actual compiler with -buildvcs=true -trimpath; embedded vcs.revision matches this source and vcs.modified=false. Both the clone and owned source were clean before and after validation. The binary SHA-256 is 380e3d855187828d31ab22b8258870ff636503fcd2cd93b6e7b97b4ade96d8fc.

Two production graph tests passed, including eleven rejected causal corruptions. The strict selector/callback family passed all 33 selected Python tests. Actual phase zero compiled 57 reviewed fixtures, including the new nil diagnostic, and rejected all three compilation-profile/body corruptions and five publication/source-binding corruptions. These totals include existing controls, not 57 newly added scenarios. Shell syntax also passed.

The new fixture source SHA-256 is 5acf0db40267630c067d21a61c297b653e5c58e31387fdc96ad60f721263f8d5; its exact unsigned 28-action plist in this clean invocation hashes to 704d65a6db22de4b2b2e7e066de3f7240f44b552a08a7374471c6111eddb2fa5. The before/after trace and four Count values remain required. Source and compiler identities must be refreshed for the final signed candidate.

The compiler nil implementation is unchanged. Its declaration and assignment currently omit WFInput without an immediately preceding Nothing; the independent control explicitly uses Nothing then Set Variable. The expected native result remains binding=0:assignment=0:control=0:present=1. This local receipt does not claim that omitted input clears native output. No simulator, signing or Apple runtime was invoked, and a wrong native count must remain FAIL_RUNTIME.

An independent read-only review strengthened the oracle to require the reassigned variable's earlier nonempty initialization. Removing or retargeting that initialization now fails a causal control. Repeated generic native display names such as Text are permitted; only the six explicitly consumed diagnostic result names must be unique. Earlier development logs are retained outside tracked publication.

The JSON receipt binds every published log by SHA-256 and byte count. Host paths in commands and the VCS log are replaced by explicit placeholders; the go version output's trailing whitespace is removed. Executable and source hashes are unchanged.
