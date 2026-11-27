"""One real metadata-deadline failure; no native, Go or acceptance execution."""
import importlib.util
import json
import os
from pathlib import Path
import stat
import sys
import tempfile
import unittest
from unittest.mock import patch

HERE=Path(__file__).resolve().parent
QA=Path(os.environ.get("CHERRI_SOURCE_BINDING_HELPER_ROOT",str(HERE)))
sys.path.insert(0,str(QA))
spec=importlib.util.spec_from_file_location("binding_under_control",os.environ.get("CHERRI_SOURCE_BINDING_CONTROL_SOURCE",str(HERE/"source_binding_observation.py")))
binding=importlib.util.module_from_spec(spec)
spec.loader.exec_module(binding)


class SourceBindingFailureReapTests(unittest.TestCase):
    def test_exhausted_metadata_keeps_failure_reap_inside_whole_budget(self):
        configured=os.environ.get("CHERRI_SOURCE_BINDING_CONTROL_ORIGINALS")
        if configured:
            root=Path(configured).resolve()
            root.mkdir(mode=0o700)
        else:
            temporary=tempfile.TemporaryDirectory(prefix="cherri-source-reap-")
            self.addCleanup(temporary.cleanup)
            root=Path(temporary.name).resolve()
        repo=root/"repo";repo.mkdir(mode=0o700)
        (repo/"fixture.txt").write_bytes(b"controlled source-read timeout\n")
        fake=root/"owned-blocked-git"
        fake.write_bytes(b"#!/bin/sh\nexec /bin/sleep 30\n")
        fake.chmod(0o700)
        output=root/"source-observation"
        held=[]
        original_kill=binding.bounded.kill_group
        def keep_owned_process(process):
            held.append(process)
            return original_kill(process)
        binding.bounded.kill_group=keep_owned_process
        try:
            self.assertEqual((binding.METADATA_SECONDS,binding.CLOSING_SECONDS,binding.WHOLE_SECONDS),(10,15,120))
            with patch.object(binding.shutil,"which",return_value=str(fake)):
                with self.assertRaises(ValueError) as rejected:
                    binding.source_inventory(repo,"1"*40,output,["fixture.txt"])
            result=json.loads((output/"result.original.json").read_bytes())
            process=json.loads((output/"identity/process.original.json").read_bytes())
            self.assertEqual(result["status"],"SOURCE_UNVERIFIED")
            self.assertFalse((output/"snapshot.original.json").exists())
            self.assertEqual(process["observation"]["result"],"TIMEOUT")
            self.assertEqual(len(held),1)
            actual=held[0]
            before={"pid":actual.pid,"returnCode":actual.returncode,
                    "group":binding.bounded.group_observation(actual.pid),
                    "rejection":str(rejected.exception)}
            (root/"before-rescue.original.json").write_text(json.dumps(before,sort_keys=True)+"\n")
            self.assertTrue(process["process"]["cleanupGroupSignal"]["signalSent"])
            self.assertTrue(process["process"]["cleanupParentReaped"],
                            "failed metadata command must reap its actual child after the unchanged cutoff")
            self.assertEqual(process["process"]["groupAfterCleanup"]["state"],"ABSENT")
            self.assertEqual(actual.returncode,-9)
            self.assertEqual(str(rejected.exception),"SOURCE_BINDING_PROCESS_identity")
            self.assertLessEqual(process["cleanupDeadlineMonotonic"]-process["executionDeadlineMonotonic"],2.000001)
            self.assertGreater(process["cleanupDeadlineMonotonic"],process["executionDeadlineMonotonic"])
        finally:
            binding.bounded.kill_group=original_kill
            rescue=[]
            for process in held:
                needed=process.returncode is None
                if needed:
                    original_kill(process)
                    process.wait(timeout=2)
                for pipe in (process.stdout,process.stderr):
                    if pipe is not None and not pipe.closed:
                        pipe.close()
                rescue.append({"pid":process.pid,"rescueNeeded":needed,
                               "parentReaped":process.returncode is not None,
                               "group":binding.bounded.group_observation(process.pid)})
            (root/"rescue.original.json").write_text(json.dumps(rescue,sort_keys=True)+"\n")


if __name__=="__main__":
    unittest.main()
