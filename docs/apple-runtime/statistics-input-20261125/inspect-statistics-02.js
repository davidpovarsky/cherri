
ObjC.import('Foundation');
function run(argv) {
 var result={frameworks:[],identifier:'is.workflow.actions.statistics',parameters:[]};
 for(var path of ['/System/Library/PrivateFrameworks/WorkflowKit.framework','/System/Library/PrivateFrameworks/ActionKit.framework']) {
  var bundle=$.NSBundle.bundleWithPath(path);
  result.frameworks.push({path:path,loaded:!!bundle.loadAndReturnError(null)});
 }
 try {
  var cls=$.NSClassFromString('WFBundledActionProvider');
  var provider=cls.alloc.init;
  var actions=provider.createAllAvailableActionsIncludingMissingActions(true).allObjects;
  result.collectionClass=ObjC.unwrap($.NSStringFromClass(actions.class));
  result.count=Number(actions.count);
  for(var i=0;i<actions.count;i++) {
   var action=actions.objectAtIndex(i);
   if(ObjC.unwrap(action.identifier)!==result.identifier)continue;
   result.found=true;
   result.actionClass=ObjC.unwrap($.NSStringFromClass(action.class));
   var params=action.parameters;
   for(var j=0;j<params.count;j++) {
    var parameter=params.objectAtIndex(j);
    var entry={key:ObjC.unwrap(parameter.key),class:ObjC.unwrap($.NSStringFromClass(parameter.class))};
    try{entry.stateClass=ObjC.unwrap($.NSStringFromClass(parameter.singleStateClass));}catch(e){entry.stateError=String(e);}
    try{entry.default=ObjC.deepUnwrap(parameter.defaultSerializedRepresentation);}catch(e){entry.defaultError=String(e);}
    result.parameters.push(entry);
   }
  }
 }catch(e){result.error=String(e);}
 var raw=JSON.stringify(result,null,2);
 $(raw).writeToFileAtomicallyEncodingError(argv[0],true,4,null);
 return raw;
}
