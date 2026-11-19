"""Execute source selection and callback rejection for the two public bodies."""
import base64
import hashlib
import json
import os
from pathlib import Path
import subprocess
import unittest

import binding_defaults_probe as probe
import test_runtime_callback_contract as callback_contract
from test_workflow_input_contract import bash_path

EXPECTED = {
    "CherriBindingCapturePOC": "CHERRI_BINDING_CAPTURE_OK\nNUM_CAPTURED=1\nNUM_AGAIN=1\nCLIP_CAPTURED=clip_initial\nCLIP_AGAIN=clip_initial",
    "CherriTypedDefaultsPOC": "CHERRI_TYPED_DEFAULTS_OK\nDEFAULTS_EXPLICIT=0:0::0:0\nDEFAULTS_OMITTED=1:9:def:1:1\nDEFAULTS_MIXED=0:9:def:0:1",
}
ORIGINAL_HASHES = {
    "CherriBindingCapturePOC": "4c649f8d53ed164b73a0324b7941999e5c9e849d28670655fafc13c87cee5558",
    "CherriTypedDefaultsPOC": "94165361845193c63fda7740a6cac826b68dea2d829b870c7e87f2f37f8f8063",
}


class BindingDefaultsProbeTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.root = Path(__file__).resolve().parents[2]
        cls.shell = (cls.root / "scripts/ios27_shortcuts_simulator_qa.sh").read_text(encoding="utf-8")
        cls.bash = bash_path()
        callback_contract.CallbackContractTests.setUpClass()
        cls.oracle = callback_contract.CallbackContractTests()

    def test_original_bodies_and_names_are_preserved_with_no_local_runtime_credit(self):
        contract = json.loads((self.root / "docs/apple-runtime/p10r15/binding-defaults-native-contract.json").read_text(encoding="utf-8"))
        self.assertIs(contract["formalAcceptanceGranted"], False)
        self.assertEqual({c["fixtureId"] for c in contract["cases"]}, set(EXPECTED))
        for fid, expected in EXPECTED.items():
            with self.subTest(fixture=fid):
                original = (self.root / "tests/runtime_poc" / (fid + ".cherri")).read_bytes()
                self.assertEqual(hashlib.sha256(original).hexdigest(), ORIGINAL_HASHES[fid])
                actual = (self.root / "tools/shortcuts-simulator-qa/fixtures" / probe.FOLDER / (fid + ".cherri")).read_bytes()
                self.assertEqual(actual.count(original), 1)
                observed = probe.verify_adapter_source(self.root, fid, actual)
                self.assertIs(observed["originalBodyByteIdentical"], True)
                self.assertEqual(observed["appleRuntime"], "NOT_RUN")
                self.assertEqual(probe.CASES[fid]["clipboard"], expected)
                self.assertEqual(base64.b64decode(probe.expected_summary(fid).split(":",1)[1]).decode("utf-8"), expected)
                row = next(c for c in contract["cases"] if c["fixtureId"] == fid)
                self.assertEqual(row["expectedClipboard"], expected)
                self.assertEqual(row["adapterSourceSha256"], hashlib.sha256(actual).hexdigest())
                for damaged in (actual.replace(original, original+b"comment(\"changed\")\n",1),
                                actual.replace(b"getClipboard()",b"getVariable(Clipboard)",1),
                                actual.replace(b"< 513",b"< 999999",1)):
                    with self.assertRaisesRegex(ValueError,"BINDING_DEFAULTS_SOURCE_CONTRACT"):
                        probe.verify_adapter_source(self.root,fid,damaged)

    def test_actual_selector_binds_exact_nested_source_and_shortcut_name(self):
        if not self.bash:
            self.skipTest("bash unavailable; actual selection was not run")
        selector = self.shell.split("# Select primary target fixture from SAMPLE_ID\n",1)[1].split("LEGACY_COUNTERPART_SOURCE=",1)[0]
        selector += '\nprintf "%s\\n" "$TARGET_FIXTURE_ID" "$TARGET_SOURCE_FILE" "$EXPECTED_SHORTCUT_NAME" "$EXPECTED_SEMANTIC_OUTPUT"\n'
        validation = self.shell[:self.shell.index('ROOT_DIR="')]
        for fid,case in probe.CASES.items():
            with self.subTest(fixture=fid):
                result=subprocess.run([self.bash,"-s"],input=selector,capture_output=True,text=True,timeout=10,
                    env={**os.environ,"ROOT_DIR":"/reviewed","SAMPLE_ID":case["sample"],"TARGET_SOURCE_RELATIVE":"../../private.cherri"})
                self.assertEqual(result.returncode,0,result.stderr)
                self.assertEqual(result.stdout.splitlines(),[fid,"/reviewed/tools/shortcuts-simulator-qa/fixtures/"+probe.FOLDER+"/"+fid+".cherri",fid,probe.expected_summary(fid)])
                for sample,code in [(case["sample"],0),(case["sample"]+"-private",2),(case["sample"]+";exit 0",2)]:
                    result=subprocess.run([self.bash,"-c",validation,"qa-validation","runtime","ipad-default",sample,"false","1","true"],
                                          capture_output=True,text=True,timeout=10)
                    self.assertEqual(result.returncode,code,result.stderr)
        # The workflow bundle expands to two fixed jobs; the raw shell must not
        # silently map the bundle name to the default functional fixture.
        result=subprocess.run([self.bash,"-c",validation,"qa-validation","runtime","ipad-default","qa-binding-defaults","false","1","true"],
                              capture_output=True,text=True,timeout=10)
        self.assertEqual(result.returncode,2)

    def test_exact_clipboard_bytes_and_all_zero_counts_are_required(self):
        for fid,expected in EXPECTED.items():
            summary=probe.expected_summary(fid)
            event=self.oracle.event()
            event.update(fixtureId=fid,expectedShortcutName=fid,resultSummary=summary)
            event["tracePayload"]=event["tracePayload"].replace("qa-unit",fid)
            good=self.oracle.verify([event],fid,summary)
            self.assertEqual((good["runGate"],good["traceGate"]),("PASS","PASS"))
            payloads=[expected+"\n",expected.replace("_OK","_FAIL"),"",expected.replace("clip_initial","clip_overwritten")]
            if fid=="CherriTypedDefaultsPOC":
                payloads += [expected.replace("DEFAULTS_EXPLICIT=0:0::0:0","DEFAULTS_EXPLICIT=0:0::1:0"),
                             expected.replace("DEFAULTS_EXPLICIT=0:0::0:0","DEFAULTS_EXPLICIT=0:0::0:1"),
                             expected.replace("DEFAULTS_MIXED=0:9:def:0:1","DEFAULTS_MIXED=0:9:def:1:1"),
                             expected.replace("DEFAULTS_OMITTED=1:9:def:1:1","DEFAULTS_OMITTED=0:0::0:0")]
            else:
                payloads += [expected.replace("NUM_CAPTURED=1","NUM_CAPTURED=2"),expected.replace("NUM_AGAIN=1","NUM_AGAIN=2")]
            for wrong in set(payloads)-{expected}:
                observed=probe.CASES[fid]["prefix"]+base64.b64encode(wrong.encode("utf-8")).decode("ascii")
                self.assertEqual(self.oracle.verify([{**event,"resultSummary":observed}],fid,summary)["runGate"],"FAIL_RUNTIME")
            for wrong in ("OVERSIZE","invalid-base64"):
                self.assertEqual(self.oracle.verify([{**event,"resultSummary":probe.CASES[fid]["prefix"]+wrong}],fid,summary)["runGate"],"FAIL_RUNTIME")
            self.assertEqual(self.oracle.verify([{**event,"tracePayload":""}],fid,summary)["traceGate"],"FAIL_RUNTIME")
            self.assertEqual(self.oracle.verify([{**event,"runId":"stale"}],fid,summary)["runGate"],"FAIL_RUNTIME")


if __name__=="__main__":
    unittest.main()
