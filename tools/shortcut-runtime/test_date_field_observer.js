// Explicit Node mocks test case construction and native-object transport only.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");
const path = require("node:path");
const fixture = JSON.parse(fs.readFileSync(path.join(__dirname, "fixtures/date-field-compiled-controls.json")));
const script = fs.readFileSync(path.join(__dirname, "jxa/observe_json_container_null.js"), "utf8");
const copy = value => JSON.parse(JSON.stringify(value));
let assertions = 0;
function check(ok, message) { assertions++; assert.ok(ok, message); }
function equal(a, b, message) { assertions++; assert.deepEqual(copy(a), copy(b), message); }
function context() {
  const ctx = vm.createContext({ console: { log() {} }, isNil: value => value == null,
    cls: value => value.className || "WFDateFieldParameterState", hasClass: () => true });
  vm.runInContext(script, ctx, { timeout: 1000 });
  return ctx;
}
const ctx = context(), before = JSON.stringify(fixture), specimens = ctx.dateFieldSpecimens(fixture);
check(JSON.stringify(fixture) === before, "original compiled wire is not mutated");
check(specimens.direct.length === 6 && new Set(specimens.direct.map(x => x.name)).size === 6, "six unique direct cases");
check(specimens.cases.length === 8 && new Set(specimens.cases.map(x => x.name)).size === 8, "eight unique action copies");
check(specimens.cases.reduce((n, x) => n + x.keys.length, 0) === 14, "fourteen exact parameter observations");
const original = fixture.positive_seconds.WFWorkflowActionParameters;
equal(specimens.cases[0].parameters, original, "actual local interval parameters unchanged");
equal(specimens.cases[6].parameters, fixture.format_start.WFWorkflowActionParameters, "actual successful format wire unchanged");
for (const key of ["WFTimeUntilFromDate", "WFInput"]) {
  const token = specimens.cases[1].parameters[key];
  check(token.WFSerializationType === "WFTextTokenString" && token.Value.string === "\uFFFC", "one actual replacement character");
  equal(token.Value.attachmentsByRange, { "{0, 1}": original[key].Value }, "exact original producer reference and range");
  check(!Object.hasOwn(specimens.cases[5].parameters, key), "missing parameter remains absent, not empty text");
}
equal(specimens.cases[2].parameters.WFInput, original.WFInput, "first-only conversion leaves second bare");
equal(specimens.cases[3].parameters.WFTimeUntilFromDate, original.WFTimeUntilFromDate, "second-only conversion leaves first bare");
equal(specimens.cases[7].parameters.WFDate, original.WFTimeUntilFromDate, "format negative control uses original same producer");
check(specimens.cases[4].parameters.WFTimeUntilFromDate === "2001-02-03T04:05:06Z" &&
      specimens.cases[4].parameters.WFInput === "2001-02-03T04:06:06Z", "literal positive interval retained");
for (const [name, corrupt] of [
  ["source", f => f.source_sha256 = "0".repeat(64)],
  ["action", f => f.positive_seconds.WFWorkflowActionIdentifier = "is.workflow.actions.downloadurl"],
  ["date-mode", f => f.date_producers[0].WFWorkflowActionParameters.WFDateActionMode = "Current Date"],
  ["first-reference", f => f.positive_seconds.WFWorkflowActionParameters.WFTimeUntilFromDate.Value.OutputUUID = "different"],
  ["second-reference", f => f.positive_seconds.WFWorkflowActionParameters.WFInput.Value.OutputUUID = "different"],
  ["reference-name", f => f.positive_seconds.WFWorkflowActionParameters.WFInput.Value.OutputName = "wrong"],
  ["format-reference", f => f.format_start.WFWorkflowActionParameters.WFDate.Value.attachmentsByRange["{0, 1}"].OutputUUID = "different"],
  ["unit", f => f.positive_seconds.WFWorkflowActionParameters.WFTimeUntilUnit = "Minutes"],
  ["duplicate-producer", f => f.date_producers.push(copy(f.date_producers[0]))]
]) {
  const changed = copy(fixture); corrupt(changed);
  assertions++; assert.throws(() => ctx.dateFieldSpecimens(changed), /Date/, name);
}

function driver() {
  const d = context(), nsNull = {};
  const dollar = value => value;
  dollar.NSNull = { null: nsNull }; dollar.NSStringFromClass = value => value; d.$ = dollar;
  d.nativeJSONSnapshot = value => value == null ? { objcNil: true, jsonAvailable: false }
    : { objcNil: false, nsNull: value === nsNull, jsonAvailable: true, jsonValue: value === nsNull ? null : value };
  d.loadEngine = () => {}; d.readJSON = () => fixture; d.items = x => x; d.count = x => x.count; d.str = x => x;
  d.make = (_class, _selector, input) => ({ serializedRepresentation: input });
  const array = items => ({ count: items.length, objectAtIndex: i => items[i] });
  let copied = 0, reads = 0;
  const actions = [
    ["is.workflow.actions.gettimebetweendates", ["WFTimeUntilFromDate", "WFInput", "WFTimeUntilUnit"]],
    ["is.workflow.actions.format.date", ["WFDate"]]
  ].map(([identifier, keys]) => ({
    identifier, isMissing: false,
    parameters: array(keys.map(key => ({ key, singleStateClass: "WFDateFieldParameterState", defaultSerializedRepresentation: null }))),
    copyWithSerializedParameters(parameters) {
      copied++; check(!Object.hasOwn(parameters, "WFURL"), "metadata has no network URL");
      return { parameterStateForKey(key) { reads++; return { serializedRepresentation: Object.hasOwn(parameters, key) ? parameters[key] : null }; } };
    },
    execute() { throw new Error("Metadata observer must never execute an action"); }
  }));
  d.klass = name => {
    check(name === "WFBundledActionProvider", "known provider only");
    return { alloc: { init: { createAllAvailableActionsIncludingMissingActions: () => array(actions) } } };
  };
  let report;
  d.writeJSON = (_out, value) => { report = value; };
  return { d, actions, get report() { return report; }, get copied() { return copied; }, get reads() { return reads; } };
}
const realDriver = driver();
realDriver.d.run(["synthetic-output", "date-field-parameters", "fixed-input"]);
check(realDriver.report.status === "OBSERVED" && realDriver.report.foundationControl.passed, "mock driver reaches execution marker");
check(realDriver.report.directStateObservations.length === 6 && realDriver.report.actionStateObservations.length === 14, "all twenty observation rows retained");
check(realDriver.copied === 14 && realDriver.reads === 14, "bounded metadata calls only");
check(realDriver.report.actionStateObservations.every(r => r.actionExecuted === false), "no per-row runtime credit");
check(realDriver.report.shortcutRuntime === false && realDriver.report.httpExecuted === false &&
      realDriver.report.formalAcceptanceGranted === false && realDriver.report.mock === false,
      "actual driver declares intended native scope, not a fake successful contract");
check(realDriver.report.inputProvenance.nativeUnsignedBytesDownloaded === false, "local compilation not mislabelled downloaded");
check(realDriver.report.actionStateObservations.every(r => r.hops.length === 3), "three actual driver hops per state");
realDriver.actions[0].isMissing = true;
check(!Object.hasOwn(realDriver.d.findDateMetadataActions(), "is.workflow.actions.gettimebetweendates"), "missing action not treated as available");
const partial = driver(); let calls = 0;
partial.d.make = (_class, _selector, input) => { calls++; if (calls === 2) throw new Error("synthetic failed hop"); return { serializedRepresentation: input }; };
partial.d.run(["synthetic-output", "date-field-parameters", "fixed-input"]);
const failed = partial.report.directStateObservations[0].hops;
check(failed.length === 2 && failed[0].accepted === true && !!failed[1].error, "earlier native-hop observations survive a later failure");
check(partial.report.status === "OBSERVED", "OBSERVED is distinct from specimen success");
const duplicate = driver(); duplicate.actions.push(duplicate.actions[0]);
assertions++; assert.throws(() => duplicate.d.findDateMetadataActions(), /Duplicate Date/);
console.log(JSON.stringify({ status: "PASS_LOCAL_DATE_DRIVER_CONTROLS_ONLY", assertions, nativeRuntime: "NOT_RUN", mock: true }));
