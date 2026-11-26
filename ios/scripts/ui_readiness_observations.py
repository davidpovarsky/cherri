"""Retain only closed diagnostics from existing Sign and WebKit predicates.

No row contributes acceptance or changes an observed test result.
"""
import json

PREFIX = "[CHERRI_UI_READINESS] "
REJECTED = "[CHERRI_UI_READINESS_REJECTED] "
SIGN_TEST = "CherriJoinedContractUITests/testBRG25ActualAppRejectedAndLiteralRoutesUI"
FIELD_TEST = "CherriPreviewDocumentUITests/testBRG28ActualWebKitFieldEditAndNumericRejectionUI"
BOUNDARIES = {
    "brg25-type-sign": (SIGN_TEST, "button:signButton", 5000),
    "brg25-malformed-sign": (SIGN_TEST, "button:signButton", 5000),
    "brg28-first-text": (FIELD_TEST, "firstWebView:Text", 8000),
    "brg28-kept-text": (FIELD_TEST, "firstWebView:Text", 8000),
}


def require(value):
    if not value:
        raise ValueError("INVALID_OBSERVATION")


def integer(value, low, high):
    return type(value) is int and low <= value <= high


def unique(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result)
        result[key] = value
    return result


def parse(raw_line, active_test):
    require(len(raw_line.encode("utf-8")) <= 2048 + len(PREFIX))
    line = raw_line.strip()
    require(line.startswith(PREFIX))
    row = json.loads(line[len(PREFIX):], object_pairs_hook=unique)
    require(type(row) is dict)
    boundary = row.get("boundary")
    require(type(boundary) is str and boundary in BOUNDARIES)
    test, query, budget = BOUNDARIES[boundary]
    sign = test == SIGN_TEST
    keys = {"schema", "testId", "boundary", "query", "budgetMilliseconds",
            "elapsedMilliseconds", "observations", "observationsClamped",
            "waitCompleted", "state"}
    require(set(row) == keys | (set() if sign else {"pane"}))
    require(row["schema"] == "cherri-ui-readiness-observation/1" and
            row["testId"] == active_test == test and row["query"] == query)
    require(type(row["budgetMilliseconds"]) is int and row["budgetMilliseconds"] == budget)
    require(integer(row["elapsedMilliseconds"], 0, 120000) and
            integer(row["observations"], 0, 128) and
            type(row["observationsClamped"]) is bool and
            (not row["observationsClamped"] or row["observations"] == 128) and
            type(row["waitCompleted"]) is bool)
    state = row["state"]
    require(type(state) is dict)
    if row["observations"] == 0:
        require(state == {} and not row["waitCompleted"] and not row["observationsClamped"])
    elif sign:
        require(set(state) == {"exists", "hittable", "enabled"} and type(state["exists"]) is bool)
        require(state["hittable"] is None or type(state["hittable"]) is bool)
        require(state["enabled"] is None or type(state["enabled"]) is bool)
        require((state["hittable"] is not None) == state["exists"])
        require((state["enabled"] is not None) == (state["hittable"] is True))
        if row["waitCompleted"]:
            require(all(state[k] is True for k in ("exists", "hittable", "enabled")))
    else:
        require(set(state) == {"webExists", "candidateCount", "matchingCount", "countsClamped"})
        require(type(state["webExists"]) is bool and type(state["countsClamped"]) is bool)
        require(integer(state["candidateCount"], 0, 255) and
                integer(state["matchingCount"], 0, state["candidateCount"]))
        require(not state["countsClamped"] or state["candidateCount"] == 255)
        if not state["webExists"]:
            require(state["candidateCount"] == state["matchingCount"] == 0 and not state["countsClamped"])
        if row["waitCompleted"]:
            require(state["webExists"] and state["matchingCount"] == 1)
    if not sign:
        pane = row["pane"]
        require(type(pane) is dict and set(pane) ==
                {"target", "exists", "hittable", "tapIssued", "selectionObserved"})
        require(pane["target"] == "Preview" and pane["selectionObserved"] is False and
                type(pane["tapIssued"]) is bool)
        require(pane["exists"] is None or type(pane["exists"]) is bool)
        require(pane["hittable"] is None or type(pane["hittable"]) is bool)
        require((pane["hittable"] is not None) == (pane["exists"] is True))
        require(pane["tapIssued"] == (pane["exists"] is True and pane["hittable"] is True))
    return row


class LogGuard:
    def __init__(self):
        self.events = 0
        self.seen = set()

    def observe(self, raw_line, active_test):
        line = raw_line.strip()
        if not line.startswith((PREFIX, REJECTED)):
            return None
        self.events += 1
        if self.events > 8:
            return ""
        try:
            row = parse(raw_line, active_test)
            require(row["boundary"] not in self.seen)
            self.seen.add(row["boundary"])
            return raw_line
        except (ValueError, TypeError, UnicodeError, RecursionError):
            return REJECTED + "INVALID_OBSERVATION"
