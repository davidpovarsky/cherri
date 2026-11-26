"""Actual paired R2 no-input policy controls; no UIKit or Shortcut execution.

Reuse the original source-authored vectors and exact Receiver trigger/reset
methods. R2 changes only the fixture/workflow pair; cross-pair controls remain
independent, and the original verifier still runs unchanged for the old pair.
"""
import argparse
import hashlib
import json
from pathlib import Path
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit

import verify_primary_no_input as original

FIXTURE = "qa-function-envelope-presence-v2-r2"
WORKFLOW = "CherriFunctionEnvelopePresenceProbeR2"


def replace_query(url, key, value):
    parts = urlsplit(url)
    pairs = parse_qsl(parts.query, keep_blank_values=True)
    if sum(name == key for name, _ in pairs) != 1:
        raise ValueError("unambiguous authored URL key required")
    pairs = [(name, value if name == key else item) for name, item in pairs]
    return urlunsplit(parts._replace(query=urlencode(pairs)))


def vectors():
    # This transformation is confined to authored test inputs, not observed
    # results, the actual policy, or the copied receiver methods.
    cases = original.control_vectors()
    for item in cases:
        item["contract"] = item["contract"].replace(original.FIXTURE, FIXTURE).replace(original.WORKFLOW, WORKFLOW)
    healthy = json.loads(cases[0]["contract"])
    old = original.plan()
    def add(name, value, accepted=False, reason=None):
        cases.append({"name": name, "contract": original.canonical(value),
                      "accepted": accepted, "reason": reason})
    add("paired_original_still_accepted", old, True)
    add("cross_pair_old_fixture_r2_workflow", {**healthy, "fixtureId": original.FIXTURE}, reason="CONTRACT_IDENTITY")
    add("cross_pair_r2_fixture_old_workflow", {**healthy, "workflowName": original.WORKFLOW}, reason="CONTRACT_IDENTITY")
    add("unregistered_pair", {**healthy, "fixtureId": FIXTURE + "-r3", "workflowName": WORKFLOW + "R3"}, reason="CONTRACT_IDENTITY")
    add("r2_contract_old_primary", {**healthy, "primaryURL": old["primaryURL"]}, reason="PRIMARY_URL")
    add("old_contract_r2_primary", {**old, "primaryURL": healthy["primaryURL"]}, reason="PRIMARY_URL")
    for callback in ("x-success", "x-error", "x-cancel"):
        old_callbacks = dict(parse_qsl(urlsplit(old["primaryURL"]).query))
        add("r2_contract_old_" + callback, {**healthy,
            "primaryURL": replace_query(healthy["primaryURL"], callback, old_callbacks[callback])}, reason="CALLBACK_URL")
        current_callbacks = dict(parse_qsl(urlsplit(healthy["primaryURL"]).query))
        add("old_contract_r2_" + callback, {**old,
            "primaryURL": replace_query(old["primaryURL"], callback, current_callbacks[callback])}, reason="CALLBACK_URL")
    if len({item["name"] for item in cases}) != len(cases):
        raise ValueError("duplicate paired control name")
    return cases


def replace_once(source, old, new):
    if source.count(old) != 1:
        raise ValueError("original Swift control template boundary changed")
    return source.replace(old, new)


def prepare(root, artifacts):
    base = root / original.BASE
    receiver_path = base / "XcodeProject/ReceiverApp/ReceiverApp.swift"
    policy_path = base / "XcodeProject/Shared/PrimaryNoInputLaunch.swift"
    template_path = base / "verify_primary_no_input.swift"
    receiver = receiver_path.read_text()
    trigger = original.method(receiver, "    private func triggerExternalShortcutURL(", "    private func handleLaunchArguments()")
    reset = original.method(receiver, "    func resetStore() {", "    func loadExisting() {")
    program = replace_once(template_path.read_text(), "/*ACTUAL_TRIGGER_METHOD*/", trigger)
    program = replace_once(program, "/*ACTUAL_RESET_METHOD*/", reset)
    marker = '        let healthyRaw = vectors.first { $0.name == "healthy" }!.contract\n'
    partial = '''        // Keep the actual contract failures even when the unextended policy
        // cannot construct the R2 healthy plan. No trigger is credited in RED.
        let contractResult: [String: Any] = [
            "schema": "cherri-primary-no-input-r2-contract-controls/1",
            "checks": checks, "count": checks.count,
            "failures": checks.filter { $0["outcome"] as? String == "FAIL" }.count,
            "actualReceiverTriggerExecuted": false,
            "appleRuntimeExecuted": false, "formalAcceptanceGranted": false
        ]
        try JSONSerialization.data(withJSONObject: contractResult, options: [.sortedKeys])
            .write(to: root.appendingPathComponent("contract-result.original.json"), options: .withoutOverwriting)
'''
    program = replace_once(program, marker, partial + marker)
    marker = '            let originalContract = try? Data(contentsOf: directory(store).appendingPathComponent("contract.original.json"))\n'
    paired_receipts = '''            require(originals.allSatisfy {
                $0["fixtureId"] as? String == "qa-function-envelope-presence-v2-r2" &&
                $0["workflowName"] as? String == "CherriFunctionEnvelopePresenceProbeR2"
            }, "callsite/r2_receipt_identity_" + String(success), scope: "actual-trigger-method")
'''
    program = replace_once(program, marker, paired_receipts + marker)
    marker = '        let failures = checks.filter { $0["outcome"] as? String == "FAIL" }\n'
    cross_call = '''        let crossPair = try makeStore("cross-pair-trigger")
        let oldRaw = vectors.first { $0.name == "paired_original_still_accepted" }!.contract
        let oldContract = try QAPrimaryNoInputContract(oldRaw)
        app.clear(success: true)
        let crossTrigger = ActualTriggerBoundary(crossPair)
        crossTrigger.invoke(oldContract.primaryURL, source: "env")
        try drain()
        crossTrigger.invoke(healthy.primaryURL, source: "onOpenURL")
        try drain()
        require(app.primaryURLs.isEmpty && app.sceneURLs.isEmpty,
                "callsite/cross_pair_first_consumes_attempt", scope: "actual-trigger-method")
        let crossRows = rows(crossPair)
        require(crossRows.count == 2 &&
                crossRows.first?["reason"] as? String == "REQUESTED_URL_MISMATCH" &&
                crossRows.last?["reason"] as? String == "ATTEMPT_ALREADY_CONSUMED" &&
                crossRows.first?["requestedURLUTF8Base64"] as? String == Data(oldContract.primaryURL.utf8).base64EncodedString() &&
                crossRows.allSatisfy { $0["attemptOrdinal"] as? Int == 0 &&
                    $0["fixtureId"] as? String == "qa-function-envelope-presence-v2-r2" &&
                    $0["workflowName"] as? String == "CherriFunctionEnvelopePresenceProbeR2" },
                "callsite/cross_pair_originals_retain_selected_identity")

'''
    program = replace_once(program, marker, cross_call + marker)
    artifacts.mkdir(parents=False, exist_ok=False)
    generated = artifacts / "ActualReceiverNoInputR2.swift"
    generated.write_text(program)
    controls = artifacts / "controls.original.json"
    controls.write_text(original.canonical(vectors()))
    paths = [receiver_path, policy_path, template_path, Path(original.__file__).resolve(), Path(__file__).resolve()]
    pins = {path.relative_to(root).as_posix(): {
        "bytes": path.stat().st_size, "sha256": hashlib.sha256(path.read_bytes()).hexdigest()} for path in paths}
    extraction = {"schema": "cherri-primary-no-input-r2-extraction/1", "sourcePins": pins,
        "originalTemplateSHA256": hashlib.sha256(template_path.read_bytes()).hexdigest(),
        "triggerMethodSHA256": hashlib.sha256(trigger.encode()).hexdigest(),
        "resetMethodSHA256": hashlib.sha256(reset.encode()).hexdigest(),
        "generatedSHA256": hashlib.sha256(generated.read_bytes()).hexdigest(),
        "controlsSHA256": hashlib.sha256(controls.read_bytes()).hexdigest(),
        "controls": len(vectors()), "originalVectorCount": len(original.control_vectors()),
        "selectedFixture": FIXTURE, "selectedWorkflow": WORKFLOW,
        "actualTriggerCopiedByteForByte": True, "actualResetCopiedByteForByte": True,
        "nativeRuntimeExecuted": False, "formalAcceptanceGranted": False}
    (artifacts / "extraction.original.json").write_text(original.canonical(extraction))
    return policy_path, generated, controls, pins


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--swiftc", default="swiftc")
    parser.add_argument("--artifacts-dir", type=Path, required=True)
    parser.add_argument("--prepare-only", action="store_true")
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    policy, generated, controls, pins = prepare(root, args.artifacts_dir)
    if args.prepare_only:
        print("PRIMARY_NO_INPUT_R2_PREPARED nativeRuntime=NOT_RUN")
        return 0
    binary = args.artifacts_dir / "verify-primary-no-input-r2"
    code = original.run_process([args.swiftc, "-parse-as-library", str(policy), str(generated), "-o", str(binary)],
                                args.artifacts_dir, "compile", 120)
    if code == 0:
        code = original.run_process([str(binary), str(controls), str(args.artifacts_dir / "execution")],
                                    args.artifacts_dir, "execute", 30)
    if not all((root / rel).stat().st_size == pin["bytes"] and
               hashlib.sha256((root / rel).read_bytes()).hexdigest() == pin["sha256"] for rel, pin in pins.items()):
        raise ValueError("source changed during actual R2 policy controls")
    return code


if __name__ == "__main__":
    raise SystemExit(main())
