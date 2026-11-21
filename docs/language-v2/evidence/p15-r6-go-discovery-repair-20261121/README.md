# Historical Go evidence package discovery repair - 21 November 2026

Actual Linux, macOS and Nix Build & Test run44936881496 and iOS run44936763355 stopped before their planned tests because earlier editor evidence was stored with executable Go file extensions. Some historical directories mixed packages and others contained incomplete packages.

The relocation manifest lists every historical .go snapshot moved to .go.txt. Each Git blob and local byte stream is unchanged; original proof manifests, archives, recorded failures and source identities remain available. Historical manifest paths resolve through this explicit relocation map. No active Go source/test, contract, workflow, package exclusion or timeout was changed.

The failing local go list output and three original CI job logs are retained separately. Successful package discovery/build replay will be recorded against the clean repair commit. This repair grants no semantic, native or formal179 acceptance.

## Clean committed replay

Source 7e27a7f7139106d90e633396b9c7840f63b26ead was clean before and after both actual commands. go list ./... returned all 30 active packages in 12.469 seconds; go build ./... returned exit 0 in 25.938 seconds. Original command outputs and the executing script are preserved in clean-replay.zip (SHA-256 6873eb5aeec99043ca16b74f1adc3761d949b6bea43f1d17c3b5155aaa84d89e), with clean-replay.json binding source, commands and each original stream. This local Windows replay does not grant full Go-suite, Linux/macOS/Nix CI, native runtime, or 179-requirement acceptance. The earlier RED logs remain retained without modification.
