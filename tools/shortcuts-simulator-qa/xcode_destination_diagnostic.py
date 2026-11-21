"""Bounded active-Xcode Simulator launch and safe failure evidence.

Only fixed classifications, hashes and numeric OS error codes are public.
Original combined stdout/stderr stays in the existing quarantined builds area.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import time

from simulator_session_recovery import integer, require, run_process

LIMIT = 256 * 1024
ERROR_PATTERNS = {
    "APPLICATION_LOOKUP_FAILED": r"unable to find application named|could not find application|application .{0,256} does not exist",
    "APPLICATION_LAUNCH_FAILED": r"cannot be opened|could not be opened|failed to launch|LSOpenURLsWithRole",
    "RUNTIME_UNAVAILABLE": r"(?:iOS|watchOS|tvOS|visionOS).{0,80}(?:not installed|unavailable)",
    "DESTINATION_UNAVAILABLE": r"unable to find a destination|no devices? available",
}
DOMAINS = ("NSOSStatusErrorDomain", "NSCocoaErrorDomain", "RBSRequestErrorDomain", "FBSOpenApplicationServiceErrorDomain")


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def encoded(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False) + "\n").encode()


def safe_detail(raw):
    result = {"encoding": "UTF8", "knownErrors": [], "systemErrorCodes": [],
              "destinationRows": 0, "placeholderRows": 0, "platforms": [], "osVersions": []}
    if len(raw) > LIMIT:
        result["encoding"] = "OVERSIZED"
        return result
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError:
        result["encoding"] = "INVALID_UTF8"
        return result
    if "\x00" in text:
        result["encoding"] = "NUL_PRESENT"
        return result
    result["knownErrors"] = sorted(key for key, pattern in ERROR_PATTERNS.items()
                                   if re.search(pattern, text, flags=re.IGNORECASE))
    codes = set()
    for domain in DOMAINS:
        for match in re.finditer(re.escape(domain) + r"[^\r\n]{0,80}?\bCode\s*=\s*(-?[0-9]{1,10})\b", text):
            value = int(match[1])
            if -(2**31) <= value < 2**31:
                codes.add((domain, value))
    for match in re.finditer(r"LSOpenURLsWithRole[^\r\n]{0,120}?\berror\s+(-?[0-9]{1,10})\b", text):
        value = int(match[1])
        if -(2**31) <= value < 2**31:
            codes.add(("NSOSStatusErrorDomain", value))
    result["systemErrorCodes"] = [{"domain": domain, "code": code} for domain, code in sorted(codes)[:16]]
    platforms, versions = set(), set()
    for line in text.splitlines():
        line = line.strip()
        if not line.startswith("{") or not line.endswith("}"):
            continue
        result["destinationRows"] += 1
        result["placeholderRows"] += int("placeholder" in line.lower())
        for platform in re.findall(r"(?:^|,)\s*platform:\s*([^,{}]+)", line[1:-1]):
            if platform.strip() in ("iOS", "iOS Simulator", "macOS", "tvOS", "tvOS Simulator", "watchOS", "watchOS Simulator", "visionOS", "visionOS Simulator"):
                platforms.add(platform.strip())
        versions.update(re.findall(r"(?:^|,)\s*OS:\s*([0-9]{1,3}(?:\.[0-9]{1,3}){0,2})(?=,|$)", line[1:-1]))
    result["platforms"], result["osVersions"] = sorted(platforms), sorted(versions)
    return result


def output_record(root, relative):
    root = Path(root)
    path = root / relative
    require(root.is_dir() and not root.is_symlink() and path.is_file()
            and not path.is_symlink() and not path.parent.is_symlink()
            and path.resolve().is_relative_to(root.resolve()), "DESTINATION_DIAGNOSTIC_ORIGINAL")
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(65536), b""):
            digest.update(block)
    raw = path.read_bytes() if path.stat().st_size <= LIMIT else b"X" * (LIMIT + 1)
    return {"path": relative, "bytes": path.stat().st_size, "sha256": digest.hexdigest(),
            "safeObservation": safe_detail(raw)}


def selected_application(raw):
    require(0 < len(raw) <= 8192, "SIMULATOR_TOOL_OUTPUT_SIZE")
    text = raw.decode("utf-8").strip()
    require("\n" not in text and "\r" not in text and "\x00" not in text, "SIMULATOR_TOOL_OUTPUT")
    tool = Path(text)
    require(tool.is_absolute() and tool.name == "simctl"
            and tool.parent.name == "bin" and tool.parent.parent.name == "usr"
            and tool.parent.parent.parent.name == "Developer"
            and tool.parent.parent.parent.parent.name == "Contents"
            and tool.parent.parent.parent.parent.parent.suffix == ".app", "SIMULATOR_TOOL_PATH")
    return tool, tool.parent.parent.parent / "Applications/Simulator.app"


def prepare(root, udid, allowance, *, runner=run_process, now=time.monotonic):
    """Select one active-Xcode preparation path, without asserting readiness.

    A headless selection is followed by the caller's existing owned-device
    bootstatus and fresh Xcode destination query. Tool resolution alone never
    authorizes a test and is never reported as a graphical application launch.
    """
    return launch(root, udid, allowance, runner=runner, now=now, headless=True)


def launch(root, udid, allowance, *, runner=run_process, now=time.monotonic, headless=False):
    started = now()
    receipt = {"schemaVersion": "1.0", "selectionMethod": "XCRUN_ACTIVE_SIMCTL",
               "allowanceMs": round(allowance * 1000), "elapsedMs": 0,
               "toolPathSha256": None, "simulatorAppPathSha256": None,
               "toolExists": False, "simulatorAppExists": False,
               "selectionStatus": "NOT_RESOLVED", "commands": [], "formalAcceptanceGranted": False}
    if headless:
        receipt.update(schemaVersion="1.1", preparationMode="UNRESOLVED",
                       toolExecutable=False, toolBoundToActiveDeveloper=False,
                       simulatorAppAbsent=False, simulatorAppBoundToActiveDeveloper=False)
    result = {"result": "DEADLINE_EXHAUSTED", "exitCode": None, "elapsedMs": 0}
    def command(stage, argv, limit):
        before = now()
        budget = max(0, min(limit, allowance - (before - started)))
        relative = "builds/xcode-destination-" + stage + ".log"
        path = Path(root) / relative
        require(not path.exists() and not path.is_symlink(), "SIMULATOR_LAUNCH_ORIGINAL_EXISTS")
        with path.open("xb") as stream:
            observed = runner(argv, budget, output=stream) if budget else {
                "result": "DEADLINE_EXHAUSTED", "exitCode": None, "elapsedMs": 0}
        if observed["result"] == "PASS" and (now() - before > budget
                or observed["elapsedMs"] > round(budget * 1000) + 1):
            observed = {**observed, "result": "DEADLINE_EXHAUSTED"}
        receipt["commands"].append({"stage": stage, "allowanceMs": round(budget * 1000),
                                    **observed, "combinedOutput": output_record(root, relative)})
        return observed, path
    result, output = command("resolve", ["xcrun", "--find", "simctl"], 5)
    if result["result"] == "PASS":
        try:
            tool, app = selected_application(output.read_bytes())
            receipt.update(toolPathSha256=sha(str(tool).encode()), simulatorAppPathSha256=sha(str(app).encode()),
                           toolExists=tool.is_file(), simulatorAppExists=app.is_dir())
            if headless:
                # Follow aliases only within the selected physical Developer
                # root. A missing GUI app is different from a file, dangling
                # link, foreign application or missing/non-executable tool.
                developer = tool.parent.parent.parent.resolve(strict=True)
                actual_tool = tool.resolve(strict=True)
                selected_application(str(actual_tool).encode())
                receipt.update(toolExecutable=os.access(actual_tool, os.X_OK),
                    toolBoundToActiveDeveloper=actual_tool.parent.parent.parent == developer,
                    simulatorAppAbsent=not app.exists() and not app.is_symlink())
                if receipt["simulatorAppExists"]:
                    actual_app = app.resolve(strict=True)
                    receipt["simulatorAppBoundToActiveDeveloper"] = (
                        actual_app.name == "Simulator.app" and actual_app.parent.name == "Applications"
                        and actual_app.parent.parent == developer)
                require(receipt["toolExists"] and receipt["toolExecutable"]
                        and receipt["toolBoundToActiveDeveloper"], "SIMULATOR_TOOL_UNBOUND")
                require(receipt["simulatorAppAbsent"] or (
                        receipt["simulatorAppExists"] and receipt["simulatorAppBoundToActiveDeveloper"]),
                        "SIMULATOR_APP_INVALID")
            else:
                require(receipt["toolExists"] and receipt["simulatorAppExists"], "SIMULATOR_APP_MISSING")
        except (OSError, ValueError, UnicodeError):
            receipt["selectionStatus"] = "RESOLUTION_REJECTED"
            result = {"result": "LAUNCH_FAILED", "exitCode": None, "elapsedMs": 0}
        else:
            if headless and receipt["simulatorAppAbsent"]:
                receipt.update(selectionStatus="HEADLESS_TOOL_RESOLVED", preparationMode="HEADLESS_BOOTSTATUS")
                # PASS refers to prepare/resolution, never to an absent launch.
                # The actual resolver process already supplied exitCode=0.
            else:
                receipt["selectionStatus"] = "BOUND_TO_ACTIVE_XCODE"
                if headless:
                    receipt["preparationMode"] = "GRAPHICAL_LAUNCH"
                result, _ = command("launch", ["open", "-g", "-a", str(app), "--args", "-CurrentDeviceUDID", udid], 15)
    else:
        receipt["selectionStatus"] = "RESOLVE_COMMAND_FAILED"
    elapsed = max(0, round((now() - started) * 1000))
    receipt["elapsedMs"] = elapsed
    if result["result"] == "PASS" and elapsed > round(allowance * 1000):
        result = {**result, "result": "DEADLINE_EXHAUSTED"}
    return {**result, "elapsedMs": elapsed}, receipt


def document(root, destination, launch_receipt):
    return {"schemaVersion": "1.0", "scope": "DESTINATION_INFRASTRUCTURE_ONLY",
            "compilerCommitSha": destination["compilerCommitSha"], "runId": destination["runId"],
            "fixtureId": destination["fixtureId"], "deviceIdSha256": destination["deviceIdSha256"],
            "destinationReceiptCanonicalSha256": sha(encoded(destination)),
            "queryOutputs": [output_record(root, query["rawRelativePath"]) for query in destination["queries"]],
            "launch": launch_receipt, "formalAcceptanceGranted": False}


def validate(value, root, destination):
    require(isinstance(value, dict) and isinstance(destination, dict), "DESTINATION_DIAGNOSTIC_TYPE")
    launch_receipt = value.get("launch")
    preparation = destination.get("schemaVersion") == "1.2"
    require((launch_receipt is not None) == destination["refreshAttempted"], "DESTINATION_DIAGNOSTIC_REFRESH")
    if launch_receipt is not None:
        fields = {"schemaVersion", "selectionMethod", "allowanceMs", "elapsedMs", "toolPathSha256",
                  "simulatorAppPathSha256", "toolExists", "simulatorAppExists", "selectionStatus",
                  "commands", "formalAcceptanceGranted"}
        if preparation:
            fields |= {"preparationMode", "toolExecutable", "toolBoundToActiveDeveloper",
                       "simulatorAppAbsent", "simulatorAppBoundToActiveDeveloper"}
        require(isinstance(launch_receipt, dict) and set(launch_receipt) == fields
                and launch_receipt["schemaVersion"] == ("1.1" if preparation else "1.0")
                and launch_receipt["selectionMethod"] == "XCRUN_ACTIVE_SIMCTL"
                and launch_receipt["formalAcceptanceGranted"] is False, "SIMULATOR_LAUNCH_RECEIPT")
        aggregate = next(command for command in destination["commands"]
                         if command["stage"] == ("prepare" if preparation else "open"))
        require(integer(launch_receipt["allowanceMs"], 0, 15000)
                and integer(launch_receipt["elapsedMs"], 0, 86400000)
                and launch_receipt["allowanceMs"] == aggregate["allowanceMs"]
                and launch_receipt["elapsedMs"] == aggregate["elapsedMs"], "SIMULATOR_LAUNCH_TIMING")
        require(type(launch_receipt["toolExists"]) is bool
                and type(launch_receipt["simulatorAppExists"]) is bool, "SIMULATOR_LAUNCH_FLAGS")
        if preparation:
            require(all(type(launch_receipt[key]) is bool for key in (
                "toolExecutable", "toolBoundToActiveDeveloper", "simulatorAppAbsent",
                "simulatorAppBoundToActiveDeveloper")), "SIMULATOR_PREPARATION_FLAGS")
            require(not (launch_receipt["simulatorAppExists"] and launch_receipt["simulatorAppAbsent"]),
                    "SIMULATOR_PREPARATION_APP_CONTRADICTION")
        for key in ("toolPathSha256", "simulatorAppPathSha256"):
            digest = launch_receipt[key]
            require(digest is None or (isinstance(digest, str)
                    and re.fullmatch(r"[0-9a-f]{64}", digest)), "SIMULATOR_LAUNCH_PATH_HASH")
        commands = launch_receipt["commands"]
        require(isinstance(commands, list) and 1 <= len(commands) <= 2, "SIMULATOR_LAUNCH_COMMANDS")
        for index, command in enumerate(commands):
            require(isinstance(command, dict) and set(command) == {
                "stage", "allowanceMs", "elapsedMs", "exitCode", "result", "combinedOutput"}, "SIMULATOR_LAUNCH_FIELDS")
            require(command["stage"] == ("resolve", "launch")[index], "SIMULATOR_LAUNCH_ORDER")
            require(integer(command["allowanceMs"], 0, 5000 if index == 0 else 15000)
                    and integer(command["elapsedMs"], 0, 86400000), "SIMULATOR_LAUNCH_BUDGET")
            require(command["result"] in {"PASS", "EXIT_NONZERO", "TIMEOUT", "LAUNCH_FAILED", "DEADLINE_EXHAUSTED"}
                    and (command["exitCode"] is None or integer(command["exitCode"], -255, 255)), "SIMULATOR_LAUNCH_RESULT")
            require(command["result"] != "PASS" or (command["exitCode"] == 0
                    and command["elapsedMs"] <= command["allowanceMs"] + 1), "SIMULATOR_LAUNCH_LATE_PASS")
            require(command["result"] != "EXIT_NONZERO" or command["exitCode"] not in (0, None), "SIMULATOR_LAUNCH_FALSE_FAILURE")
            require(command["result"] not in ("TIMEOUT", "LAUNCH_FAILED")
                    or command["exitCode"] is None, "SIMULATOR_LAUNCH_FALSE_TIMEOUT")
            require(command["allowanceMs"] + sum(c["elapsedMs"] for c in commands[:index])
                    <= launch_receipt["allowanceMs"] + len(commands), "SIMULATOR_LAUNCH_BUDGET_EXTENSION")
            relative = "builds/xcode-destination-" + command["stage"] + ".log"
            require(encoded(command["combinedOutput"]) == encoded(output_record(root, relative)),
                    "SIMULATOR_LAUNCH_ORIGINAL_CHANGED")
        require(sum(command["elapsedMs"] for command in commands) <= launch_receipt["elapsedMs"] + len(commands),
                "SIMULATOR_LAUNCH_ELAPSED")
        status = launch_receipt["selectionStatus"]
        expected_result, expected_code = commands[-1]["result"], commands[-1]["exitCode"]
        if status == "RESOLVE_COMMAND_FAILED":
            require(len(commands) == 1 and commands[0]["result"] != "PASS"
                    and launch_receipt["toolPathSha256"] is None
                    and launch_receipt["simulatorAppPathSha256"] is None
                    and launch_receipt["toolExists"] is False and launch_receipt["simulatorAppExists"] is False,
                    "SIMULATOR_LAUNCH_FALSE_RESOLVE_FAILURE")
            if preparation:
                require(launch_receipt["preparationMode"] == "UNRESOLVED"
                        and all(launch_receipt[key] is False for key in (
                            "toolExecutable", "toolBoundToActiveDeveloper", "simulatorAppAbsent",
                            "simulatorAppBoundToActiveDeveloper")), "SIMULATOR_PREPARATION_FALSE_RESOLVE_FAILURE")
        else:
            allowed = {"BOUND_TO_ACTIVE_XCODE", "RESOLUTION_REJECTED"}
            if preparation:
                allowed.add("HEADLESS_TOOL_RESOLVED")
            require(status in allowed
                    and commands[0]["result"] == "PASS", "SIMULATOR_LAUNCH_FALSE_SELECTION")
            raw = (Path(root) / "builds/xcode-destination-resolve.log").read_bytes()
            try:
                tool, app = selected_application(raw)
            except (OSError, ValueError, UnicodeError):
                require(status == "RESOLUTION_REJECTED" and launch_receipt["toolPathSha256"] is None
                        and launch_receipt["simulatorAppPathSha256"] is None
                        and launch_receipt["toolExists"] is False and launch_receipt["simulatorAppExists"] is False,
                        "SIMULATOR_LAUNCH_INVALID_PATH")
            else:
                require(launch_receipt["toolPathSha256"] == sha(str(tool).encode())
                        and launch_receipt["simulatorAppPathSha256"] == sha(str(app).encode()), "SIMULATOR_LAUNCH_REBOUND")
            if status == "BOUND_TO_ACTIVE_XCODE":
                require(len(commands) == 2 and launch_receipt["toolExists"] is True
                        and launch_receipt["simulatorAppExists"] is True, "SIMULATOR_LAUNCH_UNBOUND")
                if preparation:
                    require(launch_receipt["preparationMode"] == "GRAPHICAL_LAUNCH"
                            and launch_receipt["toolExecutable"] and launch_receipt["toolBoundToActiveDeveloper"]
                            and launch_receipt["simulatorAppBoundToActiveDeveloper"]
                            and not launch_receipt["simulatorAppAbsent"], "SIMULATOR_PREPARATION_UNBOUND_GUI")
            elif status == "HEADLESS_TOOL_RESOLVED":
                require(preparation and len(commands) == 1 and launch_receipt["toolExists"] is True
                        and launch_receipt["toolExecutable"] and launch_receipt["toolBoundToActiveDeveloper"]
                        and launch_receipt["simulatorAppAbsent"] and not launch_receipt["simulatorAppExists"]
                        and not launch_receipt["simulatorAppBoundToActiveDeveloper"]
                        and launch_receipt["preparationMode"] == "HEADLESS_BOOTSTATUS",
                        "SIMULATOR_PREPARATION_UNBOUND_HEADLESS")
            else:
                valid_selection = launch_receipt["toolExists"] and launch_receipt["simulatorAppExists"]
                if preparation:
                    valid_selection = (launch_receipt["toolExists"] and launch_receipt["toolExecutable"]
                        and launch_receipt["toolBoundToActiveDeveloper"] and (
                            launch_receipt["simulatorAppAbsent"] or (
                                launch_receipt["simulatorAppExists"] and launch_receipt["simulatorAppBoundToActiveDeveloper"])))
                    require(launch_receipt["preparationMode"] == "UNRESOLVED", "SIMULATOR_PREPARATION_FALSE_REJECTION_MODE")
                require(len(commands) == 1 and not valid_selection, "SIMULATOR_LAUNCH_FALSE_REJECTION")
                expected_result, expected_code = "LAUNCH_FAILED", None
        require(aggregate["exitCode"] == expected_code and (
                aggregate["result"] == expected_result or (
                    aggregate["result"] == "DEADLINE_EXHAUSTED" and expected_result == "PASS"
                    and (aggregate["elapsedMs"] >= aggregate["allowanceMs"]
                         or destination["gateElapsedMs"] >= destination["gateBudgetMs"]))),
                "SIMULATOR_LAUNCH_AGGREGATE_RESULT")
    require(encoded(value) == encoded(document(root, destination, launch_receipt)), "DESTINATION_DIAGNOSTIC_RECOMPUTATION")
    return value
