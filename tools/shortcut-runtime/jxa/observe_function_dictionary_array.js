// Fixed synthetic WorkflowKit observations. Concatenate the exact pinned prelude.
// This does not run, import, sign, or modify a Shortcut.
function observationError(error) { return String(error).slice(0, 2048); }

function advertised(value, selector) {
  try { return !!value.respondsToSelector(selector); }
  catch (error) { return false; }
}

function inspectNative(value, depth) {
  const out = { className: cls(value), objcNil: isNil(value) };
  if (out.objcNil) return out;
  const methods = methodNames(out.className);
  if (advertised(value, "serializedRepresentation")) {
    try {
      const serialized = value.serializedRepresentation;
      out.serialized = { className: cls(serialized), objcNil: isNil(serialized) };
      if (!isNil(serialized)) out.serialized.value = plain(serialized, 0);
    } catch (error) { out.serializationError = observationError(error); }
  }
  if (depth >= 5) { out.depthLimitReached = true; return out; }
  if (isA(value, "NSArray")) {
    out.count = Number(value.count);
    out.items = [];
    if (out.count > 16) { out.truncated = true; return out; }
    for (let index = 0; index < out.count; index++) {
      out.items.push(inspectNative(value.objectAtIndex(index), depth + 1));
    }
    return out;
  }
  // Read only advertised, argument-free value accessors. Preserve unavailable
  // selectors instead of inventing an initializer or claiming a nested read.
  out.properties = {};
  for (const name of ["dictionaryFieldValue", "keyValuePairs", "items", "value", "values"]) {
    if (!advertised(value, name)) continue;
    try { out.properties[name] = inspectNative(value[name], depth + 1); }
    catch (error) { out.properties[name] = { error: observationError(error) }; }
  }
  return out;
}

function observeDictionaryAction(dictionary, journal) {
  const result = { surface: "full-dictionary-action", identifier: "is.workflow.actions.dictionary",
    parameters: { UUID: "AAAAAAAA-BBBB-4CCC-8DDD-EEEEEEEEEEEE", WFItems: dictionary },
    actionFound: false, copied: false, parameterRead: false };
  journal("before-action-provider", {});
  try {
    // This exact provider/copy/state entry point is the existing committed
    // probe_single_action.js path. It never executes the action.
    const provider = klass("WFBundledActionProvider").alloc.init;
    const allActions = items(provider.createAllAvailableActionsIncludingMissingActions(true));
    let action = null;
    for (let index = 0; index < count(allActions); index++) {
      const candidate = allActions.objectAtIndex(index);
      if (str(candidate.identifier) === result.identifier) { action = candidate; break; }
    }
    if (!action) { result.unavailable = "ACTION_NOT_FOUND"; return result; }
    result.actionFound = true;
    result.actionClass = cls(action);
    result.isMissing = !!action.isMissing;
    result.actionSelectors = methodNames(result.actionClass).filter(name => /serial|parameter|copyWith/.test(name)).sort();
    if (result.isMissing) { result.unavailable = "MISSING_ACTION"; return result; }
    const parameters = action.parameters;
    for (let index = 0; index < count(parameters); index++) {
      const parameter = parameters.objectAtIndex(index);
      if (str(parameter.key) !== "WFItems") continue;
      result.parameterMetadata = { className: cls(parameter), key: str(parameter.key),
        singleStateClass: str($.NSStringFromClass(parameter.singleStateClass)),
        defaultSerializedRepresentation: plain(parameter.defaultSerializedRepresentation, 0) };
    }
    journal("before-action-copy", { identifier: result.identifier, parameters: result.parameters });
    const copied = action.copyWithSerializedParameters($(result.parameters));
    result.copied = !isNil(copied);
    journal("after-action-copy", { copied: result.copied });
    if (!result.copied) return result;
    const state = copied.parameterStateForKey("WFItems");
    result.parameterRead = !isNil(state);
    journal("after-parameter-read", { parameterRead: result.parameterRead, className: cls(state) });
    result.parameterObservation = inspectNative(state, 0);
    journal("after-parameter-observation", { observation: result.parameterObservation });
    result.actionObservation = inspectNative(copied, 0);
  } catch (error) { result.error = observationError(error); }
  journal("after-action-observation", { result });
  return result;
}

function observeConstructor(className, input, journal) {
  const row = { className, input, available: hasClass(className), accepted: false };
  if (!row.available) return row;
  const methods = methodNames(className);
  row.relevantSelectors = methods.filter(name => /serial|initWith|dictionaryFieldValue|keyValuePairs|^value$|^items$/.test(name)).sort();
  const candidates = [
    ["initWithSerializedRepresentation:variableProvider:parameter:", 3],
    ["initWithSerializedRepresentation:variableProvider:", 2],
    ["initWithSerializedRepresentation:", 1],
  ];
  const selected = candidates.find(item => methods.includes(item[0]));
  if (!selected) { row.initializerUnavailable = true; return row; }
  row.initializer = selected[0];
  journal("before-constructor", { className, initializer: row.initializer });
  try {
    const nativeInput = $(input);
    row.inputNativeClass = cls(nativeInput);
    let state;
    if (selected[1] === 3) state = make(className, selected[0], nativeInput, null, null);
    else if (selected[1] === 2) state = make(className, selected[0], nativeInput, null);
    else state = make(className, selected[0], nativeInput);
    row.accepted = !isNil(state);
    journal("after-constructor", { className, accepted: row.accepted });
    if (row.accepted) row.observation = inspectNative(state, 0);
  } catch (error) { row.error = observationError(error); }
  journal("after-observation", { className, accepted: row.accepted, error: row.error || null });
  return row;
}

function run(argv) {
  if (argv.length !== 3) throw new Error("Expected output directory, fixed inputs, and case id");
  const inputText = ObjC.unwrap($.NSString.stringWithContentsOfFileEncodingError(argv[1], 4, null));
  const inputs = JSON.parse(inputText);
  const selected = inputs.cases.filter(item => item.id === argv[2]);
  if (inputs.schema !== "cherri-function-array-native-probe-inputs/1" || selected.length !== 1) {
    throw new Error("Fixed input identity missing");
  }
  const item = selected[0];
  const report = {
    schema: "cherri-function-array-native-observation/1", caseId: item.id,
    engine: "WorkflowKit action and parameter-state API", engineLoaded: false,
    shortcutInvocation: false, simulatorInvocation: false, nativeAcceptanceGranted: false,
    sourceInput: item, observations: [], status: "NOT_OBSERVED",
  };
  let sequence = 0;
  const journal = (stage, detail) => {
    sequence++;
    writeJSON(argv[0] + "/" + String(sequence).padStart(3, "0") + "-" + stage + ".original.json",
      { schema: "cherri-native-probe-stage/1", caseId: item.id, sequence, stage, detail });
  };
  journal("engine-start", {});
  try {
    loadEngine();
    report.engineLoaded = true;
    const row = { WFKey: { WFSerializationType: "WFTextTokenString", Value: { string: "arguments" } }, WFItemType: 2, WFValue: item.arrayValue };
    const dictionary = { WFSerializationType: "WFDictionaryFieldValue", Value: { WFDictionaryFieldValueItems: [row] } };
    const actionObservation = observeDictionaryAction(dictionary, journal);
    report.observations.push(actionObservation);
    writeJSON(argv[0] + "/action-observation.original.json", actionObservation);
    for (const request of [
      ["WFArrayParameterState", item.arrayValue],
      ["WFDictionaryParameterKeyValuePair", row],
      ["WFDictionaryFieldValue", dictionary],
      ["WFDictionaryParameterState", dictionary],
    ]) {
      const observed = observeConstructor(request[0], request[1], journal);
      report.observations.push(observed);
      writeJSON(argv[0] + "/" + request[0] + "-observation.original.json", observed);
    }
    report.status = "OBSERVED";
  } catch (error) { report.error = observationError(error); }
  writeJSON(argv[0] + "/observations.original.json", report);
  if (report.status !== "OBSERVED") throw new Error(report.error || "Observation incomplete");
  console.log("Observed fixed case " + item.id + "; no Shortcut invocation or acceptance credit");
}
