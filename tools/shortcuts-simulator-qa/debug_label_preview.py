"""Strict receipt intake for the fixed public label Quick Look observer.

The original exact native callback remains a separate prerequisite. A visible
date suffix is not independent proof of the value of Creation Date, and no
Simulator observation is presented as physical-device acceptance.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re

from debug_label_acceptance import SOURCE_HASHES, MODES, fixture_mode, expected_summary, require

CONTEXT = "editor/debug-label-preview-context.json"
RECEIPT = "editor/debug-label-preview.json"
DIAGNOSTIC = "runtime/debug-label-acceptance.json"
BUDGET = 22000
MAX_SWIPES = 6
CONTEXT_KEYS = {"schemaVersion", "fixtureId", "sourceSha256", "compilerCommitSha", "runId", "runAttempt", "runNonce", "mode"}
RECEIPT_KEYS = {"schemaVersion", "context", "budgetMs", "elapsedMs", "maximumSwipes", "observations", "visibleLabels", "visibleDateSuffixes", "swipeCount", "dismissed", "ownershipLost", "result", "creationDateValueIndependentlyVerified", "physicalDeviceVerified"}
OBSERVATION_KEYS = {"sequence", "startedMs", "observedMs", "foreground", "previewContainer", "doneExists", "doneHittable", "visibleLabels", "visibleDateSuffixes", "action", "actionCompletedMs"}


def read(root, relative, maximum=2_000_000):
    require(not Path(root).is_symlink(), "preview root is not a symlink")
    root = Path(root).resolve()
    path = root / relative
    require(not any((root / Path(*Path(relative).parts[:i])).is_symlink()
                    for i in range(1, len(Path(relative).parts) + 1)), "preview contained regular file")
    require(path.resolve().is_relative_to(root) and path.is_file(), "preview artifact missing")
    raw = path.read_bytes()
    require(len(raw) <= maximum, "preview artifact bound")
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "duplicate preview JSON key")
            result[key] = value
        return result
    return json.loads(raw.decode("utf-8"), object_pairs_hook=unique), raw


def source_entry(compiler, fid, commit):
    require(isinstance(fid, str) and fid in SOURCE_HASHES and isinstance(commit, str)
            and re.fullmatch(r"[0-9a-f]{40}", commit), "preview fixed source identity")
    require(compiler.get("syntheticOnly") is True and compiler.get("compilerCommitSha") == commit,
            "preview compiler identity")
    entries = [row for row in compiler.get("fixtures", []) if row.get("fixtureId") == fid]
    require(len(entries) == 1, "preview fixture identity")
    entry = entries[0]
    require(entry.get("sourceSha256") == SOURCE_HASHES[fid] and entry.get("buildProfile") == "debug"
            and entry.get("runtimeTraceExpectation") == "REQUIRED_NDJSON", "preview source/profile binding")
    require(entry.get("debugLabelSource", {}).get("sourceSha256") == SOURCE_HASHES[fid]
            and entry.get("debugLabelGraph", {}).get("scopedToReviewedSource") is True,
            "preview source and causal graph gates")
    return entry


def validate_context(context, compiler, fid, commit, run_id, nonce, attempt=None):
    require(isinstance(context, dict) and set(context) == CONTEXT_KEYS, "preview context schema")
    source_entry(compiler, fid, commit)
    require(context["schemaVersion"] == "1" and context["fixtureId"] == fid
            and context["sourceSha256"] == SOURCE_HASHES[fid]
            and context["compilerCommitSha"] == commit and context["mode"] == fixture_mode(fid)
            and context["runId"] == str(run_id) and context["runNonce"] == nonce,
            "preview context does not match actual run")
    require(re.fullmatch(r"[1-9][0-9]{0,19}", str(run_id))
            and re.fullmatch(re.escape(fid + "-" + str(run_id) + "-") + r"[0-9a-f]{32}", nonce)
            and type(context["runAttempt"]) is int and 1 <= context["runAttempt"] <= 100,
            "preview fresh run identity")
    if attempt is not None:
        require(type(attempt) is int and context["runAttempt"] == attempt, "preview run attempt mismatch")


def flags(value):
    require(isinstance(value, list) and len(value) == 5
            and all(type(item) is bool for item in value), "five bounded preview flags")
    return value


def validate_receipt(receipt, compiler, fid, commit, run_id, nonce, attempt=None):
    require(isinstance(receipt, dict) and set(receipt) == RECEIPT_KEYS, "preview receipt schema")
    validate_context(receipt["context"], compiler, fid, commit, run_id, nonce, attempt)
    mode = fixture_mode(fid)
    dates, typed, as_text = MODES[mode]
    require(typed or as_text, "unrequested preview observer")
    require(receipt["schemaVersion"] == "1" and type(receipt["budgetMs"]) is int
            and receipt["budgetMs"] == BUDGET and type(receipt["maximumSwipes"]) is int
            and receipt["maximumSwipes"] == MAX_SWIPES, "preview finite policy")
    require(receipt["creationDateValueIndependentlyVerified"] is False
            and receipt["physicalDeviceVerified"] is False, "unproved preview scope")
    elapsed = receipt["elapsedMs"]
    require(type(elapsed) is int and 0 <= elapsed <= 600000, "preview observed elapsed time")
    observations = receipt["observations"]
    require(isinstance(observations, list) and len(observations) <= 16, "preview observation bound")
    labels, suffixes = [False]*5, [False]*5
    owned = lost = dismissed = stopped = False
    swipes = previous_end = 0
    for index, row in enumerate(observations):
        require(isinstance(row, dict) and set(row) == OBSERVATION_KEYS and not stopped,
                "preview observation schema or action after stop")
        for key in ("sequence", "startedMs", "observedMs", "actionCompletedMs"):
            require(type(row[key]) is int, "preview integer timing")
        require(row["sequence"] == index and previous_end <= row["startedMs"] <= row["observedMs"]
                <= row["actionCompletedMs"] <= elapsed, "preview observed timing causality")
        previous_end = row["actionCompletedMs"]
        for key in ("foreground", "previewContainer", "doneExists", "doneHittable"):
            require(type(row[key]) is bool, "preview Boolean observation")
        visible, date_visible = flags(row["visibleLabels"]), flags(row["visibleDateSuffixes"])
        require(all(not date or label for date, label in zip(date_visible, visible)), "date suffix without label")
        require(row["action"] in ("NONE", "SWIPE", "DISMISS"), "preview bounded action")
        ready = all(row[key] for key in ("foreground", "previewContainer", "doneExists", "doneHittable"))
        if row["observedMs"] >= BUDGET:
            require(row["action"] == "NONE", "preview action after deadline")
            stopped = True
        elif not ready:
            require(row["action"] == "NONE", "preview action without current ownership")
            if owned:
                lost = stopped = True
        else:
            owned = True
            labels = [a or b for a, b in zip(labels, visible)]
            suffixes = [a or b for a, b in zip(suffixes, date_visible)]
            complete = all(labels) and (not dates or all(suffixes))
            expected_action = "DISMISS" if complete or swipes >= MAX_SWIPES or BUDGET-row["observedMs"] < 2000 else "SWIPE"
            if row["action"] != "NONE":
                require(row["action"] == expected_action, "preview action policy")
                if row["action"] == "SWIPE":
                    swipes += 1
                    require(swipes <= MAX_SWIPES, "preview swipe bound")
                else:
                    dismissed = stopped = True
        if row["action"] == "NONE":
            require(row["actionCompletedMs"] == row["observedMs"], "invented preview action duration")
    require(flags(receipt["visibleLabels"]) == labels and flags(receipt["visibleDateSuffixes"]) == suffixes,
            "preview summary not backed by observations")
    require(type(receipt["swipeCount"]) is int and receipt["swipeCount"] == swipes
            and type(receipt["dismissed"]) is bool and receipt["dismissed"] == dismissed
            and type(receipt["ownershipLost"]) is bool and receipt["ownershipLost"] == lost,
            "preview state not backed by observations")
    result = ("BLOCKED_PREVIEW_NOT_OBSERVED" if not owned else
              "BLOCKED_PREVIEW_OWNERSHIP_LOST" if lost else
              "BLOCKED_PREVIEW_DEADLINE" if elapsed > BUDGET else
              "BLOCKED_PREVIEW_NOT_DISMISSED" if not dismissed else
              "BLOCKED_PREVIEW_LABELS_INCOMPLETE" if not all(labels) else
              "BLOCKED_PREVIEW_UNEXPECTED_DATES" if not dates and any(suffixes) else
              "BLOCKED_PREVIEW_DATES_INCOMPLETE" if dates and not all(suffixes) else "PASS_VISIBLE_LABELS")
    require(receipt["result"] == result, "preview result contradicts observations")
    return {"result": result, "visibleLabelsVerified": result == "PASS_VISIBLE_LABELS",
            "creationDateValueIndependentlyVerified": False, "physicalDeviceVerified": False}


def prepare(root, fid, commit, run_id, attempt, nonce):
    compiler, _ = read(root, "compiler-manifest.json")
    context = {"schemaVersion": "1", "fixtureId": fid, "sourceSha256": SOURCE_HASHES[fid],
               "compilerCommitSha": commit, "runId": str(run_id), "runAttempt": attempt,
               "runNonce": nonce, "mode": fixture_mode(fid)}
    validate_context(context, compiler, fid, commit, run_id, nonce, attempt)
    if any(MODES[fixture_mode(fid)][1:]):
        with (Path(root)/CONTEXT).open("x", encoding="utf-8", newline="\n") as stream:
            json.dump(context, stream, indent=2); stream.write("\n")


def finalize(root, fid, commit, run_id, attempt=1):
    compiler, _ = read(root, "compiler-manifest.json")
    assertions, _ = read(root, "runtime/output-assertions.json")
    source_entry(compiler, fid, commit)
    require(assertions.get("targetFixtureId") == fid and assertions.get("compilerCommitSha") == commit
            and assertions.get("compiledSourceSha256") == SOURCE_HASHES[fid]
            and assertions.get("expectedSemanticOutput") == expected_summary(fid), "label exact semantic/source identity")
    semantic = assertions.get("runtimeContractVerified") is True
    required = any(MODES[fixture_mode(fid)][1:])
    nonce = assertions.get("expectedRunId", "")
    result = {"result": "NOT_OBSERVED" if required else "NOT_REQUIRED",
              "visibleLabelsVerified": False, "creationDateValueIndependentlyVerified": False,
              "physicalDeviceVerified": False}
    receipt_hash = None
    if (Path(root)/RECEIPT).exists():
        receipt, raw = read(root, RECEIPT, 32768)
        result = validate_receipt(receipt, compiler, fid, commit, run_id, nonce, attempt)
        receipt_hash = hashlib.sha256(raw).hexdigest()
    verified = semantic and (not required or result["visibleLabelsVerified"])
    diagnostic = {"schemaVersion": "1", "fixtureId": fid, "compilerCommitSha": commit,
        "sourceSha256": SOURCE_HASHES[fid], "runId": str(run_id), "runAttempt": attempt, "runNonce": nonce,
        "mode": fixture_mode(fid), "semanticContractVerified": semantic,
        "previewRequired": required, "previewReceiptSha256": receipt_hash,
        "preview": result, "runtimeContractVerified": verified,
        "dateValueComparisonPrecision": "Seconds", "formalAcceptanceGranted": False}
    assertions["labelSemanticContractVerified"] = semantic
    assertions["labelRunAttempt"] = attempt
    assertions["labelPreviewContractVerified"] = not required or result["visibleLabelsVerified"]
    assertions["runtimeContractVerified"] = verified
    for name, value in ((DIAGNOSTIC, diagnostic), ("runtime/output-assertions.json", assertions)):
        path = Path(root)/name
        if name == DIAGNOSTIC:
            stream = path.open("x", encoding="utf-8", newline="\n")
        else:
            stream = path.open("w", encoding="utf-8", newline="\n")
        with stream:
            json.dump(value, stream, indent=2); stream.write("\n")
    return diagnostic


def validate_public_diagnostic(data, root):
    root = Path(root)
    compiler, _ = read(root, "compiler-manifest.json")
    assertions, _ = read(root, "runtime/output-assertions.json")
    fid, commit, run_id = data.get("fixtureId"), data.get("compilerCommitSha"), data.get("runId")
    source_entry(compiler, fid, commit)
    require(set(data) == {"schemaVersion", "fixtureId", "compilerCommitSha", "sourceSha256", "runId", "runAttempt", "runNonce", "mode", "semanticContractVerified", "previewRequired", "previewReceiptSha256", "preview", "runtimeContractVerified", "dateValueComparisonPrecision", "formalAcceptanceGranted"}, "bounded label diagnostic schema")
    require(data["schemaVersion"] == "1" and data["sourceSha256"] == SOURCE_HASHES[fid]
            and data["mode"] == fixture_mode(fid) and data["formalAcceptanceGranted"] is False
            and data["dateValueComparisonPrecision"] == "Seconds", "label diagnostic scope")
    required = any(MODES[fixture_mode(fid)][1:])
    require(type(data["runAttempt"]) is int and 1 <= data["runAttempt"] <= 100
            and data["runAttempt"] == assertions.get("labelRunAttempt"), "label run attempt binding")
    require(data["previewRequired"] is required, "label preview requirement")
    preview = {"result": "NOT_OBSERVED" if required else "NOT_REQUIRED", "visibleLabelsVerified": False,
               "creationDateValueIndependentlyVerified": False, "physicalDeviceVerified": False}
    if (root/RECEIPT).exists():
        receipt, raw = read(root, RECEIPT, 32768)
        preview = validate_receipt(receipt, compiler, fid, commit, run_id, data["runNonce"], data["runAttempt"])
        require(data["previewReceiptSha256"] == hashlib.sha256(raw).hexdigest(), "label receipt digest")
    else:
        require(data["previewReceiptSha256"] is None, "missing label receipt cannot have digest")
    require(data["preview"] == preview and data["runNonce"] == assertions.get("expectedRunId")
            and data["runId"] == str(json.loads((root/"summary.json").read_text(encoding="utf-8"))["runId"])
            and data["fixtureId"] == assertions.get("targetFixtureId")
            and assertions.get("compiledSourceSha256") == SOURCE_HASHES[fid]
            and assertions.get("compilerCommitSha") == commit, "label receipt/actual assertion binding")
    semantic = data["semanticContractVerified"]
    require(type(semantic) is bool and semantic is assertions.get("labelSemanticContractVerified"), "label semantic result")
    verified = semantic and (not required or preview["visibleLabelsVerified"])
    require(data["runtimeContractVerified"] is verified and assertions.get("runtimeContractVerified") is verified
            and assertions.get("labelPreviewContractVerified") is (not required or preview["visibleLabelsVerified"]),
            "label aggregate result")
    return data


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("prepare", "finalize"))
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--fixture", choices=tuple(SOURCE_HASHES), required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--attempt", type=int, default=1)
    parser.add_argument("--nonce", default="")
    args = parser.parse_args()
    if args.command == "prepare":
        prepare(args.root, args.fixture, args.commit, args.run_id, args.attempt, args.nonce)
    else:
        result = finalize(args.root, args.fixture, args.commit, args.run_id, args.attempt)
        print(json.dumps({"fixtureId": args.fixture, "runtimeContractVerified": result["runtimeContractVerified"], "preview": result["preview"]["result"]}))
