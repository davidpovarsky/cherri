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
    report.status = "OBSERVED";
  } catch (_) { report.status = "NOT_RUN"; report.error = "engine loading or observation failed"; }
  writeJSON(argv[0], report);
  if (report.status !== "OBSERVED") throw new Error(report.error);
  console.log("Observed " + report.observations.length + " parameter-state forms; no Shortcut runtime or acceptance credit");
}
