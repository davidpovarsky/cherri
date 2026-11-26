"""Source-derived package-count controls; no Go, compiler or live deployment."""
import copy
from pathlib import Path
import subprocess
import unittest

import package_skill
import remote_release_gates as gates
from test_remote_promotion import reports_model


class CommittedPackageInventory(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.repo = Path(__file__).resolve().parents[2]
        cls.commit = subprocess.check_output(
            ["git", "-C", str(cls.repo), "rev-parse", "HEAD"], text=True).strip()
        cls.files, cls.receipt = package_skill.expected_package(cls.repo, cls.commit, "source")
        cls.count = len(cls.files)

    def current_model(self):
        records = reports_model()
        records["sourceIntegrity"]["verifiedFiles"] = self.count
        records["runtimeIntegrity"]["verifiedFiles"] = self.count + 6
        records["unpackedIntegrity"]["verifiedFiles"] = self.count + 6
        return records

    def validate(self, records, count=None):
        provenance = records["faults"]["provenance"]
        return gates.validate_reports(
            records, records["stage"]["candidateHome"],
            provenance["skillManifest"], provenance["skillBuildSha256"],
            source_package_count=self.count if count is None else count)

    def test_current_committed_inventory_is_not_the_historical_fixed_count(self):
        self.assertIn("cherri-shortcuts/references/language-boundaries.md", self.files)
        self.assertEqual(set(self.files), {
            package_skill.PREFIX + item["path"].removeprefix(package_skill.PREFIX)
            for item in self.receipt["files"]} | {package_skill.RECEIPT})
        self.assertEqual(self.receipt["skillCommit"], self.commit)
        self.validate(self.current_model())

    def test_old_current_or_extra_receipt_counts_are_rejected(self):
        original = self.current_model()
        for name in ("sourceIntegrity", "runtimeIntegrity", "unpackedIntegrity"):
            for delta in (-1, 1):
                with self.subTest(report=name, delta=delta):
                    changed = copy.deepcopy(original)
                    changed[name]["verifiedFiles"] += delta
                    with self.assertRaisesRegex(ValueError, "package integrity|Unpacked receipt"):
                        self.validate(changed)

    def test_recounting_all_untrusted_reports_cannot_change_expected_inventory(self):
        records = self.current_model()
        for name in ("sourceIntegrity", "runtimeIntegrity", "unpackedIntegrity"):
            records[name]["verifiedFiles"] += 1
        with self.assertRaisesRegex(ValueError, "package integrity"):
            self.validate(records)

    def test_expected_count_must_be_explicit_integral_and_source_bound(self):
        records = self.current_model()
        for count in (True, float(self.count), str(self.count), 0, -1):
            with self.subTest(count=count):
                with self.assertRaisesRegex(ValueError, "source package inventory"):
                    self.validate(records, count=count)
        with self.assertRaisesRegex(ValueError, "package integrity"):
            self.validate(records, count=self.count + 1)


if __name__ == "__main__":
    unittest.main(verbosity=2)
