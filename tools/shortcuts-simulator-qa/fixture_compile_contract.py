"""Explicit compiler-only outcomes for preserved, unsupported native probes."""
import json
import os

LIST_RESULT_MESSAGE = (
    "List-valued loop results are unsupported: native Repeat Results do not "
    "preserve nested List cardinality"
)
EXPECTED_REJECTIONS = {
    "qa-f04-yield-list-v2": '[item, "TAIL"]',
    "qa-f04-yield-empty-list-v2": "[]",
}


def verify_expected_rejection(fixture_id, source, compiled, checked, output_paths):
    """Require an actual exact diagnostic and no emitted/stale native artifact."""
    expression = EXPECTED_REJECTIONS[fixture_id]
    start = source.rindex(expression)
    prefix = source[:start]
    line = prefix.count("\n")
    column = len(prefix.rsplit("\n", 1)[-1].encode("utf-16-le")) // 2
    wanted_range = {
        "start": {"line": line, "character": column},
        "end": {"line": line, "character": column + len(expression.encode("utf-16-le")) // 2},
    }
    result = json.loads(checked.stdout)
    errors = [item for item in result.get("diagnostics", []) if item.get("severity") == "ERROR"]
    if (compiled.returncode == 0 or checked.returncode == 0 or result.get("valid") is not False
            or len(errors) != 1 or errors[0].get("code") != "E_TARGET_VALUE_SHAPE"
            or errors[0].get("message") != LIST_RESULT_MESSAGE
            or errors[0].get("range") != wanted_range
            or "E_TARGET_VALUE_SHAPE" not in compiled.stdout + compiled.stderr
            or any(os.path.lexists(path) for path in output_paths)):
        raise RuntimeError("EXPECTED_FIXTURE_REJECTION_MISMATCH: " + fixture_id)
    return {
        "fixtureId": fixture_id,
        "status": "BLOCKED_PLATFORM",
        "compileOutcome": "REJECTED_UNSUPPORTED_VALUE_SHAPE",
        "compileExitCode": compiled.returncode,
        "checkExitCode": checked.returncode,
        "diagnostic": errors[0],
        "nativeArtifactEmitted": False,
        "appleRuntime": "NOT_RUN",
    }
