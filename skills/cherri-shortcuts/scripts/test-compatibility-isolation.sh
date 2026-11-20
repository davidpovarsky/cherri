#!/bin/sh
# Offline causal checks using an actual compiler and canonical generated docs.
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../../.." && pwd)
: "$CHERRI_BIN"
: "$CHERRI_MIGRATE_BIN"
if [ ! -f "$CHERRI_BIN" ] || [ ! -f "$CHERRI_MIGRATE_BIN" ]; then
  echo "Set CHERRI_BIN and CHERRI_MIGRATE_BIN to the actual clean same-source tools" >&2
  exit 1
fi
TEST_ROOT=$(mktemp -d)
trap 'rm -rf "$TEST_ROOT"' EXIT INT TERM
export CHERRI_HOME="$TEST_ROOT/home"
export CHERRI_DOCS_DIR="$TEST_ROOT/docs"
mkdir -p "$CHERRI_DOCS_DIR/fork-language"
cp "$REPO_ROOT/docs/language-v2/actions-reference.md" "$CHERRI_DOCS_DIR/fork-language/actions.md"
git --no-replace-objects -C "$CHERRI_DOCS_DIR" init -q
git --no-replace-objects -C "$CHERRI_DOCS_DIR" config core.autocrlf false
git --no-replace-objects -C "$CHERRI_DOCS_DIR" add fork-language/actions.md
git --no-replace-objects -C "$CHERRI_DOCS_DIR" -c user.name=Cherri-QA -c user.email=qa@example.invalid commit -qm 'Canonical generated docs fixture'
CHERRI_DOCS_REF=$(git --no-replace-objects -C "$CHERRI_DOCS_DIR" rev-parse HEAD)
# An explicit ref supports running a Windows worktree through WSL.
CHERRI_REF=${CHERRI_REF:-$(git --no-replace-objects -C "$REPO_ROOT" rev-parse HEAD)}
export CHERRI_DOCS_REF CHERRI_REF
# Use only the reviewed manifest; an unrelated bundled binary cannot mask a bad cache.
export SKILL_DIR="$TEST_ROOT/isolated-skill"
mkdir -p "$SKILL_DIR"
cp -R "$REPO_ROOT/skills/cherri-shortcuts/." "$SKILL_DIR/"
# Only this owned test copy is stripped of bundled candidates.
rm -rf "$SKILL_DIR/bin"
. "$SCRIPT_DIR/common.sh"
INPUT_COMPILER=$CHERRI_BIN
INPUT_MIGRATION=$CHERRI_MIGRATE_BIN
mkdir -p "$TEST_ROOT/actual-tools"
CHERRI_BIN="$TEST_ROOT/actual-tools/cherri"
CHERRI_MIGRATE_BIN="$TEST_ROOT/actual-tools/cherri-migrate"
cp "$INPUT_COMPILER" "$CHERRI_BIN"
cp "$INPUT_MIGRATION" "$CHERRI_MIGRATE_BIN"
for TOOL in "$CHERRI_BIN" "$CHERRI_MIGRATE_BIN"; do
  chmod +x "$TOOL"
  printf '%s\n' "$CHERRI_REF" > "$TOOL.commit"
  cherri_file_sha256 "$TOOL" > "$TOOL.sha256"
done
export CHERRI_BIN CHERRI_MIGRATE_BIN
CHECKS=0

positive() {
  if ! env "$@" sh "$SCRIPT_DIR/doctor.sh" >"$TEST_ROOT/doctor.log" 2>&1; then
    cat "$TEST_ROOT/doctor.log" >&2
    echo "FAIL: healthy positive control" >&2
    exit 1
  fi
  CHECKS=$((CHECKS + 1))
}

negative() {
  expected=$1
  shift
  rc=0
  env "$@" sh "$SCRIPT_DIR/doctor.sh" >"$TEST_ROOT/doctor.log" 2>&1 || rc=$?
  if [ "$rc" -eq 0 ] || ! grep -Fq "[$expected]" "$TEST_ROOT/doctor.log"; then
    cat "$TEST_ROOT/doctor.log" >&2
    echo "FAIL: expected nonzero status and $expected" >&2
    exit 1
  fi
  if [ "$expected" != "CHERRI_COMPILER_NOT_FOUND" ] && grep -Fq '[CHERRI_COMPILER_NOT_FOUND]' "$TEST_ROOT/doctor.log"; then
    echo "FAIL: missing compiler contaminated docs negative" >&2
    exit 1
  fi
  CHECKS=$((CHECKS + 1))
}

positive
cp "$CHERRI_DOCS_DIR/fork-language/actions.md" "$TEST_ROOT/valid-actions.md"
sed 's/\*\*Schema Fingerprint:\*\*[[:space:]]*`[a-f0-9]*`/**Schema Fingerprint:** `0000000000000000000000000000000000000000000000000000000000000000`/' \
  "$TEST_ROOT/valid-actions.md" >"$CHERRI_DOCS_DIR/fork-language/actions.md"
negative CHERRI_STALE_DOCS_SCHEMA
cp "$TEST_ROOT/valid-actions.md" "$CHERRI_DOCS_DIR/fork-language/actions.md"
positive

# A linked worktree has a .git file; this used to skip the docs pin entirely.
git --no-replace-objects -C "$CHERRI_DOCS_DIR" worktree add --detach -q "$TEST_ROOT/linked-docs" HEAD
positive CHERRI_DOCS_DIR="$TEST_ROOT/linked-docs"
negative CHERRI_STALE_DOCS_COMMIT CHERRI_DOCS_DIR="$TEST_ROOT/linked-docs" CHERRI_DOCS_REF=0000000000000000000000000000000000000000
negative CHERRI_STALE_DOCS_COMMIT CHERRI_DOCS_REF=0000000000000000000000000000000000000000

mkdir -p "$TEST_ROOT/broken-docs/fork-language"
cp "$TEST_ROOT/valid-actions.md" "$TEST_ROOT/broken-docs/fork-language/actions.md"
printf '%s\n' 'gitdir: nonexistent-git-directory' >"$TEST_ROOT/broken-docs/.git"
negative CHERRI_DOCS_GIT_INVALID CHERRI_DOCS_DIR="$TEST_ROOT/broken-docs"
negative CHERRI_COMPILER_NOT_FOUND CHERRI_BIN="$TEST_ROOT/missing-compiler"
positive
# Exercise actual cached binaries through setup --offline. Record and reject every
# network-capable command while allowing the existing read-only git pin checks.
export CHERRI_TEST_REAL_GIT="$(command -v git)"
export CHERRI_TEST_NETWORK_LOG="$TEST_ROOT/network-attempts.log"
NO_NETWORK="$TEST_ROOT/no-network"
mkdir -p "$NO_NETWORK" "$CHERRI_HOME/bin"
cat >"$NO_NETWORK/git" <<'CHERRI_NO_NETWORK_GIT'
#!/bin/sh
for arg in "$@"; do
  case "$arg" in
    clone|fetch|pull|push|ls-remote|submodule)
      printf 'git\n' >>"$CHERRI_TEST_NETWORK_LOG"
      exit 97 ;;
  esac
done
exec "$CHERRI_TEST_REAL_GIT" "$@"
CHERRI_NO_NETWORK_GIT
for tool in curl wget; do
  cat >"$NO_NETWORK/$tool" <<'CHERRI_NO_NETWORK_TOOL'
#!/bin/sh
printf 'network tool\n' >>"$CHERRI_TEST_NETWORK_LOG"
exit 97
CHERRI_NO_NETWORK_TOOL
done
chmod +x "$NO_NETWORK/git" "$NO_NETWORK/curl" "$NO_NETWORK/wget"
export PATH="$NO_NETWORK:$PATH"
CACHE="$CHERRI_HOME/bin/cherri"
for TOOL_NAME in cherri cherri-migrate; do
  for SUFFIX in "" .commit .sha256; do
    cp "$TEST_ROOT/actual-tools/$TOOL_NAME$SUFFIX" "$CHERRI_HOME/bin/$TOOL_NAME$SUFFIX"
  done
  chmod +x "$CHERRI_HOME/bin/$TOOL_NAME"
done

offline_positive() {
  case_name=$1
  shift
  if ! env CHERRI_BIN= CHERRI_MIGRATE_BIN= CHERRI_OFFLINE=1 "$@" sh "$SCRIPT_DIR/setup.sh" --offline >"$TEST_ROOT/offline.log" 2>&1; then
    cat "$TEST_ROOT/offline.log" >&2
    echo "FAIL: healthy offline cache: $case_name" >&2
    exit 1
  fi
  cmp -s "$CHERRI_BIN" "$CACHE" || { echo "FAIL: cached compiler bytes changed" >&2; exit 1; }
  cmp -s "$CHERRI_MIGRATE_BIN" "$CHERRI_HOME/bin/cherri-migrate" || { echo "FAIL: cached migration bytes changed" >&2; exit 1; }
  CHECKS=$((CHECKS + 1))
  printf 'CHERRI_K04_CASE:%s\n' "$case_name"
}
offline_negative() {
  case_name=$1
  diagnostic=$2
  shift 2
  rc=0
  env CHERRI_BIN= CHERRI_MIGRATE_BIN= CHERRI_OFFLINE=1 "$@" sh "$SCRIPT_DIR/setup.sh" --offline >"$TEST_ROOT/offline.log" 2>&1 || rc=$?
  if [ "$rc" -eq 0 ] || ! grep -Fq "$diagnostic" "$TEST_ROOT/offline.log"; then
    cat "$TEST_ROOT/offline.log" >&2
    echo "FAIL: expected offline failure: $case_name ($diagnostic)" >&2
    exit 1
  fi
  CHECKS=$((CHECKS + 1))
  printf 'CHERRI_K04_CASE:%s\n' "$case_name"
}

offline_positive offline_cache_healthy
sed 's/\*\*Schema Fingerprint:\*\*[[:space:]]*`[a-f0-9]*`/**Schema Fingerprint:** `0000000000000000000000000000000000000000000000000000000000000000`/' \
  "$TEST_ROOT/valid-actions.md" >"$CHERRI_DOCS_DIR/fork-language/actions.md"
offline_negative stale_schema_rejected CHERRI_STALE_DOCS_SCHEMA
cp "$TEST_ROOT/valid-actions.md" "$CHERRI_DOCS_DIR/fork-language/actions.md"
offline_positive restore_after_schema
offline_negative stale_docs_commit_rejected CHERRI_STALE_DOCS_COMMIT CHERRI_DOCS_REF=0000000000000000000000000000000000000000
offline_positive restore_after_commit
printf '%s\n' 0000000000000000000000000000000000000000 >"$CACHE.commit"
offline_negative incompatible_cache_rejected "no compatible cached or bundled binary"
printf '%s\n' "$CHERRI_REF" >"$CACHE.commit"
offline_positive restore_cache
offline_negative missing_docs_rejected CHERRI_DOCS_MISSING CHERRI_DOCS_DIR="$TEST_ROOT/missing-docs"
if [ -s "$CHERRI_TEST_NETWORK_LOG" ]; then
  echo "FAIL: offline mode attempted network access" >&2
  exit 1
fi
CHECKS=$((CHECKS + 1))
printf 'CHERRI_K04_CASE:no_network_observed\n'
printf 'PASS: %s compatibility isolation checks using real compiler and canonical docs\n' "$CHECKS"
