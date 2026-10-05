// tools/shortcut-runtime/jxa/private_framework_probe.js
// Probes private framework availability and loads them via NSBundle.
// Writes output directly to disk as JSON to prevent stream redirection/stderr truncation bugs.

ObjC.import('Foundation');

function inspect(path) {
  var result = { path: path, exists: false, loaded: false, error: null };
  try {
    var fm = $.NSFileManager.defaultManager;
    result.exists = !!fm.fileExistsAtPath(path);
    if (!result.exists) return result;
    var bundle = $.NSBundle.bundleWithPath(path);
    result.loaded = !!bundle.loadAndReturnError(null);
  } catch (e) {
    result.error = String(e);
  }
  return result;
}

function run(argv) {
  var outPath = argv.length > 0 ? argv[0] : null;
  var data = {
    WorkflowKit: inspect('/System/Library/PrivateFrameworks/WorkflowKit.framework'),
    ActionKit: inspect('/System/Library/PrivateFrameworks/ActionKit.framework')
  };
  var jsonStr = JSON.stringify(data, null, 2);
  if (outPath) {
    var nsStr = $(jsonStr);
    var err = Ref();
    var ok = nsStr.writeToFileAtomicallyEncodingError(outPath, true, 4, err); // 4 = NSUTF8StringEncoding
    if (!ok) {
      console.log("Failed to write to " + outPath);
    }
  }
  return jsonStr;
}
