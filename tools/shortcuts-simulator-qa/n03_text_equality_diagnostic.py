"""Bind the public N03 diagnostic to its independently reviewed target contract.

The complete emitted graph is checked by the real compiler tests in
p15_n03_text_equality_diagnostic_test.go. This signer precondition binds the
unchanged public source and exact target bytes; it grants no native credit.
"""
import hashlib
import json
from pathlib import Path

FIXTURE_ID = "qa-n03-text-equality-diagnostic-v2"
SOURCE_PATH = "tools/shortcuts-simulator-qa/fixtures/" + FIXTURE_ID + ".cherri"
SOURCE_SHA256 = "e4acefccc8a81c77a8336fa7a5f07eacaf84fc25a6dc251eb8150c607fac80ab"
CONTRACT_SHA256 = "007b9eb4edaf6822e587cb72c753aa2b7a1b112810834ea6593df54a82dbb98c"
PREFIX = "CHERRI_QA_N03_TEXT_EQ_DIAG:"
EXPECTED_FIELDS = (
    ("LEFT64", "YWJj"), ("SAME64", "YWJj"), ("CASE64", "QUJD"),
    ("FOLD_SAME_EQ", "Y"), ("FOLD_SAME_NE", "N"),
    ("FOLD_CASE_EQ", "N"), ("FOLD_CASE_NE", "Y"),
    ("DYN_SAME_EQ", "Y"), ("DYN_SAME_NE", "N"),
    ("DYN_CASE_EQ", "N"), ("DYN_CASE_NE", "Y"),
    ("IF_SAME_EQ", "Y"), ("IF_SAME_NE", "N"),
    ("IF_CASE_EQ", "N"), ("IF_CASE_NE", "Y"),
    ("MATCH_SAME_CS", "1"), ("MATCH_CASE_CS", "0"), ("MATCH_CASE_CI", "1"),
)
EXPECTED_OUTPUT = PREFIX + ":".join(key + "=" + value for key, value in EXPECTED_FIELDS)


def verify_source(repo, fixture_id, source_bytes):
    if fixture_id != FIXTURE_ID:
        return None
    source = Path(repo) / SOURCE_PATH
    contract_path = source.with_suffix(".expected.json")
    if (source.is_symlink() or contract_path.is_symlink()
            or hashlib.sha256(source_bytes).hexdigest() != SOURCE_SHA256
            or source.read_bytes() != source_bytes):
        raise ValueError("N03 reviewed public source changed")
    data = contract_path.read_bytes()
    if hashlib.sha256(data).hexdigest() != CONTRACT_SHA256:
        raise ValueError("N03 independently reviewed target contract changed")
    contract = json.loads(data)
    if (contract["source_sha256"] != SOURCE_SHA256
            or contract["source_path"] != SOURCE_PATH
            or contract["fixture_id"] != FIXTURE_ID
            or contract["diagnostic_prefix"] != PREFIX
            or contract["field_order"] != [key for key, _ in EXPECTED_FIELDS]
            or contract["expected_fields_for_exact_equality"] != dict(EXPECTED_FIELDS)
            or contract["runtime_status"] != "NOT_RUN"
            or contract["formal_requirements_proved"] != []):
        raise ValueError("N03 source/contract identity or scope mismatch")
    return {"status": "PASS_REVIEWED_PUBLIC_SOURCE_ONLY",
            "sourceSha256": SOURCE_SHA256, "expectedContractSha256": CONTRACT_SHA256,
            "observedFields": len(EXPECTED_FIELDS), "appleRuntime": "NOT_RUN",
            "formalAcceptanceGranted": False}
