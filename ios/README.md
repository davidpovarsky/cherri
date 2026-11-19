# Cherri for iOS / iPadOS

This directory contains a native iPhone and iPad editor for Cherri while deliberately keeping the existing Cherri compiler and ecosystem as the source of truth.

## Design rule: reuse first

The iOS app follows this order when adding functionality:

1. Reuse existing Cherri code or a project already used by Cherri.
2. Reuse a maintained third-party library.
3. Add app-specific code only where the mobile platform requires an adapter.

The goal is to keep normal upstream merges from `electrikmilk/cherri` small and understandable.

## Reused components

- **Compiler and decompiler:** the Go implementation in the repository root remains authoritative.
- **Editor:** [`mchakravarty/CodeEditorView`](https://github.com/mchakravarty/CodeEditorView), the same editor dependency used by `electrikmilk/cherri-macos-app`.
- **Syntax vocabulary:** adapted from Cherri's token definitions and `electrikmilk/cherri-vscode` into `LanguageSupport.LanguageConfiguration`.
- **Shortcut preview:** [`electrikmilk/preview-shortcut`](https://github.com/electrikmilk/preview-shortcut), the renderer already used by cherrilang.org and the Cherri Playground. It is bundled locally in the app; the app does not depend on the website being online.
- **Signing:** Cherri's existing HubSign client is reused. Signing is an explicit network operation; normal editing, compiling and previewing stay on-device.
- **Project generation:** XcodeGen keeps the Xcode project generated from the text file `project.yml` instead of committing a frequently-conflicting `.xcodeproj`.

## Architecture

```text
.cherri document
      |
      v
CodeEditorView (SwiftUI)
      |
      v
Cherri Go compiler (same package main)
      |
      |  c-archive + tiny C ABI adapter
      v
CherriCore.xcframework
      |
      +----------> unsigned plist ----------> preview-shortcut in local WKWebView
      |
      +----------> existing HubSign code ---> signed .shortcut ---> ShareLink / Files
```

The Go compiler is intentionally **not** moved into a new package. `ios_bridge.go` is compiled only for `ios && cgo` and exports a narrow C ABI. This avoids a large compiler refactor that would make upstream merges harder.

Because the current compiler uses package-level mutable state, the mobile bridge serializes compiler calls with a mutex. A future upstream compiler-session refactor can remove that limitation without changing the Swift-facing API.

## Files

- `project.yml` — XcodeGen project definition.
- `scripts/build_cherri_core.sh` — builds device and Simulator Go C archives and packages them as `CherriCore.xcframework`.
- `scripts/bootstrap.sh` — builds the web preview, Go core and generated Xcode project.
- `WebPreview/` — very small Vite host around the upstream `preview-shortcut` npm package.
- `CherriApp/` — SwiftUI document app, editor, compiler bridge and preview host.
- `Vendor/CherriCore.xcframework` — generated; do not edit by hand.

## Building without a Mac

The repository's **iOS Build** GitHub Actions workflow is the reference build environment. It runs on `main` pushes and explicit workflow dispatches; a development branch must be selected explicitly for dispatch. Stale iOS runs for the same ref are cancelled automatically. The workflow performs:

1. Existing Cherri Go tests.
2. `preview-shortcut` web bundle build.
3. Cherri Go C archive builds for iPhone arm64 and iOS Simulator arm64 + x86_64.
4. Universal Simulator archive and XCFramework creation.
5. XcodeGen project generation.
6. Swift Package resolution and exact-revision editor patch verification.
7. Unsigned iOS Simulator app build with `xcodebuild`.

This validates the source and integration without requiring a local Mac or Apple signing identity.

## Bundled documentation

The workspace's Documentation screen searches the active fork-language guide
and the generated action reference offline. The migration guide is available
through an explicit toggle and labels Legacy code as migration reference.
Search and reading use the bundled index; explicit links to the pinned source
on GitHub require a network connection.

Bootstrap and the iOS workflow run `scripts/build_documentation_index.py`.
The generator uses the exact docs commit from the shared
`skills/cherri-shortcuts/compatibility-manifest.json`, checks the repository,
clean working tree, original Git bytes and schema/macro fingerprints, and
bundles only the four current fork-language documents. Action signatures come
from the existing generated reference; Swift contains no action catalog copy.
The generated JSON is ignored by Git. Each native build checks its exact digest
against the generation receipt and its version fields against the current shared
manifest. A missing, damaged or stale index stops the build; rerun bootstrap to
regenerate it from the pinned source.

The workflow's optional `documentation_component_only` input runs a separate
SwiftUI type-check and generator check before app integration. It deliberately
does not represent a full iOS app build or Simulator test run.

## Local Mac build

When a Mac is available:

```sh
brew install xcodegen
bash ios/scripts/bootstrap.sh
cd ios
python3 scripts/prepare_editor_dependency.py --check-only
xcodebuild -project Cherri.xcodeproj -scheme Cherri \
  -clonedSourcePackagesDirPath .build/SourcePackages \
  -disableAutomaticPackageResolution \
  -configuration Debug -sdk iphonesimulator \
  -destination 'generic/platform=iOS Simulator' \
  -derivedDataPath .build/simulator CODE_SIGNING_ALLOWED=NO build
open Cherri.xcodeproj
```

The generated project is disposable; regenerate it after changing `project.yml`.
Bootstrap resolves the pinned packages into `ios/.build/SourcePackages` and applies
Cherri's verified editor input patch there. Keep the two package flags above on
subsequent build and test commands. CI uses this same prepared checkout and checks
its exact source hashes before every native build/test.

You may open the generated project to edit and inspect sources after bootstrap.
A direct Xcode GUI build can select a different SwiftPM checkout; automatic patch
application in that alternate cache is not covered. Use the prepared command
above for the reproducible app, and rerun bootstrap after resetting package
caches. Do not treat an unprepared GUI build as verification of this editor fix.

## Preview updates

The app pins `preview-shortcut` in `WebPreview/package.json`. To take an upstream renderer update, change the package version and let CI rebuild the local bundle. Do not copy action rendering code into Swift.

## Editor updates

`CodeEditorView` is pinned by revision in `project.yml` for reproducible builds. The Cherri language configuration lives in `CherriApp/Editor/CherriLanguage.swift` and should continue to track Cherri's own tokens and the existing VS Code grammar rather than becoming an independent language definition.

The pin remains `5386056ab53d43363083cb96069715a9608aa048`. The bounded patch in
`DependencyPatches/CodeEditorView/generated-closers.json` fixes UIKit typing of
known auto-generated delimiter suffixes without replacing the editor. The
installer verifies the Git revision, original Git blobs and original/patched
SHA-256 values, and refuses unexpected files. It is idempotent. See
[`DependencyPatches/CodeEditorView/README.md`](DependencyPatches/CodeEditorView/README.md)
for scope and native validation requirements.

CodeEditorView currently provides its code-completion service on macOS, not iOS. iOS completion should therefore be implemented as a thin Cherri-specific UI backed by the compiler's existing action definitions instead of replacing the editor.

## Signing and privacy

Live preview does **not** sign and does not need a network request. The `Sign` action invokes Cherri's existing remote signing path. Shortcut source/plist data may contain personal information, so the UI should make the network boundary explicit before sending data and the app must not silently sign during live preview.

## Upstream merge policy

Keep general compiler changes minimal and independently useful. Prefer:

- new iOS-only files guarded by Go build tags;
- app code under `ios/`;
- small hooks in shared compiler files only when unavoidable;
- no duplicated action database;
- no duplicated Shortcut renderer;
- no committed generated Xcode project or XCFramework.

When `electrikmilk/cherri` changes action support, parsing or plist generation, the iOS app should receive those changes through a normal upstream merge rather than a separate port.

Open **Options → Documentation** in the editor to search the bundled current guide and action reference. The migration reference appears only after enabling **Include migration reference**. Searching and reading documentation preserve the editor source.
