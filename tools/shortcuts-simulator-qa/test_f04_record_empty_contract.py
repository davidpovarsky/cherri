"""Run the real selector/callback gates for the bounded Dictionary feasibility probe."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import f04_record_empty_probe as probe
import test_runtime_callback_contract as callback
import test_workflow_input_contract as workflow

class RecordEmptyContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        callback.CallbackContractTests.setUpClass()
        workflow.WorkflowInputContractTests.setUpClass()
        cls.callback=callback.CallbackContractTests()
        cls.workflow=workflow.WorkflowInputContractTests()
        cls.root=Path(__file__).resolve().parents[2]

    def test_reviewed_source_and_original_f04_failures_are_preserved(self):
        fixtures=self.root/"tools/shortcuts-simulator-qa/fixtures"
        raw=(fixtures/(probe.FIXTURE_ID+".cherri")).read_bytes()
        result=probe.verify_source(self.root,probe.FIXTURE_ID,raw)
        self.assertFalse(result["formalF04Acceptance"])
        self.assertEqual(result["appleRuntime"],"NOT_RUN")
        with self.assertRaisesRegex(ValueError,"F04_RECORD_EMPTY_SOURCE_CONTRACT"):
            probe.verify_source(self.root,probe.FIXTURE_ID,raw.replace(b'"items": []',b'"items": ["changed"]'))
        with tempfile.TemporaryDirectory() as directory:
            other=Path(directory);folder=other/"tools/shortcuts-simulator-qa/fixtures";folder.mkdir(parents=True)
            for name in probe.ORIGINALS:(folder/name).write_bytes((fixtures/name).read_bytes())
            first=folder/next(iter(probe.ORIGINALS));first.write_bytes(first.read_bytes()+b"// altered original\n")
            with self.assertRaisesRegex(ValueError,"F04_ORIGINAL_ORACLE_CHANGED"):
                probe.verify_source(other,probe.FIXTURE_ID,raw)

    def test_exact_selector_and_unchanged_original_oracles(self):
        script=(self.root/"scripts/ios27_shortcuts_simulator_qa.sh").read_text(encoding="utf-8")
        selector=script.split("# Select primary target fixture from SAMPLE_ID\n",1)[1].split("LEGACY_COUNTERPART_SOURCE=",1)[0]
        selector+='\nprintf "%s\\n" "$TARGET_FIXTURE_ID" "$TARGET_SOURCE_FILE" "$EXPECTED_SHORTCUT_NAME" "$EXPECTED_SEMANTIC_OUTPUT"\n'
        cases={
            probe.SAMPLE_ID:(probe.FIXTURE_ID,probe.EXPECTED),
            "qa-f04-yield-list":("qa-f04-yield-list-v2","CHERRI_QA_F04_LIST_RESULT:pairs=2:scalars=2:shape=LEFT,TAIL|RIGHT,TAIL"),
            "qa-f04-yield-empty-list":("qa-f04-yield-empty-list-v2","CHERRI_QA_F04_EMPTY_LIST_RESULT:outer=2:scalars=2:shape=0,0"),
        }
        for sample,(fid,expected) in cases.items():
            with self.subTest(sample=sample):
                self.assertEqual(self.workflow.validate(["runtime","ipad-default",sample,"false","1","true"]).returncode,0)
                p=subprocess.run([self.workflow.bash,"-s"],input=selector,env={**os.environ,"SAMPLE_ID":sample,"ROOT_DIR":"/reviewed",
                    "TARGET_SOURCE_RELATIVE":"../../private.cherri"},capture_output=True,text=True,timeout=10)
                self.assertEqual(p.returncode,0,p.stderr)
                self.assertEqual(p.stdout.splitlines(),[fid,"/reviewed/tools/shortcuts-simulator-qa/fixtures/"+fid+".cherri",fid,expected])
        for sample in ("qa-f04-record-empty-v2","qa-f04-record-empty;exit 0","qa-f04-record-empty-private"):
            self.assertEqual(self.workflow.validate(["runtime","ipad-default",sample,"false","1","true"]).returncode,2)

    def test_actual_counts_field_presence_tags_and_trace_required(self):
        event=self.callback.event()
        event.update(fixtureId=probe.FIXTURE_ID,expectedShortcutName=probe.FIXTURE_ID,resultSummary=probe.EXPECTED)
        event["tracePayload"]=event["tracePayload"].replace("qa-unit",probe.FIXTURE_ID)
        proof=self.callback.verify([event],probe.FIXTURE_ID,probe.EXPECTED)
        self.assertEqual((proof["runGate"],proof["traceGate"]),("PASS","PASS"))
        for old,new in (("outer=2","outer=0"),("outer=2","outer=4"),("scalars=2","scalars=1"),
                        ("tags=LEFT,RIGHT","tags=RIGHT,LEFT"),("keys=2,2","keys=1,1"),
                        ("items=0,0","items=1,0"),("empty=0","empty=1"),("dictionary=Y","dictionary=N")):
            with self.subTest(mutation=new):
                altered={**event,"resultSummary":probe.EXPECTED.replace(old,new)}
                self.assertEqual(self.callback.verify([altered],probe.FIXTURE_ID,probe.EXPECTED)["runGate"],"FAIL_RUNTIME")
        for altered in ({**event,"runId":"stale"},{**event,"status":"ERROR"}):
            self.assertEqual(self.callback.verify([altered],probe.FIXTURE_ID,probe.EXPECTED)["runGate"],"FAIL_RUNTIME")
        self.assertEqual(self.callback.verify([{**event,"tracePayload":""}],probe.FIXTURE_ID,probe.EXPECTED)["traceGate"],"FAIL_RUNTIME")

if __name__=="__main__":unittest.main()
