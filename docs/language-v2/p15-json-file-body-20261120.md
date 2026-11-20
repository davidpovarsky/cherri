# Registered File-body JSON transport diagnostic

## Scope

This component adds one fixed public fixture to the existing signed Shortcuts QA route. It tests the registered `fileRequest` API with a Text action as the File request body and an explicit `application/json` header. The body is 222 exact UTF-8 bytes, including a literal JSON null, an independently absent key, false, numeric zero, empty Text, and nested arrays.

The previous Dictionary JSON candidate was observed on Apple with the candidate key absent. That original observation is unchanged. The new component does not implement compiler Optional-to-JSON conversion and cannot prove general Optional, List, function-result, or T05 semantics.

## Source identity

- Implementation: `7cd57eb0f04611dcbe9fc9430f55bd67ece69350`.
- Parent source: `03f17a785f31f7bdc8c7607bef1ac9a16c28d3b4`.
- Fixture: `tools/shortcuts-simulator-qa/fixtures/qa-json-body-file-utf8-v2.cherri`.
- Fixture SHA-256: `8e69b72584bedbfc101c8b2e4cfec92e07afc70a47c7f9405abde9bea6a73247`.
- Expected-contract SHA-256: `ea45b8953cd0cfbbcc10120f677f8b52b0b710ef6b7eea54bebb086af0f2286b`.
- Exact body SHA-256: `310514cc82c5926d5b2f63ff61f23e4d41967f60e83cfd6a60db058656f3bdac`.

The evidence JSON binds all seven changed Git files and retains the original command and artifact hashes. No compiler production file, existing fixture, expected value, receiver, publisher, loopback-server implementation, signing endpoint, or existing timeout was changed.

## Actual local validation

The complete independent native graph requires 17 actions, the exact Text producer and File-body UUID edge, a single loopback POST, the separately observed continuation GET, exact header encoding, the response UUID, both trace events, their stores, and the final output. All 31 specific graph mutations were rejected. Consistent UUID renaming remained valid, while a Boolean substituted for numeric dictionary item metadata was rejected.

On the clean implementation, the original Phase0 Python heredoc ran without modification: 84 compilation attempts produced 82 artifacts and retained the two existing expected rejections. Its actual execution completed successfully in 10.266 seconds. The five Python suites discovered 82 tests: 80 passed and two POSIX-only controls were explicitly skipped on Windows. The clean replay completed in 73.969 seconds. Shell syntax and Git whitespace checks also passed, with a clean worktree before and after.

The local Phase0 replay reused the retained compiler built at `a8b69008d70f48989bf3c3db6258e0e22ee85eda`, after verifying unchanged production Git files through this implementation. Its binary SHA-256 is `70c37704293081896af89304505a28eb009504fe739dc88e96b85cdb1bb0bedd`. A hard link avoided another large binary copy. This binary has the previously recorded linked-worktree limitation of absent embedded VCS metadata; the proof does not claim a fresh embedded identity. The native workflow will build its own exact published source.

The first clean local invocation failed after compilation because the extraction driver omitted the `runtime` directory normally created by the shell preamble. The original failed command, stderr hash and partial artifacts are retained. The successful retry used the original shell's editor/runtime/tests/docs/builds directory preconditions, in a separate directory, without a source or oracle change.

## Observation and acceptance

The existing owned listener, nonce, source/hash, privacy, request-size, continuation, and callback checks remain mandatory. The new `rawUtf8BodyVerified` field is derived from the validated original request bytes and a genuine fixture callback. A semantically equivalent reformatted body, an absent/null/false/zero substitution, missing callback, borrowed source, or forged graph receipt cannot establish exact transport success. Public-artifact validation recomputes this result; raw private receipt files remain quarantined.

Local HTTP clients and constructed callbacks are test controls only. They are not Apple evidence. The local proof reports Apple runtime NOT_RUN and formal acceptance false. A future run may prove this one registered File/Text-body transport only. It cannot be inherited by a later compiler candidate or used as general container/Optional acceptance.

## Planned native route

Use the existing `cherri-ios27-shortcuts-simulator-qa.yml` workflow with `sample_id=qa-json-body-file-utf8`, full probe, iPad default profile, exact runtime required, one editor page, full-editor capture disabled, and sanitized artifacts enabled. The existing 45-minute job, public-fixture signer and all callback assertions remain unchanged. The component is a separate audit branch; no unchanged native retry is authorized by this proof.

See [machine-readable evidence](evidence/p15-json-file-body-20261120.json) for the full source manifest, exact local receipts, negative-control inventory and retained failure.
