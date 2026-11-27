"""Explicit GitHub identity and namespaces for the FN04 hosted profile.

This module admits no local-to-hosted relabeling. The supplied context must equal
the current GitHub environment, and each native/build path has one fixed role.
The original local owner uses none of these paths unless this profile is explicit.
"""
from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import re
import stat

SCHEMA = "cherri-fn04-hosted-execution-context/1"
REPOSITORY = "davidpovarsky/cherri"
WORKFLOW = "FN04 signed native caller input"
WORKFLOW_FILE = ".github/workflows/p10r15-fn04-native.yml"
BRANCH = "probe/cherri-p10r15-fn04-native-sealed-20261127"
JOB = "native-caller-input"
CASES = ("nested_false", "nested_true", "without_functions")


def require(value, code):
    if not value:
        raise ValueError(code)


def canonical_owned_directory(value):
    require(type(value) is str and 1 < len(value.encode()) <= 4096 and
            Path(value).is_absolute() and ".." not in Path(value).parts and
            all(c not in value for c in ("\n", "\r", "\0")), "HOSTED_ENVIRONMENT_PATH")
    path = Path(value).resolve(strict=True)
    require(path.is_dir() and path.stat().st_uid == os.getuid(), "HOSTED_PATH_OWNER")
    return path


def read_create_event(env):
    value = env.get("GITHUB_EVENT_PATH")
    require(type(value) is str and Path(value).is_absolute(), "ACTUAL_CREATE_EVENT_PATH")
    path = Path(value)
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and not path.is_symlink()
            and before.st_uid == os.getuid() and 0 < before.st_size <= 65536,
            "BOUNDED_CREATE_EVENT_FILE")
    raw = path.read_bytes()
    after = path.lstat()
    require(len(raw) == before.st_size and
            all(getattr(before, field) == getattr(after, field) for field in
                ("st_dev", "st_ino", "st_mode", "st_uid", "st_gid", "st_size",
                 "st_mtime_ns", "st_ctime_ns")), "CREATE_EVENT_CHANGED")
    def pairs(rows):
        result = {}
        for key, item in rows:
            require(key not in result, "DUPLICATE_CREATE_EVENT_KEY")
            result[key] = item
        return result
    event = json.loads(raw, object_pairs_hook=pairs,
        parse_constant=lambda value: require(False, "NONFINITE_CREATE_EVENT"))
    require(type(event) is dict and event.get("ref_type") == "branch"
            and event.get("ref") == BRANCH and type(event.get("repository")) is dict
            and event["repository"].get("full_name") == REPOSITORY,
            "FIRST_EXACT_CREATE_EVENT")
    return event, raw


def context_from_environment(env=None):
    env = os.environ if env is None else env
    require(env.get("GITHUB_ACTIONS") == "true" and
            env.get("GITHUB_REPOSITORY") == REPOSITORY and
            env.get("GITHUB_WORKFLOW") == WORKFLOW and env.get("GITHUB_JOB") == JOB and
            env.get("GITHUB_EVENT_NAME") == "create" and
            env.get("RUNNER_OS") == "macOS", "ACTUAL_FN04_WORKFLOW_CONTEXT")
    source, run, attempt = (env.get(k, "") for k in
        ("GITHUB_SHA", "GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT"))
    require(type(source) is str and re.fullmatch("[0-9a-f]{40}", source) is not None and
            type(run) is str and re.fullmatch("[1-9][0-9]{0,19}", run) is not None and
            attempt == "1", "ACTUAL_FN04_RUN_IDENTITY")
    ref = "refs/heads/" + BRANCH
    require(env.get("GITHUB_REF") == ref and
            env.get("GITHUB_WORKFLOW_REF") == REPOSITORY + "/" + WORKFLOW_FILE + "@" + ref and
            env.get("GITHUB_WORKFLOW_SHA") == source, "ACTUAL_FN04_WORKFLOW_SOURCE")
    event, event_raw = read_create_event(env)
    case = env.get("CHERRI_FN04_CASE")
    require(case == "nested_false", "ACTUAL_FN04_CASE")
    repo = canonical_owned_directory(env.get("GITHUB_WORKSPACE"))
    temporary = canonical_owned_directory(env.get("RUNNER_TEMP"))
    require(repo != temporary and repo not in temporary.parents and
            temporary not in repo.parents, "HOSTED_SOURCE_TEMP_ALIAS")
    namespace = temporary / ("cherri-fn04-" + run + "-1")
    return {"schema": SCHEMA, "repository": REPOSITORY, "workflow": WORKFLOW,
        "workflowFile": WORKFLOW_FILE, "ref": ref, "job": JOB,
        "sourceCommitSha": source, "runId": run, "runAttempt": 1,
        "caseId": case, "runnerOS": "macOS", "workspace": str(repo),
        "runnerTemp": str(temporary), "namespace": str(namespace), "uid": os.getuid(),
        "executionLocation": "GITHUB_ACTIONS_MACOS", "eventName": "create",
        "eventPayloadSHA256": hashlib.sha256(event_raw).hexdigest()}


def validate_context(value):
    require(type(value) is dict and value == context_from_environment(),
            "HOSTED_CONTEXT_CHANGED")
    return value


def paths(value):
    namespace = Path(value["namespace"])
    private = namespace / "private"
    return {"repo": Path(value["workspace"]), "namespace": namespace,
        "private": private, "root": private / "native", "buildRoot": private / "build",
        "bundleRoot": private / "packets", "signingRoot": private / "signatures",
        "derivedData": namespace / "DerivedData", "public": namespace / "public"}


def validate_plan_paths(plan, value):
    value = validate_context(value)
    expected = paths(value)
    require(all(plan[key] == str(expected[key]) for key in
                ("repo", "root", "buildRoot", "bundleRoot")), "HOSTED_PLAN_NAMESPACES")
    require(plan["controllerCommitSha"] == plan["builtHarnessCommitSha"] ==
            value["sourceCommitSha"] and plan["runId"] == value["runId"] and
            plan["caseId"] == value["caseId"], "HOSTED_PLAN_IDENTITY")
    return value


def validate_signing_path(path, plan, value):
    validate_plan_paths(plan, value)
    require(path == str(paths(value)["signingRoot"]), "HOSTED_SIGNING_NAMESPACE")


def product_scope(xctestrun, value):
    validate_context(value)
    return Path(xctestrun).parent == paths(value)["derivedData"] / "Build/Products"


def validate_build_origin(value, context):
    validate_context(context)
    require(value.get("hostedContext") == context and
            value.get("executionLocation") == "GITHUB_ACTIONS_MACOS",
            "ACTUAL_HOSTED_BUILD_CONTEXT")
