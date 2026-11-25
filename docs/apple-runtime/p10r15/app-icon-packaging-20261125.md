# iOS app icon packaging — 2026-11-25

The actual unsigned Release archive of source
`1e02755bbfaf470b4dba1802aa410f29ab4d1188` had no app icon assets or
`CFBundleIcons` entries. The application now includes an opaque RGB
1024 × 1024 AppIcon asset and selects it explicitly in XcodeGen. Xcode generates
the required iPhone and iPad renditions from this single asset. The bundle
identifier, application version, and signing configuration are unchanged.

The icon follows the existing two-cherry/green-leaf brand in
`assets/cherri_icon.png`; that original file remains intact. The built-in
ImageGen tool prepared an opaque white-background version with the prompt:
“Preserve the existing two red cherries, green stems and leaf, their arrangement
and colours; square icon, no text, border, rounded container or transparency.”
The generated source was normalized to the required pixel dimensions with the
Mac's existing `/usr/bin/sips --resampleHeightWidth 1024 1024`. This is a
packaging derivative, not a replacement of the original brand asset.

## Native validation

Actual `xcrun actool` for iphoneos, deployment target 17.0, and both iphone
and ipad completed with exit 0 and no warnings. The generated partial plist
contains `CFBundleIcons` and `CFBundleIcons~ipad`, each selecting AppIcon.
Compiled outputs include Assets.car and the 120 × 120 iPhone and 152 × 152 iPad
PNG renditions.

- Source image SHA-256: 8441c9794af39ecf2a7cdfca38f3fef6cfae2263b110e6315d0eff89897e72c1
- Packaged icon SHA-256: 18c5f8029f6ad7b9f098301561859afac6f1e600f47881b01253ef3c4260ce5d
- Native actool receipt SHA-256: 1f0f8cde43b3892a31a2cb16b44976e2a444496580a8f7c87a45c66627c89b7f

Original commands, logs, receipt and output manifest are retained under
`/mnt/development/projects/evidence/cherri-p10r15-apple-completion-20261125/icon-packaging-artifacts-01`
and the matching Mac `packaging-icon-probe-02` directory. Full app archive
validation is a separate follow-up. This evidence does not claim distribution
signing, TestFlight upload, physical-device validation, or full acceptance.

## Exact-source archive follow-up

Source `b2121e866df812351e9a0c054a1087230360d1a0` was archived on the existing
Scaleway Mac with the existing generated documentation, WebPreview and CABI
dependencies. Actual Release archive exit 0 took 22.955 seconds.
`assetutil --info` verified four icon renditions including the 1024 × 1024
App Store rendition. Both iPhone and iPad icon plist entries were present,
`org.cherrilang.Cherri` was retained, and the checkout remained clean.
The original archive contains 16 files / 109909827 bytes and remains unsigned.

- Archive: `/Users/m1/builds/cherri-app-icon-release01/Cherri.xcarchive`
- Receipt SHA-256: b696f6d0b7c3e2d3517750004082925b23feef1904ff074f22c868445ed9a9bd
- Archive manifest SHA-256: b76ed86522556b0fd3688983e09dd1aa6bc06c8fe2341018eab25e499d849dab

## PoC runtime inventory provenance

The existing PoC printed runtime inventory through a pipeline without retaining
the actual simctl process exit separately. The workflow now invokes
`runtime_poc_preflight.py` once before its unchanged single PoC execution.
The collector reuses the existing bounded process-group capture, retains raw
stdout/stderr and actual argv, return code, timing and cleanup fields, and binds
the clean Git HEAD/tree before and after to the GitHub source/run context.
It records the actual platform; local validation is explicitly labelled local
and never invents a GitHub run. Late, overflowing, incomplete, nonzero,
wrong-platform, source-drift and missing-iOS27 observations cannot be READY.

The inventory is a separate original artifact. It does not run Shortcuts, sign
anything, publish runtime acceptance, or change the existing PoC fixture and
signing provenance checks. Twelve new negative-control tests and the existing
17 privacy / 8 orchestration tests passed. Native inventory validation is kept
as a separate source-bound process receipt.
