# Shortcut Corpus Operations

This is the short operational runbook for recurring Shortcut action-development sessions. Read `AGENTS.md`, `CONTRIBUTING.md`, `docs/corpus-platform.md`, and `docs/shortcut-action-development-playbook.md` before changing action semantics.

## Goal

A normal future instruction can be as short as:

> Work another corpus session: fetch fresh approved Shortcut evidence, implement the strongest genuinely missing/variant Cherri actions you can verify, and stop when the time/work budget is exhausted.

The agent should not manually scan known actions. The corpus tool is responsible for filtering them.

## Start

Verify repository state:

```sh
git status --short
git branch --show-current
git log -1 --oneline
gh pr view 2
```

For the current experimental phase, action/corpus work remains based on `agent/shortcut-corpus-platform`; acquisition infrastructure may live on a child Draft PR until explicitly integrated. Never merge to a base branch without user instruction.

Build:

```sh
go build -o dist/cherri .
go build -o dist/shortcut-corpus ./tools/shortcut-corpus
```

## Normal session

```sh
dist/shortcut-corpus session \
  -sources routinehub \
  -state corpus-state.json \
  -acquisition-state corpus-acquisition-state.json \
  -inbox corpus-inbox \
  -out analysis/latest \
  -max-items 100 \
  -max-actionable 20 \
  -cherri dist/cherri
```

For a prepared list of public iCloud links:

```sh
dist/shortcut-corpus session \
  -sources routinehub,seed \
  -seed public-icloud-links.txt \
  -state corpus-state.json \
  -acquisition-state corpus-acquisition-state.json \
  -inbox corpus-inbox \
  -out analysis/latest \
  -max-actionable 20 \
  -cherri dist/cherri
```

Historical bootstrap, when needed:

```sh
dist/shortcut-corpus session \
  -sources shortcutsbench \
  -max-items 250 \
  -state corpus-state.json \
  -acquisition-state corpus-acquisition-state.json \
  -inbox corpus-inbox \
  -out analysis/bootstrap \
  -cherri dist/cherri
```

ShortcutsBench is a bootstrap source, not something to redownload every session. Acquisition state makes subsequent runs incremental.

## Read only high-signal output first

Read:

```text
analysis/latest/summary.md
analysis/latest/agent-queue.md
```

Do not open the entire raw corpus or `known-actions.json` into model context.

If `agent-queue.md` is empty, report that there is no actionable new/variant evidence and do not churn definitions.

## Choose work

Default priority is already ranked by the queue:

1. repeated new Apple/system schema;
2. repeated structural variant of a Cherri-known action;
3. strong single-sample declarative candidate that can be independently confirmed;
4. complex/custom serialization with repeated evidence;
5. lower-confidence review items.

Third-party App Intents remain outside the default queue. Do not bulk-add random app actions.

## Investigate one item

Use the queue item to locate:

- `WFWorkflowActionIdentifier`;
- parameter keys;
- unknown keys versus current catalog;
- serialization/reference envelopes;
- sanitized structural constants;
- distinct evidence hashes;
- generated candidate, if any.

Then inspect:

1. current `cherri --actions-json` entry;
2. closest existing `actions/*.cherri` definition;
3. custom implementation/decompiler only if relevant;
4. the minimum distinct raw plist action dictionaries needed to settle ambiguity.

Never infer requiredness/defaults/platform support from one observation.

## Implement

Prefer declarative definitions. Read `docs/agent-technical-reference.md` before custom parser/compiler/plist/decompiler changes.

After each action/variant addition, run targeted tests before broad suites.

Typical checks:

```sh
go test -run '<target>' -v ./...
go test -run TestCherriNoSign -v ./...
go test -run TestDecomp -v ./...
go test ./tools/... -v
```

Use structural round trip where appropriate:

```sh
sh tools/shortcut-corpus/scripts/roundtrip.sh tests/<relevant>.cherri
```

## Closure is mandatory

After implementing an action, do not manually mark the corpus item complete.

Run:

```sh
dist/shortcut-corpus reclassify \
  -state corpus-state.json \
  -out analysis/after-change \
  -cherri dist/cherri
```

The implemented item must become `KNOWN` and disappear from `agent-queue.md`.

If it remains queued, investigate the real cause:

- incomplete action definition;
- missing shared catalog metadata;
- real additional variant;
- classifier bug.

Do not suppress it with an ignore list merely to make the queue green.

## Propagation checklist

A verified action change should flow from Cherri's source of truth to:

- compiler output;
- decompiler/round trip;
- `cherri --actions-json`;
- generated docs;
- Open Minis Skill catalog lookup/use;
- iOS Action Palette through the Go bridge;
- preview metadata/generic fallback.

Do not hand-maintain separate action tables in consumers.

## Fork provenance

Every fork-specific action addition or material behavior/parameter/serialization change must follow `docs/fork-action-provenance.json` rules in the playbook. Infrastructure-only corpus changes belong under infrastructure documentation, not fake action entries.

## Source policy

Run:

```sh
dist/shortcut-corpus sources
```

Do not turn a `MANUAL_ONLY`/`DISABLED` source into a crawler during an action session. Use an independently obtained public iCloud link via `seed` instead.

If one live source fails or changes format, continue with healthy sources and existing inbox/state. Do not redesign the analyzer around one website.

## Failure recovery

- Acquisition failures are source/item failures, not reasons to throw away semantic state.
- A dead/stopped iCloud share should be recorded as failed rather than retried infinitely in one run.
- Duplicate iCloud IDs/plist hashes are expected and should not increase evidence.
- State files are written atomically.
- Raw corpus/state/analysis files remain ignored.

## End of session report

Report:

- branch/commits;
- acquisition counts;
- queue before/after;
- actions added or updated;
- evidence used;
- exact tests run and outcomes;
- closure proof for each implemented item;
- any real unresolved limitations.

Do not merge the Draft corpus/action branches unless the user explicitly requests it.
