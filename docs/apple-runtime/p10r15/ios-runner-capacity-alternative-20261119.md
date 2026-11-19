# Standard Intel alternative for the full iOS build

The b296 full iOS run 44608094926 ended without runner acquisition on 2026-11-19 at 21:48:23 UTC. Job 135857083339 remained runner_id=0, with GitHub annotations reporting that the hosted job was not acquired after repeated attempts and macOS arm64 capacity constraints. The source-bound check receipt SHA256 is 0ac684353ade32560893f536788f3b5e8e2fe27185ca78276b43d760512138bf. This is infrastructure evidence, not a failed app test. The run was not cancelled by this continuation agent.

Official image inventories inspected on 2026-11-19:

- https://github.com/actions/runner-images/blob/main/images/macos/macos-15-arm64-Readme.md
- https://github.com/actions/runner-images/blob/main/images/macos/macos-15-Readme.md
- https://docs.github.com/en/actions/reference/runners/github-hosted-runners

Both macOS15 architectures list Xcode16.4 build16F6 as the default, iOS18.5 device and Simulator SDKs, and available iPhone runtimes18.5/18.6/26.0/26.1/26.2. The full iOS workflow uses the default Xcode, generic Simulator/device builds, and an available iPhone for its UI tests; it has no iOS27 runtime-selection gate. The app deployment target is17.0. The existing CherriCore script already builds device arm64 plus both Simulator arm64/x86_64 slices.

The new optional workflow-dispatch choice is limited to standard macos-15 and macos-15-intel runners. The default remains macos-15, including push-triggered builds. Only the full ios-build job uses this choice; the isolated documentation-component job is unchanged. All full-build steps, test invocations, dependency pins, test/artifact collectors, timeouts and concurrency settings are byte-preserved. This does not select a paid larger runner or alter account configuration.

The separate formal PoC remains on xcode-27. Its device selector requires a real available iOS27 runtime. The retained b296 formal job actually used xcode-27-arm64 image20261104.0337, macOS27.0.1, Xcode27.1 build27A9275 and iOS27.2. The Intel macOS15/26 inventories do not establish an equivalent iOS27 route, so no replacement or runtime downgrade is authorized by this change.

Actual full app build and test execution on the selected Intel runner remain required before claiming success. Changing the available runner choice grants no runtime, UI, signing, physical-device or formal acceptance credit. Existing frozen-e61 jobs remain unchanged.
