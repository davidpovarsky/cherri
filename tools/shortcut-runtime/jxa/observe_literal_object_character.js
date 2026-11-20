// Synthetic, read-only WorkflowKit observations for the S01 literal-U+FFFC failure.
// Concatenate the pinned shortcutkit prelude before this file. No Shortcut runs here.
function observeTokenValue(value, methods) {
  const out = { className: cls(value) };
  for (const name of ["string", "serializedRepresentation"]) {
    if (methods.includes(name)) {
      try { out[name] = plain(value[name], 0); }
      catch (_) { out[name + "Error"] = "native property exception"; }
    }
  }
  return out;
}

function observeSerializationHops(className, initial) {
  const hops = [];
  let serialized = initial;
  for (let pass = 2; pass <= 3; pass++) {
    const row = { pass, input: serialized };
    try {
      const state = make(className, "initWithSerializedRepresentation:variableProvider:parameter:", $(serialized), null, null);
      row.accepted = !isNil(state);
      if (row.accepted) {
        row.serializedRepresentation = plain(state.serializedRepresentation, 0);
        serialized = row.serializedRepresentation;
      }
    } catch (_) { row.error = "native repeated serialization exception"; }
    hops.push(row);
    if (!row.accepted) break;
  }
  return hops;
}

function run(argv) {
  if (argv.length !== 1) throw new Error("Expected one output JSON path");
  const report = {
    schemaVersion: 1, engine: "WorkflowKit parameter-state API", engineLoaded: false,
    scope: "S01 literal object character serialization observations only",
    shortcutRuntime: false, mock: false, observations: [], directTokenObservations: [],
  };
  try {
    loadEngine();
    report.engineLoaded = true;
    const tokenMethods = hasClass("WFTextTokenString") ? methodNames("WFTextTokenString") : [];
    report.tokenSelectors = tokenMethods.filter(name => /string|serial|init|attribut/i.test(name)).sort();
    const ref = { Type: "Variable", VariableName: "S01PublicVariable" };
    const forms = [
      { name: "plain_literal_only", value: "\uFFFC" },
      { name: "omitted_ranges_literal_only", value: { WFSerializationType: "WFTextTokenString", Value: { string: "\uFFFC" } } },
      { name: "omitted_ranges_literal_unicode", value: { WFSerializationType: "WFTextTokenString", Value: { string: "L\uFFFC🙂éשָׁR" } } },
      { name: "plain_literal_with_unicode", value: "L\uFFFC🙂éשָׁR" },
      { name: "empty_token_literal_only", value: { WFSerializationType: "WFTextTokenString", Value: { string: "\uFFFC", attachmentsByRange: {} } } },
      { name: "ordinary_token_after_astral", value: { WFSerializationType: "WFTextTokenString", Value: { string: "🙂\uFFFC", attachmentsByRange: { "{2, 1}": ref } } } },
      { name: "literal_then_astral_then_token", value: { WFSerializationType: "WFTextTokenString", Value: { string: "\uFFFC🙂\uFFFC", attachmentsByRange: { "{3, 1}": ref } } } },
      { name: "token_then_literal", value: { WFSerializationType: "WFTextTokenString", Value: { string: "\uFFFCL\uFFFC", attachmentsByRange: { "{0, 1}": ref } } } },
      { name: "ordinary_full_token_coverage", value: { WFSerializationType: "WFTextTokenString", Value: { string: "\uFFFC🙂\uFFFC", attachmentsByRange: { "{0, 1}": ref, "{3, 1}": ref } } } },
      { name: "literal_backslash_escape", value: "\\ufffc" },
    ];
    for (const className of ["WFStringParameterState", "WFVariableStringParameterState"]) {
      if (!hasClass(className)) {
        report.observations.push({ className, unavailable: true });
        continue;
      }
      const selectors = methodNames(className);
      for (const form of forms) {
        const row = { className, name: form.name, input: form.value };
        try {
          const state = make(className, "initWithSerializedRepresentation:variableProvider:parameter:", $(form.value), null, null);
          row.accepted = !isNil(state);
          if (row.accepted) {
            row.serializedRepresentation = plain(state.serializedRepresentation, 0);
            row.roundTrips = observeSerializationHops(className, row.serializedRepresentation);
            for (const property of ["string", "value", "stringValue", "textTokenString"]) {
              if (!selectors.includes(property)) continue;
              try {
                const observed = state[property];
                row[property] = isA(observed, "WFTextTokenString")
                  ? observeTokenValue(observed, tokenMethods) : plain(observed, 0);
              } catch (_) { row[property + "Error"] = "native property exception"; }
            }
          }
        } catch (_) { row.error = "native parameter-state exception"; }
        report.observations.push(row);
      }
    }
    if (tokenMethods.includes("initWithString:")) {
      for (const value of ["\uFFFC", "L\uFFFC🙂éשָׁR", "\\ufffc"]) {
        const row = { initializer: "initWithString:", input: value };
        try {
          const token = make("WFTextTokenString", "initWithString:", $(value));
          row.accepted = !isNil(token);
          if (row.accepted) row.value = observeTokenValue(token, tokenMethods);
        } catch (_) { row.error = "native token-string exception"; }
        report.directTokenObservations.push(row);
      }
    }

    // Inspect actual typed Dictionary/Array states using only public literals.
    // Rejected or unavailable forms remain observations, never runtime evidence.
    report.typedContainerObservations = [];
    const literal = "L\uFFFC🙂éשָׁR";
    const literalKey = "K\uFFFCK";
    const token = text => ({ WFSerializationType: "WFTextTokenString", Value: { string: text, attachmentsByRange: {} } });
    const literalToken = text => ({ WFSerializationType: "WFTextTokenString", Value: { string: text } });
    const dict = rows => ({ WFSerializationType: "WFDictionaryFieldValue", Value: { WFDictionaryFieldValueItems: rows } });
    const array = rows => ({ WFSerializationType: "WFArrayParameterState", Value: rows });
    const textRow = value => ({ WFItemType: 0, WFValue: value });
    const fieldForms = [
      { name: "dictionary_omitted_ranges_value_key", value: dict([{ WFItemType: 0, WFKey: literalToken("key"), WFValue: literalToken(literal) }]) },
      { name: "dictionary_omitted_ranges_literal_key", value: dict([{ WFItemType: 0, WFKey: literalToken(literalKey), WFValue: literalToken("value") }]) },
      { name: "dictionary_nested_array_omitted_ranges", value: dict([{ WFItemType: 2, WFKey: literalToken("list"), WFValue: array([textRow(literalToken(literal))]) }]) },
      { name: "array_omitted_ranges_value", value: array([textRow(literalToken(literal))]) },
      { name: "list_omitted_ranges_value", value: [textRow(literalToken(literal))] },
      { name: "dictionary_plain_value_plain_key", value: dict([{ WFItemType: 0, WFKey: "key", WFValue: literal }]) },
      { name: "dictionary_plain_value_token_key", value: dict([{ WFItemType: 0, WFKey: token("key"), WFValue: literal }]) },
      { name: "dictionary_token_value_token_key", value: dict([{ WFItemType: 0, WFKey: token("key"), WFValue: token(literal) }]) },
      { name: "dictionary_plain_literal_key", value: dict([{ WFItemType: 0, WFKey: literalKey, WFValue: "value" }]) },
      { name: "dictionary_token_literal_key", value: dict([{ WFItemType: 0, WFKey: token(literalKey), WFValue: token("value") }]) },
      { name: "dictionary_nested_array_plain_value", value: dict([{ WFItemType: 2, WFKey: token("list"), WFValue: array([textRow(literal)]) }]) },
      { name: "dictionary_nested_array_token_value", value: dict([{ WFItemType: 2, WFKey: token("list"), WFValue: array([textRow(token(literal))]) }]) },
      { name: "array_plain_value", value: array([textRow(literal)]) },
      { name: "array_token_value", value: array([textRow(token(literal))]) },
      { name: "list_plain_value", value: [textRow(literal)] },
      { name: "list_token_value", value: [textRow(token(literal))] },
    ];
    for (const className of ["WFDictionaryParameterState", "WFArrayParameterState", "WFContentArrayParameterState"]) {
      if (!hasClass(className)) {
        report.typedContainerObservations.push({ className, unavailable: true });
        continue;
      }
      report.typedContainerSelectors = report.typedContainerSelectors || {};
      report.typedContainerSelectors[className] = methodNames(className).filter(name => /serial|init|items|pairs/i.test(name)).sort();
      for (const form of fieldForms) {
        const row = { className, name: form.name, input: form.value };
        try {
          const state = make(className, "initWithSerializedRepresentation:variableProvider:parameter:", $(form.value), null, null);
          row.accepted = !isNil(state);
          if (row.accepted) {
            row.serializedRepresentation = plain(state.serializedRepresentation, 0);
            row.roundTrips = observeSerializationHops(className, row.serializedRepresentation);
          }
        } catch (_) { row.error = "native container-state exception"; }
        report.typedContainerObservations.push(row);
      }
    }

    report.status = "OBSERVED";
  } catch (_) { report.status = "NOT_RUN"; report.error = "engine loading or observation failed"; }
  writeJSON(argv[0], report);
  if (report.status !== "OBSERVED") throw new Error(report.error);
  console.log("Observed " + report.observations.length + " parameter-state forms; no Shortcut runtime or acceptance credit");
}
