"""Bounded engineering observation for the original v2 function sender.

This accepts only the public three-case semantic block. It does not consume a
callback, trace, signed shortcut, run receipt or launch authority. Its no-input
goals are hypotheses to test with an independently authenticated no-input run.
Parsing never grants RP26/BRG15 acceptance or Apple execution.
"""
import base64
import binascii
import re

FIXTURE = "qa-function-envelope-presence-v2"
PREFIX = "CHERRI_FUNCTION_ENVELOPE_PRESENCE_V1:"
CASES = ("probeZero", "probeDefault", "probeOptional")
FIELDS = (
    "CASE", "KEY_COUNT", "CALLER_KEY", "ARGUMENTS_KEY", "ARG_FLAG_KEY",
    "ARG_VALUE_KEY", "ARG_FLAG", "CALLER_FIELD_COUNT", "CALLER_SAVED_COUNT",
    "CALLER_FIELD_TYPE64", "CALLER_SAVED_TYPE64", "PARAMETER",
)
FLAGS = ("CALLER_KEY", "ARGUMENTS_KEY", "ARG_FLAG_KEY", "ARG_VALUE_KEY")
GOALS = {
    "probeZero": {
        "KEY_COUNT": "5", "CALLER_KEY": "1", "ARGUMENTS_KEY": "1",
        "ARG_FLAG_KEY": "0", "ARG_VALUE_KEY": "0", "ARG_FLAG": "UNOBSERVED",
        "CALLER_FIELD_COUNT": "0", "CALLER_SAVED_COUNT": "0",
        "CALLER_FIELD_TYPE64": "UNOBSERVED", "CALLER_SAVED_TYPE64": "UNOBSERVED",
        "PARAMETER": "NO_PARAMETER",
    },
    "probeDefault": {
        "KEY_COUNT": "6", "CALLER_KEY": "1", "ARGUMENTS_KEY": "1",
        "ARG_FLAG_KEY": "1", "ARG_VALUE_KEY": "0", "ARG_FLAG": "0",
        "CALLER_FIELD_COUNT": "0", "CALLER_SAVED_COUNT": "0",
        "CALLER_FIELD_TYPE64": "UNOBSERVED", "CALLER_SAVED_TYPE64": "UNOBSERVED",
        "PARAMETER": "REVGQVVMVA==",
    },
    "probeOptional": {
        "KEY_COUNT": "7", "CALLER_KEY": "1", "ARGUMENTS_KEY": "1",
        "ARG_FLAG_KEY": "1", "ARG_VALUE_KEY": "1", "ARG_FLAG": "1",
        "CALLER_FIELD_COUNT": "0", "CALLER_SAVED_COUNT": "0",
        "CALLER_FIELD_TYPE64": "UNOBSERVED", "CALLER_SAVED_TYPE64": "UNOBSERVED",
        "PARAMETER": "NONE",
    },
}


def require(ok, code):
    if not ok:
        raise ValueError("FUNCTION_ENVELOPE_" + code)


def count(value, field):
    require(re.fullmatch(r"0|[1-9][0-9]{0,3}", value) is not None, "COUNT_" + field)
    return int(value)


def decode64(value, field, *, witness=False):
    limit = 128 if witness else 256
    require(len(value) <= ((limit + 2) // 3) * 4, "BASE64_SIZE_" + field)
    try:
        raw = base64.b64decode(value, validate=True)
        result = raw.decode("utf-8", errors="strict")
    except (ValueError, binascii.Error, UnicodeError) as exc:
        raise ValueError("FUNCTION_ENVELOPE_BASE64_ENCODING_" + field) from exc
    require(len(raw) <= limit and base64.b64encode(raw).decode("ascii") == value,
            "BASE64_CANONICAL_" + field)
    if witness:
        require(bool(result) and result != "UNOBSERVED", "TYPE_WITNESS_" + field)
    return result


def validate_summary(summary):
    require(isinstance(summary, str) and summary.isascii()
            and len(summary) <= 4096 and "\r" not in summary, "SUMMARY_BYTES")
    lines = summary.split("\n")
    require(len(lines) == 4 and lines[0] == PREFIX, "SUMMARY_LINES")
    observations, all_goals = {}, {}
    for expected_case, line in zip(CASES, lines[1:]):
        parts = line.split(";")
        require(len(parts) == len(FIELDS), "FIELD_COUNT")
        fields = {}
        for field, part in zip(FIELDS, parts):
            require(part.startswith(field + "="), "FIELD_ORDER")
            fields[field] = part[len(field) + 1:]
        require(fields["CASE"] == expected_case, "CASE_ORDER")
        count(fields["KEY_COUNT"], "KEY_COUNT")
        for field in FLAGS:
            require(fields[field] in ("0", "1"), "FLAG_" + field)

        # An absent key deliberately remains a valid negative observation.
        # The fixed source does not read its value or type in that branch.
        decoded = {}
        if fields["CALLER_KEY"] == "0":
            require(fields["CALLER_FIELD_COUNT"] == "UNOBSERVED"
                    and fields["CALLER_FIELD_TYPE64"] == "UNOBSERVED",
                    "ABSENT_CALLER_NOT_READ")
        else:
            raw_count = count(fields["CALLER_FIELD_COUNT"], "CALLER_FIELD_COUNT")
            if raw_count == 0:
                require(fields["CALLER_FIELD_TYPE64"] == "UNOBSERVED",
                        "EMPTY_CALLER_NOT_TYPED")
            else:
                decoded["CALLER_FIELD_TYPE64"] = decode64(
                    fields["CALLER_FIELD_TYPE64"], "CALLER_FIELD_TYPE64", witness=True)

        saved_count = count(fields["CALLER_SAVED_COUNT"], "CALLER_SAVED_COUNT")
        if saved_count == 0:
            require(fields["CALLER_SAVED_TYPE64"] == "UNOBSERVED",
                    "EMPTY_SAVED_NOT_TYPED")
        else:
            decoded["CALLER_SAVED_TYPE64"] = decode64(
                fields["CALLER_SAVED_TYPE64"], "CALLER_SAVED_TYPE64", witness=True)

        if fields["ARG_FLAG_KEY"] == "0":
            require(fields["ARG_FLAG"] == "UNOBSERVED", "ABSENT_FLAG_NOT_READ")
        else:
            # Preserve an observed empty/wrong literal as a negative result.
            # Do not coerce it into an absent flag or a known valid 0/1.
            require(len(fields["ARG_FLAG"]) <= 64
                    and all(32 <= ord(ch) <= 126 and ch != ";"
                            for ch in fields["ARG_FLAG"]), "ARG_FLAG_BYTES")

        if expected_case == "probeZero":
            require(fields["PARAMETER"] == "NO_PARAMETER", "ZERO_PARAMETER")
        elif expected_case == "probeOptional" and fields["PARAMETER"] == "NONE":
            pass
        else:
            decoded["PARAMETER"] = decode64(fields["PARAMETER"], "PARAMETER")

        goals = {key: fields[key] == expected
                 for key, expected in GOALS[expected_case].items()}
        # Equality is only a type observation; it does not prove value identity
        # or the external launch input. No missing witness can compare equal.
        type_fields = ("CALLER_FIELD_TYPE64", "CALLER_SAVED_TYPE64")
        witnessed = all(field in decoded for field in type_fields)
        observations[expected_case] = {
            "actualFields": fields, "decodedBase64": decoded,
            "noInputGoalChecks": goals,
            "observedCallerTypesBothPresent": witnessed,
            "observedCallerTypesEqual": witnessed
            and decoded[type_fields[0]] == decoded[type_fields[1]],
        }
        all_goals[expected_case] = all(goals.values())
    return {
        "status": "ENGINEERING_OBSERVATION_PARSED", "fixtureId": FIXTURE,
        "caseObservations": observations, "caseNoInputGoalsMatched": all_goals,
        "declaredNoInputGoalsMatched": all(all_goals.values()),
        "requiredLaunchMode": "no-input-first-and-only-semantic-invocation",
        "launchInputAuthenticated": False, "sourceAuthenticated": False,
        "appleExecutionAuthenticated": False, "formalAcceptanceGranted": False,
    }
