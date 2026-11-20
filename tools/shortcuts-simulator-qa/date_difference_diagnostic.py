"""Bind a public Date difference diagnostic to source, independent target and causal graph.

This check never fabricates a native result or normalizes empty output to zero.
Only the existing exact callback oracle evaluates the actual Apple run.
"""
import copy
import hashlib
import json
from pathlib import Path
from debug_label_acceptance import Graph, params, kind, token, literal, text_value

FIXTURE_ID = "qa-date-difference-diagnostic-v2"
SAMPLE_ID = "qa-date-difference-diagnostic"
SOURCE_PATH = "tools/shortcuts-simulator-qa/fixtures/" + FIXTURE_ID + ".cherri"
SOURCE_SHA256 = '91e84ce74eae0adbbbbe2c88efb560de812289190b0de44644620dee5094c2ba'
CONTRACT_SHA256 = '5f96a34fef09a25a3a6261bbbd2556703a61dc41a25992202c80b88dee7e17d2'
PREFIX = 'CHERRI_QA_DATE_DIFFERENCE_DIAG'
DATES = [['START', 'startDate', '2001-02-03T04:05:06Z'],
 ['SAME', 'sameDate', '2001-02-03T04:05:06Z'],
 ['LATER', 'laterDate', '2001-02-03T04:06:06Z']]
INTERVALS = [['ZERO_SECONDS', 'zeroSeconds', 'startDate', 'sameDate', 'Seconds', '0'],
 ['POSITIVE_SECONDS', 'positiveSeconds', 'startDate', 'laterDate', 'Seconds', '60'],
 ['ZERO_MINUTES', 'zeroMinutes', 'startDate', 'sameDate', 'Minutes', '0'],
 ['POSITIVE_MINUTES', 'positiveMinutes', 'startDate', 'laterDate', 'Minutes', '1'],
 ['SAME_PRODUCER_SECONDS', 'sameProducerSeconds', 'startDate', 'startDate', 'Seconds', '0']]
RESULTS = [['ZERO_SECONDS', 'zeroSeconds', '0'],
 ['POSITIVE_SECONDS', 'positiveSeconds', '60'],
 ['ZERO_MINUTES', 'zeroMinutes', '0'],
 ['POSITIVE_MINUTES', 'positiveMinutes', '1'],
 ['SAME_PRODUCER_SECONDS', 'sameProducerSeconds', '0'],
 ['NUMERIC_ZERO', 'numericZero', '0']]
FIELDS = [['START_TYPE64', 'startDateType64', 'RGF0ZQ=='],
 ['START_COUNT', 'startDateCount', '1'],
 ['START_FORMAT64', 'startDateFormat64', 'MjAwMS0wMi0wM1QwNDowNTowNlo='],
 ['SAME_TYPE64', 'sameDateType64', 'RGF0ZQ=='],
 ['SAME_COUNT', 'sameDateCount', '1'],
 ['SAME_FORMAT64', 'sameDateFormat64', 'MjAwMS0wMi0wM1QwNDowNTowNlo='],
 ['LATER_TYPE64', 'laterDateType64', 'RGF0ZQ=='],
 ['LATER_COUNT', 'laterDateCount', '1'],
 ['LATER_FORMAT64', 'laterDateFormat64', 'MjAwMS0wMi0wM1QwNDowNjowNlo='],
 ['ZERO_SECONDS_TYPE64', 'zeroSecondsType64', 'TnVtYmVy'],
 ['ZERO_SECONDS_COUNT', 'zeroSecondsCount', '1'],
 ['ZERO_SECONDS_TEXT64', 'zeroSecondsText64', 'MA=='],
 ['POSITIVE_SECONDS_TYPE64', 'positiveSecondsType64', 'TnVtYmVy'],
 ['POSITIVE_SECONDS_COUNT', 'positiveSecondsCount', '1'],
 ['POSITIVE_SECONDS_TEXT64', 'positiveSecondsText64', 'NjA='],
 ['ZERO_MINUTES_TYPE64', 'zeroMinutesType64', 'TnVtYmVy'],
 ['ZERO_MINUTES_COUNT', 'zeroMinutesCount', '1'],
 ['ZERO_MINUTES_TEXT64', 'zeroMinutesText64', 'MA=='],
 ['POSITIVE_MINUTES_TYPE64', 'positiveMinutesType64', 'TnVtYmVy'],
 ['POSITIVE_MINUTES_COUNT', 'positiveMinutesCount', '1'],
 ['POSITIVE_MINUTES_TEXT64', 'positiveMinutesText64', 'MQ=='],
 ['SAME_PRODUCER_SECONDS_TYPE64', 'sameProducerSecondsType64', 'TnVtYmVy'],
 ['SAME_PRODUCER_SECONDS_COUNT', 'sameProducerSecondsCount', '1'],
 ['SAME_PRODUCER_SECONDS_TEXT64', 'sameProducerSecondsText64', 'MA=='],
 ['NUMERIC_ZERO_TYPE64', 'numericZeroType64', 'TnVtYmVy'],
 ['NUMERIC_ZERO_COUNT', 'numericZeroCount', '1'],
 ['NUMERIC_ZERO_TEXT64', 'numericZeroText64', 'MA==']]
EXPECTED_OUTPUT = 'CHERRI_QA_DATE_DIFFERENCE_DIAG:START_TYPE64=RGF0ZQ==:START_COUNT=1:START_FORMAT64=MjAwMS0wMi0wM1QwNDowNTowNlo=:SAME_TYPE64=RGF0ZQ==:SAME_COUNT=1:SAME_FORMAT64=MjAwMS0wMi0wM1QwNDowNTowNlo=:LATER_TYPE64=RGF0ZQ==:LATER_COUNT=1:LATER_FORMAT64=MjAwMS0wMi0wM1QwNDowNjowNlo=:ZERO_SECONDS_TYPE64=TnVtYmVy:ZERO_SECONDS_COUNT=1:ZERO_SECONDS_TEXT64=MA==:POSITIVE_SECONDS_TYPE64=TnVtYmVy:POSITIVE_SECONDS_COUNT=1:POSITIVE_SECONDS_TEXT64=NjA=:ZERO_MINUTES_TYPE64=TnVtYmVy:ZERO_MINUTES_COUNT=1:ZERO_MINUTES_TEXT64=MA==:POSITIVE_MINUTES_TYPE64=TnVtYmVy:POSITIVE_MINUTES_COUNT=1:POSITIVE_MINUTES_TEXT64=MQ==:SAME_PRODUCER_SECONDS_TYPE64=TnVtYmVy:SAME_PRODUCER_SECONDS_COUNT=1:SAME_PRODUCER_SECONDS_TEXT64=MA==:NUMERIC_ZERO_TYPE64=TnVtYmVy:NUMERIC_ZERO_COUNT=1:NUMERIC_ZERO_TEXT64=MA=='


def require(ok, detail):
    if not ok:
        raise ValueError("DATE_DIFFERENCE_DIAGNOSTIC: " + detail)


def verify_source(repo, fid, source):
    if fid != FIXTURE_ID:
        return None
    root = Path(repo).resolve()
    path = root / SOURCE_PATH
    expected_path = path.with_suffix(".expected.json")
    require(not path.is_symlink() and not expected_path.is_symlink()
            and path.resolve().is_relative_to(root)
            and expected_path.resolve().is_relative_to(root), "contained public fixture")
    require(hashlib.sha256(source).hexdigest() == SOURCE_SHA256
            and path.read_bytes() == source, "reviewed public source bytes")
    raw = expected_path.read_bytes()
    require(hashlib.sha256(raw).hexdigest() == CONTRACT_SHA256, "reviewed independent contract bytes")
    expected = json.loads(raw)
    require(expected["source_sha256"] == SOURCE_SHA256
            and expected["source_path"] == SOURCE_PATH
            and expected["fixture_id"] == FIXTURE_ID
            and expected["diagnostic_prefix"] == PREFIX
            and expected["field_order"] == [label for label, _, _ in FIELDS]
            and expected["expected_fields_for_numeric_date_contract"] == {label: value for label, _, value in FIELDS}
            and expected["runtime_status"] == "NOT_RUN"
            and expected["formal_requirements_proved"] == [], "source/contract identity and scope")
    return {"sourceSha256": SOURCE_SHA256, "expectedContractSha256": CONTRACT_SHA256,
            "syntheticOnly": True, "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False}


def verify_graph(fid, actions):
    if fid != FIXTURE_ID:
        return None
    g = Graph(actions)
    allowed = {"comment", "nothing", "setvariable", "gettext", "appendvariable",
               "date", "gettimebetweendates", "number", "getitemtype", "count",
               "format.date", "base64encode", "text.combine", "output"}
    require(all(kind(a) in allowed for a in actions), "external or unreviewed effect")
    require(sum(kind(a) == "date" for a in actions) == len(DATES), "exact Date producers")
    require(sum(kind(a) == "gettimebetweendates" for a in actions) == len(INTERVALS), "exact interval producers")
    require(sum(kind(a) == "number" for a in actions) == 1, "independent numeric control")
    require(sum(kind(a) == "format.date" for a in actions) == len(DATES), "independent date formats")
    clears = [i for i, a in enumerate(actions) if kind(a) == "nothing"]
    require(len(clears) == 1, "one owned raw-log clear; no labeled metadata used")
    g.clear(clears[0], "__cherri_debug_log")
    require(sum(kind(a) == "setvariable" for a in actions) == 1, "no value replacement")
    require(sum(kind(a) == "appendvariable" for a in actions) == 2, "trace appends only; no debugLog")

    for _, name, date in DATES:
        p = g.at(g.get(name), "date")
        require(p.get("WFDateActionMode") == "Specified Date", "specified Date mode " + name)
        literal(p.get("WFDateActionDate"), date)
        g.linked(name + "Type", "getitemtype", name)
        g.encode(name + "Type64", name + "Type")
        g.count(name + "Count", name)
        formatted = g.linked(name + "Formatted", "format.date", name, "WFDate")
        require(formatted.get("WFDateFormatStyle") == "Custom"
                and formatted.get("WFTimeFormatStyle") == "None"
                and formatted.get("WFDateFormat") == "yyyy-MM-dd'T'HH:mm:ssXXXXX"
                and formatted.get("WFLocale") == "en_US_POSIX", "explicit independent date formatting")
        g.encode(name + "Format64", name + "Formatted")

    for _, name, start, end, unit, _ in INTERVALS:
        p = g.at(g.get(name), "gettimebetweendates")
        g.ref(p.get("WFTimeUntilFromDate"), start)
        g.ref(p.get("WFInput"), end)
        require(p.get("WFTimeUntilUnit") == unit, "interval units " + name)
        require(g.get(start) < g.get(name) and g.get(end) < g.get(name), "input dominance " + name)
    p = g.at(g.get("numericZero"), "number")
    value = p.get("WFNumberActionNumber")
    require(type(value) in (str, int) and str(value) == "0", "numeric-zero literal control")
    for _, name, _ in RESULTS:
        g.linked(name + "Type", "getitemtype", name)
        g.encode(name + "Type64", name + "Type")
        g.count(name + "Count", name)
        g.linked(name + "Text", "gettext", name, "WFTextActionText")
        g.encode(name + "Text64", name + "Text")

    summary = PREFIX
    attachments = {}
    for label, name, _ in FIELDS:
        summary += ":" + label + "="
        p = params(actions[g.get(name)])
        attachments["{%d, 1}" % len(summary)] = {"Type": "ActionOutput", "OutputName": name, "OutputUUID": p["UUID"]}
        summary += "\ufffc"
    summary += "\n"
    p = g.at(g.get("traceEnvelope"), "gettext")
    attachments["{%d, 1}" % len(summary)] = {"Type": "ActionOutput", "OutputName": "traceEnvelope", "OutputUUID": p["UUID"]}
    summary += "\ufffc"
    actual = text_value(g.at(g.get("finalSummary"), "gettext").get("WFTextActionText"))
    require(actual == {"string": summary, "attachmentsByRange": attachments}, "exact observed summary; no constants or borrowed fields")
    require(kind(actions[-1]) == "output", "native final output")
    g.ref(params(actions[-1]).get("WFOutput"), "finalSummary")
    return {"status": "PASS_LOCAL_CAUSAL_GRAPH_ONLY", "observedFields": len(FIELDS),
            "dateProducers": len(DATES), "intervalProducers": len(INTERVALS),
            "numericZeroControl": True, "dateValuesIndependentlyFormatted": True,
            "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False}


def verify_causal_mutations(actions):
    verify_graph(FIXTURE_ID, actions)
    def named(graph, name):
        return next(a for a in graph if params(a).get("CustomOutputName") == name)
    def update(name, **values):
        return lambda graph: params(named(graph, name)).update(values)
    def redirect(name, key, other):
        def mutate(graph):
            token(params(named(graph, name))[key])["OutputUUID"] = params(named(graph, other))["UUID"]
        return mutate
    tests = [
        ("wrong-specified-date", update("laterDate", WFDateActionDate="2001-02-03T04:05:06Z")),
        ("current-date-mode", update("startDate", WFDateActionMode="Current Date")),
        ("zero-borrow-positive-input", redirect("zeroSeconds", "WFInput", "laterDate")),
        ("positive-borrow-zero-input", redirect("positiveSeconds", "WFInput", "sameDate")),
        ("wrong-start-input", redirect("positiveSeconds", "WFTimeUntilFromDate", "laterDate")),
        ("wrong-second-unit", update("zeroSeconds", WFTimeUntilUnit="Minutes")),
        ("wrong-minute-unit", update("positiveMinutes", WFTimeUntilUnit="Seconds")),
        ("same-producer-replaced", redirect("sameProducerSeconds", "WFInput", "sameDate")),
        ("zero-type-borrowed", redirect("zeroSecondsType", "WFInput", "numericZero")),
        ("zero-count-borrowed", redirect("zeroSecondsCount", "Input", "numericZero")),
        ("zero-count-characters", update("zeroSecondsCount", WFCountType="Characters")),
        ("zero-text-constant", update("zeroSecondsText", WFTextActionText="0")),
        ("zero-text-borrowed", redirect("zeroSecondsText", "WFTextActionText", "numericZero")),
        ("numeric-control-one", update("numericZero", WFNumberActionNumber="1")),
        ("date-format-borrowed", redirect("laterDateFormatted", "WFDate", "startDate")),
        ("date-format-loses-time", update("startDateFormatted", WFDateFormat="yyyy-MM-dd")),
        ("date-bytes-borrowed", redirect("laterDateFormat64", "WFInput", "startDateFormatted")),
        ("base64-decode", update("zeroSecondsText64", WFEncodeMode="Decode")),
        ("constant-summary", update("finalSummary", WFTextActionText=EXPECTED_OUTPUT)),
        ("constant-output", lambda graph: params(graph[-1]).update(WFOutput=EXPECTED_OUTPUT)),
        ("clear-with-input", lambda graph: params(next(a for a in graph if kind(a) == "nothing")).update(WFInput="")),
        ("extra-unowned-clear", lambda graph: graph.insert(1, {"WFWorkflowActionIdentifier": "is.workflow.actions.nothing", "WFWorkflowActionParameters": {}})),
        ("unreviewed-network-effect", lambda graph: graph.insert(1, {"WFWorkflowActionIdentifier": "is.workflow.actions.geturl", "WFWorkflowActionParameters": {}})),
    ]
    rejected = []
    for name, mutate in tests:
        changed = copy.deepcopy(actions)
        mutate(changed)
        try:
            verify_graph(FIXTURE_ID, changed)
        except ValueError:
            rejected.append(name)
        else:
            raise AssertionError("Date diagnostic causal corruption accepted: " + name)
    return {"status": "PASS_LOCAL_NEGATIVE_CONTROLS", "rejected": rejected,
            "count": len(rejected), "appleRuntime": "NOT_RUN", "formalAcceptanceGranted": False}


def main():
    import argparse
    import plistlib
    import subprocess
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--compiler", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    args.out.mkdir(parents=True, exist_ok=True)
    src = (args.repo / SOURCE_PATH).resolve()
    source_receipt = verify_source(args.repo, FIXTURE_ID, src.read_bytes())
    native = args.out / (FIXTURE_ID + ".shortcut")
    require(not native.exists(), "fresh owned compilation output required")
    compiled = subprocess.run([str(args.compiler.resolve()), str(src), "--skip-sign", "--build-profile=debug", "--output=" + str(native.resolve())],
                              cwd=args.out, capture_output=True, timeout=60)
    (args.out / "compile.stdout").write_bytes(compiled.stdout)
    (args.out / "compile.stderr").write_bytes(compiled.stderr)
    require(compiled.returncode == 0 and native.is_file(), "actual fixture compilation failed")
    raw = native.read_bytes()
    actions = plistlib.loads(raw)["WFWorkflowActions"]
    report = {"source": source_receipt, "graph": verify_graph(FIXTURE_ID, actions),
              "negativeControls": verify_causal_mutations(actions), "nativeActions": len(actions),
              "compiledUnsignedSha256": hashlib.sha256(raw).hexdigest(),
              "compilerSha256": hashlib.sha256(args.compiler.read_bytes()).hexdigest(),
              "appleRuntime": "NOT_RUN"}
    (args.out / "date-diagnostic-local.json").write_bytes((json.dumps(report, indent=2) + "\n").encode())
    print(json.dumps(report))


if __name__ == "__main__":
    main()
