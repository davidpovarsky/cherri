// Round-trip each synthetic form through WorkflowKit's actual parameter-state API.
// A failed REQUIRED expectation must fail the process after retaining its observations.

function canonical(v) {
  return JSON.stringify(v, (k, x) => (x && typeof x === "object" && !Array.isArray(x))
    ? Object.keys(x).sort().reduce((o, key) => (o[key] = x[key], o), {}) : x);
}
function gateError(code, detail) {
  const error = new Error(code + ": " + detail);
  error.gateCode = code;
  return error;
}
function isRecord(value) { return value !== null && typeof value === "object" && !Array.isArray(value); }

function encodingForms() {
  const attachmentV = { WFSerializationType: "WFTextTokenAttachment", Value: { Type: "Variable", VariableName: "MyVar" } };
  const attachmentO = { WFSerializationType: "WFTextTokenAttachment", Value: { Type: "ActionOutput", OutputUUID: "11111111-2222-3333-4444-555555555555", OutputName: "Stored Content" } };
  return {
    string: "hello", number: 5, bool: true,
    "attachment(variable)": attachmentV, "attachment(output)": attachmentO,
    tokenString: { WFSerializationType: "WFTextTokenString", Value: { string: "Hello ￼", attachmentsByRange: { "{6, 1}": { Type: "Variable", VariableName: "MyVar" } } } },
    wrappedSubject: { Type: "Variable", Variable: attachmentV },
  };
}

function readGateJSON(path) {
  try { return readJSON(path); }
  catch (_) { throw gateError("E_REQUIRED_ENCODING_INPUT", "required JSON input is unreadable or invalid"); }
}

function requiredExpectations(document, forms) {
  if (!isRecord(document) || document.schemaVersion !== 1 || !Array.isArray(document.REQUIRED)
      || document.REQUIRED.length === 0 || document.REQUIRED.length > 256) {
    throw gateError("E_REQUIRED_ENCODING_INPUT", "nonempty schemaVersion 1 REQUIRED array is mandatory");
  }
  const ids = new Set(), pairs = new Set();
  for (const entry of document.REQUIRED) {
    if (!isRecord(entry) || Object.keys(entry).sort().join(",") !== "expect,form,id,kind,stateClass"
        || typeof entry.id !== "string" || !/^[A-Za-z][A-Za-z0-9_.()-]{0,191}$/.test(entry.id)
        || typeof entry.kind !== "string" || !/^[A-Za-z][A-Za-z0-9_]{0,31}$/.test(entry.kind)
        || typeof entry.stateClass !== "string" || !/^[A-Za-z_][A-Za-z0-9_]{0,127}$/.test(entry.stateClass)
        || typeof entry.form !== "string" || !Object.prototype.hasOwnProperty.call(forms, entry.form)
        || !["accept", "reject"].includes(entry.expect)
        || ids.has(entry.id) || pairs.has(entry.stateClass + "/" + entry.form)) {
      throw gateError("E_REQUIRED_ENCODING_INPUT", "invalid or duplicate REQUIRED entry");
    }
    ids.add(entry.id); pairs.add(entry.stateClass + "/" + entry.form);
  }
  return document.REQUIRED;
}

function isolatedExpectations(template, override, forms) {
  const changed = requiredExpectations(override, forms);
  if (changed.length !== template.length) {
    throw gateError("E_REQUIRED_ENCODING_INPUT", "isolated expectations must retain every REQUIRED case");
  }
  const baseline = new Map(template.map(entry => [entry.id, entry]));
  for (const entry of changed) {
    const original = baseline.get(entry.id);
    if (!original || ["kind", "stateClass", "form"].some(key => entry[key] !== original[key])) {
      throw gateError("E_REQUIRED_ENCODING_INPUT", "isolated expectations may change only a known case's expect field");
    }
  }
  return changed;
}

function legacyExpectations(required) {
  const grouped = {};
  for (const entry of required) {
    if (entry.expect !== "accept") continue;
    const group = grouped[entry.kind] || (grouped[entry.kind] = { states: [], forms: [] });
    if (!group.states.includes(entry.stateClass)) group.states.push(entry.stateClass);
    if (!group.forms.includes(entry.form)) group.forms.push(entry.form);
  }
  return grouped;
}

function classifyObservation(value) {
  if (value === "roundtrip" || (typeof value === "string" && value.startsWith("accepted, reserialized as "))) return "accept";
  if (value === "rejected") return "reject";
  return "unavailable";
}

function run(argv) {
  const forms = encodingForms();
  const report = {
    schemaVersion: 2, status: "NOT_RUN", engine: "WorkflowKit parameter-state API", engineLoaded: false,
    forms: Object.keys(forms), expectations: {}, requiredExpectations: [], requiredResults: [],
    results: {}, failures: [], discovery: {},
  };
  try {
    if (argv.length !== 4 && argv.length !== 5) {
      throw gateError("E_REQUIRED_ENCODING_INPUT",
        "usage: verify_encodings.js parameter-encodings.json encoding-table.json out.json required-expectations.json [isolated-expectations.json]");
    }
    const enc = readGateJSON(argv[0]), table = readGateJSON(argv[1]);
    if (!isRecord(enc) || !isRecord(enc.parameterClasses)
        || !isRecord(table) || !isRecord(table.appIntentValueTypes)) {
      throw gateError("E_REQUIRED_ENCODING_INPUT", "both discovery inputs must contain their declared state-class maps");
    }
    const discovered = entries => Object.values(entries).map(entry => isRecord(entry) ? entry.stateClass : null)
      .filter(name => typeof name === "string" && /^[A-Za-z_][A-Za-z0-9_]*$/.test(name) && !name.startsWith("_TtGC"));
    const parameterNames = discovered(enc.parameterClasses), intentNames = discovered(table.appIntentValueTypes);
    if (parameterNames.length === 0 || intentNames.length === 0) {
      throw gateError("E_REQUIRED_ENCODING_DISCOVERY", "both discovery inputs must contain actual named state classes");
    }
    const template = requiredExpectations(readGateJSON(argv[3]), forms);
    const required = argv.length === 5 ? isolatedExpectations(template, readGateJSON(argv[4]), forms) : template;
    report.requiredExpectations = required;
    report.expectations = legacyExpectations(required);
    const names = new Set([...parameterNames, ...intentNames, ...required.map(entry => entry.stateClass)]);
    report.discovery = { parameterStateClasses: parameterNames.length, appIntentStateClasses: intentNames.length, uniqueStateClasses: names.size };
    loadEngine();
    report.engineLoaded = true;

    for (const cn of [...names].sort()) {
      if (!hasClass(cn)) { report.results[cn] = "class not present"; continue; }
      const row = {};
      for (const [label, value] of Object.entries(forms)) {
        try {
          const state = make(cn, "initWithSerializedRepresentation:variableProvider:parameter:", $(value), null, null);
          if (isNil(state)) { row[label] = "rejected"; continue; }
          const back = plain(state.serializedRepresentation, 0);
          row[label] = canonical(back) === canonical(value)
            ? "roundtrip" : "accepted, reserialized as " + JSON.stringify(back).slice(0, 80);
        } catch (_) {
          // Only synthetic values are probed; arbitrary native exception text is not published.
          row[label] = "threw: native parameter-state exception";
        }
      }
      report.results[cn] = row;
    }

    for (const expected of required) {
      const row = report.results[expected.stateClass];
      const observation = isRecord(row) ? row[expected.form] : row;
      const actual = classifyObservation(observation);
      const passed = actual === expected.expect;
      report.requiredResults.push({ id: expected.id, kind: expected.kind, stateClass: expected.stateClass,
        form: expected.form, expected: expected.expect, actual: actual, observation: observation, passed: passed });
      if (!passed) {
        report.failures.push({ code: actual === "unavailable" ? "E_REQUIRED_ENCODING_ENGINE" : "E_REQUIRED_ENCODING_EXPECTATION",
          id: expected.id, expected: expected.expect, actual: actual });
      }
    }
    report.status = report.failures.length === 0 ? "PASS"
      : report.failures.some(failure => failure.code === "E_REQUIRED_ENCODING_ENGINE") ? "NOT_RUN" : "FAIL";
  } catch (error) {
    report.status = "NOT_RUN";
    report.failures.push({ code: error.gateCode || "E_REQUIRED_ENCODING_ENGINE", stage: report.engineLoaded ? "probe" : "input-or-engine-load" });
  }

  if (argv.length >= 3) writeJSON(argv[2], report);
  console.log("REQUIRED encoding gate " + report.status + ": " + report.requiredResults.length
    + " required observations; " + report.failures.length + " failures");
  if (report.status !== "PASS") {
    const first = report.failures[0] || { code: "E_REQUIRED_ENCODING_ENGINE" };
    throw gateError(first.code, first.id || first.stage || "required encoding verification did not run");
  }
}
