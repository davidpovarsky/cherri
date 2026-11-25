"""Compile the receiver's actual Foundation-only sanitizer and split methods.

No receiver behavior is reimplemented. Controls preserve contrary public values,
normal trace separation, and existing private-data rejection before the new exact
diagnostic prefix is accepted. This does not typecheck SwiftUI or run a Simulator.
"""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile

from optional_forwarding_diagnostic import EXPECTED_OUTPUT, PREFIX


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--swiftc", default="swiftc")
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    source = (root / "tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/ReceiverApp.swift").read_text(encoding="utf-8")
    start = "    private static let forbiddenPattern: NSRegularExpression? = {"
    end = "    func handleIncomingURL(_ url: URL) {"
    if source.count(start) != 1 or source.count(end) != 1:
        raise ValueError("receiver policy boundary is ambiguous")
    policy = source[source.index(start):source.index(end)]
    program = "import Foundation\nenum ActualReceiverPolicy {\n" + policy + "\n}\n" + r'''
struct Control: Decodable { let raw: String?; let sanitized: String?; let summary: String?; let trace: String? }
@main enum VerifyActualReceiver {
  static func main() throws {
    let data = try Data(contentsOf: URL(fileURLWithPath: CommandLine.arguments[1]))
    let cases = try JSONDecoder().decode([Control].self, from: data)
    for (index, control) in cases.enumerated() {
      let split = ActualReceiverPolicy.splitResultAndTrace(control.raw)
      guard ActualReceiverPolicy.sanitizeAllowlisted(control.raw) == control.sanitized,
            split.summary == control.summary, split.trace == control.trace else {
        throw NSError(domain: "ActualReceiverPolicy", code: index + 1,
                      userInfo: [NSLocalizedDescriptionKey: "receiver policy control failed"])
      }
    }
    print("OPTIONAL_RECEIVER_POLICY_CONTROLS_PASS count=\(cases.count) nativeRuntime=NOT_RUN")
  }
}
'''
    controls = []
    def add(raw, sanitized, summary=None, trace=None):
        controls.append(dict(raw=raw, sanitized=sanitized, summary=summary, trace=trace))
    add(EXPECTED_OUTPUT, EXPECTED_OUTPUT, EXPECTED_OUTPUT)
    contrary = EXPECTED_OUTPUT.replace("NONE=NONE", "NONE=TEXT[]")
    add(contrary, contrary, contrary)
    for raw in (PREFIX.replace("_V1:", "_V2:") + "NONE=NONE", "private-prefix " + EXPECTED_OUTPUT, "PRIVATE_CANARY"):
        redacted = "[REDACTED_NON_ALLOWLISTED_LEN_" + str(len(raw)) + "]"
        add(raw, redacted, redacted)
    for secret in ("ghp_SYNTHETIC_CANARY", "canary@example.invalid", "/Users/SYNTHETIC", "BEGIN PRIVATE KEY"):
        add(PREFIX + secret, "[REDACTED_SECRET_OR_PII]", "[REDACTED_SECRET_OR_PII]")
    for raw in (None, "", " \n"):
        add(raw, None)
    add("CHERRI_QA_EXISTING", "CHERRI_QA_EXISTING", "CHERRI_QA_EXISTING")
    trace = '{"schemaVersion":"1.4","kind":"COLLECTOR_EVENT","stage":"collector_begin"}'
    raw = EXPECTED_OUTPUT + "\n" + trace
    add(raw, raw, EXPECTED_OUTPUT, trace)
    # Exercise the actual receiver policy for the two exact public nested prefixes.
    for prefix in ("CHERRI_NESTED_LIST_CONTROL_V1:", "CHERRI_NESTED_LIST_CANDIDATE_V1:"):
        for value in ("OUTER_COUNT=0", "CONTRARY_PUBLIC_VALUE"):
            raw = prefix + value
            add(raw, raw, raw)
            add(raw + "\n" + trace, raw + "\n" + trace, raw, trace)
        raw = prefix.replace("_V1:", "_V2:") + "PUBLIC"
        redacted = "[REDACTED_NON_ALLOWLISTED_LEN_" + str(len(raw)) + "]"
        add(raw, redacted, redacted)
        raw = prefix + "canary@example.invalid"
        add(raw, "[REDACTED_SECRET_OR_PII]", "[REDACTED_SECRET_OR_PII]")
    # The tagged engineering fixture has its own exact versioned public prefix.
    # Contrary observations must remain readable; this policy never asserts them.
    tagged = "CHERRI_TAGGED_VALUE_CARRIER_V1:"
    for value in ("PRESENT:TYPE_MATCH", "ABSENT:TYPE_DIFFERENT"):
        raw = tagged + value
        add(raw, raw, raw)
        add(raw + "\n" + trace, raw + "\n" + trace, raw, trace)
    for raw in (tagged.replace("_V1:", "_V2:") + "PUBLIC",
                tagged.removesuffix(":") + "PUBLIC",
                "private-prefix " + tagged + "PUBLIC"):
        redacted = "[REDACTED_NON_ALLOWLISTED_LEN_" + str(len(raw)) + "]"
        add(raw, redacted, redacted)
    for secret in ("ghp_SYNTHETIC_CANARY", "canary@example.invalid",
                   "/Users/SYNTHETIC", "BEGIN PRIVATE KEY"):
        add(tagged + secret, "[REDACTED_SECRET_OR_PII]", "[REDACTED_SECRET_OR_PII]")
    bounded = tagged + "A" * 20000
    add(bounded, bounded[:16384], bounded[:16384])

    with tempfile.TemporaryDirectory(prefix="optional-receiver-policy-") as directory:
        temp = Path(directory)
        code, cases = temp / "ReceiverPolicy.swift", temp / "controls.json"
        executable = temp / ("verify-receiver.exe" if os.name == "nt" else "verify-receiver")
        code.write_text(program, encoding="utf-8")
        cases.write_text(json.dumps(controls), encoding="utf-8")
        subprocess.run([args.swiftc, "-parse-as-library", str(code), "-o", str(executable)], check=True, timeout=120)
        subprocess.run([str(executable), str(cases)], check=True, timeout=30)


if __name__ == "__main__":
    main()
