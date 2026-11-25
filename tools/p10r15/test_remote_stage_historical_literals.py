"""The historical wrapper path must retain the reviewed base shell literal bytes."""
import ast
import hashlib
import os
from pathlib import Path
import unittest

BASE_SOURCE = "450bc9a93c327181c06d1cdbc89b68f563d1959e"
BASE_STAGE_SHA256 = "ccc2c1533516f687acbe6e9ced86537c721d2d4105826c537c19cce9bf23e132"
EXPECTED = {'sync': 'f87509e377eb820873ecec4023e639eac2f11a36f78c769f3eb1814d157f3f90', 'exact_preflight': 'f3c9db946bd72b42072328ea6ff560c35cbd48c7cc49e0c280ade861645820d8', 'exact_signing': '9162cb2010cef13cbb9c730eed983622f9fa21e14c9e3b25dd898261f0c694a9', 'exact_output': 'b58c76df22c572fdc5ac6f03c413b895931a11e4f8ed9d4cd5a3476229c4d88e'}
def shell_literals(text):
    result = {}
    for node in ast.walk(ast.parse(text)):
        if not isinstance(node, ast.Assign) or len(node.targets) != 1:
            continue
        target = node.targets[0]
        if not isinstance(target, ast.Name) or target.id not in EXPECTED:
            continue
        value = node.value
        if (not isinstance(value, ast.Call) or not isinstance(value.func, ast.Attribute)
                or value.func.attr != "replace" or not isinstance(value.func.value, ast.Constant)
                or [ast.literal_eval(x) for x in value.args] != ["__DOLLAR__", "$"]
                or target.id in result):
            raise ValueError("historical shell literal AST changed")
        result[target.id] = value.func.value.value.replace("__DOLLAR__", "$")
    if set(result) != set(EXPECTED):
        raise ValueError("historical shell literal inventory changed")
    return result

class HistoricalRendererLiteralTests(unittest.TestCase):
    def test_exact_historical_shell_bytes(self):
        path = Path(os.environ.get("P15_HISTORICAL_STAGE_SOURCE",
                    str(Path(__file__).with_name("stage_remote_release.py"))))
        actual = shell_literals(path.read_text())
        for name, expected in EXPECTED.items():
            with self.subTest(name=name):
                self.assertEqual(hashlib.sha256(actual[name].encode()).hexdigest(), expected)

    def test_added_indentation_is_not_equivalent(self):
        path = Path(os.environ.get("P15_HISTORICAL_STAGE_SOURCE",
                    str(Path(__file__).with_name("stage_remote_release.py"))))
        actual = shell_literals(path.read_text())
        for name, value in actual.items():
            with self.subTest(name=name):
                mutated = value.replace("\n", "\n    ", 1)
                self.assertNotEqual(hashlib.sha256(mutated.encode()).hexdigest(), EXPECTED[name])

if __name__ == "__main__":
    unittest.main()
