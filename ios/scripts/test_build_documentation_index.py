import hashlib
import json
from pathlib import Path
import tempfile
import unittest

import build_documentation_index as docs


class DocumentationIndexTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / "docs"
        self.root.mkdir()
        self.git("init", "-q")
        self.git("config", "user.name", "Documentation test")
        self.git("config", "user.email", "test@example.invalid")
        self.git("config", "core.autocrlf", "false")
        self.git("remote", "add", "origin", docs.DOCS_REPO)
        self.manifest = {"languageVersion": "2.0", "docsRepo": docs.DOCS_REPO,
                         "compilerCommit": "a" * 40, "schemaFingerprint": "b" * 64,
                         "macroFingerprint": "c" * 64}
        for path in docs.SOURCES:
            self.write(path, "---\ntitle: Current documentation\n---\n# Current section\n\nCurrent V2 prose.\n")
        self.write("fork-language/guide.md", self.read("fork-language/guide.md") +
                   '\n## Unicode & code\n\n```cherri\nlet שלום = "👩‍💻"\n// # This is code\nshow(שלום)\n```\n')
        self.write("fork-language/actions.md", self.read("fork-language/actions.md") +
                   "\n**Schema Fingerprint:** `" + "b" * 64 + "`\n" +
                   "**Compiler Macro Fingerprint:** `" + "c" * 64 + "`\n" +
                   "\n### `show`\n\nShow Result.\n\n```cherri\nshow(input: Any)\n```\n")
        self.write("fork-language/migration.md", self.read("fork-language/migration.md") +
                   '\n```cherri-legacy\n@old = "Migration only"\n```\n')
        self.write("language/obsolete.md", "# OBSOLETE_ONLY_SENTINEL\nLegacy page.")
        self.commit()

    def git(self, *args):
        return docs.git(self.root, *args)

    def write(self, path, text):
        target = self.root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(text.encode("utf-8"))

    def read(self, path):
        return (self.root / path).read_bytes().decode("utf-8")

    def commit(self):
        self.git("add", ".")
        self.git("commit", "-qm", "Synthetic documentation fixture")
        self.manifest["docsCommit"] = self.git("rev-parse", "HEAD").decode().strip()

    def build(self):
        return docs.build_index(self.root, self.manifest)

    def testExactCodeAndSourceBinding(self):
        index = self.build()
        section = next(row for row in index["sections"] if row["title"] == "Unicode & code")
        self.assertEqual(section["blocks"][-1]["text"],
                         'let שלום = "👩‍💻"\n// # This is code\nshow(שלום)')
        self.assertEqual({r["path"] for r in index["sourceFiles"]}, set(docs.SOURCES))
        for row in index["sourceFiles"]:
            self.assertEqual(row["sha256"], hashlib.sha256((self.root / row["path"]).read_bytes()).hexdigest())
        self.assertEqual(index["docsCommit"], self.manifest["docsCommit"])

    def testHistoricalDocumentsNeverEnterCurrentIndex(self):
        self.assertNotIn("OBSOLETE_ONLY_SENTINEL", json.dumps(self.build()))

    def testDeterministicOutput(self):
        self.assertEqual(self.build(), self.build())

    def testCanonicalSourceHelperUsesExactScopeAndStableOrder(self):
        sources = {path: self.git("show", "HEAD:" + path) for path in docs.SOURCES}
        expected = self.build()
        reordered = dict(reversed(list(sources.items())))
        self.assertEqual(docs.build_index_from_sources(reordered, self.manifest), expected)
        missing = dict(sources)
        missing.pop(docs.SOURCES[-1])
        historical = dict(sources, **{"language/obsolete.md": b"Historical"})
        for damaged in (missing, historical):
            with self.subTest(keys=list(damaged)):
                with self.assertRaisesRegex(ValueError, "source scope mismatch"):
                    docs.build_index_from_sources(damaged, self.manifest)
        nonbytes = dict(sources)
        nonbytes[docs.SOURCES[0]] = nonbytes[docs.SOURCES[0]].decode("utf-8")
        with self.assertRaisesRegex(ValueError, "exact bytes"):
            docs.build_index_from_sources(nonbytes, self.manifest)

    def testDirtyTrackedSourceRejected(self):
        self.write("fork-language/guide.md", self.read("fork-language/guide.md") + "\nTampered")
        with self.assertRaisesRegex(ValueError, "dirty"):
            self.build()

    def testUntrackedSourceRejectedAndNeverResetByFetch(self):
        self.write("canary.txt", "preserve this unrelated work")
        docs.fetch_docs(self.root, self.manifest)
        with self.assertRaisesRegex(ValueError, "dirty"):
            self.build()
        self.assertEqual(self.read("canary.txt"), "preserve this unrelated work")

    def testWrongRevisionRejected(self):
        self.manifest["docsCommit"] = "d" * 40
        with self.assertRaisesRegex(ValueError, "revision mismatch"):
            self.build()

    def testWrongOriginRejected(self):
        self.git("remote", "set-url", "origin", "https://example.invalid/docs.git")
        with self.assertRaisesRegex(ValueError, "origin mismatch"):
            self.build()

    def testSchemaMismatchRejectedEvenInCleanCommit(self):
        self.write("fork-language/actions.md", self.read("fork-language/actions.md").replace("b" * 64, "e" * 64))
        self.commit()
        with self.assertRaisesRegex(ValueError, "Schema Fingerprint mismatch"):
            self.build()

    def testMacroMismatchRejectedEvenInCleanCommit(self):
        self.write("fork-language/actions.md", self.read("fork-language/actions.md").replace("c" * 64, "e" * 64))
        self.commit()
        with self.assertRaisesRegex(ValueError, "Macro Fingerprint mismatch"):
            self.build()

    def testBorrowedObjectStoreRejected(self):
        alternate = self.root / ".git/objects/info/alternates"
        alternate.write_bytes(b"/missing/borrowed/objects\n")
        with self.assertRaisesRegex(ValueError, "Borrowed"):
            self.build()

    def testMigrationExamplesExplicitlyMarked(self):
        sections = self.build()["sections"]
        migration = [s for s in sections if s["migrationOnly"]]
        self.assertTrue(migration)
        self.assertTrue(all(s["sourcePath"] == "fork-language/migration.md" for s in migration))
        self.assertFalse(any(block["language"] == "cherri-legacy"
                             for s in sections if not s["migrationOnly"] for block in s["blocks"]))

    def testLegacyCodeOutsideMigrationRejected(self):
        self.write("fork-language/guide.md", self.read("fork-language/guide.md").replace("```cherri", "```cherri-legacy"))
        self.commit()
        with self.assertRaisesRegex(ValueError, "Legacy example outside"):
            self.build()

    def testLegacyFenceInfoCannotBypassMigrationPolicy(self):
        original = self.read("fork-language/guide.md")
        for info in ["cherri-legacy title=example", "ChErRi-LeGaCy", "cherri-legacy{.example}"]:
            with self.subTest(info=info):
                self.write("fork-language/guide.md", original.replace("```cherri", "```" + info))
                self.commit()
                with self.assertRaisesRegex(ValueError, "Legacy example outside|Unsupported code-fence"):
                    self.build()

    def testUnterminatedFenceRejected(self):
        self.write("fork-language/guide.md", self.read("fork-language/guide.md") + "\n```cherri\nbroken\n")
        self.commit()
        with self.assertRaisesRegex(ValueError, "Unterminated code fence"):
            self.build()

    def testDuplicateManifestKeysRejected(self):
        path = Path(self.temp.name) / "manifest.json"
        path.write_bytes(b'{"docsCommit":"first","docsCommit":"second"}')
        with self.assertRaisesRegex(ValueError, "Duplicate JSON key"):
            docs.read_manifest(path)

    def testNativeBuildRejectsStaleOrDamagedGeneratedResource(self):
        raw = (json.dumps(self.build(), ensure_ascii=False, indent=2) + "\n").encode("utf-8")
        output, receipt = Path(self.temp.name) / "index.json", Path(self.temp.name) / "receipt.json"
        output.write_bytes(raw)
        receipt.write_bytes(json.dumps({"sha256": hashlib.sha256(raw).hexdigest(), "bytes": len(raw),
                                        "docsCommit": self.manifest["docsCommit"],
                                        "compilerCommit": self.manifest["compilerCommit"],
                                        "sourceFiles": 4, "sections": len(self.build()["sections"])}).encode())
        self.assertEqual(docs.verify_output(output, receipt, self.manifest)["status"],
                         "VERIFIED_BUNDLED_DOCUMENTATION")
        changed = dict(self.manifest, compilerCommit="f" * 40)
        with self.assertRaisesRegex(ValueError, "Stale generated documentation"):
            docs.verify_output(output, receipt, changed)
        output.write_bytes(raw.replace(b"Current V2 prose", b"Changed V2 prose"))
        with self.assertRaisesRegex(ValueError, "digest mismatch"):
            docs.verify_output(output, receipt, self.manifest)
        output.write_bytes(raw)
        bad_receipt = json.loads(receipt.read_bytes())
        bad_receipt["sections"] += 1
        receipt.write_bytes(json.dumps(bad_receipt).encode())
        with self.assertRaisesRegex(ValueError, "metadata mismatch"):
            docs.verify_output(output, receipt, self.manifest)

    def testFloatingManifestRevisionRejected(self):
        path = Path(self.temp.name) / "manifest.json"
        self.manifest["docsCommit"] = "main"
        path.write_bytes(json.dumps(self.manifest).encode())
        with self.assertRaisesRegex(ValueError, "non-immutable docsCommit"):
            docs.read_manifest(path)


if __name__ == "__main__":
    unittest.main()
