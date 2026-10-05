// tools/shortcut-runtime/jxa/probe_single_action.js
// Probes parameter encodings and serialization for a single action (or a single parameter of that action).
// Requires jxa-prelude.js concatenated in front.
//
// Usage: osascript -l JavaScript runner.js <action_identifier> <out_path> [target_param_key]

const VARIABLE_FORMS = [
  ["ActionOutput", { Type: "ActionOutput", OutputUUID: "11111111-2222-3333-4444-555555555555", OutputName: "Output" }],
  ["ExtensionInput", { Type: "ExtensionInput" }],
  ["Variable", { Type: "Variable", VariableName: "MyVar" }],
  ["Clipboard", { Type: "Clipboard" }],
  ["CurrentDate", { Type: "CurrentDate" }],
  ["Ask", { Type: "Ask", Prompt: "Which?" }],
  ["DeviceDetails", { Type: "DeviceDetails" }],
  ["CurrentApp", { Type: "CurrentApp" }],
].map(([type, value]) => [type, { WFSerializationType: "WFTextTokenAttachment", Value: value }]);

const TEXT_STATES = new Set(["WFVariableStringParameterState", "WFURLStringParameterState", "WFDateFieldParameterState"]);
const tokenString = (att) => ({ WFSerializationType: "WFTextTokenString", Value: { string: "\uFFFC", attachmentsByRange: { "{0, 1}": att.Value } } });

const PLAIN_SAMPLE = {
  WFBooleanSubstitutableState: true,
  WFNumberSubstitutableState: 5,
  WFNumberStringSubstitutableState: 5,
  WFNumberParameterState: 5,
  WFStringSubstitutableState: "hello",
  WFStringParameterState: "hello",
  WFVariableStringParameterState: "hello",
  WFURLStringParameterState: "https://example.com",
  WFDateFieldParameterState: "hello"
};

const isPlain = (v) => typeof v === "string" || typeof v === "number" || typeof v === "boolean";

function run(argv) {
  if (argv.length < 2) {
    throw new Error("usage: probe_single_action.js <action_identifier> <out_path> [target_param_key]");
  }
  loadEngine();
  const targetId = argv[0];
  const outPath = argv[1];
  const targetKey = argv.length > 2 ? argv[2] : null;

  const provider = klass("WFBundledActionProvider").alloc.init;
  let action = null;
  try {
    const allActions = items(provider.createAllAvailableActionsIncludingMissingActions(true));
    for (let i = 0; i < count(allActions); i++) {
      const a = allActions.objectAtIndex(i);
      if (str(a.identifier) === targetId) { action = a; break; }
    }
  } catch (e) {}

  if (!action) {
    try {
      const intentActions = items(klass("WFIntentActionProvider").alloc.init.createAllAvailableActions);
      for (let i = 0; i < count(intentActions); i++) {
        const a = intentActions.objectAtIndex(i);
        if (str(a.identifier) === targetId) { action = a; break; }
      }
    } catch (e) {}
  }

  if (!action) {
    writeJSON(outPath, {
      identifier: targetId,
      status: "unavailable",
      isMissing: true,
      error: "Action not found in providers",
      parameters: []
    });
    return;
  }

  let isMissing = false;
  try { isMissing = !!action.isMissing; } catch (e) {}
  let outputName = null;
  try { const n = action.outputName; if (!isNil(n)) outputName = str(n); } catch (e) {}

  let plist = null;
  try { plist = action.parameters; } catch (e) {
    writeJSON(outPath, {
      identifier: targetId,
      status: "error",
      isMissing: isMissing,
      error: "Failed to read action parameters: " + e.message,
      parameters: []
    });
    return;
  }

  const parameters = [];
  const defaults = {};

  if (plist) {
    for (let j = 0; j < count(plist); j++) {
      const p = plist.objectAtIndex(j);
      const pc = cls(p);
      const rawKey = p.key;
      if (isNil(rawKey) || !isA(rawKey, "NSString")) continue;
      const k = str(rawKey);

      if (targetKey && k !== targetKey) continue;

      let variableTypes = null;
      try {
        const sv = p.supportedVariableTypes;
        variableTypes = isNil(sv) ? [] : plain(sv.allObjects, 0).slice().sort();
      } catch (e) {
        variableTypes = null;
      }

      let sc = null, dflt = null;
      try { sc = str($.NSStringFromClass(p.singleStateClass)); } catch (e) {}
      try { dflt = plain(p.defaultSerializedRepresentation, 0); } catch (e) {}

      if (dflt !== null && dflt !== undefined) {
        defaults[k] = dflt;
      }

      const paramEntry = {
        key: k,
        class: pc,
        singleStateClass: sc,
        default: dflt,
        variableTypes: variableTypes,
        reads: [],
        readsPlain: false,
        plainSample: null
      };

      if (!isMissing) {
        // Probe variable forms
        for (const [type, att] of VARIABLE_FORMS) {
          const value = TEXT_STATES.has(sc) ? tokenString(att) : att;
          let ok = false;
          try {
            const a = action.copyWithSerializedParameters($({ UUID: "00000000-0000-4000-8000-000000000001", [k]: value }));
            ok = !isNil(a) && !isNil(a.parameterStateForKey(k));
          } catch (e) {
            ok = false;
          }
          if (ok) paramEntry.reads.push(type);
        }

        // Probe plain value
        let choice;
        try {
          if (p.respondsToSelector("possibleStates")) {
            const ps = p.possibleStates;
            if (!isNil(ps) && count(ps) > 0) {
              const v = plain(ps.objectAtIndex(0).serializedRepresentation, 0);
              if (isPlain(v)) choice = v;
            }
          }
        } catch (e) {}

        const sample = isPlain(dflt) ? dflt : (choice !== undefined ? choice : PLAIN_SAMPLE[sc]);
        if (sample !== undefined) {
          paramEntry.plainSample = sample;
          let ok = false;
          try {
            const a = action.copyWithSerializedParameters($({ UUID: "00000000-0000-4000-8000-000000000002", [k]: sample }));
            ok = !isNil(a) && !isNil(a.parameterStateForKey(k));
          } catch (e) {
            ok = false;
          }
          paramEntry.readsPlain = ok;
        }
      }

      parameters.push(paramEntry);
    }
  }

  writeJSON(outPath, {
    identifier: targetId,
    status: isMissing ? "unavailable" : "success",
    isMissing: isMissing,
    outputName: outputName,
    parameters: parameters,
    defaults: defaults
  });
}
