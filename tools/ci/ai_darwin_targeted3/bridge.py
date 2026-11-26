import json, sys, unittest
root, required = sys.argv[1], json.loads(sys.argv[2])
sys.path.insert(0, root)
suite = unittest.defaultTestLoader.loadTestsFromNames(required)
def flatten(tests):
    for test in tests:
        if isinstance(test, unittest.TestSuite):
            yield from flatten(test)
        else:
            yield test.id()
found = sorted(flatten(suite))
if found != sorted(required):
    print(json.dumps({"found": found, "selection_complete": False}))
    raise SystemExit(2)
class ObservedResult(unittest.TextTestResult):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.passed_names = []
    def addSuccess(self, test):
        self.passed_names.append(test.id())
        super().addSuccess(test)
result = unittest.TextTestRunner(stream=sys.stderr, verbosity=2, resultclass=ObservedResult).run(suite)
receipt = {"selection_complete": True, "found": found, "passed": sorted(result.passed_names),
           "tests_run": result.testsRun, "failures": len(result.failures), "errors": len(result.errors),
           "skipped": len(result.skipped), "expected_failures": len(result.expectedFailures),
           "unexpected_successes": len(result.unexpectedSuccesses)}
print(json.dumps(receipt))
raise SystemExit(0 if result.wasSuccessful() else 1)
