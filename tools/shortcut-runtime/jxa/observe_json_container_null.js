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
  if (argv.length === 3 && argv[1] === "date-field-parameters") return runDateFieldObserver(argv[0], argv[2]);
  if (argv.length !== 3) throw new Error("Expected output, frozen K body and S01 controls paths");
  const report = { schemaVersion: 1, scope: "JSON/null/container and S01 serialization observations only",
    engine: "WorkflowKit parameter-state API", engineLoaded: false, shortcutRuntime: false,
    httpExecuted: false, formalAcceptanceGranted: false, mock: false, status: "IN_PROGRESS",
    foundationControl: null, containerObservations: [], actionStateObservations: [], s01ControlObservations: [] };
  const checkpoint = () => writeJSON(argv[0], report);
  try {
    loadEngine(); report.engineLoaded = true;
    report.foundationControl = typedSnapshotControl();
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

function typedSnapshotControl() {
  let deep = { number: 17, boolean: true, explicitNull: null, emptyText: "", emptyArray: [], emptyMap: {} };
  for (let level = 0; level < 14; level++) deep = { level, nested: [deep] };
  const control = { number: 0, boolean: false, explicitNull: null, emptyText: "", deep };
  const snapshot = nativeJSONSnapshot($(control));
  const nullSnapshot = nativeJSONSnapshot($.NSNull.null);
  return { passed: snapshot.jsonAvailable && sameJSON(snapshot.jsonValue, control) &&
      nullSnapshot.jsonAvailable && nullSnapshot.jsonValue === null && nullSnapshot.nsNull === true,
    observation: snapshot, nsNullObservation: nullSnapshot, absentKey: "omittedControlKey",
    omittedKeyRemainsAbsent: snapshot.jsonAvailable && !Object.prototype.hasOwnProperty.call(snapshot.jsonValue, "omittedControlKey") };
}

function dateFieldSpecimens(fixture) {
  const copy = value => JSON.parse(JSON.stringify(value));
  if (fixture.schema_version !== 1 || fixture.source_sha256 !== "91e84ce74eae0adbbbbe2c88efb560de812289190b0de44644620dee5094c2ba" ||
      fixture.positive_seconds.WFWorkflowActionIdentifier !== "is.workflow.actions.gettimebetweendates" ||
      fixture.format_start.WFWorkflowActionIdentifier !== "is.workflow.actions.format.date") throw new Error("Date fixture identity");
  const interval = fixture.positive_seconds.WFWorkflowActionParameters;
  const format = fixture.format_start.WFWorkflowActionParameters;
  const date = name => {
    const matches = fixture.date_producers.filter(action => action.WFWorkflowActionParameters.CustomOutputName === name);
    if (matches.length !== 1) throw new Error("Date producer identity");
    const action = matches[0], p = action.WFWorkflowActionParameters;
    if (action.WFWorkflowActionIdentifier !== "is.workflow.actions.date" || p.WFDateActionMode !== "Specified Date") throw new Error("Date producer mode");
    return p;
  };
  const start = date("startDate"), end = date("laterDate");
  const attachment = (value, producer) => {
    if (value.WFSerializationType !== "WFTextTokenAttachment" || value.Value.Type !== "ActionOutput" ||
        value.Value.OutputUUID !== producer.UUID || value.Value.OutputName !== producer.CustomOutputName) throw new Error("Date original reference");
    return value;
  };
  const first = attachment(interval.WFTimeUntilFromDate, start), second = attachment(interval.WFInput, end);
  const wrap = value => ({ WFSerializationType: "WFTextTokenString",
    Value: { string: "\uFFFC", attachmentsByRange: { "{0, 1}": copy(value.Value) } } });
  if (interval.WFTimeUntilUnit !== "Seconds" || !sameJSON(format.WFDate, wrap(first))) throw new Error("Date positive comparison identity");
  const direct = [];
  for (const [name, value, producer] of [["first", first, start], ["second", second, end]]) {
    direct.push({ name: name + "_original_bare_attachment", input: copy(value), form: "actual_local_compiler" });
    direct.push({ name: name + "_token_string_reference", input: wrap(value), form: "existing_native_probe_form" });
    direct.push({ name: name + "_literal_date_control", input: producer.WFDateActionDate, form: "literal_control" });
  }
  const cases = [], keys = ["WFTimeUntilFromDate", "WFInput"];
  const add = (name, parameters, form) => cases.push({ name, actionIdentifier: "is.workflow.actions.gettimebetweendates",
    parameters, keys: keys.slice(), form });
  add("interval_original_bare", copy(interval), "actual_local_compiler");
  add("interval_both_token_strings", Object.assign(copy(interval), { WFTimeUntilFromDate: wrap(first), WFInput: wrap(second) }), "existing_native_probe_form");
  add("interval_first_token_only", Object.assign(copy(interval), { WFTimeUntilFromDate: wrap(first) }), "mixed_control");
  add("interval_second_token_only", Object.assign(copy(interval), { WFInput: wrap(second) }), "mixed_control");
  add("interval_literal_dates", Object.assign(copy(interval), { WFTimeUntilFromDate: start.WFDateActionDate, WFInput: end.WFDateActionDate }), "literal_control");
  const absent = copy(interval); delete absent.WFTimeUntilFromDate; delete absent.WFInput;
  add("interval_absent_date_parameters", absent, "missing_parameter_control");
  cases.push({ name: "format_original_token_string", actionIdentifier: "is.workflow.actions.format.date", parameters: copy(format), keys: ["WFDate"], form: "actual_local_compiler" });
  cases.push({ name: "format_bare_attachment_control", actionIdentifier: "is.workflow.actions.format.date",
    parameters: Object.assign(copy(format), { WFDate: copy(first) }), keys: ["WFDate"], form: "changed_form_control" });
  return { direct, cases };
}

function findDateMetadataActions() {
  const allowed = ["is.workflow.actions.gettimebetweendates", "is.workflow.actions.format.date"];
  const found = {};
  const actions = items(klass("WFBundledActionProvider").alloc.init.createAllAvailableActionsIncludingMissingActions(true));
  for (let i = 0; i < count(actions); i++) {
    const action = actions.objectAtIndex(i), identifier = str(action.identifier);
    if (!allowed.includes(identifier) || !!action.isMissing) continue;
    if (found[identifier]) throw new Error("Duplicate Date action identity");
    const parameters = {}, nativeParameters = action.parameters;
    for (let j = 0; j < count(nativeParameters); j++) {
      const parameter = nativeParameters.objectAtIndex(j);
      parameters[str(parameter.key)] = parameter;
    }
    found[identifier] = { action, parameters };
  }
  return found;
}

function runDateFieldObserver(outputPath, fixturePath) {
  const report = { schemaVersion: 1, scope: "Date parameter-state serialization observations only",
    engine: "WorkflowKit metadata and parameter-state API", engineLoaded: false, shortcutRuntime: false,
    httpExecuted: false, formalAcceptanceGranted: false, mock: false, status: "IN_PROGRESS",
    foundationControl: null, directStateObservations: [], actionStateObservations: [] };
  const checkpoint = () => writeJSON(outputPath, report);
  try {
    loadEngine(); report.engineLoaded = true;
    report.foundationControl = typedSnapshotControl();
    if (!report.foundationControl.passed || !report.foundationControl.omittedKeyRemainsAbsent) throw new Error("Foundation snapshot control");
    const fixture = readJSON(fixturePath), specimens = dateFieldSpecimens(fixture);
    report.inputProvenance = { compilerSourceSha: fixture.compiler_source_sha, runtimeSourceSha: fixture.runtime_source_sha,
      sourceSha256: fixture.source_sha256, localUnsignedSha256: fixture.local_unsigned_sha256,
      scope: fixture.scope, nativeUnsignedBytesDownloaded: false };
    report.directSpecimenCount = specimens.direct.length;
    for (const specimen of specimens.direct) {
      const row = { specimen, className: "WFDateFieldParameterState", unavailable: !hasClass("WFDateFieldParameterState") };
      if (!row.unavailable) row.hops = captureNativeHops(row.className, $(specimen.input), null);
      report.directStateObservations.push(row); checkpoint();
    }
    const found = findDateMetadataActions();
    report.metadataActionIdentifiers = Object.keys(found).sort();
    for (const specimen of specimens.cases) {
      const entry = found[specimen.actionIdentifier];
      for (const key of specimen.keys) {
        const row = { name: specimen.name, form: specimen.form, actionIdentifier: specimen.actionIdentifier,
          parameterKey: key, copyInput: specimen.parameters, actionExecuted: false,
          actionAvailable: !!entry, missingParameterControl: !Object.prototype.hasOwnProperty.call(specimen.parameters, key) };
        const parameter = entry && entry.parameters[key];
        row.parameterAvailable = !!parameter;
        if (parameter) {
          try {
            row.parameterClass = cls(parameter);
            row.declaredStateClass = str($.NSStringFromClass(parameter.singleStateClass));
            row.defaultSerializedRepresentation = nativeJSONSnapshot(parameter.defaultSerializedRepresentation);
            const copied = entry.action.copyWithSerializedParameters($(specimen.parameters));
            row.copyAccepted = !isNil(copied);
            if (row.copyAccepted) {
              const state = copied.parameterStateForKey(key);
              row.parameterStatePresent = !isNil(state);
              if (row.parameterStatePresent) row.hops = captureNativeHops(cls(state),
                row.missingParameterControl ? null : $(specimen.parameters[key]), parameter, state);
            }
          } catch (_) { row.error = "native Date action copy or parameter-state exception"; }
        }
        report.actionStateObservations.push(row); checkpoint();
      }
    }
    report.status = "OBSERVED";
  } catch (_) { report.status = "FAILED_OBSERVATION"; report.error = "Date metadata, snapshot control or observation setup failed"; }
  checkpoint();
  if (report.status !== "OBSERVED") throw new Error(report.error);
  console.log("Observed Date parameter states only; no Shortcut execution or formal acceptance credit");
}
