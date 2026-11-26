# Cherri Apple distribution

The iOS/iPadOS application uses the existing Apple Developer team `NA6HPWARQ2`.
The Release application identifier is `com.davidpovarsky.cherri`. The original
`org.cherrilang.Cherri` identifier is unavailable for registration in this team;
the fresh Apple response on 26 November 2026 was HTTP 409. Debug continues to
use the original identifier. Document UTIs, source extensions and all test targets
remain unchanged.

## Existing signing resources

Use the existing Scaleway Mac distribution identity and App Store Connect API
key through their configured keychain and environment. Do not create a new key,
certificate, keychain, or general signing installation for this application.
Resolve an active iOS App Store profile for the Release identifier and the
existing distribution certificate. Verify its team, application identifier,
certificate, expiration and distribution entitlements before installing it in
the existing profile location. Reuse a matching profile when one exists.

## Archive and export

Generate the Xcode project from `project.yml` and build the existing `Cherri`
scheme with the Release configuration for a generic iOS device. Supply the
verified profile UUID as the Xcode setting `CHERRI_APP_STORE_PROFILE_UUID`.
The profile setting and distribution identity are scoped to the iPhoneOS SDK;
the Debug simulator and test configuration retain their existing behavior.

Use a private, generated ExportOptions plist with method `app-store-connect`,
manual signing, team `NA6HPWARQ2`, and an exact provisioning profile mapping for
`com.davidpovarsky.cherri`. Read the exported archive and IPA back to verify the
actual bundle identifier, team, version/build number, executable signature and
embedded profile. Keep the archive, dSYMs, IPA and original export receipts.

Before uploading, ensure the App Store Connect application record exists for
this exact identifier and choose an unused build number. Use the configured
App Store Connect credential and existing upload tooling. A successful archive
or upload alone is not evidence that TestFlight processing or installation
succeeded; verify the corresponding App Store Connect build and actual install.

## Verification scope

This configuration is source preparation. Registration, provisioning, archive,
export, upload, processing and device results must each have original evidence
from the actual selected source and artifact. No historical or simulator result
substitutes for those release steps.
