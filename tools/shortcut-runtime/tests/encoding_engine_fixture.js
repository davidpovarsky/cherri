// Test-only process fixture: models historical observations, never loads Apple frameworks.
const fs = require("fs");
const vm = require("vm");
const [script, matrixPath, mode, ...args] = process.argv.slice(2);
const matrix = JSON.parse(fs.readFileSync(matrixPath, "utf8")).results;
const context = {
  console: console,
  readJSON: path => JSON.parse(fs.readFileSync(path, "utf8")),
  writeJSON: (path, value) => fs.writeFileSync(path, JSON.stringify(value, null, 2) + "\n"),
  loadEngine: () => { if (mode === "engine_failure") throw new Error("synthetic unavailable engine"); },
  hasClass: name => name in matrix && typeof matrix[name] === "object"
    && !(mode === "class_missing" && name === "WFStringParameterState"),
  $: value => value,
  isNil: value => value === null,
  plain: value => value,
  make: (name, selector, value) => {
    let form = typeof value === "boolean" ? "bool" : typeof value === "number" ? "number"
      : typeof value === "string" ? "string" : value.WFSerializationType === "WFTextTokenString" ? "tokenString"
      : value.WFSerializationType === "WFTextTokenAttachment"
        ? value.Value.Type === "Variable" ? "attachment(variable)" : "attachment(output)" : "wrappedSubject";
    if (name === "WFStringParameterState" && form === "string") {
      if (mode === "native_rejection") return null;
      if (mode === "native_exception") throw new Error("synthetic native exception");
    }
    const observation = matrix[name] && matrix[name][form];
    if (observation === "rejected") return null;
    if (observation === "roundtrip") return { serializedRepresentation: value };
    if (typeof observation === "string" && observation.startsWith("accepted, reserialized as ")) {
      return { serializedRepresentation: { normalizedSyntheticValue: true } };
    }
    throw new Error("synthetic unavailable observation");
  },
};
vm.createContext(context);
vm.runInContext(fs.readFileSync(script, "utf8"), context, { filename: script });
try { context.run(args); }
catch (error) { console.error(error.message); process.exitCode = 1; }
