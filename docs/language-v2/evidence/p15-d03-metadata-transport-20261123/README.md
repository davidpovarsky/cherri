# D03 public metadata transport — 2026-11-23

## Observed failure and limit

[Original D03 run 45093532445](https://github.com/davidpovarsky/cherri/actions/runs/45093532445) failed in the publication-preparation step at 01:15:40 UTC on frozen K8 2c54ebd50fa30597cbdd6337e6ecc6b65274a903. The original failed-step log is retained on Oracle with SHA-256 0a114a07a4f6ea51e0e1bd38354bc8b2d27b82cd94be40d3f4db9546dce5c9eb.

Its only diagnostic was status D03_PREPARATION_BLOCKED, code HTTPError, formalAcceptanceGranted=false. The old handler discarded the HTTP status and endpoint. The original response cannot be reconstructed from that log, and no claim that a rate limit caused that historical failure is made.

A single bounded read-only reproduction from Oracle used the original anonymous request headers against the exact Pages run, attempt-1 jobs and artifacts endpoints. All three returned HTTP 200, with remaining anonymous quota 59, 58 and 57. This is evidence about that later Oracle observation, not the previous GitHub runner response. No unchanged D03 workflow rerun was used.

## Bounded repair

The existing workflow now supplies its existing read-only github.token only to the public metadata preparation step, through CHERRI_D03_GITHUB_TOKEN. Its contents/actions read permissions and timeout remain unchanged. No new secret or repository permission is introduced.

The metadata reader accepts only the fixed run, attempt-jobs and artifacts paths under https://api.github.com/repos/davidpovarsky/cherrilang.org/. It rejects redirects before urllib can send a second request or forward Authorization. The exact response URL, HTTP 200 and one-MiB response-size checks remain in force.

HTTP errors now report a stable D03_METADATA_HTTP_<status> code, the allowlisted relative public endpoint, the HTTP status, whether authentication was present, and only bounded numeric rate-limit/retry values. Response bodies, tokens, arbitrary headers and redirect targets are omitted. The failure remains fatal, and formalAcceptanceGranted remains false. There is no automatic retry.

The timestamp/archive/source/deployment validation functions and the complete prepare() body are byte-identical to the pre-repair source. This preserves exact checkout and pin verification, original ZIP size/digest/member checks, run/repository/source/attempt binding, successful build/deploy jobs, artifact lifetime and reread race checks, quarantine boundaries and verified-output publication.

GitHub documents authorization headers and use of the built-in workflow token in [REST authentication](https://docs.github.com/en/rest/authentication/authenticating-to-the-rest-api), and distinguishes 403/429 rate-limit failures using response headers in [REST rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api). Those documents do not identify the cause of the original unrecorded response.

## Focused causal controls

The existing transport suite passed all 15 tests: the seven original source/ZIP/deployment tests and eight added transport tests. The original suite body is unchanged. The original log reports 1.128 seconds; the enclosing process took 1.684 seconds.

The new controls exercise the real urllib opener/error/redirect processing with synthetic handlers that perform no external network request. They demonstrate:

- A synthetic anonymous 403 remains a fatal, diagnosable failure; supplying the scoped token yields the unchanged public metadata bytes.
- Invalid endpoint shapes are rejected before opening a connection.
- Each 301, 302, 303, 307 and 308 redirect is rejected before a second request, including cross-host, HTTPS-to-HTTP and same-host destinations.
- HTTP 401, 403, 404, 429 and 503 retain only the bounded status/endpoint/numeric retry fields.
- Arbitrary retry-header text, response bodies, tokens and supplied archive contents stay out of diagnostics.
- Exact response identity and the existing byte limit still reject mismatches.
- The CLI prints the safe failure record and grants no acceptance.
- Whitespace-bearing tokens fail before any request.

These are transport and regression controls, not an actual installed-consumer pass. The next source-bound D03 run remains coordinator-owned. Original CI failure and reproduction records are preserved; no compiler, documentation pin, deployment or acceptance requirement changed.

## Evidence

The receipt binds the exact tested source bytes, original stdout/stderr and original-failure hash. The companion reproduction record preserves public endpoint/status/header observations. The complete original response bodies remain in the retained Oracle evidence directory.

The repair was executed by the actual Apple/native subagent in its isolated proof worktree. It is separate from the frozen K8 workspace and from the continuing iOS run 45093282117.
