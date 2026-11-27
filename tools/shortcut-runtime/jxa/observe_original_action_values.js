// Diagnostic engine pilot: an exact original action array, with stored native representations.
// Completion-time action.input/pendingActionInput are observations, not a claimed will-run input hook.
ObjC.bindFunction("method_getTypeEncoding", ["string", ["void *"]]);
ObjC.bindFunction("objc_getMetaClass", ["void *", ["string"]]);
ObjC.bindFunction("CFGetTypeID", ["unsigned long", ["void *"]]);
ObjC.bindFunction("CFBooleanGetTypeID", ["unsigned long", []]);
ObjC.bindFunction("object_getClass", ["void *", ["void *"]]);

let pilot = null;
let controller = null;
let workflow = null;
let journal = null;
let actions = null;
let originalActions = null;
let originalActionCount = null;
function mono() { return Number($.NSProcessInfo.processInfo.systemUptime); }
function emit(kind, detail) {
    if (pilot.events.length >= 64) throw new Error("EVENT_BOUND");
    const row = Object.assign({sequence: pilot.events.length, kind: kind, uptime: mono()}, detail || {});
    const raw = JSON.stringify(row) + "\n";
    if (raw.length > 65536) throw new Error("EVENT_BYTES_BOUND");
    pilot.events.push(row);
    journal.writeData($(raw).dataUsingEncoding($.NSUTF8StringEncoding));
    journal.synchronizeFile;
}
function methodsFor(name, classSide) {
    let c = classSide ? $.objc_getMetaClass(name) : $.objc_getClass(name);
    const rows = {};
    for (let depth = 0; depth < 10; depth++) {
        const declaring = $.class_getName(c);
        if (declaring === "NSObject" || declaring === "nil") break;
        const n = Ref("unsigned int"), list = $.class_copyMethodList(c, n);
        try {
            if (Number(n[0]) > 2048) throw new Error("METHOD_BOUND");
            for (let i = 0; i < Number(n[0]); i++) {
                const key = $.sel_getName($.method_getName(list[i]));
                if (!(key in rows)) rows[key] = $.method_getTypeEncoding(list[i]);
            }
        } finally { $.free(list); }
        c = $.class_getSuperclass(c);
    }
    return rows;
}
function bridgedSelector(selector) {
    const parts=selector.split(":");
    if(parts[parts.length-1]==="")parts.pop();
    return parts.map((p,i)=>i===0?p:p.charAt(0).toUpperCase()+p.slice(1)).join("");
}
function obj(target, selector, ...args) {
    const key=bridgedSelector(selector);
    return args.length===0 ? target[key] : target[key](...args);
}
function voidCall(target, selector, ...args) {
    const key=bridgedSelector(selector);
    if(args.length===0){ target[key]; }else{ target[key](...args); }
}
function boolCall(target, selector, value) {
    target[bridgedSelector(selector)](value);
}
function bridgedMake(className, selector, ...args) {
    const instance=klass(className).alloc;
    return obj(instance,selector,...args);
}
function nativeIsA(value, name) {
    if (!isObj(value)) return false;
    let current=$.object_getClass(toRef(value));
    for(let depth=0; depth<24; depth++){
        const actual=$.class_getName(current);
        if(actual==="nil")return false;
        if(actual===name)return true;
        current=$.class_getSuperclass(current);
    }
    throw new Error("NATIVE_CLASS_HIERARCHY_BOUND");
}
function nativeValue(value, depth, state) {
    state = state || {nodes: 0};
    state.nodes += 1;
    if (state.nodes > 1024) throw new Error("VALUE_NODE_BOUND");
    if (depth > 6) throw new Error("VALUE_DEPTH_BOUND");
    if (isNil(value)) return {present: false, nativeClass: null, kind: "NIL"};
    if (!isObj(value)) throw new Error("UNWRAPPED_NATIVE_VALUE");
    const cn = $.class_getName($.object_getClass(toRef(value)));
    if (nativeIsA(value, "NSNull")) return {present: true, nativeClass: cn, kind: "NSNULL"};
    if (nativeIsA(value, "NSNumber")) {
        const boolean = Number($.CFGetTypeID(toRef(value))) === Number($.CFBooleanGetTypeID());
        const number = Number(value.doubleValue);
        if (!Number.isFinite(number)) throw new Error("NONFINITE_VALUE");
        return {present: true, nativeClass: cn, kind: boolean ? "CFBOOLEAN" : "NSNUMBER",
                cfTypeId: Number($.CFGetTypeID(toRef(value))), cfBooleanTypeId: Number($.CFBooleanGetTypeID()),
                value: boolean ? Boolean(value.boolValue) : number};
    }
    if (nativeIsA(value, "NSString")) {
        const text = str(value);
        if (text.length > 4096) throw new Error("TEXT_BOUND");
        return {present: true, nativeClass: cn, kind: "NSSTRING", value: text};
    }
    if (nativeIsA(value, "NSAttributedString")) {
        return {present: true, nativeClass: cn, kind: "NSATTRIBUTEDSTRING",
                plainString: nativeValue(obj(value, "string"), depth + 1, state)};
    }
    if (nativeIsA(value, "WFObjectRepresentation")) {
        return {present: true, nativeClass: cn, kind: "WFOBJECTREPRESENTATION",
                observationSource: "stored-object-getter",
                object: nativeValue(obj(value, "object"), depth + 1, state)};
    }
    if (nativeIsA(value, "WFObjectType")) {
        return {present: true, nativeClass: cn, kind: "WFOBJECTTYPE",
                declaredClassName: nativeValue(obj(value, "className"), depth + 1, state)};
    }
    if (nativeIsA(value, "WFNumberContentItem")) {
        return {present: true, nativeClass: cn, kind: "WFNUMBERCONTENTITEM",
                number: nativeValue(obj(value, "number"), depth + 1, state)};
    }
    if (nativeIsA(value, "WFContentCollection")) {
        const its = obj(value, "items");
        let arr;
        if (nativeIsA(its, "NSArray")) arr = its;
        else if (nativeIsA(its, "NSOrderedSet")) arr = obj(its, "array");
        else throw new Error("CONTENT_ITEMS_CONTAINER");
        const length = count(arr);
        if (length > 64) throw new Error("CARDINALITY_BOUND");
        const values = [];
        for (let i = 0; i < length; i++) values.push(nativeValue(arr.objectAtIndex(i), depth + 1, state));
        return {present: true, nativeClass: cn, kind: "WFCONTENTCOLLECTION",
                itemContainerClass: cls(its), cardinality: length, items: values};
    }
    if (nativeIsA(value, "NSArray") || nativeIsA(value, "NSOrderedSet")) {
        const orderedSet = nativeIsA(value, "NSOrderedSet");
        const arr = orderedSet ? obj(value, "array") : value;
        const length = count(arr);
        if (length > 64) throw new Error("CARDINALITY_BOUND");
        const values = [];
        for (let i = 0; i < length; i++) values.push(nativeValue(arr.objectAtIndex(i), depth + 1, state));
        return {present: true, nativeClass: cn, kind: orderedSet ? "NSORDEREDSET" : "NSARRAY",
                cardinality: length, items: values};
    }
    if (nativeIsA(value, "NSDictionary")) {
        const keys = obj(value, "allKeys"), length = count(keys);
        if (length > 64) throw new Error("CARDINALITY_BOUND");
        const entries = [];
        for (let i = 0; i < length; i++) {
            const key = keys.objectAtIndex(i);
            entries.push({key: nativeValue(key, depth + 1, state),
                          value: nativeValue(value.objectForKey(key), depth + 1, state)});
        }
        return {present: true, nativeClass: cn, kind: "NSDICTIONARY", cardinality: length, entries: entries};
    }
    if (nativeIsA(value, "WFContentItem")) {
        const kind = nativeIsA(value, "WFStringContentItem") ? "WFSTRINGCONTENTITEM" :
                     nativeIsA(value, "WFDictionaryContentItem") ? "WFDICTIONARYCONTENTITEM" : "WFCONTENTITEM";
        return {present: true, nativeClass: cn, kind: kind,
                representationSource: "stored-representationsByType",
                representations: nativeValue(obj(value, "representationsByType"), depth + 1, state)};
    }
    return {present: true, nativeClass: cn, kind: "UNOBSERVED"};
}
function actionIdentity(action) {
    const index = Number(workflow.indexOfAction(action));
    const uuid = obj(action, "UUID"), identifier = obj(action, "identifier");
    if (!Number.isInteger(index) || index < 0 || index >= originalActionCount) throw new Error("ACTION_INDEX");
    return {index: index, actualUUID: isNil(uuid) ? null : str(uuid),
            identifier: str(identifier), actualClass: cls(action)};
}
function callback(fn) {
    try { fn(); }
    catch (e) {
        pilot.callbackError = {name: String(e.name), message: String(e.message).slice(0, 1024)};
        pilot.done = true;
        emit("callback-error", pilot.callbackError);
        if (controller !== null) voidCall(controller, "stop");
    }
}
function run(argv) {
    if (argv.length !== 2) throw new Error("TWO_FIXED_PATHS");
    pilot = {schema: "cherri-workflowkit-original-action-pilot/1", events: [], done: false,
             engineRunInvocations: 0, sourceProgramsChanged: false, signedImportExercised: false,
             iOSDeviceExercised: false, formalAcceptanceGranted: false,
             construction: "original-action-array-through-WFAction-factory; transient-WFWorkflow; no-library-import"};
    journal = $.NSFileHandle.fileHandleForWritingAtPath($(argv[1]));
    if (isNil(journal)) throw new Error("CREATE_ONLY_JOURNAL_REQUIRED");
    try {
        loadEngine();
        emit("engine-loaded", {});
        for (const binding of [
            ["WFObjectRepresentation", "object", "@16@0:8"],
            ["WFObjectType", "className", "@16@0:8"]
        ]) {
            if (!hasClass(binding[0])) throw new Error("STORED_REPRESENTATION_CLASS");
            const encoding = methodsFor(binding[0], false)[binding[1]];
            if (encoding !== binding[2]) throw new Error("STORED_REPRESENTATION_GETTER_ENCODING");
            emit("stored-representation-getter-bound",
                 {nativeClass: binding[0], selector: binding[1], actualEncoding: encoding});
        }
        const registryClass = "WFActionRegistry";
        if (!hasClass(registryClass)) throw new Error("ACTION_REGISTRY_CLASS_ABSENT");
        const registryMethods = methodsFor(registryClass, true);
        emit("registry-capability", {classMethods: registryMethods});
        if (!("sharedRegistry" in registryMethods)) throw new Error("SHARED_REGISTRY_SELECTOR_ABSENT");
        emit("before-registry-instance", {});
        const registry = klass(registryClass).sharedRegistry;
        emit("after-registry-instance", {nativeClass: cls(registry)});
        const controlKey=$("cherri-binding-control-key"),controlValue=$("cherri-binding-control-value");
        const control=klass("NSDictionary").dictionaryWithObjectForKey(controlValue,controlKey);
        if(str(control.objectForKey(controlKey))!=="cherri-binding-control-value")throw new Error("DIRECT_BRIDGE_CONTROL");
        emit("direct-bridge-binding-control-passed",{nativeClass:cls(control),value:str(control.objectForKey(controlKey))});
        emit("before-original-data-read", {});
        const data = $.NSData.dataWithContentsOfFile($(argv[0]));
        emit("after-original-data-read", {bytes: Number(data.length)});
        if (isNil(data) || Number(data.length) < 1 || Number(data.length) > 2 * 1024 * 1024) throw new Error("ORIGINAL_BYTES_BOUND");
        emit("before-original-plist-decode", {});
        const error = Ref();
        const plist = $.NSPropertyListSerialization.propertyListWithDataOptionsFormatError(data, 0, null, error);
        emit("after-original-plist-decode", {nativeClass: cls(plist)});
        if (isNil(plist)) throw new Error("ORIGINAL_PLIST_PARSE");
        const encodedActions = plist.objectForKey($("WFWorkflowActions"));
        originalActions = encodedActions;
        originalActionCount = count(encodedActions);
        if (!Number.isInteger(originalActionCount) || originalActionCount < 1 || originalActionCount > 16) throw new Error("ORIGINAL_ACTION_COUNT");
        emit("before-original-actions-construction", {});
        actions = klass("WFAction").actionsFromSerializedRepresentationsActionRegistry(encodedActions, registry);
        if (isNil(actions) || count(actions) !== originalActionCount) throw new Error("MODEL_ACTION_COUNT");
        emit("after-original-actions-construction", {nativeClass: cls(actions), count: count(actions)});
        emit("before-transient-workflow-construction", {});
        workflow = bridgedMake("WFWorkflow", "initWithName:description:associatedAppBundleIdentifier:actions:",
                        $("CherriOriginalEnginePilot"), $(), $(), actions);
        if (isNil(workflow)) throw new Error("WORKFLOW_CONSTRUCTION");
        emit("after-transient-workflow-construction", {nativeClass: cls(workflow)});
        boolCall(workflow, "setSaveDisabled:", true);
        const fallback = plist.objectForKey($("WFWorkflowHasOutputFallback"));
        const inputVariables = plist.objectForKey($("WFWorkflowHasShortcutInputVariables"));
        for (const flag of [fallback, inputVariables]) {
            if (isNil(flag) || !nativeIsA(flag, "NSNumber") ||
                Number($.CFGetTypeID(toRef(flag))) !== Number($.CFBooleanGetTypeID())) throw new Error("ORIGINAL_FLAG_TYPE");
        }
        boolCall(workflow, "setHasOutputFallback:", Boolean(fallback.boolValue));
        boolCall(workflow, "setHasShortcutInputVariables:", Boolean(inputVariables.boolValue));
        const model = [], originalUUIDAbsent = [];
        for (let i = 0; i < originalActionCount; i++) {
            const identity = actionIdentity(actions.objectAtIndex(i));
            const encoded = originalActions.objectAtIndex(i);
            const originalIdentifier = encoded.objectForKey($("WFWorkflowActionIdentifier"));
            const parameters = encoded.objectForKey($("WFWorkflowActionParameters"));
            const originalUUID = isNil(parameters) ? $() : parameters.objectForKey($("UUID"));
            if (identity.index !== i || identity.identifier !== str(originalIdentifier)) throw new Error("ORIGINAL_ACTION_IDENTITY");
            if (isNil(originalUUID)) originalUUIDAbsent.push(i);
            else if (identity.actualUUID !== str(originalUUID)) throw new Error("ORIGINAL_ACTION_UUID");
            model.push(Object.assign(identity, {originalUUID: isNil(originalUUID) ? null : str(originalUUID)}));
        }
        emit("model-created", {actions: model, workflowClass: cls(workflow),
             originalUUIDAbsentAtIndex: originalUUIDAbsent,
             originalHasOutputFallback: Boolean(fallback.boolValue),
             originalHasShortcutInputVariables: Boolean(inputVariables.boolValue)});
        ObjC.registerSubclass({
            name: "CherriOriginalWorkflowObserver",
            superclass: "NSObject",
            protocols: ["WFWorkflowControllerDelegate"],
            methods: {
                "workflowControllerWillRun:": {
                    types: ["void", ["id"]],
                    implementation: function(c) { callback(() => emit("workflow-will-run",
                        {input: nativeValue(obj(c, "input"), 0)})); }
                },
                "workflowController:didRunAction:error:": {
                    types: ["void", ["id", "id", "id"]],
                    implementation: function(c, action, error) {
                        callback(() => emit("action-completed", Object.assign(actionIdentity(action), {
                            error: isNil(error) ? null : {nativeClass: cls(error), domain: str(error.domain), code: Number(error.code)},
                            inputObservationPhase: "action-completion",
                            input: nativeValue(obj(action, "input"), 0),
                            output: nativeValue(obj(action, "output"), 0),
                            controllerPendingInput: nativeValue(obj(c, "pendingActionInput"), 0)
                        })));
                    }
                },
                "workflowController:didFinishRunningWithError:cancelled:": {
                    types: ["void", ["id", "id", "bool"]],
                    implementation: function(c, error, cancelled) {
                        callback(() => {
                            emit("workflow-finished", {cancelled: Boolean(cancelled),
                                error: isNil(error) ? null : {nativeClass: cls(error), domain: str(error.domain), code: Number(error.code)},
                                output: nativeValue(obj(c, "output"), 0),
                                actionOutputsAtFinish: nativeValue(obj(c, "actionOutputs"), 0)});
                            pilot.done = true;
                        });
                    }
                }
            }
        });
        const delegate = $.CherriOriginalWorkflowObserver.alloc.init;
        controller = bridgedMake("WFWorkflowController", "init");
        voidCall(controller, "setWorkflow:", workflow);
        voidCall(controller, "setDelegate:", delegate);
        pilot.engineRunInvocations += 1;
        emit("engine-run-invoked", {controllerClass: cls(controller)});
        const stop = mono() + 8;
        voidCall(controller, "run");
        while (!pilot.done && mono() < stop) {
            $.NSRunLoop.currentRunLoop.runModeBeforeDate($.NSDefaultRunLoopMode,
                $.NSDate.dateWithTimeIntervalSinceNow(0.02));
        }
        if (!pilot.done) {
            emit("engine-observation-deadline", {});
            voidCall(controller, "stop");
            throw new Error("ENGINE_OBSERVATION_DEADLINE");
        }
        if (pilot.callbackError) throw new Error("CALLBACK_REJECTED");
        pilot.status = "ENGINE_CALLBACKS_RETAINED_VALIDATION_PENDING";
    } catch (e) {
        pilot.status = "FAILED";
        pilot.error = {name: String(e.name), message: String(e.message).slice(0, 1024)};
        emit("pilot-error", pilot.error);
    } finally {
        journal.synchronizeFile;
        journal.closeFile;
    }
    return JSON.stringify(pilot);
}
