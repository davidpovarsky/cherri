# Shortcut Action Development Playbook

This is the durable bootstrap document for every future session that analyzes, extracts, verifies, implements, tests, or documents Apple Shortcuts actions in this project.

It is intentionally written so a new ChatGPT/Codex/OpenCode session can start with little or no chat history.

## Session bootstrap

Before doing any work on Shortcut actions:

1. Read the repository root `AGENTS.md` and obey it.
2. Read `CONTRIBUTING.md` if required by `AGENTS.md`.
3. Read `docs/corpus-platform.md` for the corpus architecture and batch workflow.
4. Read this document completely.
5. Read `docs/agent-technical-reference.md` only when the task requires parser/compiler/action-definition/plist/decompiler internals.
6. Inspect the current branch, PR, and repository state before editing anything.

Do not rely on old chat history when repository state can answer the question.

## Current project state

At the time this playbook was created:

- Repository: `davidpovarsky/cherri`
- Long-lived experimental base: `agent/ios-app-foundation`
- Shortcut corpus/action-development branch: `agent/shortcut-corpus-platform`
- PR #2: `agent/shortcut-corpus-platform` -> `agent/ios-app-foundation`
- PR #2 is intentionally Draft and must not be merged unless the user explicitly asks.
- The corpus platform implementation was consolidated into one coherent implementation commit before this documentation commit.
- `main` is not the target of current experimental work.

Always verify these facts again from GitHub at the beginning of a future session; branches and SHAs may have advanced.

## Project architecture and source of truth

The desired flow is:

```text
real Apple Shortcut evidence
        -> corpus analyzer
        -> normalized structural evidence
        -> review / verification
        -> Cherri action definitions + shared metadata
        -> compiler + decompiler + action catalog
        -> generated docs + Open Minis Skill + iOS Action Palette + preview metadata
```

Cherri core/shared metadata is the source of truth.

Never create a parallel hand-maintained action database in Swift, JavaScript, Markdown, the Skill, or corpus tooling when the information can live in Cherri or be generated from it.

The shared machine-readable action catalog is produced by `buildActionCatalog()` and exposed through:

```sh
cherri --actions-json
```

Consumers should read this catalog rather than parse Cherri source files independently.

## Forks relevant to action work

Current architecture:

- Cherri active fork: `davidpovarsky/cherri`
  - upstream: `electrikmilk/cherri`
- Documentation active fork: `davidpovarsky/cherrilang.org`
  - upstream: `electrikmilk/cherrilang.org`
- Preview active fork: `davidpovarsky/preview-shortcut`
  - upstream: `electrikmilk/preview-shortcut`
- CodeEditorView shadow fork: `davidpovarsky/CodeEditorView`
  - upstream: `mchakravarty/CodeEditorView`
  - do not consume the shadow fork unless we actually need a custom editor change.

The iOS WebPreview dependency must remain pinned to an exact preview-shortcut commit/tag, never a floating development branch.

## First rule for real Shortcut files

User-provided Shortcut JSON/plist files are evidence, not trusted source code.

Raw files may contain private data such as:

- API keys and tokens
- authentication values
- URLs
- email addresses
- phone numbers
- names and personal text
- file paths
- clipboard/content payloads

Raw corpus data must not be committed.

Keep raw inputs under an ignored directory such as:

```text
corpus-inbox/<batch-name>/
```

The analyzer should sanitize persisted normalized evidence.

Only open raw files for specific unresolved records after the analyzer has already deduplicated and classified the batch.

## Standard workflow for every new Shortcut batch

### 1. Verify repository state

Confirm:

```sh
git status --short
git branch --show-current
git log -1 --oneline
gh pr view 2
```

For the current project phase, work on `agent/shortcut-corpus-platform` unless the user explicitly requests a different branch.

Do not merge to `agent/ios-app-foundation` or `main` unless explicitly instructed.

### 2. Put raw files in the ignored inbox

Example:

```text
corpus-inbox/batch-001/
```

Do not rename or manually preprocess hundreds of files unless there is a demonstrated ingestion problem.

### 3. Build Cherri and the analyzer

```sh
go build -o dist/cherri .
go build -o dist/shortcut-corpus ./tools/shortcut-corpus
```

Generate the current action catalog:

```sh
dist/cherri --actions-json > catalog.json
```

### 4. Analyze incrementally

```sh
dist/shortcut-corpus analyze \
  -in corpus-inbox/batch-001 \
  -catalog catalog.json \
  -out analysis/batch-001 \
  -state corpus-state.json
```

Reuse the same state file for later batches so identical files and already-seen action shapes are skipped.

### 5. Review reports in this order

Always inspect compact reports before opening raw files:

1. `summary.md`
2. `needs-review.json`
3. `variants.json`
4. `new-actions.json`
5. `third-party-actions.json`
6. generated candidates, where applicable

Do not dump the entire raw corpus, catalog, lockfiles, or generated reports into model context.

Use targeted filters/searches and inspect only records that need reasoning.

## Classification semantics

Treat analyzer classifications as triage, not final truth.

### KNOWN

The observed action shape is already represented by Cherri sufficiently for the analyzed evidence.

Do not rewrite a known action merely because it appeared again.

Use repeated observations to strengthen evidence or reveal variants.

### NEW

The Shortcut identifier/shape is not represented by the current catalog.

Investigate before implementation.

### VARIANT

An existing identifier appears with a materially different parameter or serialization shape.

A variant may mean:

- optional parameter not previously observed
- OS-version variation
- action mode variation
- richer serialization form
- existing Cherri definition is incomplete
- analyzer normalization needs refinement

Do not automatically create a second Cherri action solely because a variant exists.

### THIRD_PARTY

The action belongs to another application/App Intent surface.

Record the evidence and determine whether Cherri should support it generically or via app-specific metadata.

Do not pretend third-party availability is universal.

### NEEDS_REVIEW / UNKNOWN

Open only the minimal raw records required to resolve ambiguity.

### SAFE_CANDIDATE

Generated candidate metadata is a starting point only.

It is not production-ready until verified against real Apple Shortcut output and Cherri behavior.

### CUSTOM_IMPLEMENTATION_REQUIRED

The action likely requires custom plist construction, parameter transformation, control-flow behavior, variable handling, or decompiler logic.

Read `docs/agent-technical-reference.md` before implementing it.

## Evidence levels

Keep these concepts separate:

```text
observed -> inferred -> confirmed
```

A single observed sample does not prove:

- that a parameter is required
- that a missing parameter is optional
- its default value
- a complete enum domain
- supported OS versions
- exclusive input/output types
- every valid serialization form

Prefer multiple independent samples when possible.

For exact plist/action behavior, use this evidence hierarchy:

1. canonical output produced by Apple's Shortcuts app
2. multiple consistent real-world Shortcut samples
3. existing verified Cherri code/tests
4. reliable/official documentation where available
5. inference, explicitly marked and tested

Never invent plist keys from memory.

## How to investigate one NEW or VARIANT action

For each unresolved action, build a compact evidence record containing only what is needed:

- `WFWorkflowActionIdentifier`
- parameter keys
- value/serialization types
- nested structural shape
- meaningful system constants/enums
- variable/output UUID relationships
- control-flow/grouping fields where relevant
- App Intent / bundle/application identifiers where relevant
- number of independent observations

Then compare with:

1. current `cherri --actions-json` catalog entry, if any
2. existing action definitions under `actions/`
3. custom implementations in compiler internals if applicable
4. decompiler behavior
5. existing tests/fixtures
6. upstream Cherri changes that may already solve the problem

Search first; do not read the entire repository.

## Parameter verification checklist

For every parameter considered for a Cherri definition, determine only what evidence supports:

- Cherri-facing parameter name
- actual plist key
- serialization/value type
- whether literal/reference handling differs
- whether it is observed as absent/present
- whether repeated/infinite arguments are supported
- enum name and confirmed values, if any
- default value only if confirmed
- version/platform constraints only if confirmed

Unknown facts should remain unknown rather than guessed.

## Implementing a simple declarative action

Prefer adding/updating the normal Cherri action definition under the existing `actions/` architecture.

Reuse the established DSL/metadata fields.

Do not add custom Go code if the standard definition system can correctly express the action.

After implementation, verify that the shared catalog reflects it automatically.

## Implementing a complex action

Use custom compiler/decompiler logic only when necessary.

Before editing internals:

1. read `docs/agent-technical-reference.md`
2. inspect the closest existing complex action implementation
3. reuse existing helpers for plist/value construction
4. make the smallest isolated extension possible
5. avoid broad refactors of upstream-owned code

Complex behavior can include:

- special parameter serialization
- attachment/tokenized strings
- variable references
- outputs and UUID relationships
- control-flow grouping
- App Intent payloads
- nested dictionaries/arrays with typed serialization

Do not create a separate compiler path if Cherri already has an appropriate abstraction.

## App Intent policy (Phase 1)

Cherri's existing App Intent infrastructure is authoritative:

- `actionDefinition.appIntent` plus `appIntentDescriptor()` are the only representation. There is no second App Intent engine, no parallel descriptor table, and no dedicated `appIntent(...)` source syntax; generic source syntax and filter/predicate machinery remain deferred follow-up work.
- The outer `WFWorkflowActionIdentifier` is independent of the descriptor fields (`BundleIdentifier`, `Name`, `AppIntentIdentifier`, `TeamIdentifier`, optional flags). Never derive one from the other; classic-identifier hybrids such as Notes/filter actions carry both.
- `TeamIdentifier` must not be fabricated for third-party intents. Curated Apple actions emit the confirmed placeholder `0000000000` through the centralized legacy policy (`appleAppIntent(...)`); third-party definitions set `teamIdentifier` explicitly or omit it entirely.
- `ActionRequiresAppInstallation` is tri-state: unspecified omits the key; explicit true/false emit the boolean. Unspecified and false are never collapsed.
- Known curated actions always win decompilation and compile through their typed definitions. Unknown or unrepresentable actions fall back to `rawAction(...)`, which is the lossless serialization fallback: outer identifier, complete descriptors, booleans, nested dictionaries, and variable/reference envelopes all survive. Do not invent new syntax to avoid rawAction.
- The machine-readable catalog exposes an `appIntent` facet derived directly from the action definition. Palette, preview, Skill, and generated docs consume that facet; none of them maintain their own App Intent list.

Curated third-party wrappers remain evidence-backed exceptions decided case by case; bulk additions are out of scope until the shared infrastructure stays proven correct.

## Decompiler requirements

Adding compile support is not automatically complete.

For an action that Cherri should round-trip, verify decompiler behavior as well.

Check whether the generic decompiler already understands the action via shared metadata.

Only add custom decompiler handling if generic behavior is insufficient.

Preserve semantic structure rather than incidental UUID values.

## Structural comparison and round-trip

Use the corpus structural comparator rather than byte-for-byte plist equality when UUIDs or volatile metadata naturally differ.

Example:

```sh
sh tools/shortcut-corpus/scripts/roundtrip.sh tests/example.cherri
```

The intended loop is:

```text
verified Shortcut evidence
-> Cherri source/definition
-> compile
-> Shortcut plist
-> decompile
-> compile again
-> structural comparison
```

Compare meaningful semantics including:

- action order
- action identifiers
- parameter keys
- parameter structures/types
- confirmed fixed values
- nested serialization
- variable/output relationships
- control-flow grouping

Signing success is not proof of semantic correctness.

## Tests for an action addition

Use cheap tests first.

Typical order:

1. targeted action/catalog test
2. relevant compiler test
3. decompiler test if affected
4. corpus analyzer/comparator test if new shape logic was needed
5. round-trip fixture/test
6. broader Go tests once at the end
7. WebPreview/Skill/iOS validation only when their affected shared layer changed

Common commands:

```sh
go build ./...
go test -run TestCherriNoSign -v ./...
go test -run TestDecomp -v ./...
go test ./tools/... -v
```

Follow the test-isolation caveats in `docs/agent-technical-reference.md` for Cherri global-state tests.

## Fixtures

Commit only sanitized minimal fixtures.

A fixture should demonstrate the structural behavior being tested without carrying private user content.

Prefer the smallest fixture that proves the action/variant.

Do not commit an entire real user Shortcut merely because it reproduces the action.

## Fork action provenance (required)

Every fork-specific action change must be recorded in:

```text
docs/fork-action-provenance.json
```

This registry is required maintenance, not optional documentation. Whenever a task:

- adds a Cherri action,
- expands an existing action with new parameters or behavior,
- changes serialization emitted for an action,
- changes compiler or decompiler handling of an action, or
- changes fork-specific action metadata,

the corresponding provenance entry MUST be created or updated in the same task. A commit must never introduce fork-specific action behavior whose provenance is missing.

### What belongs in the registry

- `added-by-fork`: actions that do not exist in upstream Cherri.
- `modified-by-fork`: upstream actions whose parameters, serialization, or compile/decompile handling this fork extends or corrects.

Record the change, reason, evidence source (corpus batch / observed counts), affected files, and the first fork commit once known. Do not duplicate full parameter schemas or implementation code — point at the implementation; Cherri definitions remain the only semantics source.

### Non-circular firstForkCommit workflow (required)

A commit cannot contain its own final SHA: editing the registry inside the implementation commit would change that commit's hash. Therefore:

```text
1. implement + test the change
2. commit the implementation          -> stable implementation SHA
3. record firstForkCommit values in a SEPARATE provenance-metadata commit
   (batch several entries into one metadata commit when practical)
4. validate referenced SHAs and push both commits
```

`TestForkActionProvenanceRegistry` enforces this: every finalized entry must carry a full 40-hex `firstForkCommit`, and when a `.git` directory is present the test verifies the commit exists and is an ancestor of HEAD. Never amend an implementation commit after a registry entry references it; if a rewrite is unavoidable, update all referencing entries in the next provenance-metadata commit.

### What does NOT belong

- Entries for untouched inherited actions: **no entry means "inherited from upstream"**. Keep the registry small.
- Analyzer/tooling or metadata-only work goes under `infrastructure`, never as fake action additions.

### Upstream baseline and recovery

The registry records the upstream baseline SHA (merge-base with `upstream/main` at seeding time). To inspect or restore original upstream behavior:

```sh
git fetch upstream
git show 951c0bb3c34ef4e3d6cb2ce9a1bff35071c9a7a2:<path>   # original implementation
git diff 951c0bb -- <path>                                  # exact fork delta
git log --follow -- <path>                                  # file history
```

Do not keep backup copies of upstream files; git history is the recovery mechanism.

### When upstream catches up

If a later upstream version provides functionality we previously added, update the entry's `supersededByUpstream` field and append to its `history` instead of deleting it, then deliberately decide whether to inherit upstream, keep ours, or reconcile.

## Documentation after an action change

Do not manually maintain duplicate action signatures in Markdown.

Regenerate action documentation from Cherri shared metadata using the existing docs generator:

```sh
sh scripts/generate-action-docs.sh <dir> <categories>
```

The documentation fork is `davidpovarsky/cherrilang.org`.

Keep upstream documentation synchronization possible and isolate fork-specific/generated material where practical.

Document uncertain/observed behavior as such; do not present inference as guaranteed API behavior.

## Open Minis Skill propagation

The Skill under:

```text
skills/cherri-shortcuts/
```

must continue using the real Cherri CLI and machine-readable catalog.

Do not paste a giant hand-maintained action table into `SKILL.md`.

When the core action catalog changes correctly, Skill discovery should normally inherit the new action automatically.

Run the Skill self-test only when the relevant interface/wrapper changed or final validation requires it:

```sh
sh skills/cherri-shortcuts/scripts/self-test.sh
```

## iOS Action Palette/autocomplete propagation

The iOS/iPadOS app is a consumer of shared Cherri metadata.

Do not duplicate action semantics in Swift.

The Go bridge should stay narrow and expose stable catalog data.

A normal action-definition addition should ideally appear in the iOS action catalog without a separate Swift database edit.

If it does not, investigate the shared propagation path before adding a special-case Swift patch.

## Preview behavior

`preview-shortcut` is the renderer foundation.

The preferred rendering strategy is:

```text
existing renderer
-> generic metadata-aware fallback
-> custom renderer only when genuinely necessary
```

Unknown/new actions should at least render a meaningful titled/generic card when shared metadata permits.

Do not create a second Shortcut renderer in Swift.

If preview-shortcut itself must change:

- make the change in `davidpovarsky/preview-shortcut`
- preserve upstream relationship
- test/build the fork
- pin Cherri WebPreview to the exact new commit SHA
- update the lockfile through a clean install

## CI rules for action work

Follow `AGENTS.md`.

Do not use GitHub Actions as an interactive debugger.

Run local/cheap validation before push where possible.

Use the lightweight Shortcut Corpus Analysis workflow for corpus/tooling/catalog work.

Trigger iOS/Xcode validation only when the action change genuinely affects iOS/bridge/preview integration or when final cross-layer verification requires it.

When waiting for GitHub Actions, use one blocking watcher rather than repeated model/tool polling.

On failure, inspect failed logs only and fix the root cause before rerunning.

## Commit and branch discipline

For one coherent action/batch task, prefer one coherent commit after local validation.

Do not mix unrelated cleanup into action work.

Do not rewrite unrelated history.

Keep work on the requested experimental branch.

Do not merge the Draft PR or touch `main` without explicit user instruction.

If a CI-only correction is needed immediately after a task commit and rewriting is safe, amend/squash rather than building a chain of trivial fix commits.

## Definition of Done for one action

An action is not considered complete merely because it compiles.

Check all applicable items:

- evidence captured and classified
- identifier verified
- plist parameter keys verified
- serialization/value types verified
- uncertainty documented rather than guessed
- compiler support implemented
- decompiler behavior verified where applicable
- machine-readable catalog updated automatically
- structural/round-trip test passes where applicable
- sanitized fixture exists when useful
- generated docs updated
- Skill discovery remains correct
- iOS metadata propagation remains correct
- preview fallback/custom rendering remains correct
- relevant tests pass
- no raw private corpus data committed
- no unrelated diff

## Definition of Done for a corpus batch

A batch is complete when:

- all files were hashed/deduplicated
- reports were generated successfully
- KNOWN duplicates were not manually re-reviewed without reason
- NEW/VARIANT/NEEDS_REVIEW records were triaged
- important unresolved ambiguity is explicitly recorded
- verified action additions were implemented and tested
- unsafe/uncertain candidates were not promoted prematurely
- documentation/shared consumers were refreshed as required
- raw inputs remain untracked
- final report lists implemented actions, deferred actions, evidence limits, tests, branch, commit, and CI status

## Recommended prompt for a fresh agent/chat session

Use this short bootstrap rather than pasting the entire project history:

```text
We are continuing Apple Shortcuts action/corpus development in:
https://github.com/davidpovarsky/cherri

Before doing anything, read and obey:
1. AGENTS.md
2. docs/shortcut-action-development-playbook.md
3. docs/corpus-platform.md
4. CONTRIBUTING.md if AGENTS.md requires it
5. docs/agent-technical-reference.md only when compiler/parser/plist/decompiler internals are actually needed

Verify the current branch/PR state from GitHub; do not assume old SHAs are current.
For the current project phase, keep work isolated on agent/shortcut-corpus-platform unless I explicitly tell you otherwise.
Do not merge anything.

I will now provide the next Shortcut JSON/plist batch or a specific action to investigate.
Use the corpus analyzer first, inspect compact reports before raw files, preserve privacy, distinguish observed/inferred/confirmed evidence, and make verified action changes propagate through Cherri's shared metadata rather than parallel databases.
```

That prompt plus the repository documents should be enough to resume safely in a new session without replaying the old conversation.