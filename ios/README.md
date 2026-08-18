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

The repository's **iOS Build** GitHub Actions workflow is the reference build environment. Development branches under `agent/**` and `main` are built once per push, with stale iOS runs cancelled automatically. The workflow performs:

1. Existing Cherri Go tests.
2. `preview-shortcut` web bundle build.
3. Cherri Go C archive builds for iPhone arm64 and iOS Simulator arm64 + x86_64.
4. Universal Simulator archive and XCFramework creation.
5. XcodeGen project generation.
6. Swift Package resolution.
7. Unsigned iOS Simulator app build with `xcodebuild`.

This validates the source and integration without requiring a local Mac or Apple signing identity.

## Local Mac build

When a Mac is available:

```sh
brew install xcodegen
bash ios/scripts/bootstrap.sh
open ios/Cherri.xcodeproj
```

The generated project is disposable; regenerate it after changing `project.yml`.

## Preview updates

The app pins `preview-shortcut` in `WebPreview/package.json`. To take an upstream renderer update, change the package version and let CI rebuild the local bundle. Do not copy action rendering code into Swift.

## Editor updates

`CodeEditorView` is pinned by revision in `project.yml` for reproducible builds. The Cherri language configuration lives in `CherriApp/Editor/CherriLanguage.swift` and should continue to track Cherri's own tokens and the existing VS Code grammar rather than becoming an independent language definition.

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
