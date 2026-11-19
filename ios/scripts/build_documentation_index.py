#!/usr/bin/env python3
"""Bundle only the manifest-pinned fork documentation for offline app search."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess

DOCS_REPO = "https://github.com/davidpovarsky/cherrilang.org.git"
SOURCES = ("fork-language/index.md", "fork-language/guide.md",
           "fork-language/actions.md", "fork-language/migration.md")


def require(ok, message):
    if not ok:
        raise ValueError(message)


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args],
                                   stderr=subprocess.PIPE, timeout=120)


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "Duplicate JSON key: " + key)
        result[key] = value
    return result


def read_manifest(path):
    manifest = json.loads(Path(path).read_bytes(), object_pairs_hook=unique_object)
    require(manifest.get("docsRepo") == DOCS_REPO, "Unexpected docs repository")
    for key, size in (("docsCommit", 40), ("compilerCommit", 40),
                      ("schemaFingerprint", 64), ("macroFingerprint", 64)):
        require(isinstance(manifest.get(key), str) and
                re.fullmatch("[0-9a-f]{%d}" % size, manifest[key]),
                "Missing or non-immutable " + key)
    require(manifest.get("languageVersion") == "2.0", "Unsupported language version")
    return manifest


def fetch_docs(root, manifest):
    # Never reset a caller's existing checkout.
    if root.exists():
        return
    root.mkdir(parents=True)
    git(root, "init", "-q")
    git(root, "config", "core.autocrlf", "false")
    git(root, "remote", "add", "origin", manifest["docsRepo"])
    git(root, "fetch", "--depth=1", "origin", manifest["docsCommit"])
    git(root, "checkout", "--detach", "-q", "FETCH_HEAD")


def verified_sources(root, manifest):
    require((root / ".git").is_dir(), "Documentation must use an ordinary checkout")
    require(not (root / ".git/objects/info/alternates").exists(), "Borrowed docs object store")
    require(git(root, "remote", "get-url", "origin").decode().strip() == manifest["docsRepo"],
            "Documentation origin mismatch")
    require(git(root, "rev-parse", "HEAD").decode().strip() == manifest["docsCommit"],
            "Documentation revision mismatch")
    require(not git(root, "status", "--porcelain", "--untracked-files=all").strip(),
            "Documentation checkout is dirty")
    sources = {}
    for path in SOURCES:
        local = root / path
        require(not local.is_symlink() and not local.parent.is_symlink(),
                "Documentation source must not be a symlink")
        blob = git(root, "show", "HEAD:" + path)
        require(local.read_bytes() == blob, "Documentation bytes differ from pinned Git blob: " + path)
        sources[path] = blob
    return sources


def display_prose(text, commit):
    # Kramdown presentation directives have no meaning in the native reader.
    text = "\n".join(line for line in text.splitlines()
                     if not re.fullmatch(r"\s*\{:[^}]*}\s*", line))
    def pinned_link(match):
        route = match.group(1).removeprefix("/").rstrip("/")
        route = "fork-language/index" if route == "fork-language" else route
        require(route + ".md" in SOURCES, "Unsupported relative documentation route")
        return "https://github.com/davidpovarsky/cherrilang.org/blob/" + commit + "/" + route + ".md"
    text = re.sub(r"\{\{\s*['\"](/fork-language/[^'\"]*)['\"]\s*\|\s*relative_url\s*}}",
                  pinned_link, text)
    return text.strip()


def sections_for(path, raw, commit):
    lines = raw.decode("utf-8").splitlines()
    require(lines and lines[0] == "---", "Missing documentation front matter")
    end = next((i for i in range(1, len(lines)) if lines[i] == "---"), None)
    require(end is not None, "Unterminated documentation front matter")
    title = next((line[6:].strip().strip("\"'") for line in lines[1:end]
                  if line.startswith("title:")), None)
    require(title, "Documentation title missing")
    migration = path == "fork-language/migration.md"
    output, blocks, pending = [], [], []
    heading, section_line = title, end + 2
    fence, language, code = None, "", []

    def flush_prose():
        if pending:
            text = display_prose("\n".join(pending), commit)
            if text and text != "---":
                blocks.append({"kind": "prose", "language": "", "text": text})
            pending.clear()

    def flush_section():
        flush_prose()
        if blocks and not re.search(r"Table of (contents|Modules)", heading, re.I):
            output.append({"id": path + ":" + str(section_line), "sourcePath": path,
                           "sourceLine": section_line, "documentTitle": title,
                           "title": heading.strip("`"), "migrationOnly": migration,
                           "blocks": list(blocks)})
        blocks.clear()

    for number, line in enumerate(lines[end + 1:], end + 2):
        if fence:
            if re.fullmatch(r"\s*" + re.escape(fence[0]) + "{" + str(len(fence)) + r",}\s*", line):
                blocks.append({"kind": "code", "language": language, "text": "\n".join(code)})
                fence, language, code = None, "", []
            else:
                code.append(line)
            continue
        match = re.match(r"^\s*(`{3,}|~{3,})(.*)$", line)
        if match:
            flush_prose()
            fence = match.group(1)
            info = match.group(2).strip().split()
            language = info[0].lower() if info else ""
            require(re.fullmatch(r"[a-z0-9_+-]*", language) is not None,
                    "Unsupported code-fence language token")
            require(language != "cherri-legacy" or migration, "Legacy example outside migration guide")
            continue
        match = re.match(r"^#{1,6}\s+(.+?)\s*#*\s*$", line)
        if match:
            flush_section()
            heading, section_line = match.group(1), number
        else:
            pending.append(line)
    require(fence is None, "Unterminated code fence in " + path)
    flush_section()
    require(output, "Empty documentation source: " + path)
    return output


def build_index_from_sources(sources, manifest):
    """Build canonical content from four verified Git blobs, without filesystem I/O."""
    require(isinstance(sources, dict) and set(sources) == set(SOURCES),
            "Documentation source scope mismatch")
    require(all(isinstance(sources[path], bytes) for path in SOURCES),
            "Documentation sources must be exact bytes")
    sources = {path: sources[path] for path in SOURCES}
    actions = sources["fork-language/actions.md"].decode("utf-8")
    for label, key in (("Schema Fingerprint", "schemaFingerprint"),
                       ("Compiler Macro Fingerprint", "macroFingerprint")):
        observed = re.findall(r"\*\*" + label + r":\*\*\s*`([a-f0-9]{64})`", actions)
        require(observed == [manifest[key]], "Documentation " + label + " mismatch")
    index = {"formatVersion": 1, "languageVersion": manifest["languageVersion"],
             **{key: manifest[key] for key in ("docsCommit", "compilerCommit",
                                               "schemaFingerprint", "macroFingerprint")},
             "sourceFiles": [{"path": path, "sha256": hashlib.sha256(raw).hexdigest()}
                             for path, raw in sources.items()],
             "sections": [section for path, raw in sources.items()
                          for section in sections_for(path, raw, manifest["docsCommit"])]}
    require(len({section["id"] for section in index["sections"]}) == len(index["sections"]),
            "Duplicate documentation section")
    return index


def build_index(root, manifest):
    return build_index_from_sources(verified_sources(Path(root), manifest), manifest)


def verify_output(output, receipt_path, manifest):
    raw = Path(output).read_bytes()
    index = json.loads(raw, object_pairs_hook=unique_object)
    receipt = json.loads(Path(receipt_path).read_bytes(), object_pairs_hook=unique_object)
    require(receipt.get("sha256") == hashlib.sha256(raw).hexdigest() and
            receipt.get("bytes") == len(raw), "Generated documentation digest mismatch")
    for key in ("docsCommit", "compilerCommit", "schemaFingerprint", "macroFingerprint", "languageVersion"):
        require(index.get(key) == manifest[key], "Stale generated documentation: " + key)
    require(index.get("formatVersion") == 1 and isinstance(index.get("sections"), list),
            "Invalid generated documentation schema")
    require(receipt.get("docsCommit") == index["docsCommit"] and
            receipt.get("compilerCommit") == index["compilerCommit"] and
            receipt.get("sections") == len(index["sections"]) and
            receipt.get("sourceFiles") == len(SOURCES), "Generation receipt metadata mismatch")
    sources = index.get("sourceFiles", [])
    require(len(sources) == len(SOURCES) and {row["path"] for row in sources} == set(SOURCES),
            "Generated documentation source scope mismatch")
    return {"status": "VERIFIED_BUNDLED_DOCUMENTATION", "docsCommit": index["docsCommit"],
            "compilerCommit": index["compilerCommit"], "sha256": receipt["sha256"]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, default=Path(__file__).resolve().parents[2] /
                        "skills/cherri-shortcuts/compatibility-manifest.json")
    parser.add_argument("--docs", type=Path)
    parser.add_argument("--fetch", action="store_true",
                        help="Create a new ordinary checkout at the exact manifest revision")
    parser.add_argument("--output", type=Path, default=Path(__file__).resolve().parents[1] /
                        "CherriApp/Resources/Documentation/documentation-index.json")
    parser.add_argument("--verify-output", action="store_true",
                        help="Reject stale or damaged generated resources before a native build")
    parser.add_argument("--receipt", type=Path)
    args = parser.parse_args()
    manifest = read_manifest(args.manifest)
    if args.verify_output:
        require(args.receipt is not None, "--verify-output requires --receipt")
        print(json.dumps(verify_output(args.output, args.receipt, manifest)))
        return
    require(args.docs is not None, "Generation requires --docs")
    if args.fetch:
        fetch_docs(args.docs, manifest)
    index = build_index(args.docs, manifest)
    payload = (json.dumps(index, ensure_ascii=False, indent=2) + "\n").encode("utf-8")
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_bytes(payload)
    print(json.dumps({"docsCommit": index["docsCommit"], "compilerCommit": index["compilerCommit"],
                      "sourceFiles": len(index["sourceFiles"]), "sections": len(index["sections"]),
                      "bytes": len(payload), "sha256": hashlib.sha256(payload).hexdigest()}))


if __name__ == "__main__":
    main()
