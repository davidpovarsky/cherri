#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

BASE="${CHERRI_REMOTE_HOME:-$HOME/workspaces/cherri-remote}"
RUNNER="$BASE/bin/cherri-remote-runner.sh"
COMPILER="$BASE/p15-s627c-d16-k18-20261125/runtime/bin/cherri"
VALIDATOR="$BASE/bin/cherri-shortcut-lint.py"
MODE_FILE="$BASE/config/default-language-mode"
EXPECTED_RUNNER="ba1b42e6b93fda4bdc5607129915e6b895efd20baacc0aea48548989618143cd"
EXPECTED_VALIDATOR="51ffca98be2275f4015931e15d73251fff7f7041fb323c9b7aa6973eb57a454f"

fail(){ printf 'LEGACY_BUILDER_ERROR: %s\n' "$*" >&2; exit 1; }
[ -f "$MODE_FILE" ] && [ "$(cat "$MODE_FILE")" = legacy ] || fail "default language mode is not Legacy"
[ "$(sha256sum "$RUNNER" | cut -d' ' -f1)" = "$EXPECTED_RUNNER" ] || fail "runner checksum changed"
[ "$(sha256sum "$VALIDATOR" | cut -d' ' -f1)" = "$EXPECTED_VALIDATOR" ] || fail "preflight checksum changed"
# Canonical release integrity/doctor: retain exact P10R15 compiler/docs/schema tuple.
"$RUNNER" status > /dev/null || fail "canonical P10R15 runner doctor failed"
python3 - "$BASE/active.json" <<'PY'
import json,sys
x=json.load(open(sys.argv[1]))
if x.get('label')!='p15-s627c-d16-k18-20261125' or x.get('sourceBranch')!='agent/cherri-p10r15-final-consumer-pins-20261125':
    raise SystemExit("LEGACY_BUILDER_ERROR: active compiler is not P10R15")
PY
[ -x "$COMPILER" ] || fail "missing verified P10R15 compiler"
action="${1:-status}"; shift || :

if [ "$action" = status ]; then
   printf 'CHERRI_REMOTE_LANGUAGE_MODE=legacy\n'
   printf 'CHERRI_REMOTE_SOURCE_BRANCH=agent/cherri-p10r15-final-consumer-pins-20261125\n'
   python3 - "$BASE/active.json" <<'PY'
import json,sys
x=json.load(open(sys.argv[1]))
for k in ('compilerCommit','docsCommit','schemaFingerprint','sourceBranch'):
    print(k+'='+x[k])
PY
   exit 0
fi

[ "$action" = compile ] || [ "$action" = check ] || fail "use: $0 [status | check FILE.cherri | compile FILE.cherri --unsigned/--signed]"
[ "$#" -ge 1 ] && [ "$#" -le 2 ] || fail "provide source file and optional --unsigned or --signed"
source="$(realpath -- "$1")"; shift
base_real="$(realpath -- "$BASE")" || fail "cannot resolve Remote Builder home"
case "$source" in "$base_real/jobs/"*.cherri) ;; *) fail "source must be .cherri inside jobs" ;; esac
[ -f "$source" ] || fail "source file missing"
sign_mode="${1:---unsigned}"
[ "$sign_mode" = --unsigned ] || [ "$sign_mode" = --signed ] || fail "invalid signing option"
[ "$action" != check ] || [ "$sign_mode" = --unsigned ] || fail "check must be unsigned"

preflight="$(mktemp "$BASE/tmp/legacy-preflight-XXXXXXXX.shortcut")"
trap 'rm -f -- "$preflight"' EXIT
"$COMPILER" "$source" --legacy --load-all-actions --skip-sign "--output=$preflight" --no-ansi
[ -s "$preflight" ] || fail "legacy compiler returned no exact preflight Shortcut"
python3 "$VALIDATOR" "$preflight" || fail "native structural validation failed"

if [ "$action" = check ]; then
  printf 'LEGACY_CHECK=PASS source=%s\n' "$source"
  exit 0
fi
stamp="$(date -u +%Y%m%dT%H%M%S)-$$"
name="$(basename -- "${source%.cherri}")"
out="$BASE/outputs/${name}-p15-s627c-d16-k18-20261125-legacy-${stamp}.shortcut"
if [ "$sign_mode" = --unsigned ]; then
    cp -- "$preflight" "$out"
else
    # Explicit request only. HubSign is external, result must have a real AEA1 envelope.
    "$COMPILER" sign "$preflight" --source-file="$source" --hubsign --share=anyone --output="$out" --no-ansi || fail "signing command failed"
    [ -s "$out" ] && [ "$(head -c4 "$out")" = "AEA1" ] || { rm -f "$out"; fail "signer did not return AEA1"; }
fi
printf 'CHERRI_LANGUAGE=legacy\n'
printf 'CHERRI_BRANCH=agent/cherri-p10r15-final-consumer-pins-20261125\n'
printf 'BUILD_ARTIFACT=%s\n' "$out"
sha256sum "$out"
