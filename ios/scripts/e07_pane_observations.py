"""Bounded diagnostic retention for the existing E07 synthetic pane transitions.

This module grants no test result or acceptance tier. It preserves only the
fixed public observations emitted by the protected E07 navigation helper.
"""
import json

PREFIX = "[CHERRI_E07_PANE_TRANSITION] "
REJECTED = "[CHERRI_E07_PANE_REJECTED] INVALID_TRANSITION"
TEST_ID = "CherriEditorUITests/testE07VisualParameterEditingCanvasUI"
TARGET = "CherriUITests.CherriEditorUITests"
METHOD = "testE07VisualParameterEditingCanvasUI"
STAGES = (
    ("initial-preview", "Preview"), ("edited-code", "Code"),
    ("invalid-preview", "Preview"), ("rejected-code", "Code"),
    ("reopened-preview", "Preview"), ("reopened-code", "Code"),
)
BOOLS = frozenset(("targetHittable", "targetSelected", "editorExists",
                  "previewExists", "dualPaneVisible", "tapIssued"))
COUNTS = frozenset(("selectorCount", "codeCount", "previewCount"))
KEYS = BOOLS | COUNTS | frozenset((
    "schema", "testId", "stage", "target", "phase",
    "budgetMilliseconds", "elapsedMilliseconds",
))
PHASES = frozenset(("ready", "not-ready", "selected", "not-selected", "dual-pane"))


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate key")
        result[key] = value
    return result


def parse(line):
    if len(line.encode("utf-8")) > 2048 + len(PREFIX):
        raise ValueError("observation bound")
    row = json.loads(line[len(PREFIX):], object_pairs_hook=unique_object)
    if not isinstance(row, dict) or set(row) != KEYS:
        raise ValueError("observation keys")
    if (row["schema"] != "cherri-e07-pane-transition/1"
            or row["testId"] != TEST_ID or row["phase"] not in PHASES):
        raise ValueError("observation identity")
    if any(type(row[key]) is not bool for key in BOOLS):
        raise ValueError("observation Boolean")
    if any(type(row[key]) is not int or not 0 <= row[key] <= 32 for key in COUNTS):
        raise ValueError("observation count")
    if (type(row["budgetMilliseconds"]) is not int or row["budgetMilliseconds"] != 3000
            or type(row["elapsedMilliseconds"]) is not int
            or not 0 <= row["elapsedMilliseconds"] <= 60000):
        raise ValueError("observation timing")
    return row


def selectable(row):
    return (row["selectorCount"] == row["codeCount"] == row["previewCount"] == 1
            and row["targetHittable"] and not row["dualPaneVisible"])


def dual_pane(row):
    return (row["selectorCount"] == row["codeCount"] == row["previewCount"] == 0
            and row["editorExists"] and row["previewExists"] and row["dualPaneVisible"]
            and not row["targetHittable"] and not row["targetSelected"])


class LogGuard:
    def __init__(self):
        self.active = False
        self.started = False
        self.index = 0
        self.pending = None
        self.events = 0
        self.failed = False
        self.rejection_emitted = False

    def reject(self):
        self.failed = True
        if not self.rejection_emitted:
            self.rejection_emitted = True
            return REJECTED
        return None

    def boundary(self, target, method, event):
        exact = target == TARGET and method == METHOD
        if event == "started.":
            interrupted = self.active and self.events > 0
            self.active = exact and not self.started
            if exact:
                if self.started:
                    return self.reject()
                self.started = True
            if interrupted:
                return self.reject()
        elif self.active:
            self.active = False
            if not exact:
                return self.reject() if self.events else None
            if (event.startswith("passed") and self.events
                    and (self.index != len(STAGES) or self.pending is not None)):
                return self.reject()
        return None

    def observe(self, raw_line, active_test):
        line = raw_line.strip()
        if line.startswith("[CHERRI_E07_PANE_REJECTED]"):
            return self.reject()
        if not line.startswith(PREFIX):
            return None
        self.events += 1
        try:
            if len(raw_line.encode("utf-8")) > 2048 + len(PREFIX):
                raise ValueError("raw observation bound")
            if (self.failed or not self.active or active_test != TEST_ID
                    or self.events > 12 or self.index >= len(STAGES)):
                raise ValueError("observation context")
            row = parse(line)
            if (row["stage"], row["target"]) != STAGES[self.index]:
                raise ValueError("observation order")
            phase = row["phase"]
            if phase in ("ready", "not-ready"):
                if self.pending is not None or row["tapIssued"]:
                    raise ValueError("duplicate ready or early tap")
                if phase == "ready":
                    if not (selectable(row) or dual_pane(row)) or row["elapsedMilliseconds"] > 3000:
                        raise ValueError("unproved readiness")
                    self.pending = row
                else:
                    self.failed = True
            else:
                if self.pending is None or row["elapsedMilliseconds"] < self.pending["elapsedMilliseconds"]:
                    raise ValueError("missing or reversed ready")
                if phase == "dual-pane":
                    if not dual_pane(self.pending) or not dual_pane(row) or row["tapIssued"]:
                        raise ValueError("unproved dual pane")
                else:
                    if not selectable(self.pending) or not row["tapIssued"]:
                        raise ValueError("missing unique tap")
                    if phase == "selected" and not (selectable(row) and row["targetSelected"]):
                        raise ValueError("unproved selected target")
                if phase != "not-selected" and row["elapsedMilliseconds"] > 3000:
                    raise ValueError("success after deadline")
                self.pending = None
                self.index += 1
                if phase == "not-selected":
                    self.failed = True
            # Exact accepted bytes are retained. Arbitrary JSON fields, strings,
            # raw AX and source text cannot pass the closed schema above.
            return raw_line
        except (ValueError, TypeError, KeyError, UnicodeError, RecursionError):
            return self.reject()
