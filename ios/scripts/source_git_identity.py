#!/usr/bin/env python3
"""Bind an already-installed Git executable; do not run or install tools."""
import hashlib
import json
import os
from pathlib import Path
import platform
import stat
import sys
import time


HOMEBREW_GIT = Path("/opt/homebrew/bin/git")
EXECUTABLE_LIMIT = 32 * 1024 * 1024


def require(condition, reason):
    if not condition:
        raise ValueError(reason)


def within(deadline):
    require(time.monotonic() < deadline, "SOURCE_GIT_DEADLINE")


def identity(info):
    return tuple(getattr(info, name) for name in
                 ("st_dev", "st_ino", "st_mode", "st_uid", "st_gid", "st_size",
                  "st_mtime_ns", "st_ctime_ns"))


def ancestors(path):
    result = []
    for parent in path.parents:
        info = parent.lstat()
        require(stat.S_ISDIR(info.st_mode), "SOURCE_GIT_ANCESTOR")
        result.append((str(parent), info.st_dev, info.st_ino, info.st_mode,
                       info.st_uid, info.st_gid))
    return tuple(result)


class VerifiedGit:
    """Use the resolved file while checking its declared alias and original bytes."""

    def __init__(self, logical, deadline):
        self.logical = Path(logical)
        require(self.logical.is_absolute() and
                str(self.logical) == os.path.normpath(str(self.logical)), "SOURCE_GIT_PATH")
        self.metadata = self._metadata(deadline)
        self.path = Path(self.metadata["physicalPath"])
        self.bytes, self.sha256 = self._digest(deadline)
        self.check(deadline)

    def _metadata(self, deadline):
        within(deadline)
        logical_info = self.logical.lstat()
        require(stat.S_ISREG(logical_info.st_mode) or stat.S_ISLNK(logical_info.st_mode),
                "SOURCE_GIT_PATH")
        target = os.readlink(self.logical) if stat.S_ISLNK(logical_info.st_mode) else None
        physical = self.logical.resolve(strict=True)
        physical_info = physical.lstat()
        require(stat.S_ISREG(physical_info.st_mode) and physical.resolve(strict=True) == physical
                and 0 < physical_info.st_size <= EXECUTABLE_LIMIT
                and physical_info.st_mode & 0o111 and os.access(physical, os.X_OK),
                "SOURCE_GIT_EXECUTABLE")
        result = {"logicalPath": str(self.logical), "logicalStat": identity(logical_info),
                  "linkTarget": target, "physicalPath": str(physical),
                  "physicalStat": identity(physical_info),
                  "logicalAncestors": ancestors(self.logical),
                  "physicalAncestors": ancestors(physical)}
        within(deadline)
        return result

    def _digest(self, deadline):
        digest, total = hashlib.sha256(), 0
        descriptor = os.open(self.path, os.O_RDONLY | os.O_NOFOLLOW)
        with os.fdopen(descriptor, "rb") as stream:
            require(identity(os.fstat(stream.fileno())) == self.metadata["physicalStat"],
                    "SOURCE_GIT_CHANGED")
            while True:
                within(deadline)
                block = stream.read(1024 * 1024)
                if not block:
                    break
                total += len(block)
                require(total <= EXECUTABLE_LIMIT, "SOURCE_GIT_LIMIT")
                digest.update(block)
            require(identity(os.fstat(stream.fileno())) == self.metadata["physicalStat"]
                    and total == self.metadata["physicalStat"][5], "SOURCE_GIT_CHANGED")
        return total, digest.hexdigest()

    def check(self, deadline, *, contents=False):
        require(self._metadata(deadline) == self.metadata, "SOURCE_GIT_CHANGED")
        if contents:
            require(self._digest(deadline) == (self.bytes, self.sha256), "SOURCE_GIT_CHANGED")
            require(self._metadata(deadline) == self.metadata, "SOURCE_GIT_CHANGED")
        return self.path

    def emit(self, phase):
        require(phase in ("admitted", "closed"), "SOURCE_GIT_PHASE")
        value = {"schema": "cherri-source-git-identity/1", "phase": phase,
                 "logicalPath": str(self.logical), "physicalPath": str(self.path),
                 "bytes": self.bytes, "sha256": self.sha256,
                 "logicalStat": self.metadata["logicalStat"],
                 "physicalStat": self.metadata["physicalStat"],
                 "acceptanceGranted": False}
        print("SOURCE_GIT_IDENTITY:" + json.dumps(value, sort_keys=True, separators=(",", ":")),
              file=sys.stderr, flush=True)


def select_homebrew(deadline):
    require(sys.platform == "darwin" and platform.machine() == "arm64", "SOURCE_GIT_PLATFORM")
    selected = VerifiedGit(HOMEBREW_GIT, deadline)
    selected.emit("admitted")
    return selected
