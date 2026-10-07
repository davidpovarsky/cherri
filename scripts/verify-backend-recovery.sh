#!/usr/bin/env bash
# scripts/verify-backend-recovery.sh
# Acceptance and recovery verification driver for Cherri Language v2 canonical backend recovery.
set -euo pipefail

if ! command -v go >/dev/null 2>&1; then
  if [ -d "/c/Program Files/Go/bin" ]; then
    export PATH="$PATH:/c/Program Files/Go/bin"
  elif [ -d "/mnt/c/Program Files/Go/bin" ]; then
    export PATH="$PATH:/mnt/c/Program Files/Go/bin"
  fi
fi

PHASE="local"
OUT_DIR=""
EVIDENCE=""
CONTRACT=""
REPAIR_CONTRACT=""
GATES=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --phase)
      PHASE="$2"
      shift 2
      ;;
    --phase=*)
      PHASE="${1#*=}"
      shift 1
      ;;
    --out)
      OUT_DIR="$2"
      shift 2
      ;;
    --out=*)
      OUT_DIR="${1#*=}"
      shift 1
      ;;
    --evidence)
      EVIDENCE="$2"
      shift 2
      ;;
    --evidence=*)
      EVIDENCE="${1#*=}"
      shift 1
      ;;
    --contract)
      CONTRACT="$2"
      shift 2
      ;;
    --contract=*)
      CONTRACT="${1#*=}"
      shift 1
      ;;
    --repair-contract)
      REPAIR_CONTRACT="$2"
      shift 2
      ;;
    --repair-contract=*)
      REPAIR_CONTRACT="${1#*=}"
      shift 1
      ;;
    --gates)
      GATES="$2"
      shift 2
      ;;
    --gates=*)
      GATES="${1#*=}"
      shift 1
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if [ -z "$OUT_DIR" ]; then
  if [ "$PHASE" = "local" ]; then
    OUT_DIR="artifacts/backend-recovery/local"
  else
    OUT_DIR="artifacts/backend-recovery/final"
  fi
fi

mkdir -p "$OUT_DIR"

echo "=== Cherri Language v2 Backend Recovery Verification ==="
echo "Phase:     $PHASE"
echo "Output:    $OUT_DIR"
if [ -n "$EVIDENCE" ]; then
  echo "Evidence:  $EVIDENCE"
fi

EXTRA_ARGS=()
if [ -n "$CONTRACT" ]; then
  EXTRA_ARGS+=("--contract=$CONTRACT")
fi
if [ -n "$REPAIR_CONTRACT" ]; then
  EXTRA_ARGS+=("--repair-contract=$REPAIR_CONTRACT")
fi
if [ -n "$GATES" ]; then
  EXTRA_ARGS+=("--gates=$GATES")
fi

if [ "$PHASE" = "local" ]; then
  echo ""
  echo "--- Step 1: Strict reference comparator unit tests ---"
  go test -v ./internal/shortcutcompare/...

  echo ""
  echo "--- Step 2: Initial backend recovery red-to-green tests ---"
  go test -v -run TestBackendRecoveryInitial .

  echo ""
  echo "--- Step 3: Canonical vs v2 action parity matrix tests ---"
  go test -v -run "TestActionParityMatrix|TestParityMatrix_Explicit12Categories" .

  echo ""
  echo "--- Step 4: Acceptance validator meta tests & contract digests ---"
  go test -v ./tools/language-acceptance/...

  echo ""
  echo "--- Step 5: Execute acceptance runner (phase: local) ---"
  go run ./tools/language-acceptance --phase=local --out="$OUT_DIR" "${EXTRA_ARGS[@]}"

  echo ""
  echo "=== Local verification complete. Output in $OUT_DIR ==="

elif [ "$PHASE" = "final" ]; then
  if [ -z "$EVIDENCE" ]; then
    echo "ERROR: --phase final requires --evidence <manifest-path>" >&2
    exit 1
  fi
  if [ ! -f "$EVIDENCE" ]; then
    echo "ERROR: Evidence manifest not found: $EVIDENCE" >&2
    exit 1
  fi

  echo ""
  echo "--- Step 1: Validate evidence manifest & all contracts (phase: final) ---"
  go run ./tools/language-acceptance --phase=final --evidence="$EVIDENCE" --out="$OUT_DIR" "${EXTRA_ARGS[@]}"

  echo ""
  echo "=== Final verification complete. Output in $OUT_DIR ==="
else
  echo "ERROR: Unsupported phase '$PHASE' (must be 'local' or 'final')" >&2
  exit 1
fi
