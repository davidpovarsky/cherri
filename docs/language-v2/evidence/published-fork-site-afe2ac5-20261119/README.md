# Published fork documentation at afe2ac5

The fork site is live at https://davidpovarsky.github.io/cherrilang.org/ .
Docs source is afe2ac5ae9654b3ac27a717d550e74f59950d72f; its compiler pin is
7b0100a2a9ce5991ca8ffe82c805278e5aa7754e.

The feature CI run 44456938049 and actual Pages deployment 44457296257 both
completed successfully. Real compiler checks covered 39 examples (36 runnable,
3 required diagnostics), plus 12 real cache/provenance controls. The site gate
verified 520 reference signatures, 87 local links, 50 search records across the
five current routes, and six mutations of real built artifacts. The site was
restored exactly after every control.

Seven public HTTP requests at 2026-11-19T04:22:48Z returned 200. The live action
page and search-index bytes match the actual deployment-build SHA256 receipts.
The homepage links to the current fork guide. Default search/navigation excludes
historical routes, and edit/source links point to the fork. Historical pages
remain available with an explicit current-guide notice. The fork has its own
GitHub project URL; the copied upstream CNAME was removed.

The archive preserves the earlier D2 build-time failure and the later CI
recovery as distinct observations. It also retains this candidate's
initial verifier failures: the search index has separate relUrl and URL fields,
and the inherited preview.js path needed the project base URL. These failures
were resolved before publication; no failed candidate was published.

Public HTTP payloads, CI logs/receipts, deployment metadata and unsuccessful
local verifier attempts are retained in the ZIP with exact original/archive
hashes. Only user-home prefixes in local verifier tracebacks are sanitized.

This proves website publication and default website search behavior. It does
not establish the installed Skill/app integration portion of D03, Apple runtime
behavior, final compiler acceptance, or Remote Builder activation.
