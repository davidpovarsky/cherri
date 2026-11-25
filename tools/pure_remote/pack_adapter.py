#!/usr/bin/env python3
"""Resolve the preserved Pack 0.1.1 caller's root without changing Pack bytes."""
from __future__ import annotations
import argparse
import hashlib
import os
from pathlib import Path
import shlex
import subprocess
import sys
from service import ContractError, digest, regular_bytes, relative_file, require, verify_installation

CHECKSUMS_SHA256 = "b21d78b166bb68f33481707e220e649b04dab2e18b7ed2d1f6114b5a80a28331"
WRAPPER_SHA256 = "bf0b491a012b46140af31203aaaede4dc4bf0ff3fd4ad88318a512030f4a5e8e"
OLD_ROOT = 'ROOT="$HOME/workspaces/cherri-remote/pure-upstream"'


def verify_pack(root):
    pack = root / "apple-actions-pack"
    manifest = regular_bytes(pack / "checksums.sha256", 16384)
    require(digest(manifest) == CHECKSUMS_SHA256, "Original Pack checksum manifest changed")
    checked = []
    for line in manifest.decode().splitlines():
        expected, relative = line.split("  ", 1)
        require(digest(regular_bytes(relative_file(pack, relative))) == expected, "Original Pack file changed: " + relative)
        checked.append(relative)
    require(len(checked) == 29 and len(set(checked)) == 29, "Unexpected original checksum inventory")
    require(len(list((pack / "modules").glob("*.cherri"))) == 20, "Unexpected module count")
    raw = regular_bytes(pack / "bin/cherri-with-actions.sh")
    require(digest(raw) == WRAPPER_SHA256, "Original Pack wrapper changed")
    return raw.decode("utf-8")


def adapted_wrapper(original, root):
    require(digest(original.encode()) == WRAPPER_SHA256, "Unrecognized original wrapper")
    require(original.splitlines().count(OLD_ROOT) == 1, "Original root assignment is ambiguous")
    resolved = str(Path(root).resolve(strict=True))
    require("\'" not in resolved and "\n" not in resolved and "\r" not in resolved,
            "Original Pack exit trap cannot represent this root path")
    replacement = "ROOT=" + shlex.quote(resolved)
    changed = original.replace(OLD_ROOT + "\n", replacement + "\n", 1)
    require(changed.replace(replacement + "\n", OLD_ROOT + "\n", 1) == original,
            "Adapter changed more than the original root assignment")
    return changed


def main(argv=None):
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--manifest-sha256", required=True)
    parser.add_argument("pack_args", nargs=argparse.REMAINDER)
    args = parser.parse_args(argv)
    try:
        root = args.root.resolve(strict=True)
        verify_installation(root, args.manifest_sha256)
        original = verify_pack(root)
        invocation = args.pack_args
        if invocation[:1] == ["--"]:
            invocation = invocation[1:]
        require(invocation and invocation[0] in ("status", "verify", "docs", "original", "pure", "enhanced", "pack", "help", "-h", "--help"),
                "Expected the original Pack command interface")
        command = ["/bin/bash", "-c", adapted_wrapper(original, root), "cherri-with-actions.sh", *invocation]
        os.execv(command[0], command)
        raise AssertionError("execv returned")
    except (OSError, ValueError, ContractError, subprocess.TimeoutExpired) as error:
        print("PURE_PACK_ADAPTER_ERROR: " + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
