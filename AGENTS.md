# AGENTS.md

This file is the operating contract for coding agents working in this repository.

Read `CONTRIBUTING.md` at the start of every session. Then read this file before making changes.

The older, detailed compiler/language reference that previously lived here is preserved at `docs/agent-technical-reference.md`. Load that file only when the task actually requires parser, compiler, plist, action-definition, decompiler, or language-internals detail. Do not load it by default.

## Project Goal

This fork extends Cherri into a shared Apple Shortcuts platform used by the compiler, decompiler, documentation, Open Minis skill, iOS/iPadOS app, action catalog, and rich Shortcut preview.

The goal is one evolving source of truth, not parallel implementations.

## Non-Negotiable Architecture Rules

### Cherri core is the primary source of truth

Prefer this flow:

```text
real Shortcut evidence / corpus
        -> Cherri action definitions + metadata
        -> compiler / decompiler / machine-readable catalog
        -> docs / Skill / iOS Action Palette / preview metadata
```

Do not create a second hand-maintained ActionCatalog, plist schema, compiler, decompiler, or action database when the information can live in or be generated from Cherri core.

If multiple consumers need the same metadata, extend the shared Cherri definition/model or generate a common artifact instead of duplicating maps in Go, Swift, JavaScript, Markdown, or the Skill.

### Reuse first

Use this order:

1. Existing Cherri implementation or extension point.
2. Existing maintained upstream/forked project already in the architecture.
3. Small adapter/generator around existing code.
4. New custom implementation only when the first three are genuinely insufficient.

Before introducing a subsystem, search the repository and relevant upstream project for an existing mechanism.

## Fork and Upstream Strategy

This project must remain easy to synchronize with upstream repositories.

For active forks, keep the conceptual relationship:

```text
origin   = our fork
upstream = original project
```

### Isolate our changes whenever practical

Prefer, in order:

- new files;
- new packages/modules;
- generators;
- adapters;
- data/metadata extensions;
- narrowly-scoped hooks in upstream files;
- minimal patches to upstream-owned code.

Avoid broad rewrites, mass formatting, file moves, renames, or unrelated cleanup in upstream-derived files. Every unnecessary edit increases future merge conflicts.

When an upstream file must change, make the smallest stable extension point possible and keep our project-specific behavior behind it.

Do not copy an upstream subsystem into a parallel local implementation merely to make editing easier.

### Active forks vs shadow forks

An active fork may contain our maintained changes and downstream projects may pin to a commit from it.

A shadow fork exists only as an emergency/customization point. Continue consuming upstream directly until our fork actually contains a required change.

Never switch a dependency to a fork merely because the fork exists.

### Pin mutable dependencies

Production/build dependencies that point to our active forks must be pinned to a tag or commit SHA, not a floating development branch.

Keep lockfiles consistent and committed.

### Upstream synchronization

Before a substantial change to a forked component:

1. Inspect divergence from upstream.
2. Fetch upstream changes.
3. Prefer a clean sync/rebase/merge before adding new divergence when safe.
4. If upstream conflicts with our work, isolate the sync on a branch/PR and run tests; never blindly overwrite our changes.

Do not force-push upstream state over local work.

### Fork action provenance

Any fork-specific action addition or material action change (parameters, serialization, compiler/decompiler handling, fork-specific metadata) must be recorded in `docs/fork-action-provenance.json` in the same task. No entry means the action is inherited from upstream untouched. See `docs/shortcut-action-development-playbook.md` for the full policy and recovery procedure.

Provenance commits are non-circular: commit the implementation first, then record its SHA in `firstForkCommit` via a separate follow-up provenance-metadata commit before pushing. Never amend an implementation commit that a registry entry already references.

## Branch and Commit Discipline

Work on the branch explicitly requested by the user/task. Do not merge to `main` unless explicitly asked.

For one coherent task or phase, prefer one coherent commit. Build and test locally first, then commit/push once.

If CI reveals a small issue in the just-created task commit and history has not been shared in a way that makes rewriting unsafe, amend/replace that task commit rather than producing a long chain of tiny fix commits.

Do not rewrite unrelated existing history.

Never commit generated caches, raw private corpus files, temporary logs, build products, or local environment files unless they are intentionally versioned artifacts.

## Token- and Tool-Efficient Work

Optimize token use without reducing verification quality.

### Search before reading

Use targeted discovery first:

```bash
rg -n "symbol|identifier|error text" path/
find path -maxdepth 2 -type f
```

Then read only the relevant ranges/files. Do not dump entire large files merely to locate one function.

### Inspect diffs progressively

Prefer:

```bash
git status --short
git diff --stat
git diff --name-only
git diff -- path/to/relevant/file
```

Do not repeatedly re-read unchanged files or replay full repository diffs after every edit.

### Never feed giant generated data to the model when a script can summarize it

For Shortcut JSON corpora, generated catalogs, lockfiles, bundles, build logs, or other large machine data:

1. run the appropriate analyzer/search/filter locally;
2. produce a compact report;
3. inspect only NEW / VARIANT / REVIEW / failing records;
4. open raw data only for the specific records needed to resolve ambiguity.

Deduplicate and hash raw Shortcut inputs before semantic/model review.

### Batch related operations

Group related searches, reads, edits, tests, and GitHub operations instead of making one tool call per trivial item.

Do not emit frequent narrative status updates during mechanical work. Report meaningful milestones, blockers, discovered defects, and final results.

### Reuse prior evidence

If a command, log, source file, or API result was already inspected and has not changed, use that result instead of fetching it again.

When context may be lost across sessions, record durable project facts in code/tests/docs rather than relying on chat history.

## Test Strategy: Cheap First, Expensive Last

Use the narrowest trustworthy test while iterating, then broaden before finalizing.

Typical order:

1. syntax/static checks;
2. targeted unit test(s);
3. affected package/workflow tests;
4. relevant integration/round-trip tests;
5. full required suite once at the end;
6. iOS/macOS build only when the task affects those layers or final verification requires it.

Do not repeatedly run expensive Xcode/iOS workflows for changes confined to docs, corpus tooling, Skill files, or Linux-only code.

For Cherri compiler internals and action-definition details, load `docs/agent-technical-reference.md` and follow its testing caveats.

## GitHub Actions Discipline

CI is verification, not an interactive debugger.

Before pushing, run the closest local equivalent of the relevant workflow whenever practical.

Workflows should use path filters where appropriate and `concurrency` with `cancel-in-progress: true` to prevent redundant runs.

Prefer Linux runners for corpus analysis, generators, Go tests, Skill validation, and other platform-independent work. Reserve macOS/Xcode runners for work that truly requires them.

Do not manually dispatch a workflow that an existing push/PR event is already going to run unless there is a specific reason.

### Wait for CI with one blocking watcher, not repeated model polling

After a push triggers GitHub Actions, do not repeatedly ask GitHub for status in separate agent turns/tool calls.

Prefer one shell command that blocks until completion, allowing the agent/model to remain idle while GitHub works:

```bash
RUN_ID="$(gh run list --branch "$(git branch --show-current)" --limit 1 --json databaseId --jq '.[0].databaseId')"
gh run watch "$RUN_ID" --exit-status
```

For a PR with several checks, `gh pr checks --watch` is also appropriate.

This blocking watcher is preferred over model-driven polling. Waiting inside the shell/tool does not require the model to repeatedly reason about unchanged state.

If a watcher is unavailable, use a single shell loop with `sleep` inside the same command rather than issuing repeated agent/tool calls. Poll at a reasonable interval (for example 20-60 seconds), not every few seconds.

### Read logs only when necessary

On success, do not download full logs just to confirm success.

On failure, fetch the failed log only:

```bash
gh run view "$RUN_ID" --log-failed
```

Then inspect the relevant error region rather than pasting the entire log into context.

If a rerun is needed, fix the identified root cause first. Do not repeatedly rerun unchanged failing workflows.

## Shortcut Corpus Rules

Raw user-provided Shortcut JSON is evidence, not source code and not automatically safe to commit.

### Privacy

Raw corpus data may contain tokens, API keys, URLs, email addresses, phone numbers, file paths, personal text, names, clipboard content, or other sensitive values.

By default:

- keep raw/inbox corpus data outside version control;
- sanitize before persisting normalized fixtures;
- preserve types, keys, structural shapes, and necessary system/enum constants;
- do not retain personal values merely because they appeared in a Shortcut.

### Observed is not confirmed

A field observed once does not prove its requiredness, complete type domain, default value, availability, or enum set.

Keep evidence states conceptually separate:

```text
observed -> inferred -> confirmed
```

Do not promote uncertain corpus inference into a production action definition without sufficient evidence and tests.

### Incremental analysis

Use file hashes and normalized action fingerprints so already-processed files and duplicate action shapes are not re-analyzed.

Agents should inspect compact analyzer reports and only drill into new/changed/ambiguous records.

## Apple Shortcuts Evidence Hierarchy

For exact action/plist behavior, prefer evidence in this order:

1. a canonical plist/JSON produced by Apple's Shortcuts app;
2. multiple consistent real-world samples;
3. existing verified Cherri implementation/tests;
4. official/reliable documentation where available;
5. inference only when clearly marked and validated.

Do not invent plist keys or serialization structures from memory.

Signing success alone is not proof that an action is semantically correct. Validate structure and round-trip behavior where applicable.

## Action Additions Must Propagate Through Shared Infrastructure

When adding or updating an action, check the affected shared surfaces:

- compiler generation;
- decompiler/round-trip behavior;
- machine-readable action catalog;
- CLI action lookup/docs;
- generated documentation;
- Open Minis Skill discovery/use;
- iOS Action Palette/autocomplete metadata;
- rich preview/fallback rendering.

Do not hand-patch each consumer if shared metadata can make the change propagate automatically.

## Documentation Rules

General Cherri language documentation and our fork-specific/generated action documentation must remain distinguishable and synchronizable with upstream.

Prefer generated action documentation from shared Cherri metadata over manually duplicating action signatures in Markdown.

The Open Minis Skill must not contain a hand-maintained copy of the action catalog. It should query Cherri/catalog/docs.

## Preview Rules

`preview-shortcut` is the renderer foundation. Extend its generic metadata/fallback path before creating custom renderers for individual actions.

Do not create a second Shortcut renderer inside Swift or Cherri merely because an action is not yet richly rendered.

Keep our active preview fork easy to sync with its upstream and pin downstream use to a commit/tag.

## iOS App Rules

The iOS/iPadOS app is a consumer of Cherri core and shared action metadata.

Do not duplicate compiler/action semantics in Swift.

Keep the Go bridge narrow. Prefer exposing stable machine-readable data/functions from Cherri rather than mirroring internal Go structures manually across many Swift files.

Preserve the reuse-first architecture: Cherri core, `CodeEditorView`, and `preview-shortcut` remain the foundations unless a verified limitation requires otherwise.

## Open Minis Skill Rules

The Skill under `skills/cherri-shortcuts/` must drive the real Cherri CLI, not hand-author Shortcut plist.

Keep setup and wrappers POSIX/Alpine-friendly and avoid unnecessary runtime dependencies.

Use local/generated docs and machine-readable catalog lookups instead of embedding large action knowledge in `SKILL.md`.

## Failure and Completion Rules

Never claim a build, test, push, fork, artifact, signing operation, or workflow succeeded unless the relevant command/API result confirms it.

If blocked by permissions or unavailable external infrastructure, continue independent work where possible and report the exact blocker.

Do not ask the user for information that can be resolved from repository state, upstream sources, existing tests, or a targeted command.

A task is complete only when:

- requested implementation is present;
- relevant tests/checks pass;
- generated artifacts/docs are updated when required;
- `git diff` contains no accidental/unrelated changes;
- branch/commit state matches the requested workflow;
- any remaining limitation is stated explicitly.

## Essential Commands

```bash
# Build
go build ./...

# Fast compiler test on non-macOS
go test -run TestCherriNoSign -v ./...

# Decompiler test
go test -run TestDecomp -v ./...

# Inspect actions
cherri --action=actionName

# Skill self-test (when relevant)
sh skills/cherri-shortcuts/scripts/self-test.sh
```

The compiler's per-compilation mutable state has a single authoritative reset (`compiler_state.go`); the full suite is sequential-safe and `go test -count=5` is expected to pass. Round-trip suites still isolate phases in subprocesses.

## Load Detailed Technical Reference Only When Needed

Read `docs/agent-technical-reference.md` when working on any of these:

- parser/pre-parser internals;
- action DSL definitions;
- `actions_std.go` complex action implementations;
- plist serialization/value generation;
- function abstraction;
- decompiler internals;
- Cherri type-system edge cases;
- compiler test quirks and runtime verification;
- exact language syntax details.

For tasks limited to CI, docs plumbing, Skill packaging, fork management, corpus tooling, project structure, or high-level app integration, do not load the full technical reference unless a concrete need appears.
