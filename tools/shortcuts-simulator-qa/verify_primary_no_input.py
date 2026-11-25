"""Compile actual no-input policy and receiver trigger/reset methods.

UIApplication is a local observable test double only. These controls prove
policy and callsite behavior; they never invoke Shortcuts or grant native credit.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time
from urllib.parse import urlencode

SCHEMA = "cherri-function-envelope-no-input-launch/1"
FIXTURE = "qa-function-envelope-presence-v2"
WORKFLOW = "CherriFunctionEnvelopePresenceProbe"
BASE = "tools/shortcuts-simulator-qa"


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False)


def plan(nonce="owned-run-K_01"):
    callbacks = {}
    for key, host in (("x-success", "success"), ("x-error", "error"), ("x-cancel", "cancel")):
        callbacks[key] = "cherriqa://" + host + "?" + urlencode({
            "runId": nonce, "fixtureId": FIXTURE, "shortcutName": WORKFLOW,
            "source": "shortcuts-x-callback"})
    url = "shortcuts://x-callback-url/run-shortcut?" + urlencode({
        "name": WORKFLOW, "x-source": "CherriSimulatorQAReceiver", **callbacks})
    return {"schema": SCHEMA, "mode": "primary-no-input", "fixtureId": FIXTURE,
            "workflowName": WORKFLOW, "runNonce": nonce, "primaryURL": url}


def control_vectors():
    healthy = plan()
    raw = canonical(healthy)
    cases = []
    def add(name, value, accepted=False, reason=None, encoded=False):
        cases.append({"name": name, "contract": value if encoded else canonical(value),
                      "accepted": accepted, "reason": reason})
    add("healthy", healthy, True)
    add("nonce_one_byte", plan("x"), True)
    add("nonce_maximum_ascii", plan("K" * 128), True)
    for key in healthy:
        without = dict(healthy)
        del without[key]
        add("missing_" + key, without, reason="CONTRACT_FIELDS")
    add("extra_field", {**healthy, "input": None}, reason="CONTRACT_FIELDS")
    for value in (None, True, 7, [], {}):
        add("field_kind_" + type(value).__name__, {**healthy, "mode": value}, reason="CONTRACT_FIELDS")
    for key in ("schema", "mode", "fixtureId", "workflowName"):
        add("identity_" + key, {**healthy, key: str(healthy[key]) + "-other"}, reason="CONTRACT_IDENTITY")
    for name, nonce in (("empty", ""), ("long", "a"*129), ("hebrew", "שלום"),
                        ("emoji", "🙂"), ("space", "a b"), ("slash", "a/b"),
                        ("dot", "a.b"), ("newline", "a\nb")):
        add("nonce_" + name, plan(nonce), reason="CONTRACT_NONCE")
    for name, value in (("newline", raw + "\n"), ("space", " " + raw),
                        ("pretty", json.dumps(healthy, indent=2, ensure_ascii=False)),
                        ("reverse_order", json.dumps(dict(reversed(list(sorted(healthy.items())))), separators=(",", ":"))),
                        ("escaped_slash", raw.replace("/", r"\/")),
                        ("escaped_key", raw.replace('"schema"', r'"\u0073chema"')),
                        ("equal_duplicate", raw[:-1] + ',"runNonce":' + canonical(healthy["runNonce"]) + "}"),
                        ("conflicting_duplicate", raw[:-1] + ',"runNonce":"other"}')):
        add("noncanonical_" + name, value, reason="CONTRACT_NONCANONICAL", encoded=True)
    add("wrong_root", [], reason="CONTRACT_JSON")
    add("malformed_json", "{", reason="CONTRACT_JSON", encoded=True)
    add("empty_contract", "", reason="CONTRACT_SIZE", encoded=True)
    add("oversized_contract", " " * 65537, reason="CONTRACT_SIZE", encoded=True)

    url = healthy["primaryURL"]
    for name, changed in (
        ("input_text", url + "&input=text"), ("text_only", url + "&text="),
        ("unknown", url + "&unknown=x"), ("duplicate_name", url + "&name=" + WORKFLOW),
        ("duplicate_callback", url + "&x-success=x"), ("bad_percent", url + "%GG"),
        ("raw_unicode", url + "שלום"), ("fragment", url + "#fragment"),
        ("new_line", url + "\n"), ("uppercase_scheme", url.replace("shortcuts:", "SHORTCUTS:", 1)),
        ("wrong_host", url.replace("x-callback-url", "other", 1)),
        ("wrong_path", url.replace("/run-shortcut?", "/create-shortcut?", 1)),
        ("authority_user", url.replace("://x-callback-url", "://user@x-callback-url", 1)),
        ("authority_port", url.replace("://x-callback-url", "://x-callback-url:42", 1)),
        ("wrong_name", url.replace("name=" + WORKFLOW, "name=Other", 1)),
        ("wrong_x_source", url.replace("x-source=CherriSimulatorQAReceiver", "x-source=Other", 1)),
        ("too_long", url + "X" * 8192)):
        add("primary_" + name, {**healthy, "primaryURL": changed}, reason="PRIMARY_URL")

    for callback_key, host in (("x-success", "success"), ("x-error", "error"), ("x-cancel", "cancel")):
        fields = {"runId": healthy["runNonce"], "fixtureId": FIXTURE,
                  "shortcutName": WORKFLOW, "source": "shortcuts-x-callback"}
        alternatives = {
            "nonce": {**fields, "runId": "other"},
            "fixture": {**fields, "fixtureId": "other"},
            "name": {**fields, "shortcutName": "other"},
            "source": {**fields, "source": "receiver-self-test"},
            # Swift String compares canonical Unicode equivalence. The actual
            # wire contract must still reject this Kelvin-sign/K byte mismatch.
            "unicode_equivalent_nonce": {**fields, "runId": healthy["runNonce"].replace("K", "\u212a")},
            "extra_input": {**fields, "input": "text"},
        }
        callback = "cherriqa://" + host + "?" + urlencode(fields)
        wrong_callbacks = {
            key: "cherriqa://" + host + "?" + urlencode(value)
            for key, value in alternatives.items()
        }
        wrong_callbacks.update({
            "duplicate_equal": callback + "&runId=" + healthy["runNonce"],
            "missing_source": "cherriqa://" + host + "?" + urlencode({k:v for k,v in fields.items() if k!="source"}),
            "wrong_host": callback.replace("cherriqa://" + host, "cherriqa://unknown", 1),
            "path": callback.replace("?", "/extra?", 1),
            "empty": "",
        })
        for label, changed in wrong_callbacks.items():
            callbacks = {key: "cherriqa://" + h + "?" + urlencode(fields)
                         for key, h in (("x-success", "success"), ("x-error", "error"), ("x-cancel", "cancel"))}
            callbacks[callback_key] = changed
            outer = "shortcuts://x-callback-url/run-shortcut?" + urlencode({
                "name": WORKFLOW, "x-source": "CherriSimulatorQAReceiver", **callbacks})
            add(callback_key + "_" + label, {**healthy, "primaryURL": outer}, reason="CALLBACK_URL")
    names = [case["name"] for case in cases]
    if len(names) != len(set(names)):
        raise ValueError("duplicate control name")
    return cases


def method(source, start, end):
    if source.count(start) != 1 or source.count(end) != 1:
        raise ValueError("actual receiver method boundary is ambiguous")
    return source[source.index(start):source.index(end)]


def prepare(root, artifacts):
    receiver_path = root / BASE / "XcodeProject/ReceiverApp/ReceiverApp.swift"
    policy_path = root / BASE / "XcodeProject/Shared/PrimaryNoInputLaunch.swift"
    template_path = root / BASE / "verify_primary_no_input.swift"
    receiver = receiver_path.read_text()
    trigger = method(receiver, "    private func triggerExternalShortcutURL(", "    private func handleLaunchArguments()")
    reset = method(receiver, "    func resetStore() {", "    func loadExisting() {")
    template = template_path.read_text()
    for marker in ("/*ACTUAL_TRIGGER_METHOD*/", "/*ACTUAL_RESET_METHOD*/"):
        if template.count(marker) != 1:
            raise ValueError("template method boundary is ambiguous")
    program = template.replace("/*ACTUAL_TRIGGER_METHOD*/", trigger).replace("/*ACTUAL_RESET_METHOD*/", reset)
    artifacts.mkdir(parents=False, exist_ok=False)
    generated = artifacts / "ActualReceiverNoInput.swift"
    generated.write_text(program)
    cases = artifacts / "controls.original.json"
    cases.write_text(canonical(control_vectors()))
    paths = [receiver_path, policy_path, template_path, Path(__file__).resolve()]
    source_pins = {p.relative_to(root).as_posix(): {
        "bytes": p.stat().st_size, "sha256": hashlib.sha256(p.read_bytes()).hexdigest()} for p in paths}
    extraction = {"schema": "cherri-primary-no-input-extraction/1",
                  "sourcePins": source_pins,
                  "triggerMethodSHA256": hashlib.sha256(trigger.encode()).hexdigest(),
                  "resetMethodSHA256": hashlib.sha256(reset.encode()).hexdigest(),
                  "generatedSHA256": hashlib.sha256(generated.read_bytes()).hexdigest(),
                  "controlsSHA256": hashlib.sha256(cases.read_bytes()).hexdigest(),
                  "controls": len(control_vectors()), "nativeRuntimeExecuted": False,
                  "actualTriggerCopiedByteForByte": True, "actualResetCopiedByteForByte": True}
    (artifacts / "extraction.original.json").write_text(canonical(extraction))
    return policy_path, generated, cases, source_pins


def run_process(argv, artifacts, name, timeout):
    tick = time.monotonic()
    process = subprocess.run(argv, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout)
    (artifacts / (name + ".stdout.original.txt")).write_bytes(process.stdout)
    (artifacts / (name + ".stderr.original.txt")).write_bytes(process.stderr)
    receipt = {"argv": argv, "returncode": process.returncode, "elapsedSeconds": time.monotonic()-tick,
               "stdoutSHA256": hashlib.sha256(process.stdout).hexdigest(),
               "stderrSHA256": hashlib.sha256(process.stderr).hexdigest()}
    (artifacts / (name + ".process.original.json")).write_text(canonical(receipt))
    if process.stdout:
        print(process.stdout.decode("utf-8", errors="replace"), end="")
    if process.returncode:
        print(process.stderr.decode("utf-8", errors="replace"), end="")
    return process.returncode


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--swiftc", default="swiftc")
    parser.add_argument("--artifacts-dir", type=Path, required=True)
    parser.add_argument("--prepare-only", action="store_true")
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    policy, generated, cases, pins = prepare(root, args.artifacts_dir)
    if args.prepare_only:
        print("PRIMARY_NO_INPUT_PREPARED nativeRuntime=NOT_RUN")
        return 0
    executable = args.artifacts_dir / "verify-primary-no-input"
    code = run_process([args.swiftc, "-parse-as-library", str(policy), str(generated), "-o", str(executable)],
                       args.artifacts_dir, "compile", 120)
    if code == 0:
        code = run_process([str(executable), str(cases), str(args.artifacts_dir / "execution")],
                           args.artifacts_dir, "execute", 30)
    unchanged = all((root / rel).stat().st_size == pin["bytes"] and
                    hashlib.sha256((root / rel).read_bytes()).hexdigest() == pin["sha256"]
                    for rel, pin in pins.items())
    if not unchanged:
        raise ValueError("source changed during actual policy controls")
    return code


if __name__ == "__main__":
    raise SystemExit(main())
