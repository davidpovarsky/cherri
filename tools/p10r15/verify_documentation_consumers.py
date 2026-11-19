#!/usr/bin/env python3
"""Collect actual installed Skill searches and published pinned documentation.

The live install and pinned source checkouts are read-only. One missing-current
control uses a temporary documentation directory. This is the docs half of D03;
it neither launches nor claims success for the iOS application.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import tempfile
import time
from urllib.parse import urlsplit
from urllib.request import Request, urlopen


COMPILER_ORIGIN = "https://github.com/davidpovarsky/cherri.git"
DOCS_ORIGIN = "https://github.com/davidpovarsky/cherrilang.org.git"
SITE = "https://davidpovarsky.github.io/cherrilang.org/"
SOURCE_PATHS = ("fork-language/index.md", "fork-language/guide.md",
                "fork-language/actions.md", "fork-language/migration.md")
SKILL_PATHS = ("scripts/search-docs.sh", "scripts/common.sh", "compatibility-manifest.json")
SOURCE_ARTIFACTS = ("docs-index.md", "docs-guide.md", "docs-actions.md", "docs-migration.md")
SKILL_ARTIFACTS = ("installed-search-docs.sh.txt", "installed-common.sh.txt",
                   "documentation-compatibility-manifest.json")
ROUTES = (("", "published-home.html"), ("fork-language/", "published-index.html"),
          ("fork-language/guide", "published-guide.html"),
          ("fork-language/actions", "published-actions.html"),
          ("fork-language/migration", "published-migration.html"),
          ("assets/js/search-data.json", "published-search-data.json"),
          ("assets/preview.js", "published-preview.js"))
CONTROL_NAMES = (
    "historical-route-in-default-search",
    "search-result-without-project-base",
    "client-search-request-without-project-base",
    "missing-home-authoring-entry",
    "missing-home-image",
    "damaged-canonical-action-fingerprint",
)
ABSENT_QUERY = "CHERRI_D03_NO_MATCH_6ce5e096b6a7"
MISSING_QUERY = "CHERRI_D03_HISTORICAL_SENTINEL_49CE"


def require(condition, message):
    if not condition:
        raise ValueError(message)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "Duplicate JSON key: " + key)
        result[key] = value
    return result


def load_json(data):
    require(0 < len(data) <= 4 * 1024 * 1024, "JSON input exceeds the evidence bound")
    def nonfinite(_):
        raise ValueError("Nonfinite JSON number")
    value = json.loads(data, object_pairs_hook=unique_object, parse_constant=nonfinite)
    require(isinstance(value, dict), "Expected a JSON object")
    return value


def regular_bytes(path, limit=4 * 1024 * 1024):
    path = Path(path)
    require(path.is_file() and not path.is_symlink(), "Missing or linked input: " + str(path))
    require(path.stat().st_size <= limit, "Oversized input: " + str(path))
    return path.read_bytes()


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args],
                                   stderr=subprocess.PIPE, timeout=30)


def verify_checkout(root, commit, origin):
    root = Path(root)
    require((root / ".git").is_dir(), "Expected an ordinary pinned checkout")
    require(not (root / ".git/objects/info/alternates").exists(), "Borrowed object store")
    require(git(root, "remote", "get-url", "origin").decode().strip() == origin,
            "Checkout origin mismatch")
    require(git(root, "rev-parse", "HEAD").decode().strip() == commit, "Checkout revision mismatch")
    require(not git(root, "status", "--porcelain", "--untracked-files=all").strip(), "Dirty checkout")


def validate_pins(pins):
    require(pins.get("compilerRepo") == COMPILER_ORIGIN and pins.get("docsRepo") == DOCS_ORIGIN,
            "Non-canonical source repositories")
    require(pins.get("languageVersion") == "2.0", "Unsupported documentation language version")
    for key, length in (("compilerCommit", 40), ("docsCommit", 40),
                        ("schemaFingerprint", 64), ("codecFingerprint", 64),
                        ("macroFingerprint", 64)):
        require(isinstance(pins.get(key), str) and re.fullmatch("[0-9a-f]{%d}" % length, pins[key]),
                "Invalid immutable pin: " + key)


def validate_site_build(build, controls, deployment, pins, actions):
    require(type(build.get("formatVersion")) is int and build.get("formatVersion") == 2 and build.get("baseUrl") == "/cherrilang.org"
            and build.get("publicationOrigin") == "https://davidpovarsky.github.io",
            "Wrong site build origin/base")
    require(build.get("docsCommit") == pins["docsCommit"]
            and build.get("compilerCommit") == pins["compilerCommit"]
            and build.get("sourceTreeClean") is True
            and build.get("sourceReferenceSha256") == digest(actions),
            "Stale or dirty site build")
    require(type(build.get("historicalSearchEntries")) is int and build.get("historicalSearchEntries") == 0 and type(build.get("searchEntries")) is int
            and build["searchEntries"] > 0 and build.get("sitePublished") is False,
            "Site build receipt is not the original verified build")
    require(sorted(build.get("schemaFingerprints", [])) ==
            sorted((pins["schemaFingerprint"], pins["macroFingerprint"])), "Site fingerprint drift")
    expected_routes = ["/cherrilang.org", "/cherrilang.org/fork-language",
                       "/cherrilang.org/fork-language/actions", "/cherrilang.org/fork-language/guide",
                       "/cherrilang.org/fork-language/migration"]
    require(build.get("currentSearchRoutes") == expected_routes, "Site current routes differ")
    require(controls.get("formatVersion") == 1 and controls.get("docsCommit") == pins["docsCommit"]
            and controls.get("compilerCommit") == pins["compilerCommit"]
            and type(controls.get("passed")) is int and controls["passed"] == 6
            and type(controls.get("failed")) is int and controls["failed"] == 0
            and type(controls.get("skipped")) is int and controls["skipped"] == 0
            and controls.get("artifactsRestoredExactly") is True, "Incomplete site causal controls")
    rows = controls.get("controls", [])
    require(len(rows) == len(CONTROL_NAMES), "Missing site causal case")
    for row, name in zip(rows, CONTROL_NAMES):
        require(row.get("name") == name and row.get("rejected") is True
                and isinstance(row.get("diagnostic"), str) and row["diagnostic"],
                "Site negative control lacks its actual diagnostic")
    require(deployment.get("headSha") == pins["docsCommit"]
            and deployment.get("conclusion") == "success" and deployment.get("status") == "completed",
            "Documentation deployment did not complete at the pinned revision")
    url = deployment.get("url", "")
    require(re.fullmatch(r"https://github.com/davidpovarsky/cherrilang.org/actions/runs/[1-9][0-9]*", url),
            "Deployment URL is not the canonical docs workflow")
    jobs = deployment.get("jobs", [])
    for name in ("build", "deploy"):
        matched = [row for row in jobs if row.get("name") == name]
        require(len(matched) == 1 and matched[0].get("conclusion") == "success"
                and matched[0].get("status") == "completed", "Missing successful deployment job: " + name)


def execute_owned(command, env, cwd, timeout=30):
    process = subprocess.Popen(command, env=env, cwd=cwd, stdout=subprocess.PIPE,
                               stderr=subprocess.PIPE, start_new_session=True)
    timed_out = False
    try:
        stdout, stderr = process.communicate(timeout=timeout)
    except subprocess.TimeoutExpired:
        timed_out = True
        try:
            os.killpg(process.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        try:
            stdout, stderr = process.communicate(timeout=2)
        except subprocess.TimeoutExpired:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            stdout, stderr = process.communicate(timeout=2)
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
    require(len(stdout) <= 512 * 1024 and len(stderr) <= 512 * 1024, "Unbounded search output")
    return process.returncode, stdout, stderr, timed_out


def public_get(route):
    request = Request(SITE + route, headers={"User-Agent": "Cherri-D03-source-bound-verifier/1",
                                           "Cache-Control": "no-cache"})
    with urlopen(request, timeout=30) as response:
        require(response.status == 200, "Published route is not HTTP 200")
        require(response.url == SITE + route, "Published route redirected away from its exact project URL")
        data = response.read(4 * 1024 * 1024 + 1)
        require(len(data) <= 4 * 1024 * 1024, "Published route exceeded its bounded evidence size")
        headers = {key: response.headers[key] for key in
                   ("Content-Type", "ETag", "Last-Modified", "x-github-request-id")
                   if key in response.headers}
        return data, headers


def validate_search_index(data, count):
    entries = load_json(data)
    require(len(entries) == count and count > 0, "Published search entry count differs from its build")
    allowed = {"", "/fork-language", "/fork-language/actions", "/fork-language/guide",
               "/fork-language/migration"}
    action_rows = 0
    for row in entries.values():
        require(isinstance(row, dict) and all(isinstance(row.get(key), str)
                for key in ("doc", "title", "content", "url", "relUrl")), "Malformed search result")
        relative = row["relUrl"]
        parsed = urlsplit(relative)
        require(not parsed.scheme and not parsed.netloc and not parsed.query
                and parsed.path.rstrip("/") in allowed, "Historical or external default search result")
        require(row["url"] == "/cherrilang.org" + relative, "Unbased default search result")
        if parsed.path.rstrip("/") == "/fork-language/actions":
            action_rows += 1
    require(action_rows > 0, "Published search omits the current generated actions")


class Artifacts:
    def __init__(self, directory):
        self.directory = Path(directory)
        require(not self.directory.exists() or not any(self.directory.iterdir()),
                "Evidence output is not empty")
        self.directory.mkdir(parents=True, exist_ok=True)
        self.members = []

    def write(self, name, data):
        require(Path(name).name == name and not (self.directory / name).exists(),
                "Evidence member would overwrite another artifact")
        (self.directory / name).write_bytes(data)
        self.members.append({"path": str((self.directory / name).resolve()), "sha256": digest(data)})
        return digest(data)


def inspect_search(name, arguments, skill, docs, home, store, execute=execute_owned):
    shell = shutil.which("sh")
    require(shell, "POSIX sh is unavailable")
    command = [shell, str(skill / "scripts/search-docs.sh"), *arguments]
    env = os.environ.copy()
    for key in tuple(env):
        if key.startswith("CHERRI_") or key == "SKILL_DIR":
            del env[key]
    env.update({"CHERRI_DOCS_DIR": str(docs), "CHERRI_HOME": str(home), "SKILL_DIR": str(skill)})
    exit_code, stdout, stderr, timed_out = execute(command, env, str(skill))
    require(not timed_out, "Search process timed out: " + name)
    row = {"name": name, "command": command, "cwd": str(skill), "docsDirectory": str(docs),
           "exitCode": exit_code, "timedOut": timed_out,
           "stdoutSHA256": store.write(name + ".stdout.log", stdout),
           "stderrSHA256": store.write(name + ".stderr.log", stderr)}
    active = str(docs / "fork-language")
    historical = str(docs / "language")
    text = stdout.decode("utf-8")
    error = stderr.decode("utf-8")
    if name in ("active-empty", "legacy-empty"):
        require(exit_code == 0 and not stderr and text == (active if name == "active-empty" else historical) + "\n",
                "Default/explicit scope does not select its exact documentation tree")
    elif name in ("active-show", "legacy-query", "restored-active"):
        prefix = active if name != "legacy-query" else historical
        lines = text.splitlines()
        require(exit_code == 0 and not stderr and lines and
                all(re.match(re.escape(prefix) + r"/[^:\n]+:[1-9][0-9]*:", line) for line in lines),
                "Search results escape the selected tree or are empty")
        if name != "legacy-query":
            require(any(line.startswith(active + "/actions.md:") and "show" in line.lower() for line in lines),
                    "Current action search does not reach generated action documentation")
    elif name == "absent-query":
        require(exit_code == 0 and not stdout and not stderr, "No-match query is not a clean empty result")
    elif name == "invalid-regex":
        require(exit_code not in (None, 0, 1) and not stdout and stderr,
                "Search execution error was swallowed as a successful no-match")
    elif name == "missing-current":
        require(exit_code == 1 and not stdout and
                error == "Active V2 documentation directory is missing or linked: " + active + "\n",
                "Missing current docs silently fell back to a historical tree")
    else:
        raise ValueError("Unknown search scenario")
    return row


def collect(source, skill, docs, implementation, site_build, site_controls, deployment, output,
            *, fetch=public_get, execute=execute_owned):
    require(os.name == "posix", "The installed Skill collector requires the actual POSIX consumer host")
    source, skill, docs = (Path(value).resolve() for value in (source, skill, docs))
    require(re.fullmatch(r"[0-9a-f]{40}", implementation), "Expected immutable Skill/implementation SHA")
    require(skill != source / "skills/cherri-shortcuts",
            "Run the installed/packaged Skill, not the repository copy")
    store = Artifacts(output)
    started = time.monotonic()
    manifest_bytes = regular_bytes(skill / "compatibility-manifest.json")
    pins = load_json(manifest_bytes)
    validate_pins(pins)
    verify_checkout(source, implementation, COMPILER_ORIGIN)
    verify_checkout(docs, pins["docsCommit"], DOCS_ORIGIN)
    source_files, skill_files = [], []
    before = {}
    for name, artifact in zip(SOURCE_PATHS, SOURCE_ARTIFACTS):
        data = regular_bytes(docs / name)
        require(data == git(docs, "show", pins["docsCommit"] + ":" + name), "Pinned docs bytes differ")
        sha = store.write(artifact, data)
        source_files.append({"path": name, "sha256": sha})
        before["docs/" + name] = sha
    for name, artifact in zip(SKILL_PATHS, SKILL_ARTIFACTS):
        data = regular_bytes(skill / name)
        require(data == git(source, "show", implementation + ":skills/cherri-shortcuts/" + name),
                "Installed Skill bytes differ from the immutable source object")
        sha = store.write(artifact, data)
        skill_files.append({"path": name, "sha256": sha})
        before["skill/" + name] = sha
    build_bytes, controls_bytes, deployment_bytes = map(regular_bytes, (site_build, site_controls, deployment))
    build, controls, deploy = map(load_json, (build_bytes, controls_bytes, deployment_bytes))
    validate_site_build(build, controls, deploy, pins, regular_bytes(docs / "fork-language/actions.md"))
    digests = {
        "siteBuildSHA256": store.write("documentation-site-build.json", build_bytes),
        "siteControlsSHA256": store.write("documentation-site-controls.json", controls_bytes),
        "deploymentSHA256": store.write("documentation-site-deployment.json", deployment_bytes),
    }
    rows = []
    with tempfile.TemporaryDirectory(prefix="cherri-d03-consumer-") as temporary:
        home = Path(temporary) / "home"
        home.mkdir()
        for name, arguments in (
                ("active-empty", []), ("active-show", ["show"]),
                ("legacy-empty", ["--legacy-reference"]),
                ("legacy-query", ["--legacy-reference", "action"]),
                ("absent-query", [ABSENT_QUERY]), ("invalid-regex", ["["])):
            rows.append(inspect_search(name, arguments, skill, docs, home, store, execute))
        broken = Path(temporary) / "missing-current-docs"
        (broken / "language").mkdir(parents=True)
        (broken / "language/control.md").write_text(MISSING_QUERY + "\n")
        rows.append(inspect_search("missing-current", [MISSING_QUERY], skill, broken, home, store, execute))
        rows.append(inspect_search("restored-active", ["show"], skill, docs, home, store, execute))
    observations = []
    for route, artifact in ROUTES:
        body, headers = fetch(route)
        expected_type = "application/json" if route.endswith(".json") else "application/javascript" if route.endswith(".js") else "text/html"
        content_type = headers.get("Content-Type", "").split(";", 1)[0].strip()
        require(content_type == expected_type or
                (expected_type == "application/javascript" and content_type == "text/javascript"),
                "Published route has the wrong observed content type")
        sha = store.write(artifact, body)
        observations.append({"route": route, "url": SITE + route, "status": 200,
                             "bytes": len(body), "sha256": sha, "artifact": artifact, "headers": headers})
        if route == "fork-language/actions":
            require(sha == build["renderedActionPageSha256"], "Live action page differs from pinned deployment build")
        if route == "assets/js/search-data.json":
            require(sha == build["searchIndexSha256"], "Live search bytes differ from pinned deployment build")
            validate_search_index(body, build["searchEntries"])
    after = {**{"docs/" + name: digest(regular_bytes(docs / name)) for name in SOURCE_PATHS},
             **{"skill/" + name: digest(regular_bytes(skill / name)) for name in SKILL_PATHS}}
    require(after == before, "Installed Skill or pinned docs changed during read-only verification")
    verify_checkout(source, implementation, COMPILER_ORIGIN)
    verify_checkout(docs, pins["docsCommit"], DOCS_ORIGIN)
    report = {"formatVersion": 1, "mode": "actual-installed-public-site",
              "implementationSha": implementation,
              **{key: pins[key] for key in ("compilerCommit", "docsCommit", "schemaFingerprint",
                                            "codecFingerprint", "macroFingerprint")},
              "sourceOrigin": COMPILER_ORIGIN, "docsOrigin": DOCS_ORIGIN,
              "sourceClean": True, "docsClean": True,
              "sourceDirectory": str(source), "installedSkill": str(skill), "docsDirectory": str(docs),
              "sourceFiles": source_files, "skillFiles": skill_files,
              "searchCases": rows, "httpObservations": observations,
              "before": before, "after": after, **digests,
              "observedAtUTC": datetime.now(timezone.utc).isoformat(),
              "elapsedSeconds": time.monotonic() - started}
    store.write("documentation-consumers.json", (json.dumps(report, indent=2) + "\n").encode())
    record = {"evidence_id": "ev-d03-published-installed-documentation",
              "requirements": ["D03"], "tier": "docs", "implementation_sha": implementation,
              "test_id": "tools/p10r15:verify_documentation_consumers",
              "command": ["python3", "tools/p10r15/verify_documentation_consumers.py",
                          "--source", str(source), "--skill", str(skill), "--docs", str(docs),
                          "--implementation-sha", implementation, "--site-build", str(site_build),
                          "--site-controls", str(site_controls), "--deployment", str(deployment),
                          "--output", str(output)],
              "exit_code": 0, "result": "passed",
              "assertions": {"published_current_docs": "true", "installed_skill_current_docs": "true",
                             "historical_docs_explicit_only": "true"},
              "artifacts": store.members,
              "detail": "Actual installed pinned Skill searches and live public bytes equal the pinned docs deployment; app integration is a separate required producer."}
    (store.directory / "documentation-consumer-evidence.json").write_text(
        json.dumps({"records": [record]}, indent=2) + "\n")
    return report, record


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for flag in ("source", "skill", "docs", "site-build", "site-controls", "deployment", "output"):
        parser.add_argument("--" + flag, type=Path, required=True)
    parser.add_argument("--implementation-sha", required=True)
    args = parser.parse_args()
    try:
        report, record = collect(args.source, args.skill, args.docs, args.implementation_sha,
                                 args.site_build, args.site_controls, args.deployment, args.output)
    except (ValueError, OSError, subprocess.SubprocessError) as error:
        failure = {"status": "FAILED", "scope": "No D03 pass was produced", "error": str(error)}
        if args.output.is_dir():
            target = args.output / "documentation-collection-failure.json"
            if not target.exists():
                target.write_text(json.dumps(failure, indent=2) + "\n")
        print(json.dumps(failure))
        raise SystemExit(1)
    print(json.dumps({"status": "PASS_DOCS_TIER_ONLY", "implementationSha": report["implementationSha"],
                      "docsCommit": report["docsCommit"], "searchCases": len(report["searchCases"]),
                      "httpObservations": len(report["httpObservations"]), "appIntegration": "NOT_EXECUTED"}))


if __name__ == "__main__":
    main()
