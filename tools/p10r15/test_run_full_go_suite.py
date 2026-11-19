import contextlib
import io
from pathlib import Path
import subprocess
import unittest
from unittest.mock import Mock
import run_full_go_suite as suite

class FullGoSuiteIntegrityTests(unittest.TestCase):
    def setUp(self):
        self.packages = ["github.com/electrikmilk/cherri", suite.ACCEPTANCE_PACKAGE,
                         "github.com/electrikmilk/cherri/internal/language/schema"]

    def test_exact_inventory_and_ordinary_budget(self):
        plan = suite.execution_plan(self.packages)
        self.assertCountEqual(self.packages, [p for group in plan for p in group["packages"]])
        self.assertEqual(plan[0]["packages"], [self.packages[0], self.packages[2]])
        self.assertEqual(plan[0]["timeout"], "15m")
        self.assertEqual(plan[1]["packages"], [suite.ACCEPTANCE_PACKAGE])
        self.assertEqual(plan[1]["timeout"], "35m")
        for group in plan:
            self.assertEqual(group["argv"], ["go", "test", "-count=1", "-timeout=" + group["timeout"], "-v", *group["packages"]])

    def test_reject_absent_acceptance_package(self):
        with self.assertRaises(ValueError):
            suite.execution_plan(self.packages[:1])

    def test_reject_duplicate_package(self):
        with self.assertRaises(ValueError):
            suite.execution_plan(self.packages + [self.packages[0]])

    def test_reject_empty_or_malformed_inventory(self):
        for inventory in ([], [" "], [suite.ACCEPTANCE_PACKAGE], [*self.packages, " bad "]):
            with self.subTest(inventory=inventory), self.assertRaises(ValueError):
                suite.execution_plan(inventory)

    def test_both_groups_run_after_first_failure(self):
        invoke = Mock(side_effect=[subprocess.CompletedProcess([], 7), subprocess.CompletedProcess([], 0)])
        with contextlib.redirect_stdout(io.StringIO()):
            results = suite.execute_plan(suite.execution_plan(self.packages), Path("."), invoke)
        self.assertEqual(invoke.call_count, 2)
        self.assertEqual([r["exit_code"] for r in results], [7, 0])
        self.assertFalse(all(r["exit_code"] == 0 for r in results))

    def test_acceptance_failure_remains_failure(self):
        invoke = Mock(side_effect=[subprocess.CompletedProcess([], 0), subprocess.CompletedProcess([], 3)])
        with contextlib.redirect_stdout(io.StringIO()):
            results = suite.execute_plan(suite.execution_plan(self.packages), Path("."), invoke)
        self.assertEqual([r["exit_code"] for r in results], [0, 3])
        self.assertFalse(all(r["exit_code"] == 0 for r in results))

if __name__ == "__main__":
    unittest.main()
