#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"

SIGN=0
SKIP_SETUP=0
EVIDENCE_OUT="${SKILL_EVIDENCE_OUT:-}"
while [ "$#" -gt 0 ]; do
  case "$1" in
    --sign) SIGN=1; shift ;;
    --skip-setup) SKIP_SETUP=1; shift ;;
    --evidence-out)
      EVIDENCE_OUT=$2
      shift 2
      ;;
    *) echo "Unknown option: $1" >&2; exit 2 ;;
  esac
done

if [ "$SKIP_SETUP" -eq 0 ]; then
  ensure_cherri
else
  CHERRI_BIN_RESOLVED=$(resolve_cherri) || { echo 'Compatible Cherri binary missing' >&2; exit 1; }
fi

printf 'Testing compiler...\n'
"$CHERRI_BIN_RESOLVED" --version >/dev/null

printf 'Testing action discovery wrapper...\n'
sh "$SKILL_DIR/scripts/action.sh" show >/dev/null

printf 'Testing machine-readable catalog wrapper...\n'
"$CHERRI_BIN_RESOLVED" --actions-json | grep -q '"ok":true'
sh "$SKILL_DIR/scripts/catalog.sh" 'is.workflow.actions.alert' >/dev/null

TMP=${TMPDIR:-/tmp}/cherri-skill-test-$$
mkdir -p "$TMP/decompiled" "$TMP/prepared"
trap 'rm -rf "$TMP"' EXIT INT TERM

printf 'Testing unsigned compile wrapper...\n'
cat > "$TMP/test.cherri" <<'SRC'
show("Cherri Open Minis test")
SRC
sh "$SKILL_DIR/scripts/build.sh" "$TMP/test.cherri" "$TMP/test.shortcut" --unsigned >/dev/null
[ -s "$TMP/test.shortcut" ]

printf 'Testing decompile wrapper...\n'
sh "$SKILL_DIR/scripts/decompile.sh" "$TMP/test.shortcut" "$TMP/decompiled" >/dev/null
find "$TMP/decompiled" -maxdepth 1 -type f -name '*.cherri' | grep -q .

printf 'Testing decompiled-source recompilation (self-contained invariant)...\n'
cat > "$TMP/v2actions.cherri" <<'SRC'
let input = "self test"
let encoded = base64Encode(input, lineBreakMode: "None")
let events = getUpcomingEvents(3)
show(f"Input: {input}")
show(f"Base64: {encoded}")
show(f"Events: {events}")
SRC
sh "$SKILL_DIR/scripts/build.sh" "$TMP/v2actions.cherri" "$TMP/v2actions.shortcut" --unsigned >/dev/null
[ -s "$TMP/v2actions.shortcut" ]

rm -rf "$TMP/decompiled-v2"
mkdir -p "$TMP/decompiled-v2"
sh "$SKILL_DIR/scripts/decompile.sh" "$TMP/v2actions.shortcut" "$TMP/decompiled-v2" >/dev/null
V2_SRC=$(find "$TMP/decompiled-v2" -maxdepth 1 -type f -name '*.cherri' | head -n 1)
[ -n "$V2_SRC" ] || { echo 'decompile produced no .cherri source' >&2; exit 1; }

# The decompiled source must compile without manual edits.
sh "$SKILL_DIR/scripts/build.sh" "$V2_SRC" "$TMP/recompiled.shortcut" --unsigned >/dev/null
[ -s "$TMP/recompiled.shortcut" ]

printf 'Testing existing-Shortcut prepare/edit workflow...\n'
PREPARE_OUT=$(sh "$SKILL_DIR/scripts/prepare-edit.sh" "$TMP/v2actions.shortcut" "$TMP/prepared" 2>&1)
printf '%s\n' "$PREPARE_OUT" | grep -q "$TMP/prepared/original/shortcut.plist"
printf '%s\n' "$PREPARE_OUT" | grep -q "$TMP/prepared/source/.*\.cherri"
printf '%s\n' "$PREPARE_OUT" | grep -q "$TMP/prepared/builds/validation.shortcut"
[ -s "$TMP/prepared/builds/validation.shortcut" ]

# AEA1 signed input must be rejected with a clear early diagnostic.
printf 'Testing AEA1 signed-input rejection...\n'
printf 'AEA1mock-signed-payload' > "$TMP/mock-aea1.shortcut"
set +e
sh "$SKILL_DIR/scripts/prepare-edit.sh" "$TMP/mock-aea1.shortcut" "$TMP/prepared-mock-aea1" >/dev/null 2>&1
AEA1_EXIT=$?
set -e
[ "$AEA1_EXIT" -eq 3 ] || { echo "Expected exit code 3 for AEA1 input, got $AEA1_EXIT" >&2; exit 1; }

if [ "$SIGN" -eq 1 ]; then
  sh "$SKILL_DIR/scripts/build.sh" "$TMP/test.cherri" "$TMP/test-signed.shortcut" --signed >/dev/null
  [ "$(dd if="$TMP/test-signed.shortcut" bs=1 count=4 2>/dev/null)" = "AEA1" ]
  if sh "$SKILL_DIR/scripts/prepare-edit.sh" "$TMP/test-signed.shortcut" "$TMP/prepared-aea1" >/dev/null 2>&1; then
    echo 'prepare-edit accepted AEA1 input unexpectedly' >&2
    exit 1
  fi
fi

printf 'Testing 10 Skill Pinning, Compatibility, Offline, and Atomic Update Scenarios...\n'
unset CHERRI_DOCS_DIR || true

# Verify manifest does not reference moving branches or stale d2e7506 commit
if grep -q 'agent/language-redesign\|agent/language-v2-docs' "$SKILL_DIR/scripts/common.sh" "$SKILL_DIR/compatibility-manifest.json"; then
  echo "Stale mutable branch reference found in Skill configuration" >&2
  exit 1
fi
if [ "$MANIFEST_COMPILER_COMMIT" = "d2e7506f466addf687e6b5dfd3cb8198ab1a73ca" ]; then
  echo "Stale compilerCommit d2e7506f466addf687e6b5dfd3cb8198ab1a73ca found in compatibility-manifest.json" >&2
  exit 1
fi

# Helper to create a mock compiler script with custom capabilities output
make_mock_cherri() {
  _target=$1
  _cap_json=$2
  cat > "$_target" <<EOF
#!/bin/sh
if [ "\${1:-}" = "--capabilities-json" ]; then
  cat <<'JSON'
$_cap_json
JSON
  exit 0
fi
if [ "\${1:-}" = "--version" ]; then
  echo "Cherri Mock v2.0"
  exit 0
fi
exec "$CHERRI_BIN_RESOLVED" "\$@"
EOF
  chmod +x "$_target"
}

VALID_CAP_JSON=$("$CHERRI_BIN_RESOLVED" --capabilities-json)
COMPAT_MOCK_BIN="$TMP/compat-cherri"
make_mock_cherri "$COMPAT_MOCK_BIN" "$VALID_CAP_JSON"

# Create a controlled mock compiler git repository with pinned commit
MOCK_COMPILER_REPO="$TMP/mock-compiler-repo"
mkdir -p "$MOCK_COMPILER_REPO"
cp "$COMPAT_MOCK_BIN" "$MOCK_COMPILER_REPO/.cherri-prebuilt-bin"
chmod +x "$MOCK_COMPILER_REPO/.cherri-prebuilt-bin"
git -C "$MOCK_COMPILER_REPO" init -q
git -C "$MOCK_COMPILER_REPO" config user.email "test@cherri.local"
git -C "$MOCK_COMPILER_REPO" config user.name "Cherri Skill Test"
git -C "$MOCK_COMPILER_REPO" add .cherri-prebuilt-bin
git -C "$MOCK_COMPILER_REPO" commit -q -m "Initial pinned compiler commit"
PINNED_COMPILER_SHA=$(git -C "$MOCK_COMPILER_REPO" rev-parse HEAD)

# Create a controlled mock docs git repository with the exact expected schemaFingerprint
MOCK_DOCS_REPO="$TMP/mock-docs-repo"
mkdir -p "$MOCK_DOCS_REPO/fork-language"
cat > "$MOCK_DOCS_REPO/fork-language/actions.md" <<EOF
# Cherri Language v2.0 — Action Reference

**Language Version:** $MANIFEST_LANG_VERSION  
**Catalog Schema:** $MANIFEST_CATALOG_VERSION  
**Schema Fingerprint:** \`$MANIFEST_SCHEMA_FP\`  
EOF
git -C "$MOCK_DOCS_REPO" init -q
git -C "$MOCK_DOCS_REPO" config user.email "test@cherri.local"
git -C "$MOCK_DOCS_REPO" config user.name "Cherri Skill Test"
git -C "$MOCK_DOCS_REPO" add fork-language/actions.md
git -C "$MOCK_DOCS_REPO" commit -q -m "Initial pinned docs commit"
PINNED_DOCS_SHA=$(git -C "$MOCK_DOCS_REPO" rev-parse HEAD)

# Create a test skill copy pinned to PINNED_COMPILER_SHA and PINNED_DOCS_SHA so git commit verification is exercised end-to-end
SKILL_TEST_COPY="$TMP/skill-copy"
mkdir -p "$SKILL_TEST_COPY"
cp -R "$SKILL_DIR/." "$SKILL_TEST_COPY/"
rm -rf "$SKILL_TEST_COPY/bin"
cat > "$SKILL_TEST_COPY/compatibility-manifest.json" <<EOF
{
  "skillVersion": "2.0.0",
  "languageVersion": "$MANIFEST_LANG_VERSION",
  "catalogSchemaVersion": "$MANIFEST_CATALOG_VERSION",
  "schemaFingerprint": "$MANIFEST_SCHEMA_FP",
  "codecFingerprint": "$MANIFEST_CODEC_FP",
  "compilerRepo": "$MOCK_COMPILER_REPO",
  "compilerCommit": "$PINNED_COMPILER_SHA",
  "docsRepo": "$MOCK_DOCS_REPO",
  "docsCommit": "$PINNED_DOCS_SHA"
}
EOF

# Scenario 1: Fresh install on clean environment from immutable compiler and docs refs
HOME1="$TMP/home-s1"
mkdir -p "$HOME1/bin"
(
  unset CHERRI_BIN
  SKILL_DIR="$SKILL_TEST_COPY" \
  CHERRI_HOME="$HOME1" \
  CHERRI_REPO="$MOCK_COMPILER_REPO" \
  CHERRI_REF="$PINNED_COMPILER_SHA" \
  CHERRI_DOCS_REPO="$MOCK_DOCS_REPO" \
  CHERRI_DOCS_REF="$PINNED_DOCS_SHA" \
  sh "$SKILL_TEST_COPY/scripts/setup.sh" >/dev/null
  SKILL_DIR="$SKILL_TEST_COPY" \
  CHERRI_HOME="$HOME1" \
  CHERRI_REF="$PINNED_COMPILER_SHA" \
  CHERRI_DOCS_REF="$PINNED_DOCS_SHA" \
  sh "$SKILL_TEST_COPY/scripts/doctor.sh" >/dev/null
)
[ "$(git -C "$HOME1/src" rev-parse HEAD)" = "$PINNED_COMPILER_SHA" ]
[ "$(git -C "$HOME1/docs" rev-parse HEAD)" = "$PINNED_DOCS_SHA" ]
printf '  [PASS] Scenario 1: fresh install from immutable refs\n'

# Scenario 2: Compatible cached compiler offline
(
  unset CHERRI_BIN
  SKILL_DIR="$SKILL_TEST_COPY" \
  CHERRI_HOME="$HOME1" \
  CHERRI_OFFLINE=1 \
  sh "$SKILL_TEST_COPY/scripts/setup.sh" --offline >/dev/null
  SKILL_DIR="$SKILL_TEST_COPY" \
  CHERRI_HOME="$HOME1" \
  CHERRI_OFFLINE=1 \
  sh "$SKILL_TEST_COPY/scripts/doctor.sh" >/dev/null
)
printf '  [PASS] Scenario 2: compatible cached compiler offline\n'

# Scenario 3: Incompatible cached compiler offline -> clear failure
HOME3="$TMP/home-s3"
mkdir -p "$HOME3/bin"
make_mock_cherri "$HOME3/bin/cherri" '{"languageVersion":"1.0","schemaFingerprint":"0000000000000000000000000000000000000000000000000000000000000000"}'
if (
  unset CHERRI_BIN
  PATH="/usr/bin:/bin" \
  SKILL_DIR="$SKILL_TEST_COPY" \
  CHERRI_HOME="$HOME3" \
  CHERRI_OFFLINE=1 \
  sh "$SKILL_TEST_COPY/scripts/setup.sh" --offline >/dev/null 2>&1
); then
  echo "Scenario 3 failed: incompatible cached compiler succeeded offline" >&2
  exit 1
fi
printf '  [PASS] Scenario 3: incompatible cached compiler rejected offline\n'

# Scenario 4: Stale bundled binary present alongside newly built compatible cache -> compatible cache must win
SKILL_S4="$TMP/skill-s4"
cp -R "$SKILL_TEST_COPY/." "$SKILL_S4/"
mkdir -p "$SKILL_S4/bin"
make_mock_cherri "$SKILL_S4/bin/cherri" '{"languageVersion":"2.0","catalogSchemaVersion":"3.0","schemaFingerprint":"32cd14d86ebf5c6ebbe67a6f435468fd9647a3382cf47c09d845b74b9d263694"}'
RESOLVED_S4=$(
  unset CHERRI_BIN
  SKILL_DIR="$SKILL_S4" \
  CHERRI_HOME="$HOME1" \
  sh -c '. "$SKILL_DIR/scripts/common.sh" && resolve_cherri'
)
[ "$RESOLVED_S4" = "$HOME1/bin/cherri" ] || {
  echo "Scenario 4 failed: expected $HOME1/bin/cherri to win over stale bundled binary, got $RESOLVED_S4" >&2
  exit 1
}
printf '  [PASS] Scenario 4: compatible cached compiler wins over stale bundled binary\n'

# Scenario 5: Explicit incompatible CHERRI_BIN -> error, not a silent bypass
BAD_CHERRI_BIN="$TMP/bad-explicit-cherri"
make_mock_cherri "$BAD_CHERRI_BIN" '{"languageVersion":"1.0","schemaFingerprint":"0000000000000000000000000000000000000000000000000000000000000000"}'
if (
  CHERRI_BIN="$BAD_CHERRI_BIN" \
  SKILL_DIR="$SKILL_TEST_COPY" \
  CHERRI_HOME="$HOME1" \
  sh "$SKILL_TEST_COPY/scripts/doctor.sh" >/dev/null 2>&1
); then
  echo "Scenario 5 failed: explicit incompatible CHERRI_BIN was not rejected" >&2
  exit 1
fi
printf '  [PASS] Scenario 5: explicit incompatible CHERRI_BIN rejected without silent fallback\n'

# Scenario 6: Same languageVersion ("2.0") but different schemaFingerprint -> error
WRONG_FP_BIN="$TMP/wrong-fp-cherri"
make_mock_cherri "$WRONG_FP_BIN" '{"languageVersion":"2.0","catalogSchemaVersion":"3.0","schemaFingerprint":"32cd14d86ebf5c6ebbe67a6f435468fd9647a3382cf47c09d845b74b9d263694"}'
if (
  CHERRI_BIN="$WRONG_FP_BIN" \
  SKILL_DIR="$SKILL_TEST_COPY" \
  CHERRI_HOME="$HOME1" \
  sh "$SKILL_TEST_COPY/scripts/doctor.sh" >/dev/null 2>&1
); then
  echo "Scenario 6 failed: mismatched schemaFingerprint with languageVersion 2.0 was accepted" >&2
  exit 1
fi
printf '  [PASS] Scenario 6: same languageVersion with wrong schemaFingerprint rejected\n'

# Scenario 7: Missing or malformed schemaFingerprint / malformed JSON -> error
MALFORMED_FP_BIN="$TMP/malformed-fp-cherri"
make_mock_cherri "$MALFORMED_FP_BIN" '{"languageVersion":"2.0","catalogSchemaVersion":"3.0","schemaFingerprint":"not-a-64-char-hex"}'
BROKEN_JSON_BIN="$TMP/broken-json-cherri"
make_mock_cherri "$BROKEN_JSON_BIN" '{"languageVersion":"2.0","schemaFingerprint":"'"$MANIFEST_SCHEMA_FP"'"'
if (
  CHERRI_BIN="$MALFORMED_FP_BIN" \
  SKILL_DIR="$SKILL_TEST_COPY" \
  CHERRI_HOME="$HOME1" \
  sh "$SKILL_TEST_COPY/scripts/doctor.sh" >/dev/null 2>&1
); then
  echo "Scenario 7a failed: malformed schemaFingerprint was accepted" >&2
  exit 1
fi
if (
  CHERRI_BIN="$BROKEN_JSON_BIN" \
  SKILL_DIR="$SKILL_TEST_COPY" \
  CHERRI_HOME="$HOME1" \
  sh "$SKILL_TEST_COPY/scripts/doctor.sh" >/dev/null 2>&1
); then
  echo "Scenario 7b failed: malformed capabilities JSON was accepted" >&2
  exit 1
fi
printf '  [PASS] Scenario 7: missing/malformed schemaFingerprint and broken JSON rejected\n'

# Scenario 8: Docs SHA or fingerprint mismatch -> error
BAD_DOCS_DIR="$TMP/bad-docs"
mkdir -p "$BAD_DOCS_DIR/fork-language"
cat > "$BAD_DOCS_DIR/fork-language/actions.md" <<'EOF'
# Actions Reference
**Schema Fingerprint:** `32cd14d86ebf5c6ebbe67a6f435468fd9647a3382cf47c09d845b74b9d263694`
EOF
if (
  unset CHERRI_BIN
  SKILL_DIR="$SKILL_TEST_COPY" \
  CHERRI_HOME="$HOME1" \
  CHERRI_DOCS_DIR="$BAD_DOCS_DIR" \
  sh "$SKILL_TEST_COPY/scripts/doctor.sh" >/dev/null 2>&1
); then
  echo "Scenario 8 failed: docs with mismatched Schema Fingerprint were accepted" >&2
  exit 1
fi
printf '  [PASS] Scenario 8: docs SHA/fingerprint mismatch rejected\n'

# Scenario 9: Update deliberately requested -> controlled atomic replacement (no broken binary on failure)
HOME9="$TMP/home-s9"
mkdir -p "$HOME9/bin"
cp "$COMPAT_MOCK_BIN" "$HOME9/bin/cherri"
printf '%s\n' "$PINNED_COMPILER_SHA" > "$HOME9/bin/cherri.commit"
cp -R "$HOME1/docs" "$HOME9/docs"
if (
  unset CHERRI_BIN
  SKILL_DIR="$SKILL_TEST_COPY" \
  CHERRI_HOME="$HOME9" \
  CHERRI_PREBUILT_URL="http://127.0.0.1:1/unreachable-cherri-binary" \
  sh "$SKILL_TEST_COPY/scripts/setup.sh" --update >/dev/null 2>&1
); then
  echo "Scenario 9 failed: setup.sh --update with broken download URL unexpectedly succeeded" >&2
  exit 1
fi
# Verify existing working binary in HOME9 was preserved intact
(
  unset CHERRI_BIN
  SKILL_DIR="$SKILL_TEST_COPY" \
  CHERRI_HOME="$HOME9" \
  sh "$SKILL_TEST_COPY/scripts/doctor.sh" >/dev/null
)
printf '  [PASS] Scenario 9: failed update preserved existing working binary atomically\n'

# Scenario 10: Branch advance after install -> must NOT silently change the installed version
make_mock_cherri "$MOCK_COMPILER_REPO/.cherri-prebuilt-bin" '{"languageVersion":"2.0","catalogSchemaVersion":"3.0","schemaFingerprint":"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}'
git -C "$MOCK_COMPILER_REPO" add .cherri-prebuilt-bin
git -C "$MOCK_COMPILER_REPO" commit -q -m "Advance compiler branch past pinned commit"
ADVANCED_COMPILER_SHA=$(git -C "$MOCK_COMPILER_REPO" rev-parse HEAD)
[ "$ADVANCED_COMPILER_SHA" != "$PINNED_COMPILER_SHA" ]

cat > "$MOCK_DOCS_REPO/fork-language/actions.md" <<'EOF'
# Advanced Unverified Branch Commit
**Schema Fingerprint:** `ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff`
EOF
git -C "$MOCK_DOCS_REPO" add fork-language/actions.md
git -C "$MOCK_DOCS_REPO" commit -q -m "Advance docs branch past pinned commit"
ADVANCED_DOCS_SHA=$(git -C "$MOCK_DOCS_REPO" rev-parse HEAD)
[ "$ADVANCED_DOCS_SHA" != "$PINNED_DOCS_SHA" ]
(
  unset CHERRI_BIN
  SKILL_DIR="$SKILL_TEST_COPY" \
  CHERRI_HOME="$HOME1" \
  CHERRI_REPO="$MOCK_COMPILER_REPO" \
  CHERRI_DOCS_REPO="$MOCK_DOCS_REPO" \
  sh "$SKILL_TEST_COPY/scripts/setup.sh" --update >/dev/null
  SKILL_DIR="$SKILL_TEST_COPY" \
  CHERRI_HOME="$HOME1" \
  sh "$SKILL_TEST_COPY/scripts/doctor.sh" >/dev/null
)
INSTALLED_COMPILER_SHA=$(git -C "$HOME1/src" rev-parse HEAD)
[ "$INSTALLED_COMPILER_SHA" = "$PINNED_COMPILER_SHA" ] || {
  echo "Scenario 10 failed: branch advance changed installed compiler commit from $PINNED_COMPILER_SHA to $INSTALLED_COMPILER_SHA" >&2
  exit 1
}
INSTALLED_DOCS_SHA=$(git -C "$HOME1/docs" rev-parse HEAD)
[ "$INSTALLED_DOCS_SHA" = "$PINNED_DOCS_SHA" ] || {
  echo "Scenario 10 failed: branch advance changed installed docs commit from $PINNED_DOCS_SHA to $INSTALLED_DOCS_SHA" >&2
  exit 1
}
printf '  [PASS] Scenario 10: branch advance did not alter pinned installed version\n'

if [ -n "$EVIDENCE_OUT" ]; then
  mkdir -p "$(dirname -- "$EVIDENCE_OUT")"
  IMPL_SHA="${GITHUB_SHA:-$(git -C "$SKILL_DIR/../.." rev-parse HEAD 2>/dev/null || printf '%s' "$MANIFEST_COMPILER_COMMIT")}"
  cat > "$EVIDENCE_OUT" <<EOF
{
  "records": [
    {
      "evidence_id": "ev-skill-self-test-packaging-$IMPL_SHA",
      "requirements": ["K01", "RP42", "RP44", "RP45", "RP49", "BRG30"],
      "tier": "skill-package",
      "implementation_sha": "$IMPL_SHA",
      "test_id": "skills/cherri-shortcuts/scripts/self-test.sh",
      "exit_code": 0,
      "result": "passed",
      "assertions": {
        "scenario_1_fresh_immutable_install": "passed",
        "scenario_2_compatible_cached_offline": "passed",
        "scenario_3_incompatible_cached_offline_rejected": "passed",
        "scenario_4_compatible_cache_wins_over_stale_bundled": "passed",
        "scenario_5_explicit_incompatible_cherri_bin_rejected": "passed",
        "scenario_6_same_lang_wrong_schema_fp_rejected": "passed",
        "scenario_7_missing_or_malformed_schema_fp_rejected": "passed",
        "scenario_8_docs_sha_or_fp_mismatch_rejected": "passed",
        "scenario_9_atomic_update_preserves_working_binary": "passed",
        "scenario_10_branch_advance_preserves_pinned_commit": "passed",
        "compiler_commit": "$MANIFEST_COMPILER_COMMIT",
        "docs_commit": "$MANIFEST_DOCS_COMMIT",
        "schema_fingerprint": "$MANIFEST_SCHEMA_FP",
        "codec_fingerprint": "$MANIFEST_CODEC_FP"
      },
      "detail": "All 10 Skill packaging, compatibility, offline, mismatch, atomic update, and immutable ref scenarios passed"
    },
    {
      "evidence_id": "ev-skill-self-test-integration-$IMPL_SHA",
      "requirements": ["K02", "K03", "K04", "K05", "D03", "RP44", "RP45", "RP46", "BRG30"],
      "tier": "integration",
      "implementation_sha": "$IMPL_SHA",
      "test_id": "skills/cherri-shortcuts/scripts/self-test.sh",
      "exit_code": 0,
      "result": "passed",
      "assertions": {
        "scenario_1_fresh_immutable_install": "passed",
        "scenario_2_compatible_cached_offline": "passed",
        "scenario_3_incompatible_cached_offline_rejected": "passed",
        "scenario_4_compatible_cache_wins_over_stale_bundled": "passed",
        "scenario_5_explicit_incompatible_cherri_bin_rejected": "passed",
        "scenario_6_same_lang_wrong_schema_fp_rejected": "passed",
        "scenario_7_missing_or_malformed_schema_fp_rejected": "passed",
        "scenario_8_docs_sha_or_fp_mismatch_rejected": "passed",
        "scenario_9_atomic_update_preserves_working_binary": "passed",
        "scenario_10_branch_advance_preserves_pinned_commit": "passed",
        "compiler_commit": "$MANIFEST_COMPILER_COMMIT",
        "docs_commit": "$MANIFEST_DOCS_COMMIT",
        "schema_fingerprint": "$MANIFEST_SCHEMA_FP",
        "codec_fingerprint": "$MANIFEST_CODEC_FP"
      },
      "detail": "All 10 Skill CLI integration, offline cache, mismatch rejection, and atomic update scenarios passed"
    }
  ]
}
EOF
fi

echo 'Cherri Shortcuts skill self-test passed.'
