#!/usr/bin/env python3
"""Strict model for actual marker-mode pagination graphs, not Apple execution."""
import math
import re
import urllib.parse

import verify_paginate as base
from verify_debug_export import Value, as_text

NATIVE_ACTIONS = base.ALLOWED | {"urlencode", "round", "text.replace"}


class MarkerGraphModel(base.PaginateGraphModel):
    native_actions = NATIVE_ACTIONS

    def __init__(self, server, *, number_format="canonical", cursor_encoding="component"):
        super().__init__(server)
        assert number_format in {"canonical", "grouped", "exponent"}
        assert cursor_encoding in {"component", "unsafe-reserved", "lossy-unicode"}
        self.number_format = number_format
        self.cursor_encoding = cursor_encoding

    def value(self, raw):
        # This is an explicitly declared model assumption for one native Number
        # interpolated into otherwise empty Text. No Dictionary/List coercion is
        # added to the original strict text/reference decoder.
        if isinstance(raw, dict) and raw.get("WFSerializationType") == "WFTextTokenString":
            payload = raw.get("Value", {})
            if payload.get("string") == "\ufffc" and set(payload.get("attachmentsByRange", {})) == {"{0, 1}"}:
                number = super().value(payload["attachmentsByRange"]["{0, 1}"])
                if number.kind == "Number":
                    assert not isinstance(number.data, bool) and math.isfinite(number.data)
                    if self.number_format == "grouped":
                        text = format(number.data, ",")
                    elif self.number_format == "exponent":
                        text = format(number.data, "e")
                    else:
                        text = str(int(number.data)) if int(number.data) == number.data else str(number.data)
                    return Value("Text", text)
        return super().value(raw)

    def extension_action(self, ident, p):
        keys = set(p) - {"UUID", "CustomOutputName"}
        if ident == "urlencode":
            assert keys == {"WFInput", "WFEncodeMode"}
            source = as_text(self.value(p["WFInput"]))
            if p["WFEncodeMode"] == "Encode":
                if self.cursor_encoding == "lossy-unicode":
                    source = source.encode("ascii", "replace").decode("ascii")
                safe = "-._~" if self.cursor_encoding != "unsafe-reserved" else "-._~&/?#"
                return Value("Text", urllib.parse.quote(source, safe=safe, encoding="utf-8", errors="strict"))
            assert p["WFEncodeMode"] == "Decode"
            return Value("Text", urllib.parse.unquote(source, encoding="utf-8", errors="strict"))
        if ident == "round":
            assert keys == {"WFInput", "WFRoundTo", "WFRoundMode"}
            assert p["WFRoundTo"] == "Ones Place" and p["WFRoundMode"] == "Normal"
            number = self.value(p["WFInput"])
            assert number.kind == "Number" and math.isfinite(number.data)
            return Value("Number", math.floor(number.data + 0.5) if not isinstance(number.data, int) else number.data)
        if ident == "text.replace":
            assert keys == {"WFInput", "WFReplaceTextFind", "WFReplaceTextReplace", "WFReplaceTextCaseSensitive", "WFReplaceTextRegularExpression"}
            assert p["WFReplaceTextCaseSensitive"] is True and p["WFReplaceTextRegularExpression"] is True
            assert p["WFReplaceTextReplace"] == ""
            assert p["WFReplaceTextFind"] in {
                "^(?:[A-Za-z0-9._~-]|%[0-9A-Fa-f]{2})+$",
                "^(?:0|[1-9][0-9]{0,15})$",
            }
            return Value("Text", re.sub(p["WFReplaceTextFind"], "", as_text(self.value(p["WFInput"]))))
        raise AssertionError("unreviewed marker action: " + ident)


def inspect_marker_graph(graph, server, routes, expected, requests, guard=None, *,
                         headers=None, number_format="canonical", cursor_encoding="component"):
    server.routes, server.requests, server.request_headers = routes, [], []
    model = MarkerGraphModel(server, number_format=number_format, cursor_encoding=cursor_encoding)
    output, observed = None, None
    try:
        output = model.run(graph)
    except base.GuardStop as error:
        observed = str(error)
        assert observed == guard, "unexpected marker guard: " + observed
    else:
        assert guard is None, "required marker guard was bypassed"
        assert output.kind == "List" and all(v.kind == "Dictionary" for v in output.data)
        assert all(id(v) in model.source_dictionaries for v in output.data), "original native item identity changed"
        assert base.plain(output) == expected, "typed ordered result/cap changed"
    assert server.requests == requests, "request target/order/count changed: " + repr(server.requests)
    expected_headers = {} if headers is None else {key.lower(): value for key, value in headers.items()}
    for actual in server.request_headers:
        assert {key: actual[key] for key in ("authorization", "x-cherri-client") if key in actual} == expected_headers
    return {
        "status": "PASS_MODEL", "requests": list(server.requests), "guard": observed,
        "items": None if output is None else len(output.data), "executedActions": model.executed,
        "numberTextAssumption": number_format, "cursorEncodingAssumption": cursor_encoding,
        "nativeRuntimeExecuted": False,
    }
