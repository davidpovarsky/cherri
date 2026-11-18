#!/bin/sh
# Offline causal checks using an actual compiler and canonical generated docs.
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../../.." && pwd)
: "$CHERRI_BIN"
if [ ! -f "$CHERRI_BIN" ]; then
  echo "Set CHERRI_BIN to a freshly built real compiler" >&2
  exit 1
fi
TEST_ROOT=$(mktemp -d)
trap 'rm -rf "$TEST_ROOT"' EXIT INT TERM
export CHERRI_HOME="$TEST_ROOT/home"
export CHERRI_DOCS_DIR="$TEST_ROOT/docs"
mkdir -p "$CHERRI_DOCS_DIR/fork-language"
cp "$REPO_ROOT/docs/language-v2/actions-reference.md" "$CHERRI_DOCS_DIR/fork-language/actions.md"
git -C "$CHERRI_DOCS_DIR" init -q
git -C "$CHERRI_DOCS_DIR" add fork-language/actions.md
git -C "$CHERRI_DOCS_DIR" -c user.name=Cherri-QA -c user.email=qa@example.invalid commit -qm 'Canonical generated docs fixture'
CHERRI_DOCS_REF=$(git -C "$CHERRI_DOCS_DIR" rev-parse HEAD)
# An explicit ref supports running a Windows worktree through WSL.
CHERRI_REF=${CHERRI_REF:-$(git -C "$REPO_ROOT" rev-parse HEAD)}
export CHERRI_DOCS_REF CHERRI_REF
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
git -C "$CHERRI_DOCS_DIR" worktree add --detach -q "$TEST_ROOT/linked-docs" HEAD
positive CHERRI_DOCS_DIR="$TEST_ROOT/linked-docs"
negative CHERRI_STALE_DOCS_COMMIT CHERRI_DOCS_DIR="$TEST_ROOT/linked-docs" CHERRI_DOCS_REF=0000000000000000000000000000000000000000
negative CHERRI_STALE_DOCS_COMMIT CHERRI_DOCS_REF=0000000000000000000000000000000000000000

mkdir -p "$TEST_ROOT/broken-docs/fork-language"
cp "$TEST_ROOT/valid-actions.md" "$TEST_ROOT/broken-docs/fork-language/actions.md"
printf '%s\n' 'gitdir: nonexistent-git-directory' >"$TEST_ROOT/broken-docs/.git"
negative CHERRI_DOCS_GIT_INVALID CHERRI_DOCS_DIR="$TEST_ROOT/broken-docs"
negative CHERRI_COMPILER_NOT_FOUND CHERRI_BIN="$TEST_ROOT/missing-compiler"
positive
printf 'PASS: %s compatibility isolation checks using real compiler and canonical docs\n' "$CHECKS"
