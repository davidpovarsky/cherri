"""Validate the compiler's complete exact-comparison graph before Apple QA.

The verifier follows producer UUIDs and control groups. It never substitutes
native observations or accepts a direct dynamic comparison as exact Text equality.
"""


def verify_equality_region(actions, start, condition_index, left, *, require,
                           right=None, right_source=None, numeric=False,
                           coercion=None, operator=4, dynamic=False):
    def p(action):
        return action.get("WFWorkflowActionParameters", {})

    def kind(action, suffix):
        return action.get("WFWorkflowActionIdentifier") == "is.workflow.actions." + suffix

    def number(value, expected):
        return type(value) in (int, float) and value == expected

    def token(value):
        require(isinstance(value, dict), "comparison attachment")
        if value.get("Type") == "Variable" and "Variable" in value:
            value = value["Variable"]
        require(isinstance(value, dict) and value.get("WFSerializationType") == "WFTextTokenAttachment"
                and isinstance(value.get("Value"), dict), "comparison attachment serialization")
        return value["Value"]

    def edge(value, source, cast=None):
        value = token(value)
        uid = p(source).get("UUID")
        expected = [] if cast is None else [
            {"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": cast}]
        require(isinstance(uid, str) and bool(uid)
                and value.get("Type") == "ActionOutput" and value.get("OutputUUID") == uid
                and value.get("Aggrandizements", []) == expected, "comparison producer edge")

    def text(value, template, source, cast=None):
        require(isinstance(value, dict) and value.get("WFSerializationType") == "WFTextTokenString",
                "comparison text serialization")
        payload = value.get("Value", {})
        position = template.index("\ufffc")
        refs = payload.get("attachmentsByRange", {})
        require(payload.get("string") == template and set(refs) == {"{%d, 1}" % position},
                "comparison exact text template")
        edge({"WFSerializationType": "WFTextTokenAttachment",
              "Value": refs["{%d, 1}" % position]}, source, cast)

    require(type(start) is int and type(condition_index) is int
            and 0 <= start <= condition_index < len(actions), "comparison region bounds")
    condition = actions[condition_index]
    cp = p(condition)
    require(kind(condition, "conditional") and cp.get("WFControlFlowMode") == 0
            and cp.get("WFConditionalLegacyComparisonBehavior") is True,
            "comparison controlling conditional")
    common_keys = {"GroupingIdentifier", "WFCondition", "WFConditionalLegacyComparisonBehavior",
                   "WFControlFlowMode", "WFInput", "UUID", "CustomOutputName"}
    require(set(cp) <= common_keys | ({"WFConditionalActionString"} if dynamic else {"WFNumberValue"}),
            "comparison controlling parameters")
    require(not (numeric and dynamic), "comparison mode")
    if numeric:
        require(start == condition_index and cp.get("WFCondition") == operator
                and number(cp.get("WFNumberValue"), right), "direct numeric comparison")
        edge(cp.get("WFInput"), left, coercion or "WFNumberContentItem")
        return

    if dynamic:
        # Dynamic values retain native comparison semantics. Callers
        # must opt in for the specific source; exact Text never uses this route.
        require(start == condition_index and operator in (4, 5)
                and cp.get("WFCondition") == operator, "direct dynamic comparison")
        edge(cp.get("WFInput"), left, coercion)
        if right_source is not None:
            text(cp.get("WFConditionalActionString"), "\ufffc", right_source)
        else:
            require(isinstance(right, str) and cp.get("WFConditionalActionString") == right,
                    "dynamic literal operand")
        return

    require(operator in (4, 5) and cp.get("WFCondition") == 4
            and number(cp.get("WFNumberValue"), 1), "exact Boolean result condition")
    region = range(start, condition_index)
    producers = {}
    for index in region:
        uid = p(actions[index]).get("UUID")
        if uid is not None:
            require(isinstance(uid, str) and uid and uid not in producers,
                    "comparison unique producer identity")
            producers[uid] = index

    def source(value, suffix, before):
        ref = token(value)
        require(ref.get("Type") == "ActionOutput" and not ref.get("Aggrandizements")
                and ref.get("OutputUUID") in producers, "comparison local dependency")
        index = producers[ref["OutputUUID"]]
        require(index < before and kind(actions[index], suffix), "comparison ordered " + suffix)
        return index, actions[index]

    result_index, result = source(cp.get("WFInput"), "conditional", condition_index)
    rp = p(result)
    group = rp.get("GroupingIdentifier")
    members = [(i, a) for i, a in enumerate(actions) if p(a).get("GroupingIdentifier") == group]
    require(bool(group) and len(members) == 3 and all(kind(a, "conditional") for _, a in members)
            and [p(a).get("WFControlFlowMode") for _, a in members] == [0, 1, 2]
            and members[-1][0] == result_index, "comparison Boolean control group")
    test_index, test = members[0]
    else_index, otherwise = members[1]
    require(test_index + 2 == else_index and else_index + 2 == result_index,
            "comparison Boolean arms")
    true_action, false_action = actions[test_index + 1], actions[else_index + 1]
    require(kind(true_action, "number") and set(p(true_action)) == {"WFNumberActionNumber"}
            and number(p(true_action).get("WFNumberActionNumber"), 1)
            and kind(false_action, "number") and set(p(false_action)) == {"WFNumberActionNumber"}
            and number(p(false_action).get("WFNumberActionNumber"), 0), "comparison Boolean polarity")
    tp = p(test)
    require(tp.get("WFCondition") == operator and number(tp.get("WFNumberValue"), 1)
            and tp.get("WFConditionalLegacyComparisonBehavior") is True,
            "comparison match cardinality predicate")
    count_index, count = source(tp.get("WFInput"), "count", test_index)
    require(p(count).get("WFCountType", "Items") == "Items", "comparison item cardinality")
    match_index, match = source(p(count).get("Input"), "text.match", count_index)
    require(p(match).get("WFMatchTextCaseSensitive") is True, "comparison case sensitivity")

    def text_parent(value, template, suffix, before):
        require(isinstance(value, dict) and value.get("WFSerializationType") == "WFTextTokenString",
                "comparison linked text serialization")
        payload = value.get("Value", {})
        position = template.index("\ufffc")
        refs = payload.get("attachmentsByRange", {})
        require(payload.get("string") == template and set(refs) == {"{%d, 1}" % position},
                "comparison linked exact template")
        return source({"WFSerializationType": "WFTextTokenAttachment",
                       "Value": refs["{%d, 1}" % position]}, suffix, before)

    left_index, left_snapshot = text_parent(p(match).get("text"), "\ufffc", "gettext", match_index)
    quote_index, quote = text_parent(p(match).get("WFMatchTextPattern"),
                                     "\\A\\Q\ufffc\\E\\z", "text.replace", match_index)
    qp = p(quote)
    require(qp.get("WFReplaceTextFind") == "\\E"
            and qp.get("WFReplaceTextReplace") == "\\E\\\\E\\Q"
            and qp.get("WFReplaceTextCaseSensitive") is True
            and qp.get("WFReplaceTextRegularExpression") is False,
            "comparison literal regex escaping")
    right_index, right_snapshot = text_parent(qp.get("WFInput"), "\ufffc", "gettext", quote_index)
    require(left_index < right_index < quote_index < match_index < count_index < test_index,
            "comparison operand evaluation order")
    text(p(left_snapshot).get("WFTextActionText"), "~\ufffc", left, coercion)
    if right_source is not None:
        text(p(right_snapshot).get("WFTextActionText"), "~\ufffc", right_source)
    else:
        require(isinstance(right, str) and p(right_snapshot).get("WFTextActionText") == "~" + right,
                "comparison exact literal operand")

    expected_indices = {left_index, right_index, quote_index, match_index, count_index,
                        test_index, test_index + 1, else_index, else_index + 1, result_index}
    require(expected_indices == set(region), "comparison has no unreviewed actions")
    allowed = (
        (left_snapshot, {"WFTextActionText"}), (right_snapshot, {"WFTextActionText"}),
        (quote, {"WFInput", "WFReplaceTextFind", "WFReplaceTextReplace",
                 "WFReplaceTextCaseSensitive", "WFReplaceTextRegularExpression"}),
        (match, {"text", "WFMatchTextPattern", "WFMatchTextCaseSensitive"}),
        (count, {"Input", "WFCountType"}),
        (test, {"GroupingIdentifier", "WFCondition", "WFConditionalLegacyComparisonBehavior",
                "WFControlFlowMode", "WFInput", "WFNumberValue"}),
        (otherwise, {"GroupingIdentifier", "WFControlFlowMode"}),
        (result, {"GroupingIdentifier", "WFControlFlowMode"}),
    )
    for action, keys in allowed:
        require(set(p(action)) <= keys | {"UUID", "CustomOutputName"},
                "comparison unreviewed parameter")
