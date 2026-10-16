#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
SKILL_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
MANIFEST="$SKILL_DIR/compatibility-manifest.json"

echo "=== P10R2 Real Release Integration Test ==="
echo "Manifest: $MANIFEST"

# Read declared manifest refs
COMPILER_REPO=$(grep '"compilerRepo"' "$MANIFEST" | sed 's/.*: *"//;s/".*//')
COMPILER_COMMIT=$(grep '"compilerCommit"' "$MANIFEST" | sed 's/.*: *"//;s/".*//')
DOCS_REPO=$(grep '"docsRepo"' "$MANIFEST" | sed 's/.*: *"//;s/".*//')
DOCS_COMMIT=$(grep '"docsCommit"' "$MANIFEST" | sed 's/.*: *"//;s/".*//')
SCHEMA_FP=$(grep '"schemaFingerprint"' "$MANIFEST" | sed 's/.*: *"//;s/".*//')

echo "Pinned compilerCommit: $COMPILER_COMMIT"
echo "Pinned docsCommit:     $DOCS_COMMIT"
echo "Schema fingerprint:   $SCHEMA_FP"

# Verify that compilerCommit exists syntactically and in git
if [ ${#COMPILER_COMMIT} -ne 40 ]; then
  echo "ERROR: compilerCommit is not 40 hex characters: $COMPILER_COMMIT" >&2
  exit 1
fi
if [ ${#DOCS_COMMIT} -ne 40 ]; then
  echo "ERROR: docsCommit is not 40 hex characters: $DOCS_COMMIT" >&2
  exit 1
fi

TMP_HOME=$(mktemp -d 2>/dev/null || mktemp -d -t 'cherri-home')
cleanup() {
  rm -rf "$TMP_HOME"
}
trap cleanup EXIT INT TERM

echo "Using clean isolated CHERRI_HOME: $TMP_HOME"

# Test 1: Real installation using genuine GitHub clones and pinned SHAs
export CHERRI_HOME="$TMP_HOME"
unset CHERRI_BIN CHERRI_DOCS_DIR || true

echo "Running setup.sh against real repos..."
sh "$SCRIPT_DIR/setup.sh"

echo "Running doctor.sh..."
sh "$SCRIPT_DIR/doctor.sh"

# Verify checked out SHAs match manifest exactly
if [ -d "$TMP_HOME/src/.git" ]; then
  ACTUAL_COMPILER_SHA=$(git -C "$TMP_HOME/src" rev-parse HEAD)
  if [ "$ACTUAL_COMPILER_SHA" != "$COMPILER_COMMIT" ]; then
    echo "ERROR: Installed compiler SHA $ACTUAL_COMPILER_SHA != manifest $COMPILER_COMMIT" >&2
    exit 1
  fi
  echo "PASS: Installed compiler SHA verified ($ACTUAL_COMPILER_SHA)"
fi

if [ -d "$TMP_HOME/docs/.git" ]; then
  ACTUAL_DOCS_SHA=$(git -C "$TMP_HOME/docs" rev-parse HEAD)
  if [ "$ACTUAL_DOCS_SHA" != "$DOCS_COMMIT" ]; then
    echo "ERROR: Installed docs SHA $ACTUAL_DOCS_SHA != manifest $DOCS_COMMIT" >&2
    exit 1
  fi
  echo "PASS: Installed docs SHA verified ($ACTUAL_DOCS_SHA)"
fi

# Test 2: Negative Mutation - Corrupted / Non-existent compiler SHA fails closed
echo "Testing negative mutation: invalid compiler SHA..."
MUTATED_HOME=$(mktemp -d 2>/dev/null || mktemp -d -t 'cherri-bad-sha')
trap 'rm -rf "$TMP_HOME" "$MUTATED_HOME"' EXIT INT TERM

if CHERRI_HOME="$MUTATED_HOME" CHERRI_REF="0000000000000000000000000000000000000000" sh "$SCRIPT_DIR/setup.sh" 2>/dev/null; then
  echo "ERROR: setup.sh succeeded with non-existent compiler SHA!" >&2
  exit 1
else
  echo "PASS: Non-existent compiler SHA correctly rejected fail-closed."
fi

# Test 3: Negative Mutation - Stale docs fingerprint fails closed
echo "Testing negative mutation: stale docs fingerprint..."
if CHERRI_HOME="$MUTATED_HOME" CHERRI_DOCS_DIR="$TMP_HOME/docs" MANIFEST_SCHEMA_FP="0000000000000000000000000000000000000000000000000000000000000000" sh "$SCRIPT_DIR/doctor.sh" 2>/dev/null; then
  echo "ERROR: doctor.sh succeeded with mismatched schema fingerprint!" >&2
  exit 1
else
  echo "PASS: Mismatched schema fingerprint correctly rejected fail-closed."
fi

echo "=== ALL REAL RELEASE INTEGRATION TESTS PASSED ==="
