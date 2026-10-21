#!/usr/bin/env bash
set -Eeuo pipefail
umask 077
ROOT="$HOME/workspaces/cherri-remote/pure-upstream"
PACK="$ROOT/apple-actions-pack"
PURE="$ROOT/bin/pure-cherri-service.sh"
JOBS="$ROOT/jobs"
OUT="$ROOT/outputs"
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
check_pack() {
  "$PURE" verify >/dev/null
  [ -f "$PACK/checksums.sha256" ] || die "Missing integrity manifest"
  (cd "$PACK" && sha256sum -c checksums.sha256 >/dev/null) || die "Pack checksum failure"
  [ "$(find "$PACK/modules" -maxdepth 1 -type f -name '*.cherri' | wc -l)" -eq 20 ] || die "Unexpected module count"
}
status() {
  check_pack
  echo "ORIGINAL_COMPILER=VERIFIED"
  echo "UPSTREAM_SHA=bcc0095754e2cc0baaeb4d93dae172b8153b4716"
  echo "PACK=VERIFIED"
  echo "PACK_VERSION=0.1.1"
  echo "PACK_DIR=$PACK"
  echo "MODULE_COUNT=20"
  echo "ORIGINAL_SERVICE=$PURE"
  echo "OUTPUT_DIR=$OUT"
}
enhanced() {
  local action="$1" requested="$2" input srcdir stage name f idx
  [ -n "$requested" ] || die "Source argument required"
  check_pack
  input=$(realpath -e -- "$requested") || die "Source does not exist"
  case "$input" in "$JOBS/"*) ;; *) die "Source must be under $JOBS";; esac
  case "$input" in *.cherri) ;; *) die "Source must be .cherri";; esac
  srcdir=$(dirname "$input")
  name=$(basename "$input")
  stage=$(mktemp -d "$JOBS/actions-pack-run.XXXXXXXX")
  trap "rm -rf -- '$stage'" EXIT
  if [ "$srcdir" = "$JOBS" ]; then
    cp -f -- "$input" "$stage/$name"
  else
    cp -R -- "$srcdir/." "$stage/"
  fi
  [ ! -e "$stage/apple-actions-pack-index.cherri" ] || die "Reserved index filename already exists in source"
  for f in "$PACK"/modules/*.cherri; do
    idx=$(basename "$f")
    [ ! -e "$stage/$idx" ] || die "Pack module conflicts with user's file $idx"
    cp -f "$f" "$stage/$idx"
  done
  idx="$stage/apple-actions-pack-index.cherri"
  : > "$idx"
  # Complete the upstream original standard-library imports not covered by pack modules.
  for category in a11y dropbox intelligence storage translation; do
    printf "#include 'actions/%s'\n" "$category" >> "$idx"
  done
  for f in "$PACK"/modules/*.cherri; do
    printf "#include '%s'\n" "$(basename "$f")" >> "$idx"
  done
  # Remove duplicate core action includes from the temporary staging copy only.
  # Every opt-in module already loads its original category, so duplicate includes
  # would otherwise fail compilation in unmodified upstream Cherri.
  python3 - "$stage/$name" <<'PYSOURCE'
import pathlib, re, sys
p=pathlib.Path(sys.argv[1])
t=p.read_text(encoding='utf8')
categories='basic calendar contacts crypto device documents images location mac math media music network pdf photos settings sharing shortcuts text web'
for category in categories.split():
    pat=r"(?m)^[ \\t]*#include[ \\t]+['\\\"]actions/"+re.escape(category)+r"['\\\"][ \\t]*(?=\\n|$)"
    t=re.sub(pat,'// Included by Apple Actions Pack: '+category,t)
p.write_text(t,encoding='utf8')
PYSOURCE
  if ! grep -q 'apple-actions-pack-index.cherri' "$stage/$name"; then
    {
      printf "#include 'apple-actions-pack-index.cherri'\n"
      cat "$stage/$name"
    } > "$stage/packed-source.cherri"
    mv -f "$stage/packed-source.cherri" "$stage/$name"
  fi
  echo "COMPILER=PURE_UPSTREAM"
  echo "MODE=PURE_UPSTREAM_WITH_EXTERNAL_PACK"
  echo "ACTION_PACK_VERSION=0.1.1"
  "$PURE" "$action" "$stage/$name"
}
mode=help
if [ "$#" -gt 0 ]; then mode="$1"; shift; fi
case "$mode" in
  status) status;;
  verify) check_pack; echo "PURE_UPSTREAM_WITH_PACK=VERIFIED";;
  docs) "$PURE" docs "$@";;
  original|pure)
    [ "$#" -ge 2 ] || die "Usage: original compile|sign source.cherri"
    step="$1"; shift
    case "$step" in compile|sign) "$PURE" "$step" "$1";; *) die "Unsupported original command";; esac
    ;;
  enhanced|pack)
    [ "$#" -ge 2 ] || die "Usage: enhanced compile|sign source.cherri"
    step="$1"; shift
    case "$step" in compile|sign) enhanced "$step" "$1";; *) die "Unsupported enhanced command";; esac
    ;;
  help|-h|--help)
    cat <<'USAGE'
Two isolation-preserving modes:
 cherri-with-actions.sh status
 cherri-with-actions.sh original compile /path/under/pure-upstream/jobs/example.cherri
 cherri-with-actions.sh enhanced compile /path/under/pure-upstream/jobs/example.cherri
 cherri-with-actions.sh original sign /path/under/pure-upstream/jobs/example.cherri
 cherri-with-actions.sh enhanced sign /path/under/pure-upstream/jobs/example.cherri
 cherri-with-actions.sh docs [keyword]
Enhanced mode makes an ephemeral copy of the source and prepends an external
action-pack index. The PURE original compiler and original documentation are unchanged.
Signing explicitly uploads the Shortcut to RoutineHub's HubSign service.
USAGE
    ;;
  *) die "Unknown mode $mode";;
esac
