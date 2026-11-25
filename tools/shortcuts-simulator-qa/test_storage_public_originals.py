"""Original transport controls. Synthetic signing data grants no Apple credit."""
import copy
import json
from pathlib import Path
import plistlib
import shutil
import subprocess
import tempfile
import unittest

import prepare_public_artifacts as publication
import storage_public_originals as storage

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]


class StoragePublicOriginalTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name).resolve()
        self.repo, self.raw, self.public = (self.base / n for n in ("repo", "raw", "public"))
        for p in (self.repo, self.raw, self.public):
            p.mkdir()
        for relative in (storage.SOURCE_PATH, storage.MANIFEST_PATH):
            p = self.repo / relative
            p.parent.mkdir(parents=True, exist_ok=True)
            p.write_bytes((ROOT / relative).read_bytes())
        self.git("init", "-q")
        self.git("add", "--all")
        self.git("-c", "user.name=Synthetic Transport Control",
                 "-c", "user.email=control@example.invalid",
                 "-c", "commit.gpgsign=false", "commit", "-qm", "fixed public fixtures")
        self.commit = self.git("rev-parse", "HEAD").decode().strip()
        self.run_id, self.attempt = "123456", "1"
        source = (ROOT / storage.SOURCE_PATH).read_bytes()
        unsigned = (HERE / "testdata/storage-delete-compiled.original.plist").read_bytes()
        signed = b"AEA1SYNTHETIC_SIGNING_CONTROL_NOT_AN_APPLE_SIGNATURE"
        for name, raw in zip(storage.original_paths(), (source, unsigned, signed)):
            p = self.raw / name
            p.parent.mkdir(parents=True, exist_ok=True)
            p.write_bytes(raw)
        self.row = {
            "fixtureId": storage.FIXTURE, "fileName": storage.FIXTURE + ".cherri",
            "dialect": "v2", "buildProfile": "debug",
            "runtimeTraceExpectation": "REQUIRED_NDJSON",
            "sourceSha256": storage.SOURCE_SHA256,
        }
        self.manifest = {"syntheticOnly": True, "compilerCommitSha": self.commit,
                         "fixtures": [self.row]}
        self.signing = {"compilerCommitSha": self.commit,
                        "signedArtifactValidAEA1": True,
                        "signedArtifactSha256": storage.sha(signed),
                        "signedByteSize": len(signed), "targetProvenance": {}}
        self.refresh_unsigned()
        self.refresh_public()

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.repo), *args],
                                       stderr=subprocess.PIPE, timeout=10)

    def refresh_unsigned(self):
        names = storage.original_paths()
        raw = (self.raw / names[1]).read_bytes()
        actions = plistlib.loads(raw)["WFWorkflowActions"]
        self.row.update(unsignedPlistSha256=storage.sha(raw),
                        nativeActionCount=len(actions),
                        actionIdentifiers=[a["WFWorkflowActionIdentifier"] for a in actions])
        self.signing["targetProvenance"] = {
            "compilerCommitSha": self.commit, "sourceSha256": storage.SOURCE_SHA256,
            "unsignedPlistSha256": storage.sha(raw), "unsignedByteSize": len(raw),
            "nativeActionCount": len(actions),
            "signedArtifactSha256": self.signing["signedArtifactSha256"],
            "signedByteSize": self.signing["signedByteSize"], "headerMagic": "AEA1",
        }

    def refresh_public(self):
        values = {"compiler-manifest.json": self.manifest,
                  "runtime/signing-provenance.json": self.signing,
                  "summary.json": {"runId": self.run_id, "compilerCommitSha": self.commit,
                                   "privacyGatePassed": True,
                                   "scope": "SYNTHETIC_TRANSPORT_CONTROL_ONLY"}}
        rows = []
        for name, value in values.items():
            raw = storage.canonical(value)
            for root in (self.raw, self.public):
                p = root / name
                p.parent.mkdir(parents=True, exist_ok=True)
                p.write_bytes(raw)
            rows.append(storage.pin(name, raw))
        manifest = {
            "schemaVersion": "1.5", "syntheticOnly": True,
            "publicationGatePassed": True, "rasterPublicationAllowed": False,
            "policy": "EXPLICIT_SYNTHETIC_TEXT_PATHS_ONLY", "published": rows,
        }
        (self.public / "publication-manifest.json").write_bytes(storage.canonical(manifest))

    def stage(self, output=None, fixture=storage.FIXTURE, commit=None):
        return storage.stage(self.repo, self.raw, self.public,
                             output or self.base / "staged", fixture,
                             self.commit if commit is None else commit, self.run_id, self.attempt)

    def assert_blocked(self, code):
        with self.assertRaisesRegex(publication.PublicationBlocked, code):
            self.stage()
        self.assertFalse((self.base / "staged").exists())

    def test_original_bytes_and_closed_bundle_without_native_credit(self):
        value = self.stage()
        root = self.base / "staged"
        names = {p.relative_to(root).as_posix() for p in root.rglob("*") if p.is_file()}
        self.assertEqual(names, set(storage.original_paths()) | {storage.RECEIPT})
        for name in storage.original_paths():
            self.assertEqual((root/name).read_bytes(), (self.raw/name).read_bytes())
        for key in ("appleSignatureAuthenticated", "appleExecutionAuthenticated",
                    "storageSemanticsVerified", "cacheAcceptanceGranted", "formalAcceptanceGranted"):
            self.assertIs(value[key], False)
        self.assertEqual(value["sourceSha256"], storage.SOURCE_SHA256)

    def test_fixed_source_commit_fixture_and_contract_guards(self):
        with self.assertRaisesRegex(publication.PublicationBlocked, "UNREVIEWED_FIXTURE"):
            self.stage(fixture="qa-storage-scope-v2")
        with self.assertRaisesRegex(publication.PublicationBlocked, "CHECKOUT_SOURCE"):
            self.stage(commit="f"*40)
        p = self.repo / storage.SOURCE_PATH
        p.write_bytes(p.read_bytes()+b"\n")
        self.assert_blocked("FIXED_SOURCE")

    def test_compiler_identity_and_typed_field_guards(self):
        original = copy.deepcopy(self.manifest)
        cases = (
            ("source", lambda: self.manifest.update(compilerCommitSha="a"*40), "COMPILER_IDENTITY"),
            ("duplicate", lambda: self.manifest["fixtures"].append(copy.deepcopy(self.row)), "FIXTURE_MANIFEST_INVALID"),
            ("profile", lambda: self.row.update(buildProfile="release"), "COMPILER_buildProfile"),
            ("bool_count", lambda: self.row.update(nativeActionCount=True), "COMPILER_nativeActionCount"),
            ("hash", lambda: self.row.update(unsignedPlistSha256="0"*64), "COMPILER_unsignedPlistSha256"),
        )
        for name, mutate, expected in cases:
            with self.subTest(name=name):
                mutate(); self.refresh_public(); self.assert_blocked(expected)
                self.manifest=copy.deepcopy(original); self.row=self.manifest["fixtures"][0]
        self.refresh_public()

    def test_exact_signature_hash_source_and_size_joins(self):
        original = copy.deepcopy(self.signing)
        for field, bad in (("sourceSha256", "0"*64), ("unsignedPlistSha256", "0"*64),
                           ("nativeActionCount", True), ("headerMagic", "FAKE"),
                           ("signedByteSize", 1), ("compilerCommitSha", "0"*40)):
            with self.subTest(field=field):
                self.signing["targetProvenance"][field]=bad
                self.refresh_public(); self.assert_blocked("SIGNING_"+field)
                self.signing=copy.deepcopy(original)
        self.signing["signedByteSize"]=True
        self.refresh_public(); self.assert_blocked("SIGNING_SUMMARY")

    def test_effect_scope_literal_and_order_cannot_be_rebound(self):
        p=self.raw/storage.original_paths()[1]; original=p.read_bytes()
        cases = ("foreign-key", "local-scope", "changed-literal", "missing-delete", "network")
        for name in cases:
            with self.subTest(name=name):
                doc=plistlib.loads(original); actions=doc["WFWorkflowActions"]
                stores=[a for a in actions if a["WFWorkflowActionIdentifier"].endswith("storedcontent")]
                if name=="foreign-key": stores[0]["WFWorkflowActionParameters"]["WFStoredContentKey"]="foreign"
                elif name=="local-scope": stores[0]["WFWorkflowActionParameters"]["WFStoredContentGlobalValue"]=False
                elif name=="changed-literal": stores[0]["WFWorkflowActionParameters"]["WFInput"]="changed"
                elif name=="missing-delete": actions.remove(stores[2])
                else: actions.append({"WFWorkflowActionIdentifier":"is.workflow.actions.downloadurl",
                                      "WFWorkflowActionParameters":{}})
                p.write_bytes(plistlib.dumps(doc));self.refresh_unsigned();self.refresh_public()
                self.assert_blocked("STORAGE_PUBLIC_")
                p.write_bytes(original);self.refresh_unsigned();self.refresh_public()

    def test_symlink_or_missing_original_is_never_published(self):
        for name in storage.original_paths():
            with self.subTest(name=name):
                p=self.raw/name;original=p.read_bytes();p.unlink()
                with self.assertRaises(publication.PublicationBlocked): self.stage()
                target=self.base/"outside-control";target.write_bytes(original);p.symlink_to(target)
                with self.assertRaises(publication.PublicationBlocked): self.stage()
                p.unlink();p.write_bytes(original);target.unlink()
                self.assertFalse((self.base/"staged").exists())

    def test_output_is_create_only_and_outside_all_inputs(self):
        self.stage()
        with self.assertRaisesRegex(publication.PublicationBlocked,"OUTPUT_EXISTS"):self.stage()
        for parent in (self.repo,self.raw,self.public):
            with self.subTest(parent=parent.name):
                with self.assertRaisesRegex(publication.PublicationBlocked,"OUTPUT_INSIDE_INPUT"):
                    self.stage(output=parent/"must-not-appear")
                self.assertFalse((parent/"must-not-appear").exists())

    def test_original_public_byte_join_and_secret_boundary(self):
        p=self.raw/"runtime/signing-provenance.json";p.write_bytes(p.read_bytes()+b"\n")
        self.assert_blocked("PUBLIC_ORIGINAL_JOIN")
        self.refresh_public()
        self.manifest["unrelated"]="BEGIN RSA PRIVATE KEY"
        self.refresh_public()
        with self.assertRaises(publication.PublicationBlocked):self.stage()
        self.assertFalse((self.base/"staged").exists())


if __name__=="__main__":
    unittest.main()
