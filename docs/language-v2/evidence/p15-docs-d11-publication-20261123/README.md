# D11 Pages publication and live bytes — 2026-11-23

## Result

**PASS for the exact Docs publication and the seven inspected live resources.** [Pages run 45091966838](https://github.com/davidpovarsky/cherrilang.org/actions/runs/45091966838), attempt 1, deployed Docs D11 365b08c5b4c71c6d22b591babb8d1ab852e5a3b9 to [the public site](https://davidpovarsky.github.io/cherrilang.org/). The source remains pinned to compiler R7 ca316a3dee252456c4bec751594cf249c709b331. The automatic [Docs smoke run 45091966841](https://github.com/davidpovarsky/cherrilang.org/actions/runs/45091966841) also succeeded on this exact source.

The prior independent intake is commit c82e4a6ba8498f84439652617be11db53ccb8b19. This separate receipt records the later publication. It preserves the earlier originals and does not modify the frozen Cherri candidate 2c54ebd50fa30597cbdd6337e6ecc6b65274a903, Docs D11, its compiler pin, or any implementation or test.

## Deployment binding

| Evidence | Observed value |
| --- | --- |
| Ref update | Ordinary non-force fast-forward of Docs main from 56811d6bdf68238b8a213e38274d8ecad01abf70 to D11 |
| Pages workflow | .github/workflows/pages.yml, ID 376511374, push event, main, attempt 1 |
| Build and deploy jobs | 137423327721 and 137423543766; both successful |
| Deployment | 7987863190, environment github-pages, exact D11 source |
| Success status | 22885740769, recorded 2026-11-23T01:00:33Z, linked to the exact deploy job |
| Deployed artifact | github-pages, ID 14781473315; the original deploy log explicitly selects this artifact and reports success for D11 |
| Original artifact | 15,961,918 bytes, SHA-256 50127e2afec6b5a78a96302f7fca0b4ae83faa62a54ceafcf0cb0f0598b9cdeb |
| Original tar | 18,954,240 bytes, SHA-256 677f0e5f9ee2e2f22ef2a151056a59a9ab1186e2091d09488e71ab9ec95640e6, 98 regular files |

The separate original published-docs-build-evidence artifact is ID 14782045537, 1,535 bytes, SHA-256 a3319aa0b3e5d268e794f8d54d6ff3353b86e7111e60de0c3540c2b1ceadb7fa. Its two original JSON members match the selected build receipts exactly. Both artifact ZIPs match GitHub's recorded size, digest and run/source identity.

## Live HTTPS readback

Seven GET responses captured at 01:16:14–01:16:15 UTC all returned HTTP 200 under normal TLS certificate verification. Each retained response body equals its exact deployed artifact.tar member byte for byte. The original receipt retains response headers, requested/final URLs, request/response times, byte counts and SHA-256 values.

| Public resource | Exact published member | Bytes |
| --- | --- | ---: |
| /cherrilang.org/ | index.html | 12,383 |
| /cherrilang.org/fork-language/ | fork-language/index.html | 14,291 |
| /cherrilang.org/fork-language/actions | fork-language/actions.html | 798,667 |
| /cherrilang.org/fork-language/guide | fork-language/guide.html | 39,100 |
| /cherrilang.org/fork-language/migration | fork-language/migration.html | 20,082 |
| /cherrilang.org/assets/js/search-data.json | assets/js/search-data.json | 218,910 |
| /cherrilang.org/assets/js/just-the-docs.js | assets/js/just-the-docs.js | 17,461 |

The action-page digest is c863ebd0ec9535fd6da10b82d7e3da8c8366f4b07de0ce4855c51e524c5b29fc. The search-index digest is 2387c2c2f8da9996a48c3373e05117b8238bec93bd575c5aec3dcd0f237428a0. This proves exact live bytes for these seven resources; it does not claim all 98 deployed files were fetched live.

## Build receipts and scope

Read-only validation of 13 retained current site members with the immutable D11 site verifier exactly reproduces the original published-build receipt: 87 local links, 505 native actions, 520 reference signatures, 50 search entries, 21 action search entries, zero historical search entries and both schema fingerprints. The six original site controls passed and restored their inputs exactly. This intake did not rerun those controls, tests, compiler, snippets, site build or CI.

The original build receipt's sitePublished=false is unchanged: it records the build-stage scope. The separate deployment and live-response evidence establishes the subsequent publication. No historical receipt was rewritten to imply it already proved publication.

## Selected original proof

- publication-intake.json records the conclusion, limits, exact sources and packet digests.
- publication-proof.zip contains original run/job/artifact/deployment JSON, both original job logs, the normal ref-update receipt, the live HTTPS receipt, the two original build receipts, 13 current site files and three immutable source files.
- publication-manifest.json binds each packet member to its original API record, artifact member, immutable Git blob or live response.
- published-docs-build-evidence.original.zip is the complete original small publication artifact, suitable for the existing consumer workflow input.

The seven identical live bodies are stored once in published-site/ and carry both artifact and HTTP-response origins in the manifest. The full raw live bodies and complete 16 MB original Pages ZIP remain in the retained Oracle evidence location. The full site archive and nested historical proof archives are excluded from this packet.

## Consumer handoff

The frozen K8 workflow is .github/workflows/p10r15-documentation-consumers.yml. Its required inputs are docs_deployment_run_id=45091966838 and publication_evidence_zip_base64, containing Base64 of the exact included 1,535-byte original ZIP. The workflow has a 32 KiB decoded-input ceiling and its own concurrency group with cancel-in-progress: false.

This intake does not dispatch that workflow. It establishes publisher evidence; it does not claim an installed consumer, app integration, Apple runtime or complete formal179 acceptance. Those require their own actual run receipts.
