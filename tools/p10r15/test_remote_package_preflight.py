"""Exercise the real committed source-package path at the promotion boundary.

Only unrelated deployment/report prerequisites are synthetic here. Package
creation, committed-file reads, archive validation and the package-kind contract
use the production implementation. This does not authorize a live deployment.
"""
import copy
from contextlib import ExitStack
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import package_skill
import promote_remote_release as promotion


class SourcePackagePreflight(unittest.TestCase):
    def test_real_source_archive_reaches_required_runtime_boundary(self):
        repo = Path(os.environ.get("CHERRI_PROMOTION_PACKAGE_REPO",
                                   str(Path(__file__).resolve().parents[2])))
        commit = subprocess.check_output(
            ["git", "-C", str(repo), "rev-parse", "HEAD"], text=True).strip()
        source_files, source_receipt = package_skill.expected_package(repo, commit, "source")
        self.assertEqual(len(source_files), 20)
        self.assertIsNone(source_receipt["runtime"])
        self.assertNotIn(package_skill.PREFIX + "bin/cherri", source_files)

        with tempfile.TemporaryDirectory(prefix="p15-package-preflight-") as temporary:
            root = Path(temporary)
            stage, live = root / "stage", root / "live"
            candidate = stage / "remote-home"
            label = "unit-package-boundary"
            release = {"label": label, "skillSourceHead": commit}
            for home in (live, candidate):
                for relative in promotion.PROTECTED:
                    target = home / relative
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_bytes(b"legacy\n" if relative == promotion.PROTECTED[4]
                                       else b"synthetic protected unit fixture\n")
                (home / promotion.LINK).symlink_to(promotion.LINK_TARGET)
            (live / "tools").mkdir()
            (candidate / "tools").symlink_to(live / "tools", target_is_directory=True)
            skill = candidate / "source" / label
            skill.parent.mkdir(parents=True)
            skill.symlink_to(repo.resolve(), target_is_directory=True)
            (candidate / "active.json").write_text(json.dumps(release))
            archive = root / "source.zip"
            package_skill.create_archive(archive, source_files)
            reports = {
                "stage": {"tuple": release,
                          "liveHashesBefore": {p: promotion.sha((live / p).read_bytes())
                                               for p in promotion.PROTECTED},
                          "candidateHashes": {p: promotion.sha((candidate / p).read_bytes())
                                              for p in promotion.PROTECTED}},
                "sourceIntegrity": {"archiveSha256": promotion.sha(archive.read_bytes()),
                                    "runtime": None},
            }
            paths = {"sourceArchive": archive, "runtimeArchive": root / "runtime.zip"}
            with ExitStack() as stack:
                stack.enter_context(patch.object(
                    promotion, "load_evidence", return_value=({}, paths, reports)))
                stack.enter_context(patch.object(promotion, "validate_reports"))
                stack.enter_context(patch.object(promotion, "verify_runtime"))
                checked = stack.enter_context(patch.object(
                    promotion, "verify_archive", wraps=package_skill.verify_archive))

                # The absent runtime is an intentional next boundary: source
                # validation must finish first, without a compiler argument.
                with self.subTest("exact_source_then_missing_runtime"):
                    with self.assertRaises(FileNotFoundError) as failure:
                        promotion.preflight(stage, live, root / "unused-unit-evidence")
                    self.assertEqual(Path(failure.exception.filename),
                                     candidate / label / "runtime/bin/cherri")
                    self.assertEqual(checked.call_count, 1)
                    self.assertEqual(checked.call_args.args[0], archive)
                    self.assertEqual(checked.call_args.args[1], source_files)

                with self.subTest("source_runtime_receipt_rejected"):
                    reports["sourceIntegrity"]["runtime"] = {"compilerCommit": "0" * 40}
                    with self.assertRaisesRegex(ValueError, "Binary package provenance differs"):
                        promotion.preflight(stage, live, root / "unused-unit-evidence")
                    reports["sourceIntegrity"]["runtime"] = None

                with self.subTest("rehash_cannot_hide_changed_source_bytes"):
                    changed = copy.deepcopy(source_files)
                    name = package_skill.PREFIX + "SKILL.md"
                    changed[name] = (changed[name][0] + b"\ncausal mutation\n", changed[name][1])
                    archive.unlink()
                    package_skill.create_archive(archive, changed)
                    reports["sourceIntegrity"]["archiveSha256"] = promotion.sha(archive.read_bytes())
                    with self.assertRaisesRegex(ValueError, "archive (size/mode|bytes) mismatch"):
                        promotion.preflight(stage, live, root / "unused-unit-evidence")


if __name__ == "__main__":
    unittest.main(verbosity=2)
