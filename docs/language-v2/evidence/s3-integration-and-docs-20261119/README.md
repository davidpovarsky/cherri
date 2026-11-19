# S3 integrated compiler and D3 documentation evidence

This preserves original process output for immutable compiler
8b3fcf71455ea2c1b4c1cf75f929ccc96f227d46 and documentation
5e08813d1dadc7b3e72c5723f76a918f9f2e8361. It does not describe later source.

The focused combined-source Go gate executed 120 named passing tests, zero
failures and zero skips in 89.453 seconds, with a clean source before/after.
This was not the full recursive Go suite.

The D3 local documentation gate compiled 36 runnable snippets and observed
the three required negative diagnostics, all 39 passing. Twelve real cache
controls also passed. The independent Docs Snippet Smoke Test run
[44459839248](https://github.com/davidpovarsky/cherrilang.org/actions/runs/44459839248)
passed at exactly D3. Its original snippet artifact is included.
The complete generated canonical action reference matched the docs body.
This receipt does not claim D3 is the published website: the separately
verified publication remains afe2ac5ae9654b3ac27a717d550e74f59950d72f.

Original evidence is stored in a ZIP to retain byte identity and keep
historical source snapshots outside recursive package discovery. Every member
is enumerated and SHA-256 bound in verification.json; all ZIP members were
read back and compared. No compiler binaries or private corpus inputs are
included. Later S3 CI exposed an archived Go observer being discovered as
a package; that failure and its causal packaging repair are tracked separately.

S3 is an intermediate diagnostic source. These records do not claim project
closure, current app search, final installed consumer acceptance or Apple
runtime success.
