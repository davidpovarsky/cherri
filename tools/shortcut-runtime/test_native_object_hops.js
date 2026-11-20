// Causal transport/driver controls in Node. These are not native observations.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const crypto = require("node:crypto");
const root = path.resolve(__dirname, "../..");
const script = fs.readFileSync(path.join(__dirname, "jxa/observe_json_container_null.js"), "utf8");
const oldScript = fs.readFileSync(path.join(__dirname, "jxa/observe_literal_object_character.js"), "utf8");
const body = JSON.parse(fs.readFileSync(path.join(__dirname, "fixtures/json-null-frozen-k-wfjsonvalues.json")));
const controls = JSON.parse(fs.readFileSync(path.join(__dirname, "fixtures/s01-omitted-range-native-hop-controls.json")));
let assertions = 0;
function check(condition, message) { assertions++; assert.ok(condition, message); }
const plainJSON = value => JSON.parse(JSON.stringify(value));

function context() {
  const ctx = vm.createContext({ console: { log() {} }, isNil: value => value == null,
    cls: value => value.className || "SYNTHETIC_NATIVE_OBJECT_CONTROL", hasClass: () => true });
  vm.runInContext(script, ctx, { timeout: 1000 });
  return ctx;
}

// Re-evaluate the original authored fixture constructors without Foundation.
// Only their immutable input values are extracted; this cannot produce Apple evidence.
let oldReport;
const original = { loadEngine() {}, hasClass: () => true, methodNames: () => [],
  make: (_class, _selector, input) => ({ serializedRepresentation: input }), $: value => value,
  isNil: value => value == null, cls: () => "SYNTHETIC_UNIT_CONTROL", plain: value => value,
  isA: () => false, writeJSON: (_file, report) => { oldReport = report; }, console: { log() {} } };
vm.runInNewContext(oldScript + '\nrun(["synthetic-unit-control"]);', original, { timeout: 1000 });
check(controls.sourceSha256 === crypto.createHash("sha256").update(oldScript).digest("hex"), "source pin");
check(controls.controls.length === 7, "seven original follow-up specimens");
for (const control of controls.controls) {
  const rows = oldReport.observations.concat(oldReport.typedContainerObservations).filter(row => row.name === control.name);
  check(rows.length === control.classes.length, control.name + " original class selection");
  for (const row of rows) {
    assertions++; assert.deepEqual(plainJSON(row.input), control.input, control.name + " exact original authored input");
    check(control.classes.includes(row.className), "same reviewed class");
  }
}

const ctx = context();
const beforeBody = JSON.stringify(body);
const specimens = plainJSON(ctx.publicJSONSpecimens(body));
check(specimens.length === 16 && new Set(specimens.map(item => item.name)).size === 16, "bounded unique JSON selection");
check(JSON.stringify(body) === beforeBody, "original frozen body remains unchanged");
check(specimens.filter(item => item.actionRelevant).length === 10, "bounded action-state selection");
const actualEmpty = specimens.find(item => item.name === "frozen_k_authored_nil_emitted_as_empty_text");
check(actualEmpty.candidateWire === false && actualEmpty.input.Value.WFDictionaryFieldValueItems[0].WFItemType === 0,
      "actual K empty Text remains distinct from candidate item5");
for (const name of ["item_type5_missing_WFValue_candidate", "item_type5_json_null_WFValue_candidate"]) {
  const candidate = specimens.find(item => item.name === name);
  check(candidate.candidateWire === true && candidate.input.Value.WFDictionaryFieldValueItems[0].WFItemType === 5,
        "unknown item type stays explicitly candidate");
}
const absent = specimens.find(item => item.name === "missing_WFValue_candidate").input.Value.WFDictionaryFieldValueItems[0];
const presentNull = specimens.find(item => item.name === "explicit_json_null_WFValue_candidate").input.Value.WFDictionaryFieldValueItems[0];
check(!Object.hasOwn(absent, "WFValue") && Object.hasOwn(presentNull, "WFValue") && presentNull.WFValue === null,
      "absence and explicit JSON null are different observations");

// The next initializer must receive the exact prior native object, even if the
// JSON snapshot is deliberately different. Rebuilding through plain()/$(JSON)
// fails these identity controls.
const native = [0, 1, 2, 3].map(index => Object.freeze({ identity: Symbol("native-" + index) }));
const calls = [];
ctx.nativeJSONSnapshot = value => ({ jsonAvailable: true, jsonValue: "different snapshot", snapshotOf: native.indexOf(value) });
ctx.make = (name, selector, input, provider, parameter) => {
  const index = calls.length;
  assertions++; assert.strictEqual(input, native[index], "unmodified native input at hop " + index);
  check(name === "WFDictionaryParameterState" && selector === "initWithSerializedRepresentation:variableProvider:parameter:", "known initializer");
  check(provider === null && parameter === null, "fixed provider/parameter arguments");
  calls.push(input);
  return { serializedRepresentation: native[index + 1] };
};
let hops = ctx.captureNativeHops("WFDictionaryParameterState", native[0], null);
check(hops.length === 3 && calls.length === 3 && hops.every(hop => hop.accepted === true), "three bounded native-object hops");
check(hops[1].inputTransport === "unmodified previous native serializedRepresentation", "honest handoff label");

const parameter = { identity: Symbol("native-parameter") };
const firstState = { serializedRepresentation: native[1] };
calls.length = 0;
ctx.make = (_name, _selector, input, _provider, receivedParameter) => {
  const index = calls.length + 1;
  assertions++; assert.strictEqual(input, native[index]);
  assertions++; assert.strictEqual(receivedParameter, parameter);
  calls.push(input); return { serializedRepresentation: native[index + 1] };
};
hops = ctx.captureNativeHops("WFDictionaryParameterState", native[0], parameter, firstState);
check(hops.length === 3 && calls.length === 2, "actual copied action state is used as first hop");

let attempts = 0;
ctx.make = () => { attempts++; if (attempts === 2) throw new Error("synthetic exception"); return firstState; };
hops = ctx.captureNativeHops("WFDictionaryParameterState", native[0], null);
check(attempts === 2 && hops.length === 2 && hops[0].accepted === true && !!hops[1].error,
      "a failed hop preserves earlier observation and stops");
ctx.make = () => null;
hops = ctx.captureNativeHops("WFDictionaryParameterState", native[0], null);
check(hops.length === 1 && hops[0].accepted === false, "native nil initializer result is never accepted");

// Execute the actual bounded driver with explicit synthetic stand-ins. This
// verifies complete receipt shape and metadata-only API selection, not Apple behavior.
const driver = context();
const nsNull = { syntheticNSNull: true };
const dollar = value => value; dollar.NSNull = { null: nsNull }; driver.$ = dollar;
driver.nativeJSONSnapshot = value => value == null ? { objcNil: true, jsonAvailable: false }
  : { objcNil: false, nsNull: value === nsNull, jsonAvailable: true, jsonValue: value === nsNull ? null : value };
driver.make = (_class, _selector, input) => ({ serializedRepresentation: input });
driver.loadEngine = () => {};
driver.readJSON = filename => filename === "body" ? body : controls;
let copiedCount = 0, parameterReads = 0, finalReport;
const jsonParameter = { key: "WFJSONValues" };
const nativeArray = items => ({ count: items.length, objectAtIndex: index => items[index] });
const action = { identifier: "is.workflow.actions.downloadurl", isMissing: false, parameters: nativeArray([jsonParameter]),
  copyWithSerializedParameters(parameters) {
    copiedCount++;
    check(parameters.WFHTTPMethod === "POST" && parameters.WFHTTPBodyType === "JSON", "metadata-only JSON action setup");
    check(!Object.hasOwn(parameters, "WFURL"), "no network destination is provided");
    return { parameterStateForKey(key) { parameterReads++; check(key === "WFJSONValues", "exact known parameter key");
      return { serializedRepresentation: parameters.WFJSONValues === undefined ? null : parameters.WFJSONValues }; } };
  } };
driver.klass = name => {
  check(name === "WFBundledActionProvider", "only existing metadata provider");
  return { alloc: { init: { createAllAvailableActionsIncludingMissingActions: () => nativeArray([action]) } } };
};
driver.items = value => value; driver.count = value => value.count; driver.str = value => value;
driver.writeJSON = (_filename, report) => { finalReport = report; };
driver.run(["synthetic-only-output", "body", "controls"]);
check(finalReport.status === "OBSERVED" && finalReport.foundationControl.passed, "synthetic driver completed");
check(finalReport.containerObservations.length === 16 && finalReport.actionStateObservations.length === 10 &&
      finalReport.s01ControlObservations.length === 19, "full bounded observation selection");
check(copiedCount === 10 && parameterReads === 10, "only copied parameter state was inspected");
check(finalReport.shortcutRuntime === false && finalReport.httpExecuted === false && finalReport.formalAcceptanceGranted === false,
      "driver cannot declare runtime or acceptance credit");
action.isMissing = true;
check(driver.findJSONAction().missing === true, "a missing action placeholder is not treated as an available JSON action");
console.log(JSON.stringify({ status: "PASS_LOCAL_TRANSPORT_CONTROLS_ONLY", assertions, nativeRuntime: "NOT_RUN", mock: true }));
