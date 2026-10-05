// tools/shortcut-runtime/jxa/inspect_actions_meta.js
// Safe metadata inspect for a single action. Never calls copyWithSerializedParameters.
// Requires jxa-prelude.js concatenated in front.

function run(argv) {
  if (argv.length < 2) {
    throw new Error("usage: inspect_actions_meta.js <action_identifier> <out_path>");
  }
  loadEngine();
  var targetId = argv[0];
  var outPath = argv[1];

  var provider = klass("WFBundledActionProvider").alloc.init;
  var action = null;
  try {
    var allActions = items(provider.createAllAvailableActionsIncludingMissingActions(true));
    for (var i = 0; i < count(allActions); i++) {
      var a = allActions.objectAtIndex(i);
      if (str(a.identifier) === targetId) { action = a; break; }
    }
  } catch (e) {}

  if (!action) {
    try {
      var intentActions = items(klass("WFIntentActionProvider").alloc.init.createAllAvailableActions);
      for (var i = 0; i < count(intentActions); i++) {
        var a = intentActions.objectAtIndex(i);
        if (str(a.identifier) === targetId) { action = a; break; }
      }
    } catch (e) {}
  }

  if (!action) {
    writeJSON(outPath, { identifier: targetId, status: "not_found", isMissing: true, parameters: [] });
    return;
  }

  var isMissing = false;
  try { isMissing = !!action.isMissing; } catch (e) {}
  var outputName = null;
  try { var n = action.outputName; if (!isNil(n)) outputName = str(n); } catch (e) {}

  var plist = null;
  try { plist = action.parameters; } catch (e) {}
  var params = [];
  if (plist) {
    for (var j = 0; j < count(plist); j++) {
      var p = plist.objectAtIndex(j);
      var pc = cls(p);
      var key = p.key;
      var k = (!isNil(key) && isA(key, "NSString")) ? str(key) : null;
      var sc = null, dflt = null, sv = null;
      try { sc = str($.NSStringFromClass(p.singleStateClass)); } catch (e) {}
      try { dflt = plain(p.defaultSerializedRepresentation, 0); } catch (e) {}
      try {
        var svObj = p.supportedVariableTypes;
        if (!isNil(svObj)) sv = plain(svObj.allObjects, 0).slice().sort();
      } catch (e) {}
      params.push({
        key: k,
        class: pc,
        singleStateClass: sc,
        default: dflt,
        variableTypes: sv
      });
    }
  }

  writeJSON(outPath, {
    identifier: targetId,
    status: isMissing ? "unavailable" : "success",
    isMissing: isMissing,
    outputName: outputName,
    parameters: params
  });
}
