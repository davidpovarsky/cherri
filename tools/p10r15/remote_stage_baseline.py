"""Read the exact predecessor and original wrapper templates for a new stage.

A receipt pins a previous stage; matching it to live bytes does not grant new
release acceptance. The normal entrypoint, signing and promotion gates remain.
"""
import hashlib
import json
from pathlib import Path
import stat

from remote_release_gates import (
    PROTECTED, TOOL_PATHS, is_hash, require, unique_object, validate_tuple,
)

ORIGINAL_LIVE_HASHES = {
    "active.json": "ee9aac247c568fb743d4e505bcaaebe224bfca843d1091438701bd18273807d0",
    "bin/cherri-remote-runner.sh": "7ae689e9d494e7ddb9a7b9abfb5c5d96bb4321b069dc2847aec07271122cb954",
    "bin/cherri-remote-legacy.sh": "734507b7a981d7d4185fb3582bd9601d958db013dcd7b12b13201d0d3b0e0b4c",
    "bin/cherri-shortcut-lint.py": "51ffca98be2275f4015931e15d73251fff7f7041fb323c9b7aa6973eb57a454f",
}


def original_bytes(path, expected, limit=65536):
    path = Path(path)
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_size <= limit,
            "Expected a bounded regular original: " + str(path))
    raw = path.read_bytes()
    after = path.lstat()
    require((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns) ==
            (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns),
            "Original changed while reading: " + str(path))
    require(is_hash(expected) and hashlib.sha256(raw).hexdigest() == expected,
            "Original digest differs: " + str(path))
    return raw


def load_previous_stage(path, expected_sha, live_home):
    """Bind all ten live files and the active tuple to one retained receipt."""
    raw = original_bytes(path, expected_sha)
    previous = json.loads(raw, object_pairs_hook=unique_object)
    require(isinstance(previous, dict), "Invalid predecessor stage")
    require(previous.get("status") == "STAGED_NOT_ACTIVATED" and
            previous.get("livePromotion") is False, "Invalid predecessor stage state")
    release = previous.get("tuple")
    validate_tuple(release)
    expected_files = set(PROTECTED) | TOOL_PATHS
    hashes = previous.get("candidateHashes")
    require(isinstance(hashes, dict) and set(hashes) == expected_files and
            all(is_hash(value) for value in hashes.values()),
            "Incomplete predecessor protected inventory")
    tool_hashes = previous.get("externalToolchainDependencies")
    require(isinstance(tool_hashes, dict) and set(tool_hashes) == TOOL_PATHS and
            all(tool_hashes[name] == hashes[name] for name in TOOL_PATHS),
            "Predecessor toolchain binding differs")
    require(hashes["bin/cherri-shortcut-lint.py"] ==
            ORIGINAL_LIVE_HASHES["bin/cherri-shortcut-lint.py"],
            "Predecessor validator requires a separately reviewed template")
    live_home = Path(live_home)
    active = None
    for name, expected in hashes.items():
        raw_file = original_bytes(live_home / name, expected, 128 * 1024 * 1024)
        if name == "active.json":
            active = json.loads(raw_file, object_pairs_hook=unique_object)
    require(active == release, "Predecessor active tuple differs")
    return dict(hashes), {
        "path": str(Path(path).resolve()),
        "sha256": expected_sha,
        "tuple": release,
        "liveBytesMatch": True,
        "acceptanceGranted": False,
    }


def load_current_installed(path, expected_sha, live_home):
    """Use a real current before-state without granting historical acceptance."""
    from remote_current_baseline import validate_capture
    raw = original_bytes(path, expected_sha, 2 * 1024 * 1024)
    record = json.loads(raw, object_pairs_hook=unique_object)
    state = validate_capture(record, live_home)
    protected = {name: value["sha256"] for name, value in state["protected"].items()}
    require(set(protected) == set(PROTECTED) | TOOL_PATHS,
            "Incomplete current protected inventory")
    require(protected["bin/cherri-shortcut-lint.py"] ==
            ORIGINAL_LIVE_HASHES["bin/cherri-shortcut-lint.py"],
            "Current validator requires a separately reviewed template")
    return protected, {"path": str(Path(path).resolve()), "sha256": expected_sha,
                       "kind": "CURRENT_INSTALLED", "tuple": state["tuple"],
                       "historyUnavailable": True, "liveBytesMatch": True,
                       "acceptanceGranted": False}


def wrapper_templates(live_home, previous_receipt, previous_sha, runner, legacy, *,
                      installed=False, current_baseline=None, current_sha=None):
    """Keep the reviewed original transformations independent of the live version."""
    current_selected = current_baseline is not None or current_sha is not None
    if current_selected:
        require(installed and current_baseline is not None and current_sha is not None
                and all(value is None for value in (previous_receipt, previous_sha, runner, legacy)),
                "Current baseline requires installed mode, its receipt/SHA, and no historical options")
    if installed:
        if current_selected:
            protected, previous = load_current_installed(current_baseline, current_sha, live_home)
        else:
            require(previous_receipt is not None and previous_sha is not None and
                    runner is None and legacy is None,
                    "Installed immutable templates require a predecessor receipt/SHA and no historical wrapper options")
            protected, previous = load_previous_stage(previous_receipt, previous_sha, live_home)
        values, witnesses = {}, {}
        for role in ("runner", "legacy"):
            relative = "bin/cherri-remote-" + role + ".sh"
            path = Path(live_home) / relative
            expected = protected[relative]
            values[role] = original_bytes(path, expected).decode("utf-8")
            witnesses[role] = {
                "path": str(path.resolve()), "sha256": expected,
                "kind": "CURRENT_INSTALLED_IMMUTABLE",
            }
        return protected, previous, values, witnesses
    options = (previous_receipt, previous_sha, runner, legacy)
    require(not any(value is not None for value in options) or
            all(value is not None for value in options),
            "A predecessor requires its receipt, SHA256 and both original wrappers")
    previous = None
    if previous_receipt is None:
        protected = dict(ORIGINAL_LIVE_HASHES)
        runner = Path(live_home) / "bin/cherri-remote-runner.sh"
        legacy = Path(live_home) / "bin/cherri-remote-legacy.sh"
    else:
        protected, previous = load_previous_stage(previous_receipt, previous_sha, live_home)
    values = {}
    witnesses = {}
    for role, path, relative in (
            ("runner", runner, "bin/cherri-remote-runner.sh"),
            ("legacy", legacy, "bin/cherri-remote-legacy.sh")):
        expected = ORIGINAL_LIVE_HASHES[relative]
        values[role] = original_bytes(path, expected).decode("utf-8")
        witnesses[role] = {"path": str(Path(path).resolve()), "sha256": expected}
    return protected, previous, values, witnesses
