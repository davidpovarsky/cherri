"""Observe an existing immutable Remote Builder without inventing old receipts.

The current capture is a read-only before-state. It grants no historical stage,
39/58 gate, native execution, or final release acceptance.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess

from remote_release_gates import PROTECTED, TOOL_PATHS, require, unique_object, validate_tuple

SCHEMA = "cherri-current-installed-baseline/1"
LINTER_SHA256 = "51ffca98be2275f4015931e15d73251fff7f7041fb323c9b7aa6973eb57a454f"
CANONICAL = "https://github.com/davidpovarsky/cherri.git"
DOCS = "https://github.com/davidpovarsky/cherrilang.org.git"
PURE = "https://github.com/electrikmilk/cherri.git"
PURE_DOCS = "https://github.com/electrikmilk/cherrilang.org.git"
PURE_FILES = frozenset(("bin/cherri", "lib/service.py", "lib/pack_adapter.py"))


def file_state(path, maximum=128 * 1024 * 1024):
    path = Path(path)
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_size <= maximum,
            "Expected bounded regular current file: " + str(path))
    raw = path.read_bytes()
    after = path.lstat()
    identity = lambda s: (s.st_dev, s.st_ino, s.st_size, s.st_mtime_ns,
                          s.st_mode, s.st_uid, s.st_gid)
    require(identity(before) == identity(after), "Current file changed while reading: " + str(path))
    return raw, {"sha256": hashlib.sha256(raw).hexdigest(), "bytes": len(raw),
                 "mode": stat.S_IMODE(before.st_mode), "uid": before.st_uid, "gid": before.st_gid}


def checkout_state(path, revision, origin, expected_tree=None):
    path = Path(path)
    require(path.is_dir() and not path.is_symlink()
            and (path / ".git").is_dir() and not (path / ".git").is_symlink(),
            "Current source must be an ordinary Git checkout")
    require(not (path / ".git/objects/info/alternates").exists(),
            "Current source borrows another object store")
    def git(*args):
        result = subprocess.run(["git", "--no-replace-objects", "-C", str(path), *args],
                                capture_output=True, text=True, timeout=30,
                                env=dict(os.environ, GIT_OPTIONAL_LOCKS="0"))
        require(result.returncode == 0, "Cannot inspect current source: " + str(path))
        return result.stdout.strip()
    head = git("rev-parse", "HEAD")
    tree = git("rev-parse", "HEAD^{tree}")
    require(head == revision and git("remote", "get-url", "origin") == origin
            and not git("status", "--porcelain=v1", "--untracked-files=all"),
            "Current source identity/cleanliness differs")
    require(expected_tree is None or tree == expected_tree, "Current source tree differs")
    return {"head": head, "tree": tree, "origin": origin}


def pure_state(home):
    pure = home / "pure-upstream"
    require(pure.is_dir() and not pure.is_symlink(), "Separate Pure installation is missing")
    raw, manifest_file = file_state(pure / "installation.json", 65536)
    manifest = json.loads(raw, object_pairs_hook=unique_object)
    require(manifest.get("schema") == "cherri-pure-cli-installation/1"
            and manifest.get("freshBuild") is True
            and manifest.get("historicalBinaryRestored") is False
            and set(manifest.get("files", {})) == PURE_FILES,
            "Current Pure manifest differs")
    files = {"installation.json": manifest_file}
    for relative in sorted(PURE_FILES | {"bin/pure-cherri-service.sh", "bin/cherri-with-actions.sh"}):
        _, files[relative] = file_state(pure / relative)
        if relative in PURE_FILES:
            require(files[relative]["sha256"] == manifest["files"][relative],
                    "Current Pure file differs from manifest")
    sources = {}
    for role, directory, origin in (("engine", "src", PURE), ("docs", "docs", PURE_DOCS)):
        pin = manifest[role]
        require(pin.get("origin") == origin, "Current Pure source origin differs")
        sources[role] = checkout_state(pure / directory, pin["commit"], origin, pin["tree"])
    pack = pure / "apple-actions-pack"
    require(pack.is_dir() and not pack.is_symlink(), "Separate opt-in Pack is missing")
    pack_files = {}
    for path in sorted(pack.rglob("*")):
        require(not path.is_symlink(), "Current Pack contains a symlink")
        require(path.is_file() or path.is_dir(), "Current Pack contains an unsupported file type")
        if path.is_file():
            _, pack_files[path.relative_to(pack).as_posix()] = file_state(path)
    require("checksums.sha256" in pack_files and "manifest.json" in pack_files,
            "Current Pack checksums/manifest are missing")
    # Verify the original checksum list as well as retaining the complete current
    # static Pack inventory. No Pack module is loaded or executed.
    for line in (pack / "checksums.sha256").read_text().splitlines():
        if not line:
            continue
        fields = line.split(maxsplit=1)
        require(len(fields) == 2, "Invalid current Pack checksum line")
        digest, relative = fields[0], fields[1].lstrip("*")
        require(relative in pack_files and pack_files[relative]["sha256"] == digest,
                "Current Pack checksum differs")
    return {"manifestSHA256": manifest_file["sha256"], "files": files,
            "sources": sources, "packFiles": pack_files}


def observe_installed_home(live_home, public_alias):
    home = Path(live_home).resolve()
    require(home.is_dir(), "Current installed home is missing")
    raw, active_file = file_state(home / "active.json", 65536)
    release = json.loads(raw, object_pairs_hook=unique_object)
    validate_tuple(release)
    protected = {"active.json": active_file}
    for relative in sorted((set(PROTECTED) | TOOL_PATHS) - {"active.json"}):
        _, protected[relative] = file_state(home / relative)
    require(protected["bin/cherri-shortcut-lint.py"]["sha256"] == LINTER_SHA256,
            "Current validator requires a separately reviewed template")
    require((home / "config/default-language-mode").read_bytes().strip() == b"legacy",
            "Current default language mode is not Legacy")
    builder = home / "bin/cherri-remote-builder"
    require(builder.is_symlink() and os.readlink(builder) == "cherri-remote-legacy.sh",
            "Current registered alias differs")
    public_alias = Path(public_alias).absolute()
    require(public_alias.is_symlink() and public_alias.resolve() == home,
            "Current public home alias differs")
    runtime = home / release["label"] / "runtime"
    sources = {}
    for role, path, revision, origin in (
            ("skill", home / "source" / release["label"], release["skillSourceHead"], CANONICAL),
            ("compiler", runtime / "src", release["compilerCommit"], CANONICAL),
            ("docs", runtime / "docs", release["docsCommit"], DOCS)):
        sources[role] = checkout_state(path, revision, origin)
    runtime_files = {}
    for name, key in (("cherri", "compilerSHA256"), ("cherri-migrate", "migrationSHA256")):
        for suffix, content in (("", None), (".commit", release["compilerCommit"]),
                                (".sha256", release[key])):
            relative = name + suffix
            data, runtime_files[relative] = file_state(runtime / "bin" / relative)
            require((runtime_files[relative]["sha256"] == release[key]) if not suffix
                    else data == (content + "\n").encode(),
                    "Current runtime binary/sidecar differs")
    state = {"liveHome": str(home), "tuple": release, "protected": protected,
             "runtimeFiles": runtime_files, "sources": sources,
             "registeredAlias": os.readlink(builder),
             "publicAlias": {"path": str(public_alias), "target": os.readlink(public_alias)},
             "pure": pure_state(home)}
    require(file_state(home / "active.json", 65536)[1] == active_file,
            "Current tuple changed during observation")
    return state


def capture(live_home, public_alias):
    first = observe_installed_home(live_home, public_alias)
    require(observe_installed_home(live_home, public_alias) == first,
            "Current installation changed during capture")
    return {"schema": SCHEMA, "kind": "CURRENT_INSTALLED",
            "capturedAt": datetime.now(timezone.utc).isoformat(),
            "historyUnavailable": True, "historicalAcceptance": False,
            "livePromotion": False, "state": first}


def validate_capture(record, live_home):
    require(isinstance(record, dict) and set(record) == {
        "schema", "kind", "capturedAt", "historyUnavailable",
        "historicalAcceptance", "livePromotion", "state"},
        "Incomplete current baseline")
    require(record["schema"] == SCHEMA and record["kind"] == "CURRENT_INSTALLED"
            and record["historyUnavailable"] is True
            and record["historicalAcceptance"] is False
            and record["livePromotion"] is False, "Current baseline overclaims history")
    stamp = datetime.fromisoformat(record["capturedAt"])
    require(stamp.tzinfo is not None and stamp.utcoffset().total_seconds() == 0,
            "Current baseline timestamp must identify UTC")
    state = record["state"]
    require(isinstance(state, dict) and state.get("liveHome") == str(Path(live_home).resolve()),
            "Current baseline belongs to another home")
    actual = observe_installed_home(live_home, state["publicAlias"]["path"])
    require(actual == state, "Current baseline is stale or incomplete")
    return actual


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--live-home", type=Path, required=True)
    parser.add_argument("--public-alias", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    require(not args.output.is_symlink(), "Capture output cannot be a symlink")
    output = args.output.resolve()
    home = args.live_home.resolve()
    require(output != home and not output.is_relative_to(home), "Capture output must be outside live")
    require(output.parent.is_dir() and not output.exists(), "Capture output must be new")
    record = capture(home, args.public_alias)
    raw = (json.dumps(record, indent=2, sort_keys=True) + "\n").encode()
    os.umask(0o077)
    with output.open("xb") as stream:
        stream.write(raw)
    print(json.dumps({"path": str(output), "sha256": hashlib.sha256(raw).hexdigest(),
                      "kind": "CURRENT_INSTALLED", "historicalAcceptance": False}))


if __name__ == "__main__":
    main()
