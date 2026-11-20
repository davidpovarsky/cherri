#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$SCRIPT_DIR/common.sh"
. "$SCRIPT_DIR/installation.sh"

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
# macOS TMPDIR commonly ends with a slash. Compare the same absolute path
# spelling that prepare-edit reports, including normalized separators.
TMP=$(CDPATH= cd -- "$TMP" && pwd -P)
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
if ! PREPARE_OUT=$(sh "$SKILL_DIR/scripts/prepare-edit.sh" "$TMP/v2actions.shortcut" "$TMP/prepared" 2>&1); then
  printf '%s\n' "$PREPARE_OUT" >&2
  exit 1
fi
printf '%s\n' "$PREPARE_OUT" | grep -Fxq "$TMP/prepared/original/shortcut.plist"
printf '%s\n' "$PREPARE_OUT" | grep -F "$TMP/prepared/source/" | grep -q '\.cherri$'
printf '%s\n' "$PREPARE_OUT" | grep -Fxq "$TMP/prepared/builds/validation.shortcut"
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

printf 'Testing 10 controlled transport, compatibility, and pinned-document scenarios with real tools...\n'
# Download responses and the small documentation repository below are owned
# fixtures. Compiler/migration bytes and embedded identities are real. These
# controls do not prove a source build; RP45 independently executes that route.
if grep -q 'agent/language-redesign\|agent/language-v2-docs' "$SKILL_DIR/scripts/common.sh" "$SKILL_DIR/compatibility-manifest.json"; then
  echo 'Mutable branch reference found in Skill configuration' >&2
  exit 1
fi
if [ "$MANIFEST_COMPILER_COMMIT" = 'd2e7506f466addf687e6b5dfd3cb8198ab1a73ca' ]; then
  echo 'Stale compilerCommit d2e7506 remains in the manifest' >&2
  exit 1
fi

CONTROL_COMPILER_INPUT=$CHERRI_BIN_RESOLVED
CONTROL_MIGRATION_INPUT=$(cherri_migration_for_compiler "$CONTROL_COMPILER_INPUT")
validate_cherri_installation "$CONTROL_COMPILER_INPUT" "$CONTROL_MIGRATION_INPUT"
"$CONTROL_MIGRATION_INPUT" --inspect-build-json "$CONTROL_COMPILER_INPUT" > "$TMP/control-compiler-identity.json"
json_file_is_valid_object "$TMP/control-compiler-identity.json"
PINNED_COMPILER_SHA=$(json_parse_object sourceCommit < "$TMP/control-compiler-identity.json")
CONTROL_COMPILER_SHA=$(cherri_file_sha256 "$CONTROL_COMPILER_INPUT")
CONTROL_MIGRATION_SHA=$(cherri_file_sha256 "$CONTROL_MIGRATION_INPUT")

MOCK_DOCS_REPO="$TMP/controlled-docs-repo"
mkdir -p "$MOCK_DOCS_REPO/fork-language"
cat > "$MOCK_DOCS_REPO/fork-language/actions.md" <<EOF
# Controlled documentation fixture; not the published reference
**Language Version:** $MANIFEST_LANG_VERSION
**Catalog Schema:** $MANIFEST_CATALOG_VERSION
**Schema Fingerprint:** \`$MANIFEST_SCHEMA_FP\`
EOF
if [ -n "$MANIFEST_MACRO_FP" ]; then
  printf '**Compiler Macro Fingerprint:** `%s`\n' "$MANIFEST_MACRO_FP" >> "$MOCK_DOCS_REPO/fork-language/actions.md"
fi
git --no-replace-objects -C "$MOCK_DOCS_REPO" init -q
git --no-replace-objects -C "$MOCK_DOCS_REPO" config core.autocrlf false
git --no-replace-objects -C "$MOCK_DOCS_REPO" add fork-language/actions.md
git --no-replace-objects -C "$MOCK_DOCS_REPO" -c user.name=Cherri-QA -c user.email=qa@example.invalid commit -qm 'Controlled pinned docs fixture'
PINNED_DOCS_SHA=$(git --no-replace-objects -C "$MOCK_DOCS_REPO" rev-parse HEAD)

SKILL_TEST_COPY="$TMP/skill-copy"
mkdir -p "$SKILL_TEST_COPY"
cp -R "$SKILL_DIR/." "$SKILL_TEST_COPY/"
rm -rf "$SKILL_TEST_COPY/bin"
cat > "$SKILL_TEST_COPY/compatibility-manifest.json" <<EOF
{
  "skillVersion": "2.0.0",
  "languageVersion": "$MANIFEST_LANG_VERSION",
  "catalogSchemaVersion": "$MANIFEST_CATALOG_VERSION",
  "protocolVersion": "$MANIFEST_PROTOCOL_VERSION",
  "schemaFingerprint": "$MANIFEST_SCHEMA_FP",
  "macroFingerprint": "$MANIFEST_MACRO_FP",
  "codecFingerprint": "$MANIFEST_CODEC_FP",
  "compilerRepo": "$MANIFEST_COMPILER_REPO",
  "compilerCommit": "$PINNED_COMPILER_SHA",
  "docsRepo": "$MOCK_DOCS_REPO",
  "docsCommit": "$PINNED_DOCS_SHA"
}
EOF

CONTROL_TRANSPORT="$TMP/controlled-transport"
CONTROL_TRANSPORT_LOG="$TMP/controlled-downloads.log"
mkdir -p "$CONTROL_TRANSPORT"
cat > "$CONTROL_TRANSPORT/wget" <<'CONTROL_WGET'
#!/bin/sh
set -eu
destination=
request=
while [ "$#" -gt 0 ]; do
  case "$1" in
    -q) shift ;;
    -O) destination=$2; shift 2 ;;
    *) request=$1; shift ;;
  esac
done
[ -n "$destination" ] && [ -n "$request" ] || exit 96
printf '%s\n' "$request" >> "$CONTROL_TRANSPORT_LOG"
case "$request" in
  controlled://compiler) cp "$CONTROL_COMPILER_INPUT" "$destination" ;;
  controlled://migration) cp "$CONTROL_MIGRATION_INPUT" "$destination" ;;
  *) echo '[CONTROLLED_DOWNLOAD_REJECTED]' >&2; exit 97 ;;
esac
CONTROL_WGET
chmod +x "$CONTROL_TRANSPORT/wget"
export CONTROL_COMPILER_INPUT CONTROL_MIGRATION_INPUT CONTROL_TRANSPORT_LOG

control_environment() {
  # Each owned invocation is isolated from optional caller overrides.
  unset CHERRI_BIN CHERRI_MIGRATE_BIN CHERRI_DOCS_DIR CHERRI_MANIFEST_FILE
  unset CHERRI_REPO CHERRI_DOCS_REPO CHERRI_PREBUILT_URL CHERRI_PREBUILT_SHA256
  unset CHERRI_MIGRATE_PREBUILT_URL CHERRI_MIGRATE_PREBUILT_SHA256 CHERRI_OFFLINE
  SKILL_DIR=$SKILL_TEST_COPY
  CHERRI_HOME=$1
  CHERRI_REF=$PINNED_COMPILER_SHA
  CHERRI_DOCS_REF=$PINNED_DOCS_SHA
  export SKILL_DIR CHERRI_HOME CHERRI_REF CHERRI_DOCS_REF
}
control_setup() (
  control_environment "$1"
  shift
  CHERRI_PREBUILT_URL=${CONTROL_DOWNLOAD_URL:-controlled://compiler}
  CHERRI_PREBUILT_SHA256=$CONTROL_COMPILER_SHA
  CHERRI_MIGRATE_PREBUILT_URL=controlled://migration
  CHERRI_MIGRATE_PREBUILT_SHA256=$CONTROL_MIGRATION_SHA
  PATH="$CONTROL_TRANSPORT:$PATH"
  export PATH CHERRI_PREBUILT_URL CHERRI_PREBUILT_SHA256 CHERRI_MIGRATE_PREBUILT_URL CHERRI_MIGRATE_PREBUILT_SHA256
  [ "$(command -v wget)" = "$CONTROL_TRANSPORT/wget" ] || exit 95
  sh "$SKILL_TEST_COPY/scripts/setup.sh" "$@"
)
control_doctor() (
  control_environment "$1"
  shift
  env "$@" sh "$SKILL_TEST_COPY/scripts/doctor.sh"
)
control_pair_unchanged() {
  for _control_tool in cherri cherri-migrate; do
    for _control_suffix in '' .commit .sha256; do
      cmp -s "$1/bin/$_control_tool$_control_suffix" "$2/bin/$_control_tool$_control_suffix" || return 1
    done
  done
  cmp -s "$1/docs/fork-language/actions.md" "$2/docs/fork-language/actions.md"
}
control_caps() {
  printf '{"languageVersion":"%s","catalogSchemaVersion":"%s","protocolVersion":"%s","schemaFingerprint":"%s","codecFingerprint":"%s","macroFingerprint":"%s"}' \
    "$1" "$MANIFEST_CATALOG_VERSION" "$MANIFEST_PROTOCOL_VERSION" "$2" "$MANIFEST_CODEC_FP" "$MANIFEST_MACRO_FP"
}
make_mock_cherri() {
  _target=$1
  _cap_json=$2
  mkdir -p "$(dirname -- "$_target")"
  cat > "$_target" <<EOF
#!/bin/sh
if [ "\${1:-}" = '--capabilities-json' ]; then
  printf observed > "$_target.observed"
  cat <<'JSON'
$_cap_json
JSON
  exit 0
fi
exec "$CONTROL_COMPILER_INPUT" "\$@"
EOF
  chmod +x "$_target"
  printf '%s\n' "$PINNED_COMPILER_SHA" > "$_target.commit"
  cherri_file_sha256 "$_target" > "$_target.sha256"
}
reject_mock_doctor() {
  _control_candidate=$1
  _control_expected=$2
  if control_doctor "$HOME1" CHERRI_BIN="$_control_candidate" > "$TMP/controlled-rejection.log" 2>&1; then
    echo 'Adversarial capabilities were accepted' >&2
    exit 1
  fi
  [ "$(cat "$_control_candidate.observed")" = observed ]
  grep -Fq "$_control_expected" "$TMP/controlled-rejection.log"
}

# 1: Actual tools, authenticated bytes/source, controlled download transport.
HOME1="$TMP/home-s1"
control_setup "$HOME1" > "$TMP/controlled-fresh.log" 2>&1 || { cat "$TMP/controlled-fresh.log" >&2; exit 1; }
control_doctor "$HOME1" >/dev/null
[ "$(wc -l < "$CONTROL_TRANSPORT_LOG")" -eq 2 ]
cmp -s "$CONTROL_COMPILER_INPUT" "$HOME1/bin/cherri"
cmp -s "$CONTROL_MIGRATION_INPUT" "$HOME1/bin/cherri-migrate"
[ "$(git --no-replace-objects -C "$HOME1/docs" rev-parse HEAD)" = "$PINNED_DOCS_SHA" ]
printf '  [PASS] Scenario 1: real same-source pair installed through controlled downloads\n'

# 2: A valid complete cache performs no further controlled download.
control_setup "$HOME1" --offline >/dev/null
control_doctor "$HOME1" >/dev/null
[ "$(wc -l < "$CONTROL_TRANSPORT_LOG")" -eq 2 ]
printf '  [PASS] Scenario 2: compatible complete cache is reusable offline\n'

# 3: The invalid protocol response is actually requested before rejection.
HOME3="$TMP/home-s3"
make_mock_cherri "$HOME3/bin/cherri" "$(control_caps 1.0 "$MANIFEST_SCHEMA_FP")"
if control_setup "$HOME3" --offline > "$TMP/controlled-offline-rejection.log" 2>&1; then exit 1; fi
[ "$(cat "$HOME3/bin/cherri.observed")" = observed ]
grep -Fq 'no compatible cached or bundled binary available' "$TMP/controlled-offline-rejection.log"
printf '  [PASS] Scenario 3: incompatible cached response rejected offline\n'

# 4: The valid complete cache wins without executing an unselected bundle.
SKILL_S4="$TMP/skill-s4"
cp -R "$SKILL_TEST_COPY/." "$SKILL_S4/"
make_mock_cherri "$SKILL_S4/bin/cherri" "$(control_caps "$MANIFEST_LANG_VERSION" 0000000000000000000000000000000000000000000000000000000000000000)"
RESOLVED_S4=$(
  control_environment "$HOME1"
  SKILL_DIR=$SKILL_S4
  export SKILL_DIR
  sh -c '. "$SKILL_DIR/scripts/common.sh"; . "$SKILL_DIR/scripts/installation.sh"; resolve_cherri_installation'
)
[ "$RESOLVED_S4" = "$HOME1/bin/cherri" ]
[ ! -e "$SKILL_S4/bin/cherri.observed" ]
printf '  [PASS] Scenario 4: verified cache wins without running an incompatible bundle\n'

# 5-7: Synthetic response controls, with mandatory real-byte receipts and
# observed response execution; unrelated missing-receipt failures cannot pass.
BAD_CHERRI_BIN="$TMP/bad-explicit-cherri"
make_mock_cherri "$BAD_CHERRI_BIN" "$(control_caps 1.0 "$MANIFEST_SCHEMA_FP")"
reject_mock_doctor "$BAD_CHERRI_BIN" 'languageVersion mismatch'
printf '  [PASS] Scenario 5: explicit incompatible response never falls back\n'
WRONG_FP_BIN="$TMP/wrong-fp-cherri"
make_mock_cherri "$WRONG_FP_BIN" "$(control_caps "$MANIFEST_LANG_VERSION" 0000000000000000000000000000000000000000000000000000000000000000)"
reject_mock_doctor "$WRONG_FP_BIN" 'schemaFingerprint mismatch'
printf '  [PASS] Scenario 6: same language with incorrect schema is rejected\n'
MALFORMED_FP_BIN="$TMP/malformed-fp-cherri"
make_mock_cherri "$MALFORMED_FP_BIN" "$(control_caps "$MANIFEST_LANG_VERSION" not-a-64-char-hex)"
reject_mock_doctor "$MALFORMED_FP_BIN" 'malformed schemaFingerprint'
BROKEN_JSON_BIN="$TMP/broken-json-cherri"
make_mock_cherri "$BROKEN_JSON_BIN" '{"languageVersion":"2.0"'
reject_mock_doctor "$BROKEN_JSON_BIN" 'malformed JSON'
printf '  [PASS] Scenario 7: malformed schema and malformed JSON responses are rejected\n'

# 8: Alter only the reference fingerprint in an owned exact-pin Git copy.
BAD_DOCS_DIR="$TMP/bad-docs"
cp -R "$HOME1/docs" "$BAD_DOCS_DIR"
sed "s/$MANIFEST_SCHEMA_FP/0000000000000000000000000000000000000000000000000000000000000000/" \
  "$HOME1/docs/fork-language/actions.md" > "$BAD_DOCS_DIR/fork-language/actions.md"
if control_doctor "$HOME1" CHERRI_DOCS_DIR="$BAD_DOCS_DIR" > "$TMP/controlled-docs-rejection.log" 2>&1; then exit 1; fi
grep -Fq '[CHERRI_STALE_DOCS_SCHEMA]' "$TMP/controlled-docs-rejection.log"
printf '  [PASS] Scenario 8: altered documentation fingerprint is rejected\n'

# 9: A deliberately failed transport leaves every previous tool/receipt intact.
HOME9="$TMP/home-s9"
mkdir -p "$HOME9"
cp -R "$HOME1/bin" "$HOME9/bin"
cp -R "$HOME1/docs" "$HOME9/docs"
if (CONTROL_DOWNLOAD_URL=controlled://unavailable; control_setup "$HOME9" --update) > "$TMP/controlled-update-failure.log" 2>&1; then exit 1; fi
grep -Fq '[CONTROLLED_DOWNLOAD_REJECTED]' "$TMP/controlled-update-failure.log"
control_pair_unchanged "$HOME1" "$HOME9"
control_doctor "$HOME9" >/dev/null
[ ! -e "$HOME9/.install-lock" ]
printf '  [PASS] Scenario 9: controlled download failure preserves the real working pair\n'

# 10: An advanced docs branch cannot change the original pin. The compiler
# remains the real supplied immutable artifact; no fake source-build is claimed.
printf '\nAdvanced unselected documentation branch.\n' >> "$MOCK_DOCS_REPO/fork-language/actions.md"
git --no-replace-objects -C "$MOCK_DOCS_REPO" add fork-language/actions.md
git --no-replace-objects -C "$MOCK_DOCS_REPO" -c user.name=Cherri-QA -c user.email=qa@example.invalid commit -qm 'Advance controlled docs branch'
ADVANCED_DOCS_SHA=$(git --no-replace-objects -C "$MOCK_DOCS_REPO" rev-parse HEAD)
[ "$ADVANCED_DOCS_SHA" != "$PINNED_DOCS_SHA" ]
control_setup "$HOME1" --update >/dev/null
control_doctor "$HOME1" >/dev/null
cmp -s "$CONTROL_COMPILER_INPUT" "$HOME1/bin/cherri"
cmp -s "$CONTROL_MIGRATION_INPUT" "$HOME1/bin/cherri-migrate"
[ "$(cat "$HOME1/bin/cherri.commit")" = "$PINNED_COMPILER_SHA" ]
[ "$(cat "$HOME1/bin/cherri-migrate.commit")" = "$PINNED_COMPILER_SHA" ]
[ "$(git --no-replace-objects -C "$HOME1/docs" rev-parse HEAD)" = "$PINNED_DOCS_SHA" ]
printf '  [PASS] Scenario 10: controlled reinstall preserves real tool source and original docs pin\n'

if [ -n "$EVIDENCE_OUT" ]; then
  mkdir -p "$(dirname -- "$EVIDENCE_OUT")"
  # A wrapper smoke run does not itself prove ZIP integrity, real transactional
  # installs, published docs or signing. Dedicated producers own those contracts.
  cat > "$EVIDENCE_OUT" <<EOF
{
  "records": [],
  "observations": {
    "status": "PASS_WRAPPER_SELF_TEST",
    "actual_cli_operations": ["version", "actions", "catalog", "unsigned-build", "decompile", "recompile", "prepare-edit", "reject-AEA1-input"],
    "mock_installation_scenarios": 10,
    "controlled_installation_scope": "real supplied compiler and companion; synthetic download transport and owned minimal docs fixture; no source-build claim",
    "observed_control_compiler_source": "$PINNED_COMPILER_SHA",
    "observed_control_docs_pin": "$PINNED_DOCS_SHA",
    "declared_compiler_pin": "$MANIFEST_COMPILER_COMMIT",
    "declared_docs_pin": "$MANIFEST_DOCS_COMMIT",
    "schema_fingerprint": "$MANIFEST_SCHEMA_FP",
    "codec_fingerprint": "$MANIFEST_CODEC_FP",
    "macro_fingerprint": "$MANIFEST_MACRO_FP",
    "scope": "Wrapper execution and controlled-transport installation checks with actual tools only. Published docs, complete real source-build transactions and full edit/sign contracts are independently verified."
  }
}
EOF
fi

echo 'Cherri Shortcuts skill self-test passed.'
