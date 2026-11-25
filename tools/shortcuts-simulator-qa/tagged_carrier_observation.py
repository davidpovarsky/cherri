"""Engineering parser for one public carrier observation.

The returned booleans describe only the supplied semantic line. An independent
caller must authenticate source, unsigned/signed actions, callback and traces.
This module never grants Apple execution or formal acceptance.
"""
import base64
import binascii
import re

FIXTURE = "qa-tagged-value-carrier-v2"
PREFIX = "CHERRI_TAGGED_VALUE_CARRIER_V1:"
FIELDS = (
    "OUTER_COUNT", "FIRST_PRESENT", "FIRST_COUNT", "FIRST_FIRST64",
    "FIRST_LAST64", "EMPTY_PRESENT", "EMPTY_COUNT", "ORIGINAL_LIST_TYPE64",
    "DECODED_LIST_TYPE64", "TEXT_TYPE64", "MAP_OUTER_COUNT", "MAP_PRESENT",
    "MAP_ABSENT", "ORIGINAL_MAP_TYPE64", "DECODED_MAP_TYPE64",
    "MAP_KEYS", "MAP_ALPHA64", "MAP_BETA64",
)
GOALS = {
    "OUTER_COUNT": "2", "FIRST_PRESENT": "Y", "FIRST_COUNT": "2",
    "FIRST_FIRST64": "MTE=", "FIRST_LAST64": "MjI=", "EMPTY_PRESENT": "Y",
    "EMPTY_COUNT": "0", "MAP_OUTER_COUNT": "2", "MAP_PRESENT": "Y",
    "MAP_ABSENT": "Y", "MAP_KEYS": "2", "MAP_ALPHA64": "TEVGVA==",
    "MAP_BETA64": "UklHSFQ=",
}
FLAGS = {"FIRST_PRESENT", "EMPTY_PRESENT", "MAP_PRESENT", "MAP_ABSENT"}
OPTIONAL_COUNTS = {"FIRST_COUNT", "EMPTY_COUNT", "MAP_KEYS"}
WITNESSES = {"ORIGINAL_LIST_TYPE64", "ORIGINAL_MAP_TYPE64", "TEXT_TYPE64"}
TYPE_WITNESSES = (
    "ORIGINAL_LIST_TYPE64", "DECODED_LIST_TYPE64", "TEXT_TYPE64",
    "ORIGINAL_MAP_TYPE64", "DECODED_MAP_TYPE64",
)


def require(ok, code):
    if not ok:
        raise ValueError("TAGGED_CARRIER_" + code)


def decode64(value, key):
    limit = 128 if key.endswith("TYPE64") else 256
    require(isinstance(value, str) and len(value) <= ((limit + 2) // 3) * 4,
            "BASE64_SIZE_" + key)
    try:
        raw = base64.b64decode(value, validate=True)
        result = raw.decode("utf-8", errors="strict")
    except (ValueError, binascii.Error, UnicodeError) as exc:
        raise ValueError("TAGGED_CARRIER_BASE64_ENCODING_" + key) from exc
    require(len(raw) <= limit and base64.b64encode(raw).decode("ascii") == value,
            "BASE64_CANONICAL_" + key)
    if key.endswith("TYPE64"):
        require(bool(result), "EMPTY_TYPE_" + key)
    return result


def validate_summary(summary):
    require(isinstance(summary, str) and summary.isascii()
            and len(summary.encode("utf-8")) <= 8192
            and "\n" not in summary and "\r" not in summary, "SUMMARY_BYTES")
    require(summary.startswith(PREFIX), "SUMMARY_PREFIX")
    parts = summary[len(PREFIX):].split(":")
    require(len(parts) == len(FIELDS), "SUMMARY_FIELD_COUNT")
    fields, decoded = {}, {}
    for part, key in zip(parts, FIELDS):
        require(part.startswith(key + "="), "SUMMARY_FIELD_ORDER")
        value = part[len(key) + 1:]
        fields[key] = value
        if key in FLAGS:
            require(value in ("N", "Y"), "PRESENCE_FLAG_" + key)
        elif key.endswith("64"):
            if value == "UNOBSERVED":
                require(key not in WITNESSES, "MISSING_WITNESS_" + key)
            else:
                decoded[key] = decode64(value, key)
        else:
            require(re.fullmatch(r"0|[1-9][0-9]{0,3}", value) is not None
                    or key in OPTIONAL_COUNTS and value == "-1",
                    "COUNT_" + key)

    first_fields = ("DECODED_LIST_TYPE64", "FIRST_FIRST64", "FIRST_LAST64")
    if fields["FIRST_PRESENT"] == "N":
        require(fields["FIRST_COUNT"] == "-1"
                and all(fields[k] == "UNOBSERVED" for k in first_fields),
                "ABSENT_FIRST")
    else:
        require(int(fields["OUTER_COUNT"]) >= 1
                and int(fields["FIRST_COUNT"]) >= 0
                and fields["DECODED_LIST_TYPE64"] != "UNOBSERVED", "PRESENT_FIRST")
        if fields["FIRST_COUNT"] == "0":
            require(fields["FIRST_FIRST64"] == fields["FIRST_LAST64"] == "UNOBSERVED",
                    "EMPTY_FIRST")
        else:
            require(fields["FIRST_FIRST64"] != "UNOBSERVED"
                    and fields["FIRST_LAST64"] != "UNOBSERVED", "NONEMPTY_FIRST")

    if fields["EMPTY_PRESENT"] == "N":
        require(fields["EMPTY_COUNT"] == "-1", "ABSENT_EMPTY")
    else:
        require(int(fields["OUTER_COUNT"]) >= 2
                and int(fields["EMPTY_COUNT"]) >= 0, "PRESENT_EMPTY")

    map_values = ("MAP_ALPHA64", "MAP_BETA64")
    if fields["MAP_PRESENT"] == "N":
        require(fields["DECODED_MAP_TYPE64"] == "UNOBSERVED"
                and fields["MAP_KEYS"] == "-1"
                and all(fields[k] == "UNOBSERVED" for k in map_values), "ABSENT_MAP")
    else:
        require(int(fields["MAP_OUTER_COUNT"]) >= 1
                and fields["DECODED_MAP_TYPE64"] != "UNOBSERVED", "PRESENT_MAP")
        same_type = fields["DECODED_MAP_TYPE64"] == fields["ORIGINAL_MAP_TYPE64"]
        if same_type:
            require(int(fields["MAP_KEYS"]) >= 0
                    and all(fields[k] != "UNOBSERVED" for k in map_values),
                    "OBSERVED_MAP_VALUES")
        else:
            require(fields["MAP_KEYS"] == "-1"
                    and all(fields[k] == "UNOBSERVED" for k in map_values),
                    "WRONG_MAP_TYPE_GUARD")

    goals = {key: fields[key] == expected for key, expected in GOALS.items()}
    # An absent-cell diagnostic can be retained without treating its sentinel
    # as a native type witness. Every relation requires all five real witnesses.
    complete = all(key in decoded and bool(decoded[key]) for key in TYPE_WITNESSES)
    relations = {
        "listMatchesOriginal": complete and decoded["DECODED_LIST_TYPE64"] == decoded["ORIGINAL_LIST_TYPE64"],
        "listWitnessDistinctFromText": complete and decoded["ORIGINAL_LIST_TYPE64"] != decoded["TEXT_TYPE64"],
        "mapMatchesOriginal": complete and decoded["DECODED_MAP_TYPE64"] == decoded["ORIGINAL_MAP_TYPE64"],
        "mapWitnessDistinctFromText": complete and decoded["ORIGINAL_MAP_TYPE64"] != decoded["TEXT_TYPE64"],
    }
    return {
        "status": "ENGINEERING_OBSERVATION_PARSED", "fixtureId": FIXTURE,
        "actualFields": fields, "decodedBase64": decoded,
        "goalChecks": goals, "typeRelations": relations, "typeWitnessesComplete": complete,
        "semanticGoalsMatched": all(goals.values()) and all(relations.values()),
        "sourceAuthenticated": False, "appleExecutionAuthenticated": False,
        "formalAcceptanceGranted": False,
    }
