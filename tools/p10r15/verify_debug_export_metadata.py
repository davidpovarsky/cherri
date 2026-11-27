#!/usr/bin/env python3
"""Evaluate emitted metadata graphs only through their completed Text envelope.

This is an independent, bounded source-graph model. It never executes or models
the unproved Base64 Decode/Set Name File-class boundary or Apple runtime.
"""
from __future__ import annotations
import argparse
import base64
import copy
import hashlib
import json
import math
import re
from dataclasses import dataclass, field
from pathlib import Path

PREFIX = "is.workflow.actions."
PRIVATE = "__cherri_debug_export_"
RAW = "__cherri_debug_log"
LABELS = "__cherri_debug_log_labels"

@dataclass
class Native:
    kind: str
    value: object
    creation: tuple["Native", ...] = field(default_factory=tuple)

def text(value): return Native("Text", value)
def number(value): return Native("Number", value)
def scalar(values, kind=None):
    if len(values) != 1:
        raise ValueError("a scalar native input is required")
    item = values[0]
    if kind is not None and item.kind != kind:
        raise ValueError(f"implicit native {item.kind} to {kind} conversion refused")
    return item

def output_id(raw):
    if not isinstance(raw, dict): return None
    if raw.get("WFSerializationType") == "WFTextTokenAttachment": return output_id(raw.get("Value"))
    if raw.get("Type") == "ActionOutput": return raw.get("OutputUUID")
    return None

def parameters(action):
    raw = action.get("WFWorkflowActionParameters")
    # Go's actual ShortcutAction JSON emits null for parameter-free Nothing.
    # No other action may lose its required native parameter map.
    if raw is None and action["WFWorkflowActionIdentifier"] == PREFIX+"nothing":
        return {}
    if not isinstance(raw, dict):
        raise ValueError("native action parameters must be a map")
    return raw

def content_boundary(actions):
    if len(actions) < 4: raise ValueError("missing original materialization boundary")
    by_id = {}
    for index, action in enumerate(actions):
        p = parameters(action)
        ident = action["WFWorkflowActionIdentifier"]
        if "UUID" in p:
            if p["UUID"] in by_id: raise ValueError("duplicate native producer")
            by_id[p["UUID"]] = (index, action)
        if ident in {PREFIX+"setvariable", PREFIX+"appendvariable"}:
            if not p.get("WFVariableName", "").startswith(PRIVATE):
                raise ValueError("export writes shared or user state")
    last = actions[-1]
    if last["WFWorkflowActionIdentifier"] != PREFIX+"setitemname":
        raise ValueError("missing named artifact boundary")
    if last["WFWorkflowActionParameters"].get("WFName") not in {"cherri-debug.json", "cherri-debug.txt"}:
        raise ValueError("changed export filename")
    decoder_index, decoder = by_id[output_id(last["WFWorkflowActionParameters"]["WFInput"])]
    encoder_index, encoder = by_id[output_id(decoder["WFWorkflowActionParameters"]["WFInput"])]
    content_index, content = by_id[output_id(encoder["WFWorkflowActionParameters"]["WFInput"])]
    if (decoder_index, encoder_index, content_index) != (len(actions)-2, len(actions)-3, len(actions)-4):
        raise ValueError("original exact materialization order changed")
    if decoder["WFWorkflowActionIdentifier"] != PREFIX+"base64encode" or decoder["WFWorkflowActionParameters"].get("WFEncodeMode") != "Decode":
        raise ValueError("original decode boundary changed")
    if encoder["WFWorkflowActionIdentifier"] != PREFIX+"base64encode" or encoder["WFWorkflowActionParameters"].get("WFEncodeMode") != "Encode":
        raise ValueError("original encode boundary changed")
    if content["WFWorkflowActionIdentifier"] != PREFIX+"gettext":
        raise ValueError("artifact does not consume completed Text")
    return content_index

class GraphModel:
    def __init__(self, items, labels):
        self.variables = {RAW: list(items), LABELS: list(labels)}
        self.outputs = {}
        self.executed = 0
        self.label_reads = 0
        self.label_value_reads = 0
        self.date_formats = 0
        self.dictionary_field_reads = []
        self.max_steps = 2_000_000

    def value(self, raw):
        if isinstance(raw, str): return [text(raw)]
        if type(raw) in {int, float}: return [number(raw)]
        if type(raw) is bool: return [Native("Bool", raw)]
        if not isinstance(raw, dict): raise ValueError("unknown native wire value")
        kind = raw.get("WFSerializationType")
        if kind == "WFTextTokenAttachment": return self.value(raw["Value"])
        if kind == "WFTextTokenString":
            value = raw["Value"]
            units = value.get("string", "").encode("utf-16-le")
            ranges = []
            for key, ref in value.get("attachmentsByRange", {}).items():
                match = re.fullmatch(r"\{([0-9]+), 1\}", key)
                if match is None: raise ValueError("invalid UTF-16 attachment")
                offset = int(match[1])*2
                if units[offset:offset+2] != "\ufffc".encode("utf-16-le"):
                    raise ValueError("attachment does not address its placeholder")
                resolved = scalar(self.value(ref))
                if resolved.kind not in {"Text", "Number"}:
                    raise ValueError("unreviewed text interpolation coercion")
                literal = str(resolved.value)
                ranges.append((offset, literal.encode("utf-16-le")))
            if len({offset for offset, _ in ranges}) != len(ranges):
                raise ValueError("duplicate UTF-16 range")
            for offset, replacement in sorted(ranges, reverse=True):
                units = units[:offset]+replacement+units[offset+2:]
            return [text(units.decode("utf-16-le"))]
        native_kind = raw.get("Type")
        if native_kind == "ActionOutput":
            result = self.outputs[raw["OutputUUID"]]
        elif native_kind == "Variable":
            if "Variable" in raw: return self.value(raw["Variable"])
            name = raw["VariableName"]
            if name == LABELS: self.label_reads += 1
            result = self.variables[name]
        elif native_kind == "CurrentDate":
            result = [Native("Date", "2001-01-01T00:00:00Z")]
        else:
            raise ValueError("unknown native reference")
        for transform in raw.get("Aggrandizements", []):
            if transform != {"Type":"WFPropertyVariableAggrandizement", "PropertyName":"Creation Date", "PropertyUserInfo":"WFFileCreationDate"}:
                raise ValueError("unreviewed native aggrandizement")
            result = [date for item in result for date in item.creation]
        return list(result)

    def field(self, raw, kind):
        # Number/Date field tokens with one exact placeholder carry the native
        # object; they are not a user-visible Text interpolation conversion.
        if isinstance(raw, dict) and raw.get("WFSerializationType") == "WFTextTokenString":
            v = raw["Value"]
            refs = v.get("attachmentsByRange", {})
            if v.get("string") == "\ufffc" and set(refs) == {"{0, 1}"}:
                return scalar(self.value(refs["{0, 1}"]), kind).value
        return scalar(self.value(raw), kind).value

    def run(self, actions, start=0, end=None):
        end = len(actions) if end is None else end
        index, previous = start, []
        while index < end:
            action = actions[index]
            ident = action["WFWorkflowActionIdentifier"]
            if not ident.startswith(PREFIX): raise ValueError("nonlocal action")
            ident = ident[len(PREFIX):]
            p = parameters(action)
            self.executed += 1
            if self.executed > self.max_steps: raise ValueError("bounded graph model exhausted")
            if ident in {"repeat.each", "repeat.count", "conditional"} and p.get("WFControlFlowMode") == 0:
                group = p["GroupingIdentifier"]
                closings = [i for i in range(index+1, end)
                            if parameters(actions[i]).get("GroupingIdentifier") == group
                            and parameters(actions[i]).get("WFControlFlowMode") == 2]
                if len(closings) != 1: raise ValueError("unbalanced native control group")
                closing = closings[0]
                if ident in {"repeat.each", "repeat.count"}:
                    if ident == "repeat.each":
                        items = self.value(p["WFInput"])
                    else:
                        count = self.field(p["WFRepeatCount"], "Number")
                        if not math.isfinite(count) or count < 0 or count != int(count) or count > 100_000:
                            raise ValueError("invalid repeat count")
                        items = [None]*int(count)
                        if actions[closing-1]["WFWorkflowActionIdentifier"] != PREFIX+"nothing":
                            raise ValueError("private cursor leaks helper Repeat Results")
                    collected = []
                    body_ids = [parameters(a).get("UUID") for a in actions[index+1:closing]]
                    for iteration, item in enumerate(items, 1):
                        for producer in body_ids:
                            if producer: self.outputs.pop(producer, None)
                        self.variables["Repeat Index"] = [number(iteration)]
                        if item is not None: self.variables["Repeat Item"] = [item]
                        collected.extend(self.run(actions, index+1, closing))
                    previous = collected
                else:
                    alternatives = [i for i in range(index+1, closing)
                                    if parameters(actions[i]).get("GroupingIdentifier") == group
                                    and parameters(actions[i]).get("WFControlFlowMode") == 1]
                    if len(alternatives) > 1: raise ValueError("duplicate else boundary")
                    alternative = alternatives[0] if alternatives else closing
                    if "WFNumberValue" in p:
                        left = self.field(p["WFInput"], "Number")
                        right = self.field(p["WFNumberValue"], "Number")
                    elif "WFConditionalActionString" in p:
                        left = self.field(p["WFInput"], "Text")
                        right = self.field(p["WFConditionalActionString"], "Text")
                    else: raise ValueError("missing typed comparison")
                    code = p["WFCondition"]
                    if code not in {0, 1, 2, 3, 4, 5}: raise ValueError("unknown condition")
                    match = {0:lambda:left<right, 1:lambda:left<=right, 2:lambda:left>right,
                             3:lambda:left>=right, 4:lambda:left==right, 5:lambda:left!=right}[code]()
                    if match:
                        previous = self.run(actions, index+1, alternative)
                    else:
                        previous = self.run(actions, alternative+1, closing) if alternatives else []
                close = parameters(actions[closing])
                if "UUID" in close: self.outputs[close["UUID"]] = list(previous)
                index = closing+1
                continue
            if ident == "gettext": previous = self.value(p["WFTextActionText"])
            elif ident == "number": previous = [number(self.field(p["WFNumberActionNumber"], "Number"))]
            elif ident == "nothing":
                if set(p)-{"UUID", "CustomOutputName"}: raise ValueError("Nothing acquired input")
                previous = []
            elif ident == "getvariable": previous = self.value(p["WFVariable"])
            elif ident == "getitemtype":
                source = scalar(self.value(p["WFInput"]))
                previous = [text("native:"+source.kind)]
            elif ident == "count":
                if p.get("WFCountType") != "Items": raise ValueError("wrong cardinality operation")
                previous = [number(len(self.value(p["Input"])))]
            elif ident == "dictionary":
                wrapper = p["WFItems"]
                fields = wrapper.get("Value", {}).get("WFDictionaryFieldValueItems")
                if fields != []: raise ValueError("only the actual empty Dictionary type witness is modeled")
                previous = [Native("Dictionary", {})]
            elif ident == "getvalueforkey":
                source = scalar(self.value(p["WFInput"]), "Dictionary")
                mode = p.get("WFGetDictionaryValueType")
                if mode == "All Keys":
                    previous = [text(key) for key in source.value]
                elif mode == "Value":
                    key = self.field(p["WFDictionaryKey"], "Text")
                    if "." in key: raise ValueError("native dotted key path was not excluded")
                    if key == "label": self.label_value_reads += 1
                    self.dictionary_field_reads.append((id(source), key))
                    previous = list(source.value.get(key, []))
                else: raise ValueError("unreviewed dictionary read")
            elif ident == "getitemfromlist":
                if p.get("WFItemSpecifier") != "Item At Index": raise ValueError("unreviewed list projection")
                values = self.value(p["WFInput"])
                position = self.field(p["WFItemIndex"], "Number")
                if position != int(position) or not 1 <= position <= len(values):
                    raise ValueError("out-of-range native item lookup")
                previous = [values[int(position)-1]]
            elif ident == "setvariable":
                name = p["WFVariableName"]
                if not name.startswith(PRIVATE): raise ValueError("shared state mutation")
                if "WFInput" in p: previous = self.value(p["WFInput"])
                elif previous: raise ValueError("empty rows initialization has real implicit input")
                self.variables[name] = list(previous)
            elif ident == "appendvariable":
                name = p["WFVariableName"]
                if not name.startswith(PRIVATE): raise ValueError("shared state append")
                incoming = self.value(p["WFInput"])
                self.variables[name] = self.variables[name]+incoming
                previous = incoming
            elif ident == "math":
                left = self.field(p["WFInput"], "Number")
                right = self.field(p["WFMathOperand"], "Number")
                if p["WFMathOperation"] == "+": value = left+right
                elif p["WFMathOperation"] == "-": value = left-right
                else: raise ValueError("unreviewed arithmetic")
                previous = [number(value)]
            elif ident == "round":
                if p.get("WFRoundTo") != "Ones Place" or p.get("WFRoundMode") != "Always Round Down":
                    raise ValueError("changed integer validation")
                previous = [number(math.floor(self.field(p["WFInput"], "Number")))]
            elif ident == "format.date":
                if p.get("WFDateFormatStyle") != "ISO 8601" or p.get("WFISO8601IncludeTime") is not True:
                    raise ValueError("date format contract changed")
                date = self.field(p["WFDate"], "Date")
                self.date_formats += 1
                previous = [text(date)]
            elif ident == "base64encode":
                if p.get("WFEncodeMode") != "Encode":
                    raise ValueError("unproved native materialization must not be modeled")
                if p.get("WFBase64LineBreakMode") != "None":
                    raise ValueError("Base64 contains line breaks")
                encoded = base64.b64encode(self.field(p["WFInput"], "Text").encode("utf-8")).decode("ascii")
                previous = [text(encoded)]
            elif ident == "text.match":
                pattern = p["WFMatchTextPattern"]
                subject = self.field(p["text"], "Text")
                flags = 0 if p["WFMatchTextCaseSensitive"] else re.IGNORECASE
                previous = [text(match.group(0)) for match in re.finditer(pattern, subject, flags)]
            elif ident == "text.combine":
                if p.get("WFTextSeparator") != "Custom": raise ValueError("unknown separator")
                values = self.value(p["text"])
                if any(v.kind != "Text" for v in values): raise ValueError("non-Text row escaped")
                previous = [text(p["WFTextCustomSeparator"].join(v.value for v in values))]
            else:
                raise ValueError("unreviewed native action: "+ident)
            if "UUID" in p: self.outputs[p["UUID"]] = list(previous)
            index += 1
        return previous

def label_range(first, last, label):
    return Native("Dictionary", {"first":[number(first)], "last":[number(last)], "label":[text(label)]})

def label_contract(labels, count):
    invalid, ambiguous, previous = False, False, 0
    ranges = []
    for entry in labels:
        if entry.kind != "Dictionary":
            invalid = True
            continue
        try:
            first = scalar(entry.value.get("first", []), "Number").value
            last = scalar(entry.value.get("last", []), "Number").value
            label = scalar(entry.value.get("label", []), "Text").value
            if any(not math.isfinite(v) or v != int(v) for v in [first,last]):
                raise ValueError("fractional range")
            if not (first >= 1 and last >= 0 and last <= count and first <= last+1):
                raise ValueError("range is outside the actual native log")
        except (ValueError, TypeError):
            invalid = True
            continue
        if last < previous or (first <= previous and last >= first):
            ambiguous = True
        previous = last
        ranges.append((int(first),int(last),label))
    if invalid: return "UNSUPPORTED_LABEL_METADATA", []
    if ambiguous: return "AMBIGUOUS", []
    return "VALID", ranges

def decode(value):
    if value["encoding"] != "base64-utf8": raise AssertionError("wrong encoding")
    return base64.b64decode(value["value"], validate=True).decode("utf-8")


def payload_policy(redact, keys=()):
    return ("selective-dictionary-diagnostics" if keys else "redacted") if redact else "native-scalar-and-dictionary-diagnostics"

PROTECTED_KEYS = {"authorization", "proxy-authorization", "headers", "cookie", "cookies", "set-cookie",
                  "password", "passwords", "secret", "secrets", "token", "access_token", "refresh_token",
                  "id_token", "api_key", "apikey", "credentials", "private_key", "url", "uri", "link",
                  "query", "querystring", "queryparameters"}

def inspect_payload(row, item, include_types, redact=False, keys=(), nested=False):
    if redact and not nested and (not keys or item.kind != "Dictionary"):
        assert row["status"] == "REDACTED" and row["payload"] is None
        return 0
    if item.kind == "Text":
        if redact:
            assert row["status"] == "REDACTED_TEXT_CONTENT" and row["payload"] is None
        else:
            assert row["status"] == "TEXT_PAYLOAD_ENCODED" and decode(row["payload"]) == item.value
        return 0
    if item.kind == "Number":
        assert row["status"] == "NATIVE_NUMBER_TEXT_ENCODED"
        assert row["representation"] == "native-number-text"
        assert decode(row["payload"]) == str(item.value)
        return 0
    if item.kind == "Date":
        assert row["status"] == "DATE_ISO8601_ENCODED" and row["representation"] == "iso8601-native-format"
        assert decode(row["payload"]) == item.value
        return 1
    if item.kind != "Dictionary":
        assert row["status"] == "UNSUPPORTED_NATIVE_PAYLOAD_TYPE" and row["payload"] is None
        return 0
    if nested:
        assert row["status"] == "UNSUPPORTED_NESTED_DICTIONARY" and row["payload"] is None
        return 0
    assert row["status"] == "DICTIONARY_FIELD_DIAGNOSTICS"
    payload = row["payload"]
    assert payload["encoding"] == "diagnostic-fields" and payload["lossless"] is False
    fields = payload["fields"]
    assert len(fields) == len(item.value)
    date_count = 0
    for field, (key, values) in zip(fields, item.value.items()):
        assert decode(field["key"]) == key
        diagnostic = field["diagnostic"]
        if keys and key.casefold() in PROTECTED_KEYS.union(x.casefold() for x in keys):
            assert diagnostic == {"status":"REDACTED_SENSITIVE_FIELD", "payload":None}
        elif "." in key:
            assert diagnostic == {"status":"UNSUPPORTED_KEY_PATH", "payload":None}
        elif len(values) == 0:
            assert diagnostic == {"status":"ABSENT_NULL_OR_EMPTY_COLLECTION", "payload":None}
        elif len(values) != 1:
            assert diagnostic == {"status":"UNSUPPORTED_MULTIPLE_NATIVE_ITEMS", "payload":None}
        else:
            value = values[0]
            date_count += inspect_payload(diagnostic, value, include_types, redact, keys, nested=True)
            if include_types:
                assert decode(diagnostic["nativeType"]) == "native:"+value.kind
            else:
                assert "nativeType" not in diagnostic
    return date_count


def inspect(actions, items, labels, options):
    before = copy.deepcopy((items,labels))
    ids_before = ([id(x) for x in items], [id(x) for x in labels])
    boundary = content_boundary(actions)
    model = GraphModel(items,labels)
    completed = scalar(model.run(actions, end=boundary+1), "Text").value
    if options["format"] == "json":
        document = json.loads(completed)
        assert document["schemaVersion"] == "cherri.debug-export/1"
        assert document["scope"] == "run-local-native-items"
        assert document["payloadPolicy"] == payload_policy(options["redact"])
        assert document["dates"] == ("source-creation-date" if options["dates"] else "not-collected")
        expected_labels = "not-collected"
        if options["labels"]: expected_labels = "redacted" if options["redact"] else "source-range-metadata"
        assert document["labels"] == expected_labels
        rows = document["entries"]
    else:
        header, body = completed.split("\n\n",1)
        assert header.startswith("Cherri Debug Export 1\n")
        assert "Dates: "+("source-creation-date" if options["dates"] else "not-collected") in header
        rows = [json.loads(line) for line in body.splitlines()]
    assert len(rows) == len(items), "native entry cardinality changed"
    status, ranges = label_contract(labels,len(items))
    payload_date_formats = 0
    for position,(row,item) in enumerate(zip(rows,items),1):
        payload_date_formats += inspect_payload(row,item,options["types"],options["redact"])
        if options["types"]:
            assert decode(row["nativeType"]) == "native:"+item.kind
        else: assert "nativeType" not in row
        if not options["dates"]:
            assert "date" not in row
        else:
            date = row["date"]
            assert date["source"] == "source-creation-date"
            expected = "ABSENT" if not item.creation else ("AMBIGUOUS" if len(item.creation)>1 else
                       ("PRESENT" if item.creation[0].kind=="Date" else "UNSUPPORTED_DATE_TYPE"))
            assert date["status"] == expected
            if expected == "PRESENT":
                assert date["format"] == "iso8601" and decode(date) == item.creation[0].value
            else: assert date["value"] is None
        if not options["labels"]:
            assert "label" not in row
        else:
            label = row["label"]
            if options["redact"]:
                assert label == {"status":"REDACTED","value":None}
            elif status != "VALID":
                assert label == {"status":status,"value":None}
            else:
                found = [label for first,last,label in ranges if first<=position<=last]
                assert len(found) <= 1
                if not found: assert label == {"status":"ABSENT","value":None}
                else: assert label["status"] == "PRESENT" and decode(label) == found[0]
    if options["redact"] or not options["labels"]:
        assert model.label_reads == 0 and model.label_value_reads == 0, "redacted/omitted label metadata was read"
    expected_metadata_formats = sum(len(item.creation)==1 and item.creation[0].kind=="Date" for item in items) if options["dates"] else 0
    assert model.date_formats == expected_metadata_formats + payload_date_formats
    assert (items,labels) == before, "native inputs changed"
    assert ids_before == ([id(x) for x in items],[id(x) for x in labels]), "native identities replaced"
    assert model.executed <= 180*(len(items)+len(labels))+100, "model action growth exceeded the linear source bound"
    return {"entries":len(rows),"textBytes":len(completed.encode()),"executedNativeActionsInModel":model.executed,
            "labelBufferReadsInModel":model.label_reads,"labelValueReadsInModel":model.label_value_reads,
            "creationDateFormatsInModel":model.date_formats,"completedTextSHA256":hashlib.sha256(completed.encode()).hexdigest()}

def samples():
    tricky = "\"\\\n\r\t שלום שָׁלוֹם 🍒 \ufffc {literal} "+"".join(chr(x) for x in range(32))+" SYNTHETIC_SECRET"
    date = Native("Date","2026-11-25T12:34:56+02:00")
    dated = Native("Text",tricky,(date,))
    cases = {
        "empty":([],[]),
        "empty-text-and-empty-label":([text("")],[label_range(1,1,"")]),
        "unicode-dated-label":([dated],[label_range(1,1,tricky)]),
        "native-kind-and-date-matrix":([
            dated, text("absent"), Native("Number",0,(date,date)), Native("Bool",False,(text("not-date"),)),
            Native("Null",None),Native("Dictionary",{"x":"secret"}),Native("List",[]),
            Native("File",b"\0\xff"),Native("Image",b"\0\xff")],
            [label_range(1,1,"first"),label_range(3,5,"middle"),label_range(9,9,tricky)]),
        "unlabeled-gaps":([text(str(x)) for x in range(7)],
                          [label_range(2,3,"two-three"),label_range(6,6,"six")]),
        "zero-count-ranges":([text("one"),text("two")],
                            [label_range(1,0,"empty-before"),label_range(1,1,"one"),
                             label_range(2,1,"empty-between"),label_range(2,2,"two"),label_range(3,2,"empty-after")]),
        "overlap":([text(str(x)) for x in range(4)],[label_range(1,3,"a"),label_range(3,4,"b")]),
        "out-of-order":([text(str(x)) for x in range(4)],[label_range(3,4,"a"),label_range(1,2,"b")]),
        "fractional-range":([text("one")],[label_range(1.5,1,"bad")]),
        "out-of-bounds":([text("one")],[label_range(1,2,"bad")]),
        "missing-label":([text("one")],[Native("Dictionary",{"first":[number(1)],"last":[number(1)]})]),
        "number-coercion":([text("one")],[Native("Dictionary",{"first":[text("1")],"last":[number(1)],"label":[text("x")]})]),
        "multiple-label-values":([text("one")],[Native("Dictionary",{"first":[number(1)],"last":[number(1)],"label":[text("a"),text("b")]})]),
        "wrong-metadata-class":([text("one")],[text("not-a-dictionary")]),
        "invalid-dominates-overlap":([text(str(x)) for x in range(3)],
                                    [label_range(1,2,"a"),label_range(2,3,"b"),label_range(0,0,"invalid")]),
    }
    return cases

def causal_faults(graphs):
    chosen = {}
    for entry in graphs:
        o = entry["options"]
        if o["legacy"] or o["format"]!="json" or not o["types"]: continue
        if o["dates"] and o["labels"] and not o["redact"]: chosen["full"] = entry
        if o["labels"] and not o["dates"] and o["redact"]: chosen["redacted"] = entry
    assert set(chosen)=={"full","redacted"}
    fixtures = samples()
    results = []
    def attempt(name,which,scenario,mutate):
        original = chosen[which]
        actions = copy.deepcopy(original["actions"])
        mutate(actions)
        items,labels = fixtures[scenario]
        try:
            inspect(actions,items,labels,original["options"])
        except (AssertionError,ValueError,KeyError,IndexError,TypeError):
            pass
        else: raise AssertionError("causal graph fault accepted: "+name)
        inspect(original["actions"],items,labels,original["options"])
        results.append({"id":name,"negativeRejected":True,"restoredControl":"PASS"})
    def find(actions,ident,predicate=lambda p:True):
        return next(a for a in actions if a["WFWorkflowActionIdentifier"]==PREFIX+ident and predicate(parameters(a)))
    def mutate_date_source(actions):
        p=find(actions,"format.date")["WFWorkflowActionParameters"]
        p["WFDate"]={"WFSerializationType":"WFTextTokenAttachment","Value":{"Type":"CurrentDate"}}
    def mutate_date_style(actions):
        find(actions,"format.date")["WFWorkflowActionParameters"]["WFISO8601IncludeTime"]=False
    def mutate_linebreak(actions):
        find(actions,"base64encode",lambda p:p.get("WFEncodeMode")=="Encode")["WFWorkflowActionParameters"]["WFBase64LineBreakMode"]="Every 76 Characters"
    def mutate_shared_write(actions):
        find(actions,"setvariable")["WFWorkflowActionParameters"]["WFVariableName"]=RAW
    def mutate_rows_seed(actions):
        for i,a in enumerate(actions):
            p=parameters(a)
            if a["WFWorkflowActionIdentifier"]==PREFIX+"setvariable" and p.get("WFVariableName","").endswith("_rows"):
                actions[i-1]={"WFWorkflowActionIdentifier":PREFIX+"gettext","WFWorkflowActionParameters":{"WFTextActionText":""}}
                return
        raise AssertionError("missing rows initialization")
    def mutate_no_advance(actions):
        p=find(actions,"setvariable",lambda p:p.get("WFVariableName","").endswith("_label_index") and output_id(p.get("WFInput")))["WFWorkflowActionParameters"]
        # Select the later advancing write, not its literal initializer.
        writes=[a for a in actions if a["WFWorkflowActionIdentifier"]==PREFIX+"setvariable"
                and a["WFWorkflowActionParameters"].get("WFVariableName","").endswith("_label_index")]
        target=writes[-1]["WFWorkflowActionParameters"]
        target["WFInput"]={"WFSerializationType":"WFTextTokenAttachment","Value":{"Type":"Variable","VariableName":target["WFVariableName"]}}
    def mutate_cursor_tail(actions):
        close=next(i for i,a in enumerate(actions) if a["WFWorkflowActionIdentifier"]==PREFIX+"repeat.count"
                   and a["WFWorkflowActionParameters"].get("WFControlFlowMode")==2)
        assert actions[close-1]["WFWorkflowActionIdentifier"]==PREFIX+"nothing"
        actions[close-1]={"WFWorkflowActionIdentifier":PREFIX+"gettext","WFWorkflowActionParameters":{"WFTextActionText":"leak"}}
    def mutate_label_range(actions):
        for action in actions:
            if action["WFWorkflowActionIdentifier"]==PREFIX+"getvalueforkey" and action["WFWorkflowActionParameters"].get("WFDictionaryKey")=="first":
                action["WFWorkflowActionParameters"]["WFDictionaryKey"]="last"
    def mutate_label_payload(actions):
        matches=[a for a in actions if a["WFWorkflowActionIdentifier"]==PREFIX+"getvalueforkey" and a["WFWorkflowActionParameters"].get("WFDictionaryKey")=="label"]
        matches[-1]["WFWorkflowActionParameters"]["WFDictionaryKey"]="first"
    def mutate_redacted_read(actions):
        actions.insert(0,{"WFWorkflowActionIdentifier":PREFIX+"count","WFWorkflowActionParameters":{"Input":{"WFSerializationType":"WFTextTokenAttachment","Value":{"Type":"Variable","VariableName":LABELS}},"WFCountType":"Items"}})
    def mutate_filename(actions):
        actions[-1]["WFWorkflowActionParameters"]["WFName"]="../unsafe.json"
    def mutate_decoder(actions):
        actions[-2]["WFWorkflowActionParameters"]["WFEncodeMode"]="Encode"
    for spec in [
        ("date-from-export-time","full","unicode-dated-label",mutate_date_source),
        ("date-format-time-omitted","full","unicode-dated-label",mutate_date_style),
        ("base64-linebreaks","full","unicode-dated-label",mutate_linebreak),
        ("raw-buffer-write","full","unicode-dated-label",mutate_shared_write),
        ("synthetic-empty-row","full","empty",mutate_rows_seed),
        ("stalled-range-cursor","full","unlabeled-gaps",mutate_no_advance),
        ("cursor-helper-output-leak","full","unicode-dated-label",mutate_cursor_tail),
        ("range-boundary-changed","full","unlabeled-gaps",mutate_label_range),
        ("label-value-replaced-by-range","full","unicode-dated-label",mutate_label_payload),
        ("redacted-metadata-read","redacted","unicode-dated-label",mutate_redacted_read),
        ("unsafe-artifact-name","full","unicode-dated-label",mutate_filename),
        ("unproved-carrier-direction","full","unicode-dated-label",mutate_decoder),
    ]: attempt(*spec)
    return results

def run(graph_dir,report):
    paths=sorted(graph_dir.glob("*.json"))
    graphs=[json.loads(path.read_text()) for path in paths]
    expected={(d,l,legacy,fmt,types,redact) for d,l in [(True,False),(False,True),(True,True)]
              for legacy in [False,True] for fmt in ["json","text"] for types in [False,True] for redact in [False,True]}
    actual=[tuple(g["options"][key] for key in ["dates","labels","legacy","format","types","redact"]) for g in graphs]
    assert len(actual)==48 and len(set(actual))==48 and set(actual)==expected, "actual source graph cohort is incomplete"
    cases=samples()
    results=[]
    for path,entry in zip(paths,graphs):
        models={name:inspect(entry["actions"],items,labels,entry["options"]) for name,(items,labels) in cases.items()}
        results.append({"sourceTest":entry["id"],"graphPath":str(path),"graphSHA256":hashlib.sha256(path.read_bytes()).hexdigest(),
                        "actionCount":len(entry["actions"]),"models":models})
    stress=[]
    for entry in graphs:
        o=entry["options"]
        if not (o["dates"] and o["labels"] and not o["redact"] and o["format"]=="json"):continue
        items=[Native("Text","item-"+str(i),(Native("Date","2026-11-25T00:00:00Z"),)) for i in range(1000)]
        for name,labels in [("one-range",[label_range(1,1000,"all")]),
                            ("thousand-ranges",[label_range(i+1,i+1,"label-"+str(i)) for i in range(1000)])]:
            stress.append({"sourceTest":entry["id"],"case":name,"result":inspect(entry["actions"],items,labels,o)})
    faults=causal_faults(graphs)
    data={"schema":"cherri.debug-export-metadata-text-model/1","status":"PASS_COMPLETED_TEXT_ENVELOPE_MODEL_ONLY",
          "graphSourceConfigurations":len(graphs),"smallModelCases":sum(len(r["models"]) for r in results),
          "stressCases":len(stress),"causalFaults":len(faults),"source":results,"stress":stress,"faults":faults,
          "verifierSHA256":hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
          "nativeRuntimeExecuted":False,"materializationBoundaryExecuted":False,"fileClassAccepted":False,
          "nativeAssumptions":["Get Variable property absence/cardinality","native empty Text remains an item",
            "Dictionary numeric/Text field types","list-index and Repeat/If result semantics","ISO8601 Date field format",
            "UTF-8 Base64 including empty Text and controls"],
          "scope":"actual emitted graphs through completed Text only; preserves historical File counterevidence"}
    report.parent.mkdir(parents=True,exist_ok=True);report.write_text(json.dumps(data,indent=2)+"\n")
    print(json.dumps({k:data[k] for k in ["status","graphSourceConfigurations","smallModelCases","stressCases","causalFaults","nativeRuntimeExecuted","fileClassAccepted"]}))

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--graphs",type=Path,required=True)
    parser.add_argument("--report",type=Path,required=True)
    args=parser.parse_args()
    try:run(args.graphs,args.report)
    except Exception as error:
        args.report.parent.mkdir(parents=True,exist_ok=True)
        args.report.write_text(json.dumps({"status":"FAIL_COMPLETED_TEXT_MODEL","exceptionType":type(error).__name__,"detail":str(error),
                                           "nativeRuntimeExecuted":False,"fileClassAccepted":False},indent=2)+"\n")
        raise
if __name__=="__main__":main()
