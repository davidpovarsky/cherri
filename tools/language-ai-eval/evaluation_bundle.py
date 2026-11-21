#!/usr/bin/env python3
"""Seal six fresh public task instances before any provider request.

The signature authenticates a frozen bundle; the separately reread Asana story
anchors that commitment in server time. Neither proves unseen task patterns.
"""

import argparse
import base64
import datetime as dt
import hashlib
import hmac
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import stat
import subprocess
import tempfile
import random


SCHEMA = "cherri-language-ai-eval/v1"
BUNDLE_SCHEMA = "cherri-language-ai-eval-bundle/v1"
PROFILE = "fresh-six-exact-instances/v1"
MODEL = "claude-sonnet-5-5"
API_VERSION = "2026-10-21"
ORIGINAL_SUITE_SHA256 = "fcdb35966a50e2bd7daaa1311a552bde46136e5824109d71d8bf8918db6b4cb5"
COMPILER_SOURCE = "b791ce0616d1821cebfc57909a6bc50a195ca68c"
COMPILER_SHA256 = "50e73c811c6829448f7406a6be4731b632cf27ef038615c83bdd5c96cb3b6ef3"
ASANA_TASK = "1220432417414104"
TASK_IDS = (
    "generate_unicode_literal", "generate_sum", "generate_list_selection",
    "edit_literal_only", "edit_binding_reference", "edit_arithmetic_operand",
)
GUIDE_FILES = {"legacy": "docs/agent-technical-reference.md",
               "v2": "docs/language-v2/language-guide.md"}
GUIDE_HASHES = {
    "legacy": "d0fc72289957f6f94eef37f21949b71befc363fa80bffb9c2e5bddab83b0d86e",
    "v2": "e2bda24b3431c62c6307cd4a817edd553e7c2efbbb69600c63d43cbaf5c028b3",
}
PRODUCER_FILES = (
    "runner.py", "evaluation_bundle.py", "anthropic_adapter.py",
    "graph_validator.py", "evaluation_intake.py", "tasks.json",
)
HTTP_TIMEOUTS = {"count": 15, "message": 90}
PROVIDER = {"model": MODEL, "api_version": API_VERSION,
            "settings": {"thinking": {"type": "disabled"}, "stream": False}}


class BundleError(ValueError):
    """Public diagnostics contain fixed codes, never request/credential data."""


def require(ok, code):
    if not ok:
        raise BundleError(code)


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def canonical(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":"),
                       ensure_ascii=False, allow_nan=False) + "\n").encode("utf-8")


def unique_json(raw):
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result, "DUPLICATE_JSON_KEY")
            result[key] = value
        return result
    return json.loads(raw, object_pairs_hook=pairs,
                      parse_constant=lambda _: (_ for _ in ()).throw(BundleError("NONFINITE_JSON")))


def read_regular(path, limit=4 * 1024 * 1024):
    path = Path(path)
    require(type(limit) is int and limit >= 0, "INPUT_SIZE_LIMIT")
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and not path.is_symlink() and not getattr(path, "is_junction", lambda: False)(), "INPUT_NOT_REGULAR")
    require(all(not p.is_symlink() and not getattr(p, "is_junction", lambda: False)() for p in path.parents), "INPUT_PARENT_SYMLINK")
    require(before.st_size <= limit, "INPUT_SIZE_LIMIT")
    # Python 3.12 on Windows exposes creation time through path.lstat().ctime
    # but change time through fstat().ctime. Compare that field within each
    # API, never between those distinct meanings. Device/inode/type/size/mtime
    # still bind the path and opened handle exactly before and after the read.
    identity = lambda value: (value.st_dev, value.st_ino, value.st_mode, value.st_size, value.st_mtime_ns)
    with path.open("rb") as stream:
        opened = os.fstat(stream.fileno())
        require(stat.S_ISREG(opened.st_mode) and identity(opened) == identity(before), "INPUT_CHANGED_DURING_READ")
        raw = stream.read(limit + 1)
        after = os.fstat(stream.fileno())
    require(len(raw) <= limit, "INPUT_SIZE_LIMIT")
    path_after = path.lstat()
    require(identity(after) == identity(before) == identity(path_after) and len(raw) == before.st_size
            and before.st_ctime_ns == path_after.st_ctime_ns and opened.st_ctime_ns == after.st_ctime_ns,
            "INPUT_CHANGED_DURING_READ")
    require(not getattr(path, "is_junction", lambda: False)() and all(
            not p.is_symlink() and not getattr(p, "is_junction", lambda: False)() for p in path.parents), "INPUT_PARENT_SYMLINK")
    return raw


def write_new(path, raw):
    path = Path(path)
    require(not path.is_symlink(), "OUTPUT_SYMLINK")
    with path.open("xb") as stream:
        stream.write(raw)


def utc_now():
    return dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z")


def utc_time(value):
    require(isinstance(value, str), "TIME_NOT_STRING")
    try:
        parsed = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as error:
        raise BundleError("INVALID_TIME") from error
    require(parsed.tzinfo is not None and parsed.utcoffset() == dt.timedelta(0), "TIME_NOT_UTC")
    return parsed


def openssl_path():
    supplied = os.environ.get("CHERRI_AI_EVAL_OPENSSL")
    candidate = supplied or shutil.which("openssl")
    if not candidate and os.name == "nt":
        candidate = r"C:\Program Files\Git\usr\bin\openssl.exe"
    require(candidate is not None and Path(candidate).is_file(), "OPENSSL_UNAVAILABLE")
    return str(Path(candidate).resolve())


def crypto(*args):
    try:
        result = subprocess.run([openssl_path(), *map(str, args)], capture_output=True,
                                timeout=10, check=False)
    except (OSError, subprocess.SubprocessError) as error:
        raise BundleError("SIGNATURE_PROCESS_FAILED") from error
    require(result.returncode == 0, "SIGNATURE_VERIFICATION_FAILED")
    return result.stdout


def verify_signature(manifest_raw, public_raw, signature_raw):
    require(len(signature_raw) == 64, "SIGNATURE_SIZE")
    require(public_raw.startswith(b"-----BEGIN PUBLIC KEY-----"), "PUBLIC_KEY_FORMAT")
    with tempfile.TemporaryDirectory(prefix="cherri-eval-signature-") as temporary:
        root = Path(temporary)
        (root / "manifest.json").write_bytes(manifest_raw)
        (root / "public.pem").write_bytes(public_raw)
        (root / "signature.bin").write_bytes(signature_raw)
        key = crypto("pkey", "-pubin", "-in", root / "public.pem", "-text", "-noout")
        require(b"ED25519" in key.upper(), "SIGNATURE_ALGORITHM")
        crypto("pkeyutl", "-verify", "-rawin", "-pubin", "-inkey", root / "public.pem",
               "-in", root / "manifest.json", "-sigfile", root / "signature.bin")


def derive(seed, label):
    require(type(seed) is bytes and len(seed) == 32, "SEED_SIZE")
    return hmac.new(seed, (PROFILE + ":" + label).encode(), hashlib.sha256).digest()


def generate_suite(seed):
    """No corpus, model, clock or mutable file is an input to task generation."""
    def nonce(label):
        return derive(seed, label).hex()[:32]
    def number(label):
        return 1 + int.from_bytes(derive(seed, label)[:4], "big") % 1_000_000_000
    text = 'שלום 🍒 {literal_' + nonce("unicode") + '} "done"'
    sum_name = "sum_" + nonce("sum-binding")
    left, right = number("sum-left"), number("sum-right")
    colors = [color + "_" + nonce("list-" + color) for color in ("red", "green", "blue")]
    count_name, count = "count_" + nonce("count-binding"), number("count-value")
    draft, ready = "draft_" + nonce("draft"), "ready_" + nonce("ready")
    title_name, typo = "title_" + nonce("title-binding"), "titel_" + nonce("title-binding")
    title = "Cherri_" + nonce("title-value")
    result_name = "result_" + nonce("result-binding")
    edit_left, edit_right = number("edit-left"), number("edit-right")
    q = lambda value: json.dumps(value, ensure_ascii=False)
    tasks = [
        {"id": TASK_IDS[0], "kind": "generation",
         "prompt": "Create a shortcut that displays exactly the supplied literal, preserving the emoji, Hebrew, braces, and double quote: " + text + ". Return source only.",
         "starter": {"legacy": "", "v2": ""},
         "oracle": {"visible_text": text, "type": "Text", "side_effects": ["show"]}},
        {"id": TASK_IDS[1], "kind": "generation",
         "prompt": f"Compute {left} plus {right} using a numeric expression, bind its result as {sum_name}, and display that bound numeric result. Return source only.",
         "starter": {"legacy": "", "v2": ""},
         "oracle": {"numeric_result": left + right, "requires_computation": True, "side_effects": ["show"]}},
        {"id": TASK_IDS[2], "kind": "generation",
         "prompt": "Create a list containing " + ", ".join(colors) + " in this order. Select the middle item from that list and display it. Return source only.",
         "starter": {"legacy": "", "v2": ""},
         "oracle": {"list": colors, "selected_text": colors[1], "side_effects": ["show"]}},
        {"id": TASK_IDS[3], "kind": "edit",
         "prompt": f"Change only the displayed text from {draft} to {ready}. Keep the existing number binding and do not add side effects. Return complete source only.",
         "starter": {"legacy": f"@{count_name} = {count}\nshow({q(draft)})\n", "v2": f"let {count_name} = {count}\nshow({q(draft)})\n"},
         "oracle": {"visible_text": ready, "unchanged_binding": {count_name: count}, "side_effects": ["show"]}},
        {"id": TASK_IDS[4], "kind": "edit",
         "prompt": "Fix the misspelled reference so the shortcut displays the existing title binding. Do not replace the reference with a hard-coded duplicate. Return complete source only.",
         "starter": {"legacy": f"@{title_name} = {q(title)}\nshow(@{typo})\n", "v2": f"let {title_name} = {q(title)}\nshow({typo})\n"},
         "oracle": {"visible_text": title, "requires_binding_reference": title_name, "side_effects": ["show"]}},
        {"id": TASK_IDS[5], "kind": "edit",
         "prompt": f"Fix the missing right operand by adding the number {edit_right}, retain the result binding and display that bound result. Return complete source only.",
         "starter": {"legacy": f"@{result_name} = {edit_left} +\nshow(@{result_name})\n", "v2": f"let {result_name} = {edit_left} +\nshow({result_name})\n"},
         "oracle": {"numeric_result": edit_left + edit_right, "requires_binding_reference": result_name, "side_effects": ["show"]}},
    ]
    parameters = {TASK_IDS[1]: {"left": left, "right": right, "binding": sum_name},
                  TASK_IDS[5]: {"left": edit_left, "right": edit_right, "binding": result_name}}
    return {"schema": SCHEMA, "tasks": tasks}, parameters


def model_request(task, arm, bundle):
    manifest = bundle["manifest"]
    request = {"schema": SCHEMA, "model": manifest["provider"]["model"],
               "budget": manifest["budget"], "arm": arm, "task_id": task["id"],
               "kind": task["kind"], "prompt": task["prompt"],
               "starter": task["starter"][arm], "guide": bundle["guides"][arm],
               "seed": manifest["order_seed"], "suite_sha256": manifest["suite_sha256"]}
    # Match the existing runner's identity encoding exactly.
    request["request_id"] = digest(json.dumps(request, sort_keys=True).encode())
    return request


def validate_model_request(request, bundle):
    require(isinstance(request, dict), "MODEL_REQUEST_TYPE")
    tasks = {task["id"]: task for task in bundle["suite"]["tasks"]}
    require(request.get("task_id") in tasks and request.get("arm") in ("legacy", "v2"), "MODEL_REQUEST_CASE")
    expected = model_request(tasks[request["task_id"]], request["arm"], bundle)
    require(request == expected, "MODEL_REQUEST_NOT_SEALED_PUBLIC_INPUT")


def request_sequence(bundle):
    sequence = [(task, arm) for task in bundle["suite"]["tasks"] for arm in ("legacy", "v2")]
    random.Random(bundle["manifest"]["order_seed"]).shuffle(sequence)
    return [model_request(task, arm, bundle) for task, arm in sequence]


def anchor_text(bundle):
    manifest = bundle["manifest"]
    signature = base64.b64encode(read_regular(bundle["root"] / "signature.bin", 64)).decode()
    return ("[רוני | QA] [SUBAGENT /root/apple_runtime]\n"
            "AI01 fresh exact-instance evaluation commitment; no generations yet.\n"
            f"Manifest SHA256: {digest(canonical(manifest))}\n"
            f"Public key SHA256: {manifest['public_key_sha256']}\n"
            f"Ed25519 signature: {signature}\n"
            f"Suite SHA256: {manifest['suite_sha256']}\n"
            f"Oracle SHA256: {manifest['oracle_sha256']}\n"
            f"Seed commitment SHA256: {manifest['seed_commitment_sha256']}\n"
            f"Model: {MODEL}; API: {API_VERSION}\n"
            "Six paired tasks, twelve outcomes; zero retries or outcome-driven regeneration.\n"
            "Fresh exact instances only; unseen-pattern and Apple-runtime claims remain false.")


def validate_anchor(story, bundle):
    require(isinstance(story, dict), "ANCHOR_RESPONSE_TYPE")
    story = story.get("data", story)
    if isinstance(story, dict) and story.get("gid") == ASANA_TASK and isinstance(story.get("comments"), list):
        comments = [item for item in story["comments"] if isinstance(item, dict) and item.get("text") == anchor_text(bundle)]
        require(len(comments) == 1, "ANCHOR_TASK_COMMENT_INVENTORY")
        story = {**comments[0], "task": {"gid": ASANA_TASK}}
    require(isinstance(story, dict) and re.fullmatch(r"[0-9]{10,30}", str(story.get("gid", ""))) is not None, "ANCHOR_STORY_ID")
    require(story.get("resource_subtype") == "comment_added" or story.get("type") == "comment", "ANCHOR_NOT_COMMENT")
    require(story.get("text") == anchor_text(bundle), "ANCHOR_COMMITMENT_MISMATCH")
    target = story.get("target", story.get("task", {}))
    require(isinstance(target, dict) and target.get("gid") == ASANA_TASK, "ANCHOR_TASK_MISMATCH")
    created = utc_time(story.get("created_at"))
    require(created >= utc_time(bundle["manifest"]["created_at"]), "ANCHOR_PRECEDES_BUNDLE")
    return {"story_id": story["gid"], "created_at": story["created_at"],
            "task_id": ASANA_TASK, "text_sha256": digest(story["text"].encode())}


def load_bundle(path, require_anchor=True):
    root = Path(path)
    require(root.is_absolute() and root.is_dir() and not root.is_symlink(), "BUNDLE_ROOT")
    raw = read_regular(root / "manifest.json")
    manifest = unique_json(raw)
    require(isinstance(manifest, dict) and raw == canonical(manifest), "MANIFEST_NOT_CANONICAL")
    require(manifest.get("schema") == BUNDLE_SCHEMA and manifest.get("profile") == PROFILE, "BUNDLE_PROFILE")
    require(manifest.get("provider") == PROVIDER, "PROVIDER_SETTINGS")
    require(manifest.get("budget") == {"max_input_tokens": 12000, "max_output_tokens": 2000}, "EQUAL_BUDGET")
    require(all(type(value) is int for value in manifest["budget"].values()), "TOKEN_CAP_TYPES")
    require(manifest.get("http_timeout_seconds") == HTTP_TIMEOUTS and manifest.get("process_timeout_seconds") == 120, "SEALED_TIMEOUTS")
    require(manifest.get("expected_outcomes") == 12 and manifest.get("retry_limit") == 0, "OUTCOME_POLICY")
    require(type(manifest["expected_outcomes"]) is int and type(manifest["retry_limit"]) is int
            and type(manifest["process_timeout_seconds"]) is int
            and all(type(value) is int for value in manifest["http_timeout_seconds"].values()), "BOUND_TYPES")
    require(manifest.get("task_ids") == list(TASK_IDS), "TASK_INVENTORY")
    require(manifest.get("provenance_scope") == "fresh_exact_instances_after_code_and_model_freeze"
            and manifest.get("unseen_pattern_provenance_proved") is False
            and manifest.get("apple_runtime_proved") is False, "PROVENANCE_SCOPE")
    require(re.fullmatch(r"[0-9a-f]{40}", str(manifest.get("implementation_sha", ""))) is not None, "IMPLEMENTATION_SHA")
    require(type(manifest.get("order_seed")) is int and 0 <= manifest["order_seed"] < 2**63, "ORDER_SEED")
    require(manifest.get("original_suite_sha256") == ORIGINAL_SUITE_SHA256, "ORIGINAL_SUITE_IDENTITY")
    public = read_regular(root / "public-key.pem", 8192)
    require(digest(public) == manifest.get("public_key_sha256"), "PUBLIC_KEY_HASH")
    verify_signature(raw, public, read_regular(root / "signature.bin", 64))
    seed = read_regular(root / "private" / "seed.bin", 32)
    require(digest(seed) == manifest.get("seed_commitment_sha256"), "SEED_COMMITMENT")
    require(manifest["order_seed"] == int.from_bytes(derive(seed, "order")[:8], "big") % 2**63, "ORDER_SEED_NOT_GENERATED")
    suite, parameters = generate_suite(seed)
    suite_raw = read_regular(root / "suite.json")
    require(suite_raw == canonical(suite) and digest(suite_raw) == manifest.get("suite_sha256"), "GENERATED_SUITE_MISMATCH")
    oracle = {task["id"]: task["oracle"] for task in suite["tasks"]}
    require(digest(canonical(oracle)) == manifest.get("oracle_sha256"), "ORACLE_HASH")
    original = read_regular(Path(__file__).with_name("tasks.json"))
    require(digest(original) == ORIGINAL_SUITE_SHA256, "ORIGINAL_SUITE_CHANGED")
    originals = unique_json(original)["tasks"]
    require([task["id"] for task in originals] == list(TASK_IDS), "ORIGINAL_TASK_INVENTORY")
    for task, template in zip(suite["tasks"], originals):
        require(task["kind"] == template["kind"] and set(task["oracle"]) == set(template["oracle"]), "ORIGINAL_CONTRACT_SHAPE_CHANGED")
    producer_hashes = manifest.get("producer_sha256")
    require(isinstance(producer_hashes, dict) and set(producer_hashes) == set(PRODUCER_FILES), "PRODUCER_INVENTORY")
    for name, expected in producer_hashes.items():
        require(digest(read_regular(Path(__file__).with_name(name))) == expected, "PRODUCER_CHANGED")
    guides = {}
    require(manifest.get("guide_sha256") == GUIDE_HASHES, "UNREVIEWED_GUIDE_IDENTITY")
    for arm, expected in GUIDE_HASHES.items():
        guide = read_regular(root / (arm + "-guide.md"))
        require(digest(guide) == expected, "GUIDE_HASH")
        guides[arm] = guide.decode("utf-8")
    require(manifest.get("compiler") == {"source_sha": COMPILER_SOURCE, "binary_sha256": COMPILER_SHA256}, "COMPILER_IDENTITY")
    require(manifest.get("compiler_environment_profile") == "isolated-defaults/v1", "COMPILER_ENVIRONMENT_PROFILE")
    require(digest(read_regular(root / "compiler.exe", 512 * 1024 * 1024)) == COMPILER_SHA256, "COMPILER_BYTES_CHANGED")
    metadata = read_regular(root / "model-metadata-receipt.json")
    require(digest(metadata) == manifest.get("model_metadata_receipt_sha256"), "MODEL_METADATA_CHANGED")
    validate_model_metadata(unique_json(metadata))
    model_raw = read_regular(root / "model-metadata-response.original.body")
    metadata_object = unique_json(metadata)
    require(digest(model_raw) == metadata_object.get("responseSha256")
            and len(model_raw) == metadata_object.get("responseBytes"), "MODEL_METADATA_ORIGINAL_BINDING")
    model_object = unique_json(model_raw)
    require({key: model_object.get(key) for key in ("id", "created_at", "lifecycle")} == metadata_object["returnedModel"], "MODEL_METADATA_ORIGINAL_IDENTITY")
    require(utc_time(metadata_object["finishedAtUtc"]) < utc_time(manifest["created_at"]), "MODEL_NOT_FROZEN_BEFORE_SUITE")
    result = {"root": root, "manifest": manifest, "suite": suite, "parameters": parameters,
              "guides": guides, "anchor": {}}
    if require_anchor:
        story_raw = read_regular(root / "anchor-story.json")
        reread_raw = read_regular(root / "anchor-reread.json")
        first = validate_anchor(unique_json(story_raw), result)
        second = validate_anchor(unique_json(reread_raw), result)
        require(first == second, "ANCHOR_REREAD_MISMATCH")
        require(utc_time(second["created_at"]) <= utc_time(utc_now()), "ANCHOR_IN_FUTURE")
        result["anchor"] = second
    return result


def claim_request(bundle, request, stage):
    require(stage in ("provider", "validator"), "ATTEMPT_STAGE")
    identity = request.get("request_id", "")
    require(re.fullmatch(r"[0-9a-f]{64}", identity) is not None, "ATTEMPT_ID")
    expected_ids = {model_request(task, arm, bundle)["request_id"]
                    for task in bundle["suite"]["tasks"] for arm in ("legacy", "v2")}
    require(identity in expected_ids, "ATTEMPT_NOT_IN_SEALED_SUITE")
    root = bundle["root"] / "attempts"
    root.mkdir(exist_ok=True)
    parent = root / identity
    parent.mkdir(exist_ok=True)
    require(not root.is_symlink() and not parent.is_symlink(), "ATTEMPT_SYMLINK")
    target = parent / stage
    try:
        target.mkdir()
    except FileExistsError as error:
        raise BundleError("ATTEMPT_ALREADY_CLAIMED_NO_RETRY") from error
    write_new(target / "intent.json", canonical({"request_id": identity, "stage": stage,
              "started_at": utc_now(), "manifest_sha256": digest(canonical(bundle["manifest"])),
              "anchor": bundle["anchor"]}))
    return target


def seal_outcome(bundle, record):
    """Bind originals after an outcome; the signing key is never published.

    Reconciled file/receipt hash tampering cannot replace this signed inventory
    whose verification key was anchored before any request.
    """
    identity = record.get("request_id")
    require(identity in {item["request_id"] for item in request_sequence(bundle)}, "OUTCOME_NOT_DECLARED")
    root = bundle["root"]
    originals = {}
    for directory in (root / "attempts" / identity, root / "runner-processes" / identity):
        if directory.exists():
            require(directory.is_dir() and not directory.is_symlink(), "OUTCOME_DIRECTORY")
            for path in sorted(directory.rglob("*")):
                require(not path.is_symlink(), "OUTCOME_SYMLINK")
                if path.is_file():
                    raw = read_regular(path)
                    originals[path.relative_to(root).as_posix()] = {"sha256": digest(raw), "bytes": len(raw)}
    require(originals, "OUTCOME_ORIGINALS_MISSING")
    seals = root / "outcome-seals"
    seals.mkdir(exist_ok=True)
    require(not seals.is_symlink(), "OUTCOME_SEAL_SYMLINK")
    payload = {"schema": "cherri-ai-outcome-seal/v1", "manifest_sha256": digest(canonical(bundle["manifest"])),
               "request_id": identity, "record": record, "sealed_at": utc_now(), "originals": originals}
    path, signature = seals / (identity+".json"), seals / (identity+".sig")
    write_new(path, canonical(payload))
    require(not signature.exists(), "OUTCOME_ALREADY_SEALED")
    crypto("pkeyutl", "-sign", "-rawin", "-inkey", root / "private" / "signing-key.pem",
           "-in", path, "-out", signature)
    verify_signature(read_regular(path), read_regular(root / "public-key.pem"), read_regular(signature, 64))
    return {"outcome_seal_sha256": digest(read_regular(path)), "outcome_signature_sha256": digest(read_regular(signature, 64))}


def verify_outcome_seal(bundle, record):
    root = bundle["root"]
    identity = record.get("request_id", "")
    require(re.fullmatch(r"[0-9a-f]{64}", identity) is not None, "OUTCOME_ID")
    raw = read_regular(root / "outcome-seals" / (identity+".json"))
    signature = read_regular(root / "outcome-seals" / (identity+".sig"), 64)
    require(digest(raw) == record.get("outcome_seal_sha256")
            and digest(signature) == record.get("outcome_signature_sha256"), "OUTCOME_SEAL_HASH")
    verify_signature(raw, read_regular(root / "public-key.pem"), signature)
    payload = unique_json(raw)
    require(raw == canonical(payload) and payload.get("schema") == "cherri-ai-outcome-seal/v1"
            and payload.get("manifest_sha256") == digest(canonical(bundle["manifest"]))
            and payload.get("request_id") == identity, "OUTCOME_SEAL_IDENTITY")
    plain = {key: value for key, value in record.items() if key not in ("outcome_seal_sha256", "outcome_signature_sha256")}
    require(payload.get("record") == plain, "OUTCOME_RECORD_CHANGED")
    require(utc_time(payload["sealed_at"]) >= utc_time(record["finished_at"]), "OUTCOME_SEAL_CHRONOLOGY")
    paths = payload.get("originals")
    require(isinstance(paths, dict) and paths, "OUTCOME_SEAL_INVENTORY")
    actual = set()
    for directory in (root / "attempts" / identity, root / "runner-processes" / identity):
        if directory.exists():
            actual.update(path.relative_to(root).as_posix() for path in directory.rglob("*") if path.is_file())
    require(actual == set(paths), "OUTCOME_ORIGINAL_INVENTORY_CHANGED")
    for name, expected in paths.items():
        candidate = root / name
        require(candidate.resolve().is_relative_to(root.resolve()) and ".." not in Path(name).parts, "OUTCOME_ORIGINAL_PATH")
        original = read_regular(candidate)
        require(expected == {"sha256": digest(original), "bytes": len(original)}, "SEALED_OUTCOME_ORIGINAL_CHANGED")
    return payload


def create_bundle(output, code_root, compiler, model_receipt, model_response):
    output, code_root, compiler = Path(output), Path(code_root).resolve(), Path(compiler)
    require(output.is_absolute() and not output.exists(), "BUNDLE_OUTPUT_MUST_BE_NEW")
    git = lambda *args: subprocess.check_output(["git", "--no-replace-objects", *args], cwd=code_root, timeout=15)
    require(git("status", "--porcelain") == b"", "SOURCE_NOT_FROZEN_CLEAN")
    head = git("rev-parse", "HEAD").decode().strip()
    require(re.fullmatch(r"[0-9a-f]{40}", head) is not None, "SOURCE_HEAD")
    producer_root = code_root / "tools" / "language-ai-eval"
    require(producer_root.resolve() == Path(__file__).parent.resolve(), "SEALER_NOT_FROM_FROZEN_SOURCE")
    hashes = {}
    for name in PRODUCER_FILES:
        raw = read_regular(producer_root / name)
        require(raw == git("show", head + ":tools/language-ai-eval/" + name), "SOURCE_WORKING_BYTES_CHANGED")
        hashes[name] = digest(raw)
    require(hashes["tasks.json"] == ORIGINAL_SUITE_SHA256, "ORIGINAL_SUITE_CHANGED")
    compiler_bytes = read_regular(compiler, 512 * 1024 * 1024)
    require(digest(compiler_bytes) == COMPILER_SHA256, "UNREVIEWED_COMPILER_BINARY")
    metadata_raw = read_regular(model_receipt)
    metadata = unique_json(metadata_raw)
    validate_model_metadata(metadata)
    model_raw = read_regular(model_response)
    require(digest(model_raw) == metadata.get("responseSha256") and len(model_raw) == metadata.get("responseBytes"), "MODEL_METADATA_ORIGINAL_BINDING")
    output.mkdir()
    private = output / "private"
    private.mkdir(mode=0o700)
    seed = secrets.token_bytes(32)
    write_new(private / "seed.bin", seed)
    suite, _ = generate_suite(seed)
    suite_raw = canonical(suite)
    write_new(output / "suite.json", suite_raw)
    write_new(output / "compiler.exe", compiler_bytes)
    write_new(output / "model-metadata-receipt.json", metadata_raw)
    write_new(output / "model-metadata-response.original.body", model_raw)
    for arm, relative in GUIDE_FILES.items():
        guide = read_regular(code_root / relative)
        require(digest(guide) == GUIDE_HASHES[arm] and guide == git("show", head + ":" + relative), "UNREVIEWED_GUIDE")
        write_new(output / (arm + "-guide.md"), guide)
    crypto("genpkey", "-algorithm", "ED25519", "-out", private / "signing-key.pem")
    crypto("pkey", "-in", private / "signing-key.pem", "-pubout", "-out", output / "public-key.pem")
    manifest = {"schema": BUNDLE_SCHEMA, "profile": PROFILE, "created_at": utc_now(),
                "implementation_sha": head, "producer_sha256": hashes,
                "original_suite_sha256": ORIGINAL_SUITE_SHA256,
                "seed_commitment_sha256": digest(seed), "suite_sha256": digest(suite_raw),
                "oracle_sha256": digest(canonical({t["id"]: t["oracle"] for t in suite["tasks"]})),
                "public_key_sha256": digest(read_regular(output / "public-key.pem")),
                "provider": PROVIDER,
                "model_metadata_receipt_sha256": digest(metadata_raw),
                "budget": {"max_input_tokens": 12000, "max_output_tokens": 2000},
                "http_timeout_seconds": HTTP_TIMEOUTS,
                "process_timeout_seconds": 120, "order_seed": int.from_bytes(derive(seed, "order")[:8], "big") % 2**63,
                "guide_sha256": GUIDE_HASHES, "compiler": {"source_sha": COMPILER_SOURCE, "binary_sha256": COMPILER_SHA256},
                "compiler_environment_profile": "isolated-defaults/v1",
                "task_ids": list(TASK_IDS), "expected_outcomes": 12, "retry_limit": 0,
                "provenance_scope": "fresh_exact_instances_after_code_and_model_freeze",
                "unseen_pattern_provenance_proved": False, "apple_runtime_proved": False}
    write_new(output / "manifest.json", canonical(manifest))
    crypto("pkeyutl", "-sign", "-rawin", "-inkey", private / "signing-key.pem",
           "-in", output / "manifest.json", "-out", output / "signature.bin")
    require(git("status", "--porcelain") == b"" and git("rev-parse", "HEAD").decode().strip() == head, "SOURCE_CHANGED_DURING_SEAL")
    bundle = load_bundle(output, require_anchor=False)
    write_new(output / "asana-commitment.txt", anchor_text(bundle).encode())
    return {"manifest_sha256": digest(canonical(manifest)), "suite_sha256": manifest["suite_sha256"],
            "expected_outcomes": 12, "anchor_required": True, "generations_started": 0}


def validate_model_metadata(metadata):
    require(isinstance(metadata, dict) and metadata.get("httpStatus") == 200
            and metadata.get("operation") == "PROVIDER_MODEL_IDENTITY_ONLY"
            and metadata.get("requestedModelId") == MODEL
            and metadata.get("transportErrorType") is None and metadata.get("parseErrorType") is None
            and metadata.get("canonicalIdentityVerified") is True
            and metadata.get("returnedModel", {}).get("id") == MODEL
            and metadata.get("returnedModel", {}).get("lifecycle") == "active", "MODEL_METADATA_UNVERIFIED")
    require(utc_time(metadata["returnedModel"]["created_at"]) <= utc_time(metadata["finishedAtUtc"]), "MODEL_IDENTITY_CHRONOLOGY")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--prepare", action="store_true", required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--code-root", type=Path, required=True)
    parser.add_argument("--compiler", type=Path, required=True)
    parser.add_argument("--model-receipt", type=Path, required=True)
    parser.add_argument("--model-response", type=Path, required=True)
    args = parser.parse_args()
    try:
        print(json.dumps(create_bundle(args.output, args.code_root, args.compiler, args.model_receipt, args.model_response)))
    except (OSError, ValueError, subprocess.SubprocessError):
        print("EVALUATION_BUNDLE_NOT_SEALED", file=__import__("sys").stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
