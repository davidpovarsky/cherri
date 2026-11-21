# Historical Go evidence package discovery repair - 21 November 2026

Actual Linux, macOS and Nix Build & Test run44936881496 and iOS run44936763355 stopped before their planned tests because earlier editor evidence was stored with executable Go file extensions. Some historical directories mixed packages and others contained incomplete packages.

The relocation manifest lists every historical .go snapshot moved to .go.txt. Each Git blob and local byte stream is unchanged; original proof manifests, archives, recorded failures and source identities remain available. Historical manifest paths resolve through this explicit relocation map. No active Go source/test, contract, workflow, package exclusion or timeout was changed.

The failing local go list output and three original CI job logs are retained separately. Successful package discovery/build replay will be recorded against the clean repair commit. This repair grants no semantic, native or formal179 acceptance.
