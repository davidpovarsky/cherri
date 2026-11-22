"""Read-only reconciliation of retained public originals; never invokes tests."""
import ast
import collections
import datetime
import hashlib
import json
import math
import pathlib
import zipfile

HERE = pathlib.Path(__file__).resolve().parent
ORIGINALS = HERE / "public-proof-originals.root-fetched"
CURRENT = ORIGINALS / "windows/ai01-clean-29f35c71-20261122"
SEALED = ORIGINALS / "windows/ai01-stat-worker-source-20261122"
PACKAGE = HERE.parents[1] / "acceptance-ai-integration-29f35c71/python29-git/tools/language-ai-eval"
SOURCE = "29f35c71d4932ac2ed135f977ac99ec43cddfe3d"

def digest(raw):
    return hashlib.sha256(raw).hexdigest()

def pairs(values):
    result = {}
    for key, value in values:
        assert key not in result, ("duplicate JSON key", key)
        result[key] = value
    return result

def parse(raw):
    return json.loads(raw, object_pairs_hook=pairs, parse_constant=lambda value: (_ for _ in ()).throw(ValueError(value)))

def read(path):
    return parse(path.read_bytes())

manifest_raw = (HERE / "public-proof-manifest.root-fetched.json").read_bytes()
assert digest(manifest_raw) == "7472ca03f3f92cf6a93b6904b5661000cc2b3868db0d9f2e5f60df7ad75b2b40"
manifest = parse(manifest_raw)
assert manifest["tested_source_sha"] == SOURCE
assert (ORIGINALS / "manifest.json").read_bytes() == manifest_raw
members = manifest["members"]
assert len(members) == len({item["name"] for item in members}) == 347
with zipfile.ZipFile(HERE / "public-proof-originals.root-fetched.zip") as archive:
    assert set(archive.namelist()) == {item["name"] for item in members} | {"manifest.json"}
    for item in members:
        raw = archive.read(item["name"])
        assert len(raw) == item["bytes"] and digest(raw) == item["sha256"], item["name"]
        assert (ORIGINALS / item["name"]).read_bytes() == raw

directory = read(HERE / "github29-directory.root-fetched.json")
assert isinstance(directory, list) and len(directory) == 15
remote = {entry["path"]: entry for entry in directory}
bindings = read(CURRENT / "source-bindings.json")
assert bindings["source_sha"] == SOURCE
assert bindings["bindings"] == manifest["source_files"]
assert len(bindings["bindings"]) == len(remote) == 15
ast_inventory = {}
for item in bindings["bindings"]:
    name = pathlib.PurePosixPath(item["path"]).name
    raw = (PACKAGE / name).read_bytes()
    blob = hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest()
    assert len(raw) == item["bytes"] == remote[item["path"]]["size"]
    assert blob == item["git_blob"] == remote[item["path"]]["sha"]
    assert digest(raw) == item["sha256"]
    if name.endswith(".md"):
        assert (HERE / "github29-docs" / name).read_bytes() == raw
    if name.startswith("test_") and name.endswith(".py"):
        module = name[:-3]
        names = []
        for node in ast.parse(raw).body:
            if isinstance(node, ast.ClassDef):
                names.extend(module + "." + node.name + "." + method.name for method in node.body
                             if isinstance(method, (ast.FunctionDef, ast.AsyncFunctionDef)) and method.name.startswith("test_"))
        ast_inventory[name] = sorted(names)

frozen = read(SEALED / "parent-inventory.json")
assert ast_inventory == frozen
assert (SEALED / "candidate-input.original.json").read_bytes() == (HERE / "windows-candidate-input.json").read_bytes()
frozen_raw = (SEALED / "parent-inventory.json").read_bytes()
assert len(frozen_raw) == 14389 and digest(frozen_raw) == "4f93cc9f7c31cbb01010444aa2fce0b082570354b8fa16ca949decf0e26bbf0d"
expected = sorted(name for names in frozen.values() for name in names)
actual_inventory = read(CURRENT / "inventory.json")
assert len(expected) == len(set(expected)) == 132
assert sorted(actual_inventory["testIds"]) == expected and actual_inventory["count"] == 132
process = read(CURRENT / "process.json")
assert process == manifest["current_full_suite"]
assert process["source_sha"] == SOURCE and process["pid"] == 26740
assert process["returncode"] == 0 and process["timed_out"] is False
assert process["outer_budget_seconds"] == 150 and process["unchanged_test_cli_budget_seconds"] == 15
assert math.isfinite(process["elapsed_seconds"]) and 0 < process["elapsed_seconds"] < 150
assert digest((CURRENT / "run-controls.py").read_bytes()) == process["driver_sha256"] == "f12fc917ed3c1f1aefccdd0bc7eb5dc1d6a548290ef5105a9af36f26d351ecd0"
assert digest((CURRENT / "submitted-wrapper.original.py").read_bytes()) == "954513e7abf894399d858663ab76636efe36111f950366def2145aa5a4ba9e60"
for stream in ("stdout", "stderr"):
    assert digest((CURRENT / (stream + ".original")).read_bytes()) == process[stream + "_sha256"]
for phase in ("before", "after"):
    assert read(CURRENT / ("git-" + phase + ".json")) == {"head": SOURCE, "status": ""}

events = [parse(line) for line in (CURRENT / "test-events.jsonl").read_bytes().splitlines() if line]
counts = collections.Counter()
started = []
terminal = []
active = None
previous = -math.inf
for event in events:
    stamp = event["monotonic"]
    assert type(stamp) in (int, float) and math.isfinite(stamp) and stamp >= previous
    previous = stamp
    action, test = event["event"], event["test"]
    assert test in expected
    counts[action] += 1
    if action == "START":
        assert active is None and test not in started
        started.append(test)
        active = test
    elif action in ("PASS", "FAIL", "ERROR", "SKIP"):
        assert test == active and test not in terminal
        terminal.append(test)
        active = None
    else:
        assert action in ("SUBCASE_PASS", "SUBCASE_FAIL") and test == active
assert active is None and sorted(started) == sorted(terminal) == expected
for name, number in process["counts"].items():
    assert counts[name] == number, (name, counts[name], number)
assert dict(counts) == {"START": 132, "PASS": 132, "SUBCASE_PASS": 78}
assert events[-1]["monotonic"] - events[0]["monotonic"] <= process["elapsed_seconds"]
assert read(CURRENT / "test-result.json") == {"testsRun": 132, "failures": 0, "errors": 0, "skipped": [], "successful": True}
reconciliation = read(CURRENT / "process-reconciliation.json")
assert reconciliation["no_owned_process_active"] is True
assert reconciliation["owned_processes"] == [{"pid": 26740, "status": "EXITED", "exit_code": 0, "error": 0}]
assert datetime.datetime.fromisoformat(reconciliation["observed_at"]) > datetime.datetime.fromisoformat(process["finished_at"])

for stage in manifest["historical_stage_accounting"]:
    stage_file = ORIGINALS / "windows" / stage["directory"] / "test-events.jsonl"
    stage_counts = collections.Counter(parse(line)["event"] for line in stage_file.read_bytes().splitlines() if line)
    assert all(stage_counts[key] == number for key, number in stage["event_counts"].items())
assert manifest["model_generation_requests"] == 0
assert manifest["ai_quality_measured"] is manifest["formal_acceptance_granted"] is False
report = {
    "audited_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "reviewer": "root independent read-only original reconciliation",
    "proof_commit": "04be8b60953521dce72597754e41a270d604a968",
    "tested_source": SOURCE,
    "archive_sha256": digest((HERE / "public-proof-originals.root-fetched.zip").read_bytes()),
    "original_members_verified": len(members),
    "package_files_verified_against_fresh_github_blobs": len(remote),
    "frozen_windows_inventory_sha256": digest(frozen_raw),
    "static_ast_and_execution_inventory_count": len(expected),
    "actual_event_counts": dict(counts),
    "historical_stage_counts_reconciled": len(manifest["historical_stage_accounting"]),
    "elapsed_seconds": process["elapsed_seconds"],
    "monotonic_event_span_seconds": events[-1]["monotonic"] - events[0]["monotonic"],
    "utc_receipt_envelope_seconds": (datetime.datetime.fromisoformat(process["finished_at"]) - datetime.datetime.fromisoformat(process["started_at"])).total_seconds(),
    "original_budgets_seconds": {"suite": 150, "fake_cli": 15},
    "owned_process_active": False,
    "new_test_or_model_execution": False,
    "formal_acceptance_granted": False,
}
out = HERE / "root-final132-independent-intake.json"
assert not out.exists(), "preserve prior intake; do not overwrite"
out.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8", newline="\n")
print(json.dumps(report))
