#!/usr/bin/env python3
"""Bind a freshly built Pure binary, original checkouts, and external CLI files."""
from __future__ import annotations
import argparse
import json
import os
from pathlib import Path
import re
import sys
from service import (ContractError, DOCS, SCHEMA, UPSTREAM, VERSION, digest, git,
                     regular_bytes, require, verify_checkout, verify_installation)


def launcher(module, manifest_sha, module_sha, pack=False, service_sha=None):
    require(module in ("service.py", "pack_adapter.py"), "Unexpected launcher module")
    require(pack == (module == "pack_adapter.py"), "Launcher mode does not match its module")
    hashes = [(module, module_sha)]
    if pack:
        hashes.insert(0, ("service.py", service_sha))
    for value in (manifest_sha, *(value for _, value in hashes)):
        require(isinstance(value, str) and re.fullmatch("[0-9a-f]{64}", value) is not None,
                "Invalid launcher digest")
    tail = '-- "$@"' if pack else '"$@"'
    checks = ""
    for name, expected in hashes:
        checks += ('ACTUAL=$(/usr/bin/sha256sum "$ROOT/lib/' + name +
                   '" | /usr/bin/cut -d " " -f 1)\n'
                   '[ "$ACTUAL" = "' + expected +
                   '" ] || { echo "Pure CLI code digest mismatch" >&2; exit 1; }\n')
    return ('#!/bin/sh\nset -eu\numask 077\n'
            'ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)\n' +
            checks +
            'export PYTHONDONTWRITEBYTECODE=1\n'
            'exec /usr/bin/python3 -B "$ROOT/lib/' + module + '" --root "$ROOT" --manifest-sha256 "' +
            manifest_sha + '" ' + tail + '\n')


def assemble(root, build_receipt, build_sha, controller):
    root = Path(root).resolve(strict=True)
    controller = Path(controller).resolve(strict=True)
    require(not (root / "installation.json").exists(), "Installation is already assembled")
    verify_checkout(root / "src", UPSTREAM)
    verify_checkout(root / "docs", DOCS)
    binary_sha = digest(regular_bytes(root / "bin/cherri"))
    raw_build = regular_bytes(build_receipt, 1024 * 1024)
    require(digest(raw_build) == build_sha, "Build receipt digest differs")
    build = json.loads(raw_build)
    require(build.get("status") == "PASS_PINNED_PURE_BUILD", "Fresh Pure build did not pass")
    require(build.get("source") == UPSTREAM[0] and build.get("tree") == UPSTREAM[1],
            "Build receipt names another source")
    require(build.get("binarySHA256") == binary_sha and build.get("sourceUnchanged") is True,
            "Build receipt does not bind this binary and unchanged source")
    require(build.get("vcsModified") is False, "Build receipt has a modified VCS identity")
    require(build.get("historicalBinaryRestored") is False, "This assembler requires a fresh build")
    require(git(controller, "status", "--porcelain=v1", "--untracked-files=all").strip() == b"",
            "External wrapper source is not clean")
    controller_sha = git(controller, "rev-parse", "HEAD").decode().strip()
    controller_tree = git(controller, "rev-parse", "HEAD^{tree}").decode().strip()
    lib = root / "lib"
    lib.mkdir()
    file_hashes = {"bin/cherri": binary_sha}
    for name in ("service.py", "pack_adapter.py"):
        raw = regular_bytes(controller / "tools/pure_remote" / name)
        path = lib / name
        with path.open("xb") as stream:
            stream.write(raw)
        path.chmod(0o600)
        file_hashes["lib/" + name] = digest(raw)
    for name in ("jobs", "outputs"):
        (root / name).mkdir(mode=0o700)
    manifest = {"schema": SCHEMA, "wrapperVersion": VERSION,
                "engine": dict(zip(("commit", "tree", "origin"), UPSTREAM)),
                "docs": dict(zip(("commit", "tree", "origin"), DOCS)),
                "files": file_hashes, "buildReceiptSHA256": build_sha,
                "freshBuild": True, "historicalBinaryRestored": False,
                "controllerSource": controller_sha, "controllerTree": controller_tree}
    raw = (json.dumps(manifest, indent=2, sort_keys=True) + "\n").encode()
    with (root / "installation.json").open("xb") as stream:
        stream.write(raw)
    pin = digest(raw)
    for name, module, pack in (("pure-cherri-service.sh", "service.py", False),
                               ("cherri-with-actions.sh", "pack_adapter.py", True)):
        path = root / "bin" / name
        with path.open("x") as stream:
            stream.write(launcher(module, pin, file_hashes["lib/" + module], pack,
                                  service_sha=file_hashes["lib/service.py"]))
        path.chmod(0o700)
    result = verify_installation(root, pin)
    result.update(status="ASSEMBLED_FRESH_PURE_CLI", manifestSHA256=pin,
                  controllerSource=controller_sha, installedRoot=str(root))
    return result


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", required=True, type=Path)
    parser.add_argument("--build-receipt", required=True, type=Path)
    parser.add_argument("--build-sha256", required=True)
    parser.add_argument("--controller", required=True, type=Path)
    args = parser.parse_args()
    try:
        result = assemble(args.root, args.build_receipt, args.build_sha256, args.controller)
        print(json.dumps(result, sort_keys=True))
        return 0
    except (OSError, ValueError, ContractError) as error:
        print("PURE_ASSEMBLY_ERROR: " + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
