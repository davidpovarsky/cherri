"""Hosted path/identity binding for the unchanged original 56-gate protocol.

The original process owner and root observer are copied byte-for-byte. Source
snapshot, dependency, read and monitor methods below retain their qualified
implementation. Only account, filesystem and published-controller bindings
change for an ephemeral GitHub-hosted Linux ARM64 worker.
"""
from pathlib import Path
import ast
import hashlib
import importlib.util
import json
import os
import platform
import re
import shutil
import stat
import sys
import time

HELPER_SHA256 = "4672474ddb270a5857914041ebb279f20431f87e9da173e6cd8bf83b3fb11c85"
OBSERVER_SHA256 = "97e699223bc7438e6d4f9de643d9ebc27497e28e9c3a31715f93652c35248ba8"
ORIGINAL_SHARED_SHA256 = "8452dcf07565838816122a027c9800998b3b67ec6b917b5ac66fe8fd38625e81"
ORIGINAL_RELEASE_OWNER_SHA256 = "e8419a3224b5c4f64c7fc690a5f386777e60a7655c98012ae6a9a11e473d4708"
SOURCE = "a6bf7b5d928670d7c011a2aa2849ba3e43b3844d"
TREE = "22cc0c394f1cada11ac11e7604a358439fcf98f4"
SOURCE_BRANCH = "agent/cherri-p10r15-integrated-c2-consumers-20261126"
CONTROLLER_REF = "refs/heads/probe/cherri-p10r15-canonical-release-a6bf-20261126"
REPOSITORY = "davidpovarsky/cherri"
GO_SHA256 = "aedb1694ac1516bbd353a7c737f31be252f59f17746133d46f9e77f2f5fe9d22"
TOOLCHAIN_PREFIX = "golang.org/toolchain@v0.0.1-go1.24.13.linux-arm64"
TOOLCHAIN_ZIP_SHA256 = "50c7b3e59fa87479d8edf3e9ad81daa477c050fb580f2661cb3b8adaf70af86c"
TOOLCHAIN_ZIP_BYTES = 79300772
TOOLCHAIN_EXPANDED_BYTES = 232670599
TOOLCHAIN_ENTRIES = 10751
CONTROLLER_FILES = (
    ".github/workflows/p10r15-canonical-release.yml",
    "tools/ci/canonical_release/hosted_source.py",
    "tools/ci/canonical_release/prepare.py",
    "tools/ci/canonical_release/run.py",
    "tools/ci/canonical_release/adopt.py",
    "tools/ci/canonical_release/owned_source_process.py",
    "tools/ci/canonical_release/observe_go_exclusive.py",
)
ADMISSION_FLOORS = {"root": 939524096, "development": 1073741824,
                    "shm": 402653184, "memoryAvailable": 3221225472}
LIVE_FLOORS = {"root": 805306368, "development": 805306368,
               "shm": 201326592, "memoryAvailable": 2147483648}

_helper = Path(__file__).with_name("owned_source_process.py")
if hashlib.sha256(_helper.read_bytes()).hexdigest() != HELPER_SHA256:
    raise SystemExit("OWNER_HELPER_BYTES_CHANGED")
_spec = importlib.util.spec_from_file_location("cherri_hosted_owned_process", _helper)
owned = importlib.util.module_from_spec(_spec)
sys.modules[_spec.name] = owned
_spec.loader.exec_module(owned)


def require(condition, code):
    if not condition:
        raise owned.OwnerAbort(code)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def hosted_context(environment=None):
    env = os.environ if environment is None else environment
    require(env.get("GITHUB_ACTIONS") == "true" and env.get("GITHUB_REPOSITORY") == REPOSITORY,
            "EXACT_GITHUB_REPOSITORY")
    require(env.get("GITHUB_EVENT_NAME") == "create" and
            env.get("GITHUB_REF") == CONTROLLER_REF, "FIXED_CREATE_CONTROLLER_REF")
    event_path = Path(env["GITHUB_EVENT_PATH"])
    event_info = event_path.lstat()
    require(stat.S_ISREG(event_info.st_mode) and event_info.st_size <= 1024*1024, "BOUNDED_CREATE_EVENT")
    event = json.loads(event_path.read_bytes())
    require(event.get("ref_type") == "branch" and event.get("ref") == CONTROLLER_REF.removeprefix("refs/heads/")
            and event.get("repository", {}).get("full_name") == REPOSITORY, "EXACT_CREATED_BRANCH_EVENT")
    require(re.fullmatch(r"[a-f0-9]{40}", env.get("GITHUB_SHA", "")) is not None and
            re.fullmatch(r"[1-9][0-9]{0,19}", env.get("GITHUB_RUN_ID", "")) is not None and
            env.get("GITHUB_RUN_ATTEMPT") == "1", "RUN_IDENTITY")
    require(env.get("GITHUB_WORKFLOW_SHA") == env["GITHUB_SHA"] and
            env.get("GITHUB_WORKFLOW_REF") == REPOSITORY + "/.github/workflows/p10r15-canonical-release.yml@" + CONTROLLER_REF,
            "EXACT_EVENT_WORKFLOW_SOURCE")
    require(platform.system() == "Linux" and platform.machine() in ("aarch64", "arm64") and
            env.get("RUNNER_ARCH") == "ARM64" and os.getuid() != 0, "LINUX_ARM64_UNPRIVILEGED_OWNER")
    workspace, temporary = (Path(env[k]) for k in ("GITHUB_WORKSPACE", "RUNNER_TEMP"))
    for path in (workspace, temporary):
        require(path.is_absolute() and path.resolve(strict=True) == path and path.is_dir() and
                not path.is_symlink(), "REAL_RUNNER_DIRECTORY")
    controller, producer = workspace/"controller", workspace/"producer"
    require(controller.resolve(strict=True) == controller and producer.resolve(strict=True) == producer and
            Path(__file__).resolve().parents[3] == controller, "SEPARATE_EXACT_CHECKOUTS")
    base = temporary/("cherri-canonical-release-" + env["GITHUB_RUN_ID"] + "-" + env["GITHUB_RUN_ATTEMPT"])
    return {"workspace": workspace, "temporary": temporary, "controller": controller, "producer": producer,
            "base": base, "evidence": base/"evidence", "controllerSha": env["GITHUB_SHA"],
            "runId": env["GITHUB_RUN_ID"], "runAttempt": 1, "workflowRef": env.get("GITHUB_WORKFLOW_REF", ""),
            "workflowSha": env.get("GITHUB_WORKFLOW_SHA", ""), "runnerImage": env.get("ImageOS", ""),
            "runnerImageVersion": env.get("ImageVersion", ""), "runnerUid": os.getuid()}


CONTEXT = hosted_context()
WORKTREE = CONTEXT["producer"]
BASE = CONTEXT["base"]
EVIDENCE = CONTEXT["evidence"]
GO = BASE/"toolchain"/TOOLCHAIN_PREFIX/"bin/go"
OBSERVER = Path(__file__).with_name("observe_go_exclusive.py")


def free():
    memory = {k: int(v.strip().split()[0]) * 1024 for k, v in
              (line.split(":", 1) for line in Path("/proc/meminfo").read_text().splitlines())}
    return {"root": shutil.disk_usage("/").free,
            "development": shutil.disk_usage(WORKTREE).free,
            "shm": shutil.disk_usage("/dev/shm").free,
            "memoryAvailable": memory["MemAvailable"]}


def runtime_identity():
    result = {k: str(v) if isinstance(v, Path) else v for k, v in CONTEXT.items()}
    result.update(schema="cherri-canonical-release-hosted-binding/1", producerSource=SOURCE,
                  producerTree=TREE, producerBranch=SOURCE_BRANCH,
                  pythonVersion=sys.version, actualPythonOriginalArgv=sys.orig_argv,
                  actualProcessId=os.getpid(), originalOracleOwnerSHA256=ORIGINAL_RELEASE_OWNER_SHA256,
                  originalOracleSharedSHA256=ORIGINAL_SHARED_SHA256,
                  capacityPathBinding={"root": "/", "development": str(WORKTREE), "shm": "/dev/shm"},
                  capacityDeviceBinding={"root": Path("/").stat().st_dev,
                                         "development": WORKTREE.stat().st_dev,
                                         "shm": Path("/dev/shm").stat().st_dev},
                  numericFloorsUnchanged=True, nativeAcceptance=False, global179Acceptance=False)
    return result


def controller_snapshot(owner):
    root = CONTEXT["controller"]
    def git(*args):
        _, stdout, _ = owner.command("controller-git", ["git", "--no-replace-objects", "-C", str(root), *args])
        return owner.read(stdout)
    head = git("rev-parse", "HEAD").decode().strip()
    require(head == CONTEXT["controllerSha"] and head != SOURCE, "ACTUAL_CONTROLLER_HEAD")
    require(not git("status", "--porcelain=v1", "--untracked-files=all"), "CLEAN_CONTROLLER")
    require(not git("replace", "-l"), "CONTROLLER_NO_REPLACE_REFS")
    git("merge-base", "--is-ancestor", SOURCE, head)
    changes = git("diff", "--name-status", SOURCE, head).decode().splitlines()
    require(sorted(changes) == sorted("A\t" + path for path in CONTROLLER_FILES), "CLOSED_ADDITIVE_CONTROLLER_FILES")
    rows = {}
    listing = git("ls-tree", "-rz", "--full-tree", "HEAD", "--", *CONTROLLER_FILES)
    for item in listing.split(b"\0"):
        if not item:
            continue
        left, name = item.split(b"\t", 1)
        mode, kind, blob = left.decode().split()
        name = name.decode()
        require(name in CONTROLLER_FILES and name not in rows and kind == "blob" and mode == "100644",
                "CONTROLLER_REGULAR_MODE")
        path = root/name
        info = path.lstat()
        require(stat.S_ISREG(info.st_mode) and not (info.st_mode & 0o111), "CONTROLLER_PHYSICAL_MODE")
        raw = owner.read(path)
        require(hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest() == blob,
                "CONTROLLER_PHYSICAL_BLOB")
        rows[name] = {"blob": blob, "sha256": sha(raw), "bytes": len(raw), "mode": mode}
    require(set(rows) == set(CONTROLLER_FILES), "CLOSED_CONTROLLER_INVENTORY")
    return {"head": head, "tree": git("rev-parse", "HEAD^{tree}").decode().strip(), "files": rows}


class SourceOwner:
    def __init__(self, config, output, budget):
        self.config, self.out, self.budget = config, output, budget
        self.worktree = Path(config["worktree"])
        role = config.get("sourceRole", "canonical-precursor")
        require(role in ("canonical-precursor", "compiler-pair"), "CLOSED_SOURCE_OWNER_ROLE")
        expected = WORKTREE if role == "canonical-precursor" else BASE/"pair/src"
        require(self.worktree == expected, "FIXED_PRODUCER_CHECKOUT")
        self.commands = output/"commands"
        self.commands.mkdir(mode=0o700)
        self.sequence = 0
        self.minimum = free()
        keep = ("PATH", "LANG", "LC_ALL", "LC_CTYPE", "TZ")
        self.env = {key: os.environ[key] for key in keep if key in os.environ}
        self.env.update(PATH=str(GO.parent) + ":/usr/local/bin:/usr/bin:/bin", GOTOOLCHAIN="go1.24.13",
            GOMAXPROCS="2", GOFLAGS="-p=2 -mod=readonly", GOPROXY="off", GOSUMDB="off",
            PYTHONDONTWRITEBYTECODE="1", GOMODCACHE=str(BASE/"module-cache"),
            GOCACHE=str(BASE/"build-cache"), GOTMPDIR=str(BASE/"tmp"), TMPDIR=str(BASE/"tmp"),
            GIT_NO_REPLACE_OBJECTS="1", GIT_OPTIONAL_LOCKS="0", GIT_TERMINAL_PROMPT="0")

    def command(self, label, argv, seconds=25, finalizing=False, require_zero=True):
        self.sequence += 1
        name = f"{self.sequence:03d}-{label}"
        actual = list(map(str, argv))
        row = owned.run_owned(actual, cwd=self.worktree, env=self.env, output=self.commands,
            name=name, budget=self.budget, seconds=seconds, monitor=self.monitor,
            post_reserve=2 if finalizing else None)
        if require_zero:
            owned.require_success(row)
        return row, self.commands/(name + ".stdout.original"), self.commands/(name + ".stderr.original")

    def observe_concurrent_owner(self):
        # The only privileged route is this exact unchanged read-only observer.
        observer_raw = self.read(OBSERVER)
        require(sha(observer_raw) == OBSERVER_SHA256, "NEW_ROOT_OBSERVER_BYTES")
        owned.original(self.out/"admission-observer.original.py", observer_raw)
        self.sequence += 1
        name = f"{self.sequence:03d}-concurrent-owner"
        row = owned.run_owned(["sudo", "-n", "/usr/bin/python3", "-I", "-B", str(OBSERVER)],
            cwd=self.worktree, env=self.env, output=self.commands, name=name,
            budget=self.budget, seconds=25, monitor=self.monitor)
        owned.require_success(row)
        decision = json.loads(self.read(self.commands/(name + ".stdout.original")))
        require(decision.get("schema") == "cherri-exclusive-go-observation/1" and
                decision.get("policy") == "new-conservative-all-live-go-20261126" and
                decision.get("observerUid") == 0 and decision.get("errors") == [],
                "NEW_ROOT_OBSERVER_PROTOCOL")
        return row, self.commands/(name + ".stdout.original"), self.commands/(name + ".stderr.original")

    def monitor(self):
        observed = free()
        for key in self.minimum:
            self.minimum[key] = min(self.minimum[key], observed[key])
        require(all(observed[k] >= v for k, v in LIVE_FLOORS.items()), "LIVE_CAPACITY_FLOOR")


    def read(self, path):
        self.budget.check()
        parts = []
        with Path(path).open("rb") as stream:
            while True:
                self.budget.check()
                chunk = stream.read(1024 * 1024)
                if not chunk:
                    break
                parts.append(chunk)
        self.budget.check()
        return b"".join(parts)


    def git(self, *args, finalizing=False):
        row, path, _ = self.command("git", ["git", "--no-replace-objects", "-C", str(self.worktree), *args],
                                    finalizing=finalizing)
        return self.read(path)


    def snapshot(self, finalizing=False):
        self.budget.check()
        w = self.worktree
        require(w.resolve(strict=True) == w and (w/".git").is_dir() and not (w/".git").is_symlink(), "ORDINARY_SOURCE_REQUIRED")
        require(not (w/".git/objects/info/alternates").exists() and not (w/".git/info/grafts").exists(), "NO_ALTERNATES_OR_GRAFTS")
        require(not self.git("replace", "-l", finalizing=finalizing), "NO_REPLACE_REFS")
        metadata = {}
        for key, args in (("head", ("rev-parse", "HEAD")), ("tree", ("rev-parse", "HEAD^{tree}")),
                          ("branch", ("branch", "--show-current")), ("status", ("status", "--porcelain=v1", "--untracked-files=all"))):
            raw = self.git(*args, finalizing=finalizing).decode()
            metadata[key] = raw if key == "status" else raw.strip()
        tracked = {}
        listing = self.git("ls-tree", "-rz", "--full-tree", "HEAD", finalizing=finalizing)
        for index, line in enumerate(listing.split(b"\0")):
            if not line:
                continue
            self.budget.check()
            if index % 128 == 0:
                self.monitor()
            left, name = line.split(b"\t", 1)
            mode, kind, blob = left.decode().split()
            name = name.decode()
            require(kind == "blob" and mode in ("100644", "100755", "120000"), "TRACKED_FILE_KIND")
            path = w/name
            info = path.lstat()
            if mode == "120000":
                require(stat.S_ISLNK(info.st_mode), "TRACKED_LINK_KIND")
                raw = os.fsencode(os.readlink(path))
            else:
                require(stat.S_ISREG(info.st_mode) and bool(info.st_mode & 0o111) == (mode == "100755"), "TRACKED_FILE_MODE")
                raw = self.read(path)
            require(hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest() == blob, "PHYSICAL_GIT_BLOB_MISMATCH")
            tracked[name] = {"mode": mode, "blob": blob, "bytes": len(raw), "sha256": sha(raw)}
        metadata["trackedPhysicalInputs"] = tracked
        return metadata


    def dependencies(self):
        target = self.worktree / "tools/p10r3/generate_catalog_and_pack_ledgers.py"
        tree = ast.parse(self.read(target))
        nodes = [n for n in tree.body if isinstance(n, ast.Assign) and
                 any(isinstance(t, ast.Name) and t.id == "DEPENDENCY_GLOBS" for t in n.targets)]
        require(len(nodes) == 1, "DEPENDENCY_GLOBS_IDENTITY")
        patterns = ast.literal_eval(nodes[0].value)
        names = sorted({p.relative_to(self.worktree).as_posix() for pattern in patterns
                        for p in self.worktree.glob(pattern) if p.is_file()})
        rows, material = [], hashlib.sha256()
        for name in names:
            raw = self.read(self.worktree/name)
            normalized = sha(raw.replace(b"\r\n", b"\n"))
            rows.append({"path": name, "bytes": len(raw), "sha256": sha(raw), "normalized_sha256": normalized})
            material.update((name+":"+normalized+"\n").encode())
        return {"fingerprint": material.hexdigest(), "patterns": patterns, "files": rows}
