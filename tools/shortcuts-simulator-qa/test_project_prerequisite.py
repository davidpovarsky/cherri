"""Actual shell regressions for the receiver Xcode project prerequisite."""
import os
import hashlib
import json
from pathlib import Path
import plistlib
import shutil
import subprocess
import unittest
import test_terminal_receipts as original

class OriginalGeneratorShellTests(unittest.TestCase):
    def setUp(self):
        self.harness=original.SourceAndShellTests("test_actual_cli_uses_exact_owned_checkout_and_source_fixture_bytes")
        self.harness.setUp()
        self.addCleanup(self.harness.doCleanups)
        self.repo,self.root,self.home=self.harness.repo,self.harness.root,self.harness.home
        self.project=self.repo/"tools/shortcuts-simulator-qa/XcodeProject"
        self.project.mkdir(parents=True,exist_ok=True)
        info=self.project/"ReceiverApp/Info.plist";info.parent.mkdir()
        info.write_bytes(plistlib.dumps({"CFBundleURLTypes":[{"CFBundleURLSchemes":["cherriqa"]}]}))
        shutil.copyfile(original.REPO/"tools/shortcuts-simulator-qa/XcodeProject/project.yml",self.project/"project.yml")
        self.harness.git("add","tools")
        self.harness.git("-c","user.name=Cherri_QA","-c","user.email=qa@example.invalid",
                         "-c","commit.gpgsign=false","commit","-qm","owned project spec")
        self.harness.source=self.harness.git("--no-replace-objects","rev-parse","HEAD").strip()
        self.harness.manifest["compilerCommitSha"]=self.harness.source
        (self.root/"compiler-manifest.json").write_text(json.dumps(self.harness.manifest))
        self.bins=self.home/"generator-bin";self.bins.mkdir()
        self.bash=os.environ.get("CHERRI_TEST_BASH")or shutil.which("bash")
        if not self.bash:self.skipTest("Bash unavailable")
        self.script=original.SCRIPT.read_text(encoding="utf-8")
        start=self.script.index('    echo "=== Phase 3 & 4: Running XCTest UI Automation & Callback Receiver ==="')
        end=self.script.index('    # A GitHub rerun reuses RUN_ID.',start)
        self.block=self.script[start:end]
        self.marker=self.home/"xcodebuild-would-be-reached"
        git=Path(shutil.which("git")).parent
        self.env=dict(os.environ,PATH=str(self.bins)+os.pathsep+str(git),
            ROOT_DIR=self.repo.as_posix(),ARTIFACT_DIR=self.root.as_posix(),
            SOURCE_SHA=self.harness.source,RUN_ID=original.RUN,TARGET_FIXTURE_ID=original.FIXTURE,
            GENERATOR_CONTROL_MARKER=self.marker.as_posix())
        executable=self.bins/"python3"
        executable.write_text("#!/bin/bash\nexec '"+original.sys.executable.replace("\\","/")+"' \"$@\"\n")
        executable.chmod(0o755)

    def program(self,name,code):
        p=self.bins/name;p.write_text("#!/bin/bash\n"+code+"\n")
        p.chmod(0o755)

    def run_block(self):
        return subprocess.run([self.bash,"-c",self.block+
            '\nprintf REACHED > "$GENERATOR_CONTROL_MARKER"\n'],
            cwd=self.repo,env=self.env,capture_output=True,timeout=30)

    def test_failed_install_cannot_be_masked_by_existing_receiver_info(self):
        self.program("brew","printf ORIGINAL_INSTALL_FAILURE >&2\nexit 7")
        result=self.run_block()
        self.assertNotEqual(result.returncode,0,result.stdout.decode(errors="replace"))
        self.assertFalse(self.marker.exists())

    def test_failed_generator_cannot_be_masked_by_existing_receiver_info(self):
        self.program("xcodegen","printf ORIGINAL_GENERATOR_FAILURE >&2\nexit 11")
        result=self.run_block()
        self.assertNotEqual(result.returncode,0,result.stdout.decode(errors="replace"))
        self.assertFalse(self.marker.exists())

    def test_zero_generator_without_actual_project_is_rejected(self):
        self.program("xcodegen","printf NO_PROJECT_CREATED\nexit 0")
        result=self.run_block()
        self.assertNotEqual(result.returncode,0,result.stdout.decode(errors="replace"))
        self.assertFalse(self.marker.exists())


    def success_program(self):
        # The deliberately minimal PATH must not depend on host coreutils.
        # Reuse the explicitly pinned Python wrapper installed by setUp.
        self.program("xcodegen", "python3 - <<'PY'\nfrom pathlib import Path\n"
                     "project = Path('CherriSimulatorQA.xcodeproj')\n"
                     "project.mkdir()\n"
                     "(project / 'project.pbxproj').write_bytes(b'REVIEWED_PROJECT')\nPY")

    def receipt(self):
        return json.loads((self.root/"builds/project-prerequisite/receipt.json").read_bytes())

    def test_actual_fresh_project_and_scheme_continue(self):
        self.success_program()
        result=self.run_block()
        self.assertEqual(result.returncode,0,result.stdout.decode(errors="replace")+result.stderr.decode(errors="replace"))
        self.assertTrue(self.marker.exists())
        record=self.receipt()
        self.assertEqual(record["status"],"PREPARED")
        self.assertEqual(record["compilerCommitSha"],self.harness.source)
        self.assertEqual(record["projectSha256"],hashlib.sha256(b"REVIEWED_PROJECT").hexdigest())
        self.assertFalse(record["formalAcceptanceGranted"])

    def test_failed_generation_retains_exact_private_error_and_return_code(self):
        self.program("xcodegen","printf ORIGINAL_GENERATOR_FAILURE >&2\nexit 11")
        result=self.run_block()
        self.assertNotEqual(result.returncode,0)
        record=self.receipt()
        self.assertEqual(record["status"],"FAILED")
        self.assertEqual(record["commands"][-1]["exitCode"],11)
        raw=(self.root/"builds/project-prerequisite/generate.stderr").read_bytes()
        self.assertEqual(raw,b"ORIGINAL_GENERATOR_FAILURE")
        self.assertEqual(record["commands"][-1]["stderr"]["sha256"],hashlib.sha256(raw).hexdigest())
        self.assertNotIn(raw,result.stdout)

    def test_wrong_scheme_rejected_after_successful_generation(self):
        (self.project/"ReceiverApp/Info.plist").write_bytes(plistlib.dumps({}))
        self.success_program()
        result=self.run_block()
        self.assertNotEqual(result.returncode,0)
        self.assertFalse(self.marker.exists())
        self.assertEqual(self.receipt()["status"],"FAILED")

    def test_stale_project_rejected_without_accepting_existing_file(self):
        p=self.project/"CherriSimulatorQA.xcodeproj/project.pbxproj"
        p.parent.mkdir();p.write_bytes(b"STALE")
        self.program("xcodegen","exit 0")
        result=self.run_block()
        self.assertNotEqual(result.returncode,0)
        self.assertFalse(self.marker.exists())
        self.assertEqual(p.read_bytes(),b"STALE")

    def test_modified_project_spec_rejected_before_generation(self):
        with (self.project/"project.yml").open("ab") as stream:stream.write(b"\n# drift")
        self.success_program()
        result=self.run_block()
        self.assertNotEqual(result.returncode,0)
        self.assertFalse(self.marker.exists())
        self.assertFalse((self.project/"CherriSimulatorQA.xcodeproj").exists())

if __name__=="__main__":unittest.main()
