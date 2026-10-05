// tools/shortcut-runtime/jxa/extract_encoding_table.js
// Produces the definitive value-encoding table for .shortcut files by asking
// WorkflowKit's own classes to serialize:
//   - every WFVariable kind (action output, named variable, shortcut input, ...)
//   - a WFVariableString with embedded variables (the WFTextTokenString form)
//   - every parameter state class, constructed with a plain value and with a variable
//   - icon palette colors (with safe fallback for macOS 15/26)
//   - conditional subject operator codes
//   - App Intents parameters
// Resilient against macOS version differences (wrapped in safe exception boundaries).

function attempt(fn) { try { return fn(); } catch (e) { return "threw: " + e.message; } }
function serialized(state) { return isNil(state) ? "nil" : plain(state.serializedRepresentation, 0); }

function run(argv) {
  if (argv.length < 1) { throw new Error("usage: extract_encoding_table.js out.json [parameter-encodings.json]"); }
  loadEngine();
  const out = {};

  // Variables
  const named = make("WFUserDefinedVariable", "initWithName:variableProvider:aggrandizements:", "MyVar", null, null);
  const output = make("WFActionOutputVariable", "initWithOutputUUID:outputName:variableProvider:aggrandizements:", "11111111-2222-3333-4444-555555555555", "Stored Content", null, null);
  const vars = {};
  vars["WFUserDefinedVariable(MyVar)"] = plain(named.serializedRepresentation, 0);
  vars["WFActionOutputVariable"] = plain(output.serializedRepresentation, 0);
  vars["WFShortcutInputVariable"] = plain(make("WFShortcutInputVariable", "initWithVariableProvider:aggrandizements:", null, null).serializedRepresentation, 0);
  for (const cn of ["WFClipboardVariable", "WFCurrentDateVariable", "WFDeviceDetailsVariable", "WFCurrentAppVariable"]) {
    try {
      vars[cn] = plain(make(cn, "initWithAggrandizements:", null).serializedRepresentation, 0);
    } catch (e) {
      vars[cn] = "threw: " + e.message;
    }
  }
  try {
    vars["WFAskEachTimeVariable(prompt)"] = plain(make("WFAskEachTimeVariable", "initWithPrompt:", "Which one?").serializedRepresentation, 0);
  } catch (e) {
    vars["WFAskEachTimeVariable(prompt)"] = "threw: " + e.message;
  }
  out.variables = vars;

  // Text with embedded variables
  const vs = make("WFVariableString", "initWithStringsAndVariables:", $(["Hello ", named, "!"]));
  out["variableString(Hello <MyVar>!)"] = plain(vs.serializedRepresentation, 0);

  // State classes: plain value and variable forms
  const states = {};
  const stateNames = new Set([
    "WFStringSubstitutableState", "WFBooleanSubstitutableState", "WFNumberSubstitutableState",
    "WFNumberStringSubstitutableState", "WFVariableStringParameterState", "WFVariableParameterState",
    "WFURLStringParameterState", "WFDateFieldParameterState", "WFStringParameterState",
    "WFNumberParameterState", "WFDictionaryParameterState", "WFAppDescriptorParameterState",
    "WFColorParameterState", "WFFileParameterState", "WFLocationParameterState",
    "WFINObjectSubstitutableState", "WFCalendarSubstitutableState", "WFWorkflowParameterState",
    "WFQuantityParameterState", "WFContactFieldEntry", "WFFontParameterState",
    "WFNSUnitSubstitutableState", "WFIntentDescriptorParameterState", "WFMediaItemState",
    "WFLinkEnumerationSubstitutableState", "WFLinkDynamicOptionSubstitutableState"
  ]);

  if (argv.length > 1) {
    try {
      const enc = readJSON(argv[1]);
      if (enc && enc.parameterClasses) {
        for (const e of Object.values(enc.parameterClasses)) {
          if (typeof e.stateClass === "string" && !e.stateClass.startsWith("_TtGC")) {
            stateNames.add(e.stateClass);
          }
        }
      }
    } catch (e) {
      console.log("Notice: could not read parameterClasses from argv[1]: " + e.message);
    }
  }

  const respondsTo = (c, sel) => c.instancesRespondToSelector && c.instancesRespondToSelector(sel);
  for (const cn of [...stateNames].sort()) {
    if (!hasClass(cn)) continue;
    const c = klass(cn);
    const relevant = respondsTo(c, "serializedRepresentation") && (
      respondsTo(c, "initWithValue:") || respondsTo(c, "initWithVariable:") || respondsTo(c, "initWithNumber:") || respondsTo(c, "initWithString:") ||
      respondsTo(c, "initWithVariableString:") || respondsTo(c, "initWithKeyValuePairs:"));
    if (!relevant) continue;
    const e = {};
    if (respondsTo(c, "initWithValue:")) {
      const cands = { "value:string": "hello", "value:number": 5, "value:bool": true };
      for (const k of Object.keys(cands)) {
        const r = attempt(() => serialized(make(cn, "initWithValue:", cands[k])));
        if (!(typeof r === "string" && r.startsWith("threw"))) e[k] = r;
      }
    }
    if (respondsTo(c, "initWithNumber:")) e["number:5"] = attempt(() => serialized(make(cn, "initWithNumber:", 5)));
    if (respondsTo(c, "initWithBoolValue:")) e["bool:YES"] = attempt(() => serialized(makeBool(cn, "initWithBoolValue:", true)));
    if (respondsTo(c, "initWithString:")) e["string:hello"] = attempt(() => serialized(make(cn, "initWithString:", "hello")));
    if (respondsTo(c, "initWithVariableString:")) e["variableString:Hello <MyVar>!"] = attempt(() => serialized(make(cn, "initWithVariableString:", vs)));
    if (respondsTo(c, "initWithVariable:")) {
      e["variable:MyVar"] = attempt(() => serialized(make(cn, "initWithVariable:", named)));
      e["variable:ActionOutput"] = attempt(() => serialized(make(cn, "initWithVariable:", output)));
    }
    if (respondsTo(c, "initWithKeyValuePairs:")) e["keyValuePairs:[]"] = attempt(() => serialized(make(cn, "initWithKeyValuePairs:", $([]))));
    if (Object.keys(e).length) states[cn] = e;
  }
  out.stateClasses = states;

  // Icon palette (safe probe for macOS 15/26)
  const palette = [];
  try {
    if (hasClass("WFWorkflowIcon")) {
      const icon = klass("WFWorkflowIcon");
      if (icon.instancesRespondToSelector && icon.instancesRespondToSelector("initWithPaletteColor:glyphCharacter:")) {
        for (let i = 0; i < 15; i++) {
          try {
            const ic = makeUInts("WFWorkflowIcon", "initWithPaletteColor:glyphCharacter:", i, 61440);
            const v = Number(ic.backgroundColorValue);
            palette.push({ palette: i, WFWorkflowIconStartColor: v >>> 0, signed: v, color: str(ic.backgroundColor.description) || "" });
          } catch (e) {}
        }
      }
      out.defaultGlyphCharacter = icon.defaultGlyphCharacter || 61440;
    }
  } catch (e) {
    console.log("icon palette skipped: " + e.message);
  }
  out.iconPalette = palette;

  // Conditional subject operator codes
  const ops = {};
  for (const cn of ["WFConditionalSubjectParameterState", "WFVariableConditionalSubjectState", "WFHomeAccessoryConditionalSubjectState"]) {
    if (!hasClass(cn)) continue;
    const c = klass(cn);
    const st = attempt(() => make(cn, "init"));
    const o = typeof st === "string" ? st : attempt(() => plain(st.supportedComparisonOperators, 0));
    ops[cn] = { subjectType: plain(attempt(() => c.subjectType), 0), operators: o === undefined ? null : o };
  }
  out.conditionalSubjectOperators = ops;

  // App Intents parameters
  try {
    $.dlopen("/System/Library/PrivateFrameworks/LinkMetadata.framework/LinkMetadata", RTLD_NOW);
  } catch (e) {}
  const appIntentValueTypes = {};
  if (hasClass("LNPrimitiveValueType") && hasClass("LNActionParameterMetadata")) {
    try {
      const P = klass("LNPrimitiveValueType");
      const metadataFor = (name, vt) => {
        ObjC.bindFunction("objc_msgSend", ["void *", ["void *", "void *", "void *", "void *", "bool", "void *", "void *", "void *"]]);
        return ObjC.castRefToObject($.objc_msgSend(allocRef("LNActionParameterMetadata"), $.sel_registerName("initWithName:valueType:optional:title:resolvableInputTypes:typeSpecificMetadata:"), toRef(name), toRef(vt), true, toRef(name), toRef($([])), toRef($({}))));
      };
      const entity = (id) => make("LNEntityValueType", "initWithIdentifier:", id);
      const enumeration = (id) => make("LNLinkEnumerationValueType", "initWithEnumerationIdentifier:", id);
      const array = (member) => make("LNArrayValueType", "initWithMemberValueType:", member);
      const intents = (id) => {
        ObjC.bindFunction("objc_msgSend", ["void *", ["void *", "void *", "unsigned long", "void *"]]);
        return ObjC.castRefToObject($.objc_msgSend(allocRef("LNIntentsValueType"), $.sel_registerName("initWithTypeIdentifier:contentType:"), id, NIL()));
      };
      const valueTypes = {
        string: P.stringValueType, richText: P.attributedStringValueType, bool: P.boolValueType, int: P.intValueType, double: P.doubleValueType,
        url: P.URLValueType, date: P.dateValueType, dateComponents: P.dateComponentsValueType, location: P.placemarkValueType,
        entity: entity("com.apple.reminders.ListEntity"), enum: enumeration("com.apple.reminders.PriorityLevel"),
        "array<string>": array(P.stringValueType), "array<bool>": array(P.boolValueType), "array<entity>": array(entity("com.apple.reminders.ReminderEntity")), "array<enum>": array(enumeration("com.apple.reminders.PriorityLevel")),
        app: intents(0), person: intents(3), file: intents(12), paymentMethod: intents(13), currencyAmount: intents(14), "array<file>": array(intents(12)),
      };
      for (const [name, vt] of Object.entries(valueTypes)) {
        appIntentValueTypes[name] = attempt(() => {
          const d = vt.wf_parameterDefinitionWithParameterMetadata(metadataFor("p", vt));
          if (isNil(d)) return { definitionClass: "(none)", parameterClass: "(none: WorkflowKit has no parameter class for this value type)" };
          const pcls = d.parameterClass;
          const e = { definitionClass: cls(d), parameterClass: isNil(pcls) ? "(none: WorkflowKit has no parameter class for this value type)" : str($.NSStringFromClass(pcls)) };
          if (isNil(pcls)) return e;
          try {
            const p = klass(e.parameterClass).parameterWithDefinition(d.parameterDefinitionDictionary);
            e.stateClass = str($.NSStringFromClass(p.singleStateClass)); e.default = plain(p.defaultSerializedRepresentation, 0);
          } catch (err) { e.stateClass = null; e.note = "parameter could not be built: " + err.message; }
          return e;
        });
      }
    } catch (e) {
      console.log("App Intents value types error: " + e.message);
    }
  }
  out.appIntentValueTypes = appIntentValueTypes;

  writeJSON(argv[0], out);
  console.log(Object.keys(states).length + " state classes, " + Object.keys(vars).length + " variables, " + Object.keys(appIntentValueTypes).length + " App Intents value types -> " + argv[0]);
}
