#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

BASE="${CHERRI_REMOTE_HOME:-${HOME}/workspaces/cherri-remote}"
REPO='https://github.com/davidpovarsky/cherri.git'
DOCS_REPO='https://github.com/davidpovarsky/cherrilang.org.git'
P10_BRANCH='agent/cherri-p10-apple-actions-coverage'
P9_BRANCH='agent/cherri-v2-product-completion'
export PATH="$BASE/tools/go/bin:$PATH"
mkdir -p "$BASE/source" "$BASE/outputs" "$BASE/jobs" "$BASE/tmp"

say() { printf '%s\n' "$*" >&2; }
die() { say "ERROR: $*"; exit 1; }
prerequisites() {
  for tool in git curl python3 go; do command -v "$tool" >/dev/null 2>&1 || die "Missing: $tool"; done
}

sync_release() {
  case "${1:-}" in
    ""|--require-p10|--require-p15) ;;
    *) die "Unknown immutable-sync option" ;;
  esac
  active_env
  say "IMMUTABLE_RELEASE_VERIFIED $LABEL $COMPILER_COMMIT"
  cat "$BASE/active.json"
}

active_env() {
  [ -f "$BASE/active.json" ] || die 'No verified release: run sync first'
  local values
  values=$(python3 - "$BASE/active.json" <<'PY'
import json,sys
from pathlib import Path
import hashlib,subprocess
x=json.load(open(sys.argv[1]))
expected={"codecFingerprint": "eb550a69bd2e0413369d6b0c0863448c755724b4103c5e4a3617125a72ee3122", "compilerCommit": "627c3e7eb4a54a0364b5a5563db89291fa953b97", "compilerSHA256": "7d7cfb5e44cc667e7f164192090ef21b031f992f61be802a563b2cc75cdbbdd8", "docsCommit": "428f067090bf54f4d2f214c36951689dd599d153", "label": "p15-s627c-d16-k18-20261125", "macroFingerprint": "acb4dd5035ee20786c68a01d323f565e880d2a0b8eedb151145181a4960d455c", "migrationSHA256": "d40086414f33251bae70a2c047e1691709de1b002712123aeb5ef3e1b6bfb3b6", "schemaFingerprint": "dc6548076b8320b15e062439f0d8e7b032e1fd62d032b19b06ab6c79a54890a5", "skillSourceHead": "31102e0ea14be7550181e110c85be1623a68f0d3", "sourceBranch": "agent/cherri-p10r15-final-consumer-pins-20261125"}
if any(x.get(key)!=value for key,value in expected.items()): raise SystemExit('IMMUTABLE_RELEASE_MISMATCH')
base=Path(sys.argv[1]).parent
runtime=base/expected['label']/'runtime'
tool_hashes={"tools/go/VERSION": "756345ba66e44025c7756e491a40b0a2fbbbd9611e462477df64fa38c6ddaba0", "tools/go/bin/go": "aedb1694ac1516bbd353a7c737f31be252f59f17746133d46f9e77f2f5fe9d22", "tools/go/pkg/tool/linux_arm64/asm": "b1e92ca41e0c771514e0b4b260dde22a0d589f44cc363bf1bc3846f686d5271d", "tools/go/pkg/tool/linux_arm64/compile": "5d3ce715273cf3bea2215369d94cbf36126aed4727a0b0d193bdb0ea3eb5f1af", "tools/go/pkg/tool/linux_arm64/link": "3d8da962f016004e0d964d169313f5b43a91eb29078357a3127e8dcc5de8e4ca"}
for path,expected_hash in tool_hashes.items():
    if hashlib.sha256((base/path).read_bytes()).hexdigest()!=expected_hash: raise SystemExit('TOOLCHAIN_DEPENDENCY_MISMATCH')
for repo,revision in [(base/'source'/expected['label'],expected['skillSourceHead']),(runtime/'src',expected['compilerCommit']),(runtime/'docs',expected['docsCommit'])]:
    head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip()
    dirty=subprocess.check_output(['git','status','--porcelain'],cwd=repo,text=True).strip()
    if head!=revision or dirty: raise SystemExit('IMMUTABLE_SOURCE_MISMATCH')
actual=hashlib.sha256((runtime/'bin/cherri').read_bytes()).hexdigest()
if actual!=expected['compilerSHA256']: raise SystemExit('IMMUTABLE_COMPILER_MISMATCH')
actual=hashlib.sha256((runtime/'bin/cherri-migrate').read_bytes()).hexdigest()
if actual!=expected['migrationSHA256']: raise SystemExit('IMMUTABLE_MIGRATION_MISMATCH')
print('|'.join(x[k] for k in ('label','compilerCommit','docsCommit','schemaFingerprint','sourceBranch','skillSourceHead')))
PY
) || die 'Cannot read active release'
  IFS='|' read -r LABEL COMPILER_COMMIT DOCS_COMMIT FP BRANCH SKILL_HEAD <<< "$values"
  SKILL_DIR="$BASE/source/$LABEL/skills/cherri-shortcuts"
  CHERRI_HOME="$BASE/$LABEL/runtime"
  CHERRI_BIN="$CHERRI_HOME/bin/cherri"
  CHERRI_REF="$COMPILER_COMMIT"
  CHERRI_DOCS_REF="$DOCS_COMMIT"
  CHERRI_DOCS_DIR="$CHERRI_HOME/docs"
  export SKILL_DIR CHERRI_HOME CHERRI_BIN CHERRI_REF CHERRI_DOCS_REF CHERRI_DOCS_DIR
  [ -f "$CHERRI_BIN" ] || die 'Verified compiler disappeared'
  sh "$SKILL_DIR/scripts/doctor.sh" >&2 || die 'Runtime is no longer compatible; run sync'
}

safe_source() {
  local input="$1" real
  [ -f "$input" ] || die "Input does not exist: $input"
  real=$(realpath -- "$input")
  local base_real
  base_real=$(realpath -- "$BASE") || die "Cannot resolve Remote Builder home"
  case "$real" in "$base_real/jobs/"*) printf '%s\n' "$real";; *) die "Source must be staged under $BASE/jobs/";; esac
}

usage() {
  cat <<EOF
Usage: $0 sync [--require-p10|--require-p15]
       $0 status
       $0 check /absolute/path/under/jobs/file.cherri
       $0 compile /absolute/path/under/jobs/file.cherri [--unsigned|--signed]
       $0 catalog QUERY
       $0 actions QUERY
       $0 docs QUERY
       $0 test
Inputs must be staged under $BASE/jobs/. Compile defaults to unsigned.
Sync verifies the installed immutable tuple. New releases use a separately validated package.
EOF
}

cmd="${1:-help}"; [ "$#" -eq 0 ] || shift
case "$cmd" in
  sync) sync_release "${1:-}" ;;
  status) active_env; cat "$BASE/active.json" ;;
  check)
    [ "$#" -eq 1 ] || die 'check requires one source path'
    active_env
    src=$(safe_source "$1")
    "$CHERRI_BIN" check "$src" --json
    ;;
  compile)
    [ "$#" -ge 1 ] && [ "$#" -le 2 ] || die 'compile requires source path and optional signing flag'
    src=$(safe_source "$1")
    mode="${2:---unsigned}"
    [ "$mode" = '--unsigned' ] || [ "$mode" = '--signed' ] || die 'Expected --unsigned or --signed'
    active_env
    name=$(basename -- "${src%.cherri}")
    stamp=$(date -u +%Y%m%dT%H%M%S)-$$
    out="$BASE/outputs/${name}-${LABEL}-${stamp}.shortcut"
    # Validate the actual native plist BEFORE releasing or signing it.
    checker="$BASE/bin/cherri-shortcut-lint.py"
    expected_checker='51ffca98be2275f4015931e15d73251fff7f7041fb323c9b7aa6973eb57a454f'
    [ -f "$checker" ] || die "Shortcuts preflight validator is missing"
    [ "$(sha256sum "$checker" | cut -d ' ' -f 1)" = "$expected_checker" ] || die "Preflight validator checksum mismatch"
    preflight="$(mktemp "$BASE/tmp/v2-preflight-XXXXXXXX.shortcut")"
    trap 'rm -f -- "$preflight"' EXIT
    sh "$SKILL_DIR/scripts/build.sh" "$src" "$preflight" --unsigned
    if ! python3 "$checker" "$preflight"; then
      rm -f "$preflight"
      die "Native Shortcut structure is incomplete; refusing to sign or release"
    fi
    if [ "$mode" = '--unsigned' ]; then
      mv -f "$preflight" "$out"
    else
      "$CHERRI_BIN" sign "$preflight" --source-file="$src" --output="$out" --hubsign --share=anyone --no-ansi
    fi
    sha256sum "$out"
    ;;
  catalog|actions|docs)
    [ "$#" -eq 1 ] || die "usage: $cmd SEARCH_QUERY"
    active_env
    if [ "$cmd" = catalog ]; then script='catalog.sh'; elif [ "$cmd" = actions ]; then script='action.sh'; else script='search-docs.sh'; fi
    sh "$SKILL_DIR/scripts/$script" "$1"
    ;;
  test)
    active_env
    ( cd "$CHERRI_HOME/src" && go test -run '^TestCherriNoSign$' -count=1 ./... && go test -run '^TestDecomp$' -count=1 ./... )
    ;;
  help|-h|--help) usage ;;
  *) die "Unknown command: $cmd" ;;
esac
