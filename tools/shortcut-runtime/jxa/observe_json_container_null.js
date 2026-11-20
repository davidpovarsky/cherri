// Public serialization observations only. This file never executes a Shortcut or HTTP action.
// Concatenate the unchanged pinned shortcutkit prelude before this file.
const NATIVE_STATE_INITIALIZER = "initWithSerializedRepresentation:variableProvider:parameter:";

function canonicalJSON(value) {
  if (Array.isArray(value)) return value.map(canonicalJSON);
  if (value && typeof value === "object") {
    const result = {};
    for (const key of Object.keys(value).sort()) result[key] = canonicalJSON(value[key]);
    return result;
  }
  return value;
}

function sameJSON(a, b) { return JSON.stringify(canonicalJSON(a)) === JSON.stringify(canonicalJSON(b)); }

// Do not use prelude plain(): it stringifies NSNumber and truncates deep values.
// Foundation encodes the actual object; the one-element array permits scalar/null snapshots.
function nativeJSONSnapshot(value) {
  if (isNil(value)) return { objcNil: true, jsonAvailable: false };
  const out = { objcNil: false, className: cls(value), nsNull: isA(value, "NSNull"), jsonAvailable: false };
  try {
    const error = Ref();
    const data = $.NSJSONSerialization.dataWithJSONObjectOptionsError($.NSArray.arrayWithObject(value), 1 | 2, error);
    if (isNil(data)) { out.error = "native value is not JSON serializable"; return out; }
    const text = $.NSString.alloc.initWithDataEncoding(data, 4);
    if (isNil(text)) { out.error = "native JSON UTF8 decoding failed"; return out; }
    const wrapper = JSON.parse(str(text));
    if (!Array.isArray(wrapper) || wrapper.length !== 1) throw new Error("snapshot wrapper");
    out.jsonAvailable = true;
    out.jsonValue = wrapper[0];
  } catch (_) { out.error = "native JSON snapshot exception"; }
  return out;
}

function captureNativeHops(className, initialNative, parameter, firstState) {
  const hops = [];
  let nextNative = initialNative;
  for (let pass = 1; pass <= 3; pass++) {
    const row = { pass, input: nativeJSONSnapshot(nextNative),
      inputTransport: pass === 1 ? "initial native input" : "unmodified previous native serializedRepresentation" };
    try {
      const state = pass === 1 && firstState !== undefined
        ? firstState : make(className, NATIVE_STATE_INITIALIZER, nextNative, null, parameter);
      row.accepted = !isNil(state);
      if (row.accepted) {
        row.stateClass = cls(state);
        // Preserve this native object for the next initializer. Snapshot JSON is never fed back.
        const serializedNative = state.serializedRepresentation;
        row.serializedRepresentation = nativeJSONSnapshot(serializedNative);
        nextNative = serializedNative;
      }
    } catch (_) { row.error = "native state initializer or serialization exception"; }
    hops.push(row);
    if (!row.accepted || row.error) break;
  }
  return hops;
}

function publicJSONSpecimens(body) {
  const copy = value => JSON.parse(JSON.stringify(value));
  const dict = rows => ({ WFSerializationType: "WFDictionaryFieldValue", Value: { WFDictionaryFieldValueItems: rows } });
  const array = rows => ({ WFSerializationType: "WFArrayParameterState", Value: rows });
  const rows = body.Value.WFDictionaryFieldValueItems;
  const field = (items, key) => {
    const matches = items.filter(row => row.WFKey.Value.string === key);
    if (matches.length !== 1) throw new Error("pinned field identity");
    return matches[0];
  };
  const empty = field(rows, "empty_val");
  const nested = field(rows, "nested");
  const matrix = field(nested.WFValue.Value.WFDictionaryFieldValueItems, "matrix");
  if (body.WFSerializationType !== "WFDictionaryFieldValue" || empty.WFItemType !== 0 ||
      empty.WFValue.WFSerializationType !== "WFTextTokenString" || empty.WFValue.Value.string !== "" ||
      matrix.WFItemType !== 2 || matrix.WFValue.WFSerializationType !== "WFArrayParameterState") {
    throw new Error("frozen K specimen shape changed");
  }
  const missing = copy(empty); delete missing.WFValue;
  const explicitNull = copy(empty); explicitNull.WFValue = null;
  const type5Missing = copy(missing); type5Missing.WFItemType = 5;
  const type5Null = copy(explicitNull); type5Null.WFItemType = 5;
  const result = [];
  function add(name, className, input, candidate, actionRelevant) {
    result.push({ name, className, inputKind: "json", input, candidateWire: candidate, actionRelevant });
  }
  add("frozen_k_complete_WFJSONValues", "WFDictionaryParameterState", copy(body), false, true);
  add("frozen_k_authored_nil_emitted_as_empty_text", "WFDictionaryParameterState", dict([copy(empty)]), false, true);
  add("frozen_k_nested_matrix_field", "WFDictionaryParameterState", dict([copy(matrix)]), false, true);
  add("missing_empty_val_key_control", "WFDictionaryParameterState", dict([]), false, true);
  add("missing_WFValue_candidate", "WFDictionaryParameterState", dict([missing]), true, true);
  add("explicit_json_null_WFValue_candidate", "WFDictionaryParameterState", dict([explicitNull]), true, true);
  add("item_type5_missing_WFValue_candidate", "WFDictionaryParameterState", dict([type5Missing]), true, true);
  add("item_type5_json_null_WFValue_candidate", "WFDictionaryParameterState", dict([type5Null]), true, true);
  add("frozen_k_nested_matrix_array", "WFArrayParameterState", copy(matrix.WFValue), false, false);
  add("empty_array_control", "WFArrayParameterState", array([]), false, false);
  const emptyItem = copy(empty); delete emptyItem.WFKey;
  add("empty_text_array_item_control", "WFArrayParameterState", array([emptyItem]), false, false);
  add("explicit_json_null_array_item_candidate", "WFArrayParameterState", array([null]), true, false);
  for (const className of ["WFDictionaryParameterState", "WFArrayParameterState"]) {
    for (const inputKind of ["objc_nil", "nsnull"]) {
      result.push({ name: className + "_" + inputKind + "_initializer", className, inputKind,
                    candidateWire: false, actionRelevant: className === "WFDictionaryParameterState" });
    }
  }
  return result;
}

function initialNative(specimen) {
  if (specimen.inputKind === "objc_nil") return null;
  if (specimen.inputKind === "nsnull") return $.NSNull.null;
  return $(specimen.input);
}

function findJSONAction() {
  const provider = klass("WFBundledActionProvider").alloc.init;
  const actions = items(provider.createAllAvailableActionsIncludingMissingActions(true));
  for (let index = 0; index < count(actions); index++) {
    const action = actions.objectAtIndex(index);
    if (str(action.identifier) !== "is.workflow.actions.downloadurl") continue;
    if (!!action.isMissing) return { missing: true };
    const parameters = action.parameters;
    for (let pos = 0; pos < count(parameters); pos++) {
      const parameter = parameters.objectAtIndex(pos);
      if (str(parameter.key) === "WFJSONValues") return { action, parameter };
    }
  }
  return null;
}

function run(argv) {
  if (argv.length !== 3) throw new Error("Expected output, frozen K body and S01 controls paths");
  const report = { schemaVersion: 1, scope: "JSON/null/container and S01 serialization observations only",
    engine: "WorkflowKit parameter-state API", engineLoaded: false, shortcutRuntime: false,
    httpExecuted: false, formalAcceptanceGranted: false, mock: false, status: "IN_PROGRESS",
    foundationControl: null, containerObservations: [], actionStateObservations: [], s01ControlObservations: [] };
  const checkpoint = () => writeJSON(argv[0], report);
  try {
    loadEngine(); report.engineLoaded = true;
    let deep = { number: 17, boolean: true, explicitNull: null, emptyText: "", emptyArray: [], emptyMap: {} };
    for (let level = 0; level < 14; level++) deep = { level, nested: [deep] };
    const control = { number: 0, boolean: false, explicitNull: null, emptyText: "", deep };
    const snapshot = nativeJSONSnapshot($(control));
    const nullSnapshot = nativeJSONSnapshot($.NSNull.null);
    report.foundationControl = { passed: snapshot.jsonAvailable && sameJSON(snapshot.jsonValue, control) &&
        nullSnapshot.jsonAvailable && nullSnapshot.jsonValue === null && nullSnapshot.nsNull === true,
      observation: snapshot, nsNullObservation: nullSnapshot, absentKey: "omittedControlKey",
      omittedKeyRemainsAbsent: snapshot.jsonAvailable && !Object.prototype.hasOwnProperty.call(snapshot.jsonValue, "omittedControlKey") };
    if (!report.foundationControl.passed || !report.foundationControl.omittedKeyRemainsAbsent) {
      throw new Error("Foundation typed snapshot control failed");
    }
    const specimens = publicJSONSpecimens(readJSON(argv[1]));
    report.specimenCount = specimens.length;
    for (const specimen of specimens) {
      const row = { specimen, unavailable: !hasClass(specimen.className) };
      if (!row.unavailable) row.hops = captureNativeHops(specimen.className, initialNative(specimen), null);
      report.containerObservations.push(row); checkpoint();
    }
    let found = null;
    try { found = findJSONAction(); } catch (_) { report.actionStateAvailabilityError = "native action metadata unavailable"; }
    report.actionStateAvailable = found !== null && !found.missing;
    report.actionIsMissing = found !== null && !!found.missing;
    if (report.actionStateAvailable) {
      for (const specimen of specimens.filter(value => value.actionRelevant)) {
        const row = { name: specimen.name, candidateWire: specimen.candidateWire,
          actionIdentifier: "is.workflow.actions.downloadurl", parameterKey: "WFJSONValues",
          actionExecuted: false, inputKind: specimen.inputKind };
        try {
          const parameters = { UUID: "00000000-0000-4000-8000-000000000003", WFHTTPMethod: "POST", WFHTTPBodyType: "JSON" };
          if (specimen.inputKind !== "objc_nil") parameters.WFJSONValues = specimen.inputKind === "nsnull" ? null : specimen.input;
          row.copyInput = parameters;
          row.missingParameterControl = specimen.inputKind === "objc_nil";
          const copied = found.action.copyWithSerializedParameters($(parameters));
          row.copyAccepted = !isNil(copied);
          if (row.copyAccepted) {
            const state = copied.parameterStateForKey("WFJSONValues");
            row.parameterStatePresent = !isNil(state);
            if (row.parameterStatePresent) row.hops = captureNativeHops(cls(state), initialNative(specimen), found.parameter, state);
          }
        } catch (_) { row.error = "native action copy or parameter-state exception"; }
        report.actionStateObservations.push(row); checkpoint();
      }
    }
    const controls = readJSON(argv[2]);
    if (controls.controls.length !== 7) throw new Error("S01 control selection changed");
    report.s01AuthoredSource = { commit: controls.sourceCommit, path: controls.sourcePath, sha256: controls.sourceSha256 };
    for (const control of controls.controls) {
      for (const className of control.classes) {
        const row = { name: control.name, className, authoredInput: control.input, unavailable: !hasClass(className) };
        if (!row.unavailable) row.hops = captureNativeHops(className, $(control.input), null);
        report.s01ControlObservations.push(row); checkpoint();
      }
    }
    report.status = "OBSERVED";
  } catch (_) { report.status = "FAILED_OBSERVATION"; report.error = "engine, snapshot control or observation setup failed"; }
  checkpoint();
  if (report.status !== "OBSERVED") throw new Error(report.error);
  console.log("Observed public parameter states using direct native-object hops; no Shortcut, HTTP or formal acceptance credit");
}
