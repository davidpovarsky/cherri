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
