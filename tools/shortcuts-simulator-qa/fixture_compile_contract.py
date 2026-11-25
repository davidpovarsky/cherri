"""Exact compiler outcomes for preserved positive and private-boundary probes."""
import hashlib
import json
import os

LIST_RESULT_MESSAGE = (
    "List-valued loop results are unsupported: native Repeat Results do not "
    "preserve nested List cardinality"
)
# These two new negative sources exercise private-carrier escape. The original
# nested/empty loop probes below must compile; their source bytes remain pinned.
EXPECTED_REJECTIONS = {
    "qa-f04-private-output-v2": "[]",
    "qa-f04-private-interpolation-v2": "[]",
}
PRESERVED_F04_COMPILATIONS = {
    "qa-f04-yield-list-v2": "c0719b046c2ed724fccef9e5b09e313c7dd72c02dc2c72ce1abe8aab1592e091",
    "qa-f04-yield-empty-list-v2": "d1322f6c7c663d266b0e9a7e272a92ae331be0e411818dae0c7d5172587578c3",
}


def verify_preserved_compilation_source(fixture_id, source):
    """Pin original source only; the caller must actually compile its artifact."""
    digest = PRESERVED_F04_COMPILATIONS.get(fixture_id)
    if digest is None:
        return None
    if not isinstance(source, bytes) or hashlib.sha256(source).hexdigest() != digest:
        raise RuntimeError("F04_PRESERVED_SOURCE_MISMATCH: " + fixture_id)
    return {"sourceSha256": digest, "compilerOutcome": "COMPILE_REQUIRED",
            "appleRuntime": "NOT_RUN", "formalF04Acceptance": False}


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
