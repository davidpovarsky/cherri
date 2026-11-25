import contextlib
import io
import os
from pathlib import Path
import subprocess
import unittest
from unittest.mock import Mock, patch
import run_full_go_suite as suite

class FullGoSuiteIntegrityTests(unittest.TestCase):
    def setUp(self):
        self.packages = ["github.com/electrikmilk/cherri", suite.ACCEPTANCE_PACKAGE,
                         "github.com/electrikmilk/cherri/internal/language/schema"]

    def test_exact_inventory_and_ordinary_budget(self):
        plan = suite.execution_plan(self.packages)
        self.assertCountEqual(self.packages, [p for group in plan for p in group["packages"]])
        self.assertEqual(plan[0]["packages"], [self.packages[0], self.packages[2]])
        self.assertEqual(plan[0]["timeout"], "20m")
        self.assertEqual(plan[1]["packages"], [suite.ACCEPTANCE_PACKAGE])
        self.assertEqual(plan[1]["timeout"], "50m")
        for group in plan:
            parallel = ["-parallel=2"] if group is plan[0] else []
            self.assertEqual(group["argv"], ["go", "test", "-count=1", "-timeout=" + group["timeout"], *parallel, "-v", *group["packages"]])

    def test_reject_changed_or_ambiguous_cumulative_budget(self):
        for group_index, expected in ((0, "20m"), (1, "50m")):
            for mutation in ("historical", "longer", "unbounded", "absent", "duplicate",
                             "split_argument", "detached_metadata", "foreign_timeout_flag"):
                with self.subTest(group=group_index, mutation=mutation):
                    group = suite.execution_plan(self.packages)[group_index]
                    index = next(i for i, arg in enumerate(group["argv"]) if arg.startswith("-timeout="))
                    group["timeout"] = expected
                    group["argv"][index] = "-timeout=" + expected
                    if mutation == "historical":
                        value = "15m" if group_index == 0 else "35m"
                        group["timeout"] = value
                        group["argv"][index] = "-timeout=" + value
                    elif mutation == "longer":
                        group["timeout"] = "90m"
                        group["argv"][index] = "-timeout=90m"
                    elif mutation == "unbounded":
                        group["timeout"] = "0"
                        group["argv"][index] = "-timeout=0"
                    elif mutation == "absent":
                        del group["argv"][index]
                    elif mutation == "duplicate":
                        group["argv"].insert(index, "-timeout=" + expected)
                    elif mutation == "split_argument":
                        group["argv"][index:index + 1] = ["-timeout", expected]
                    elif mutation == "detached_metadata":
                        group["timeout"] = "1m"
                    elif mutation == "foreign_timeout_flag":
                        group["argv"].insert(index, "-test.timeout=0")
                    with self.assertRaisesRegex(ValueError, "cumulative package budget"):
                        suite.execution_environment(group)

    def test_invocation_environments_are_owned_and_acceptance_marker_is_removed(self):
        for inherited in (None, "2", "99"):
            with self.subTest(inherited=inherited):
                parent = {"PATH": "owned-path", "UNRELATED_CONTROL": "preserved"}
                if inherited is not None:
                    parent[suite.SUBPROCESS_PARALLEL_ENV] = inherited
                environments = []

                def invoke(argv, *, cwd, check, env):
                    self.assertEqual(dict(os.environ), parent)
                    self.assertEqual(env["UNRELATED_CONTROL"], "preserved")
                    self.assertEqual(env["PATH"], "owned-path")
                    environments.append(dict(env))
                    env["UNRELATED_CONTROL"] = "child-only-change"
                    return subprocess.CompletedProcess(argv, 0)

                with patch.dict(os.environ, parent, clear=True), contextlib.redirect_stdout(io.StringIO()):
                    suite.execute_plan(suite.execution_plan(self.packages), Path("."), invoke)
                    self.assertEqual(dict(os.environ), parent)
                self.assertEqual(environments[0][suite.SUBPROCESS_PARALLEL_ENV], "2")
                self.assertNotIn(suite.SUBPROCESS_PARALLEL_ENV, environments[1])

    def test_reject_invalid_ordinary_parallel_limits_before_execution(self):
        for limits in ([], ["-parallel=1"], ["-parallel=3"], ["-parallel=0"],
                       ["-parallel=2", "-parallel=2"], ["-parallel", "2"],
                       ["-test.parallel=2"]):
            with self.subTest(limits=limits):
                plan = suite.execution_plan(self.packages)
                index = plan[0]["argv"].index("-parallel=2")
                plan[0]["argv"][index:index + 1] = limits
                invoke = Mock()
                with contextlib.redirect_stdout(io.StringIO()), self.assertRaisesRegex(ValueError, "exactly -parallel=2"):
                    suite.execute_plan(plan, Path("."), invoke)
                invoke.assert_not_called()

    def test_reject_parallel_acceptance_or_mixed_package_group(self):
        plan = suite.execution_plan(self.packages)
        plan[1]["argv"].insert(2, "-parallel=2")
        with self.assertRaisesRegex(ValueError, "sequential producer"):
            suite.execution_environment(plan[1])
        plan = suite.execution_plan(self.packages)
        plan[0]["packages"].append(suite.ACCEPTANCE_PACKAGE)
        with self.assertRaisesRegex(ValueError, "exactly -parallel=2"):
            suite.execution_environment(plan[0])

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
