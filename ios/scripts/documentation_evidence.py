"""Validate the bounded D03 app receipt against explicitly supplied build artifacts.

This is an evidence adapter, not an Apple runner. Formal intake additionally
checks the immutable Git manifest and the successful source-bound workflow.
"""
import hashlib
import json
from pathlib import Path
import re

PREFIX = "[CHERRI_DOCS_RECEIPT] "
REJECTED_PREFIX = "[CHERRI_DOCS_REJECTED] "
CHECKPOINT = "[CHECKPOINT:D03_CURRENT_DOCS_SEARCH_VERIFIED]"
POSITIVE = "DocumentationIndexTests/testD03BundledCurrentDocumentationSearch"
NEGATIVE = "DocumentationIndexTests/testD03RejectsHistoricalAndDamagedBundleRows"
UI = "CherriEditorUITests/testD03CurrentDocumentationSearchUI"
TEST_IDS = (POSITIVE, NEGATIVE, UI)
SOURCE_PATHS = frozenset("fork-language/" + name + ".md" for name in
                         ("index", "guide", "actions", "migration"))
PINS = {"docsCommit": 40, "compilerCommit": 40,
        "schemaFingerprint": 64, "macroFingerprint": 64}
RECEIPT_KEYS = frozenset(("schemaVersion", "bundleSHA256", *PINS,
                         "sourcePaths", "sourceHashes", "sections",
                         "exactActionFirst", "defaultMigrationResults",
                         "explicitMigrationResults", "missingResults"))
EVENT = re.compile(
    r"^Test Case '-\[(\w+)\.(\w+) (test[A-Za-z0-9_]+)\]' "
    r"(started\.|(?:passed|failed) \([0-9.]+ seconds\)\.)$")


class EvidenceError(ValueError):
    """Only fixed error codes may reach public diagnostics."""


def require(condition, code):
    if not condition:
        raise EvidenceError(code)


def strict_json(data):
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result, "DUPLICATE_JSON_KEY")
            result[key] = value
        return result

    def invalid_constant(_):
        raise EvidenceError("NONFINITE_JSON_NUMBER")

    try:
        return json.loads(data, object_pairs_hook=pairs,
                          parse_constant=invalid_constant)
    except (ValueError, TypeError, UnicodeError) as error:
        if isinstance(error, EvidenceError):
            raise
        raise EvidenceError("INVALID_JSON") from None


def is_hex(value, size):
    return isinstance(value, str) and re.fullmatch("[0-9a-f]{%d}" % size, value) is not None


def source_map(paths, hashes):
    require(isinstance(paths, list) and isinstance(hashes, list)
            and len(paths) == len(hashes) == 4
            and all(isinstance(path, str) for path in paths)
            and set(paths) == SOURCE_PATHS
            and all(is_hex(value, 64) for value in hashes), "INVALID_SOURCE_PAIRS")
    return dict(zip(paths, hashes))


def parse_receipt(line):
    require(isinstance(line, str) and line.startswith(PREFIX)
            and len(line.encode("utf-8")) <= 4096, "INVALID_RECEIPT_SIZE")
    value = strict_json(line[len(PREFIX):])
    require(isinstance(value, dict) and set(value) == RECEIPT_KEYS,
            "INVALID_RECEIPT_KEYS")
    require(value["schemaVersion"] == "1.0"
            and all(is_hex(value[key], size) for key, size in PINS.items())
            and is_hex(value["bundleSHA256"], 64), "INVALID_RECEIPT_PINS")
    source_map(value["sourcePaths"], value["sourceHashes"])
    require(type(value["sections"]) is int and 500 < value["sections"] <= 10000,
            "INVALID_RECEIPT_SECTION_COUNT")
    require(value["exactActionFirst"] == "show"
            and type(value["defaultMigrationResults"]) is int
            and value["defaultMigrationResults"] == 0
            and type(value["missingResults"]) is int and value["missingResults"] == 0
            and type(value["explicitMigrationResults"]) is int
            and 0 < value["explicitMigrationResults"] <= value["sections"],
            "INVALID_RECEIPT_ASSERTIONS")
    return value


def receipt_from_log(text):
    require(REJECTED_PREFIX not in text, "REJECTED_DOCUMENTATION_EVENT")
    events = {test: [] for test in TEST_IDS}
    active = None
    receipts = []
    markers = 0
    for line in text.splitlines():
        match = EVENT.fullmatch(line.strip())
        if match:
            target, suite, method, result = match.groups()
            test = suite + "/" + method
            if test in TEST_IDS:
                expected_target = "CherriUITests" if test == UI else "CherriTests"
                require(target == expected_target, "WRONG_REQUIRED_TEST_TARGET")
            if result == "started.":
                require(active is None, "INTERLEAVED_TEST_BLOCKS")
                active = test
                status = "started"
            else:
                require(active == test, "UNMATCHED_TEST_BOUNDARY")
                active = None
                status = result.split(" ", 1)[0]
            if test in events:
                events[test].append(status)
        elif line.strip().startswith(PREFIX):
            require(active == POSITIVE, "RECEIPT_OUTSIDE_POSITIVE_TEST")
            receipts.append(parse_receipt(line.strip()))
        elif line.strip() == CHECKPOINT:
            require(active == UI, "CHECKPOINT_OUTSIDE_UI_TEST")
            markers += 1
    require(active is None, "INCOMPLETE_TEST_BLOCK")
    require(all(events[test] == ["started", "passed"] for test in TEST_IDS),
            "REQUIRED_TESTS_NOT_UNIQUELY_PASSED")
    require(len(receipts) == 1, "RECEIPT_NOT_UNIQUE")
    require(markers == 1, "CHECKPOINT_NOT_UNIQUE")
    return receipts[0]


def read_artifact(path, limit, code):
    require(path is not None, "MISSING_" + code)
    path = Path(path)
    try:
        require(path.is_file() and not path.is_symlink(), "INVALID_" + code)
        require(0 < path.stat().st_size <= limit, "INVALID_" + code + "_SIZE")
        data = path.read_bytes()
    except OSError:
        raise EvidenceError("UNREADABLE_" + code) from None
    require(0 < len(data) <= limit, "INVALID_" + code + "_SIZE")
    return data, strict_json(data)


def load_artifacts(index_path, manifest_path, generation_path=None):
    index_bytes, index = read_artifact(index_path, 2_000_000, "INDEX")
    manifest_bytes, manifest = read_artifact(manifest_path, 65536, "MANIFEST")
    require(isinstance(manifest, dict) and manifest.get("languageVersion") == "2.0"
            and manifest.get("docsRepo") == "https://github.com/davidpovarsky/cherrilang.org.git"
            and manifest.get("compilerRepo") == "https://github.com/davidpovarsky/cherri.git"
            and all(is_hex(manifest.get(key), size) for key, size in PINS.items()),
            "INVALID_MANIFEST")
    require(isinstance(index, dict) and type(index.get("formatVersion")) is int
            and index["formatVersion"] == 1 and index.get("languageVersion") == "2.0"
            and all(index.get(key) == manifest[key] for key in PINS),
            "INDEX_MANIFEST_MISMATCH")
    sources = index.get("sourceFiles")
    require(isinstance(sources, list) and len(sources) == 4
            and all(isinstance(row, dict) and set(row) == {"path", "sha256"}
                    for row in sources), "INVALID_INDEX_SOURCES")
    source_map([row["path"] for row in sources], [row["sha256"] for row in sources])
    sections = index.get("sections")
    require(isinstance(sections, list) and 500 < len(sections) <= 10000,
            "INVALID_INDEX_SECTION_COUNT")
    ids = set()
    for row in sections:
        require(isinstance(row, dict) and isinstance(row.get("sourcePath"), str)
                and row["sourcePath"] in SOURCE_PATHS
                and type(row.get("sourceLine")) is int and 0 < row["sourceLine"] <= 1000000
                and isinstance(row.get("id"), str)
                and row["id"] == row["sourcePath"] + ":" + str(row["sourceLine"])
                and row["id"] not in ids
                and type(row.get("migrationOnly")) is bool
                and row["migrationOnly"] == (row["sourcePath"] == "fork-language/migration.md")
                and all(isinstance(row.get(key), str) and 0 < len(row[key]) <= 1024
                        for key in ("title", "documentTitle")), "INVALID_INDEX_SECTION")
        ids.add(row["id"])
        blocks = row.get("blocks")
        require(isinstance(blocks, list) and 0 < len(blocks) <= 10000,
                "INVALID_INDEX_BLOCKS")
        for block in blocks:
            require(isinstance(block, dict) and set(block) == {"kind", "language", "text"}
                    and block["kind"] in ("code", "prose")
                    and isinstance(block["language"], str) and len(block["language"]) <= 64
                    and re.fullmatch(r"[a-z0-9_+-]*", block["language"]) is not None
                    and isinstance(block["text"], str) and len(block["text"]) <= 250000,
                    "INVALID_INDEX_BLOCK")
            language = block["language"].split(None, 1)[0].lower() if block["language"] else ""
            require(language != "cherri-legacy" or row["migrationOnly"],
                    "HISTORICAL_CODE_OUTSIDE_MIGRATION")
    output = {"documentation-index.json": index_bytes,
              "documentation-compatibility-manifest.json": manifest_bytes}
    if generation_path is not None:
        generation_bytes, generation = read_artifact(generation_path, 16384, "GENERATION_RECEIPT")
        require(isinstance(generation, dict)
                and set(generation) == {"docsCommit", "compilerCommit", "sourceFiles", "sections", "bytes", "sha256"}
                and all(generation.get(key) == index[key] for key in ("docsCommit", "compilerCommit"))
                and all(type(generation.get(key)) is int for key in ("sourceFiles", "sections", "bytes"))
                and generation["sourceFiles"] == 4
                and generation["sections"] == len(sections)
                and generation["bytes"] == len(index_bytes)
                and generation["sha256"] == hashlib.sha256(index_bytes).hexdigest(),
                "GENERATION_RECEIPT_MISMATCH")
        output["documentation-receipt.json"] = generation_bytes
    return index, output


def bind_receipt(receipt, index, index_bytes):
    require(all(receipt[key] == index[key] for key in PINS),
            "RECEIPT_INDEX_PINS_MISMATCH")
    sources = {row["path"]: row["sha256"] for row in index["sourceFiles"]}
    require(source_map(receipt["sourcePaths"], receipt["sourceHashes"]) == sources,
            "RECEIPT_INDEX_SOURCES_MISMATCH")
    require(receipt["bundleSHA256"] == hashlib.sha256(index_bytes).hexdigest(),
            "RECEIPT_INDEX_BYTES_MISMATCH")
    require(receipt["sections"] == len(index["sections"])
            and receipt["explicitMigrationResults"] == sum(row["migrationOnly"] for row in index["sections"]),
            "RECEIPT_INDEX_COUNTS_MISMATCH")
