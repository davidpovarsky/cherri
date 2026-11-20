import copy
import json
import unittest
import editor_position_probe as probe

class PositionProbeIntakeTests(unittest.TestCase):
    def rows(self):
        # Parser controls only; not execution evidence.
        return [{"event":"control","name":name,"result":"PASS","scope":"actual Foundation UTF16 helper; no UI","observation":{}} for name in probe.NAMES] + [{"event":"summary","controls":17,"failed":0,"uiExecuted":False}]
    def test_exact_parser_control(self):
        self.assertEqual(len(probe.inspect("\n".join(map(json.dumps,self.rows())).encode())),18)
    def test_missing_duplicate_foreign_or_unearned_rows(self):
        for name in ["missing","duplicate","foreign","invalid_result","ui","ui_integer","false_count","dropped_failure","extra"]:
            with self.subTest(name=name):
                rows=self.rows()
                if name=="missing":rows.pop(0)
                elif name=="duplicate":rows[1]=copy.deepcopy(rows[0])
                elif name=="foreign":rows[0]["name"]="other"
                elif name=="invalid_result":rows[0]["result"]="SKIP"
                elif name=="ui":rows[-1]["uiExecuted"]=True
                elif name=="ui_integer":rows[-1]["uiExecuted"]=0
                elif name=="false_count":rows[-1]["failed"]=False
                elif name=="dropped_failure":rows[0]["result"]="FAIL"
                elif name=="extra":rows.append(copy.deepcopy(rows[-1]))
                with self.assertRaises(ValueError):probe.inspect("\n".join(map(json.dumps,rows)).encode())
    def test_failed_outcome_preserved(self):
        rows=self.rows();rows[0]["result"]="FAIL";rows[-1]["failed"]=1
        actual=probe.inspect("\n".join(map(json.dumps,rows)).encode())
        self.assertEqual(actual[0]["result"],"FAIL")
        self.assertEqual(actual[-1]["failed"],1)

if __name__=="__main__":unittest.main()
