"""Offline containment controls for the exact emitted Remote Builder guards.

Only Bash builtins and realpath run in disposable fixture trees. No compiler,
doctor, signer, network service or live Remote Builder is invoked.
"""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

import stage_remote_release


# These are the original guard bodies from the retained installed wrappers.
# Their surrounding driver stops after source validation.
ORIGINAL_SCRIPTS = {
    "v2": r'''#!/usr/bin/env bash
set -Eeuo pipefail
BASE="${CHERRI_REMOTE_HOME:-${HOME}/workspaces/cherri-remote}"
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
safe_source() {
  local input="$1" real
  [ -f "$input" ] || die "Input does not exist: $input"
  real=$(realpath -- "$input")
  case "$real" in "$BASE/jobs/"*) printf '%s\n' "$real";; *) die "Source must be staged under $BASE/jobs/";; esac
}
safe_source "$1"
''',
    "legacy": r'''#!/usr/bin/env bash
set -Eeuo pipefail
BASE="${CHERRI_REMOTE_HOME:-$HOME/workspaces/cherri-remote}"
fail(){ printf 'LEGACY_BUILDER_ERROR: %s\n' "$*" >&2; exit 1; }
source="$(realpath -- "$1")"; shift
case "$source" in "$BASE/jobs/"*.cherri) ;; *) fail "source must be .cherri inside jobs" ;; esac
[ -f "$source" ] || fail "source file missing"
printf '%s\n' "$source"
''',
}


class RemoteSourceContainment(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="p15-source-containment-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.user = self.root / "user"
        self.alias = self.user / "workspaces/cherri-remote"
        self.actual = self.root / "persistent root" / "remote"
        (self.actual / "jobs").mkdir(parents=True)
        self.alias.parent.mkdir(parents=True)
        self.alias.symlink_to(self.actual, target_is_directory=True)
        self.source = self.actual / "jobs/public input.cherri"
        self.source.write_text("public synthetic fixture\n", encoding="utf-8")
        self.bash = shutil.which("bash")
        self.assertIsNotNone(self.bash, "Bash is required for actual guard controls")
        self.assertIsNotNone(shutil.which("realpath"),
                             "realpath is required for actual guard controls")

    def run_guard(self, dialect, source, *, repaired=True, home=None,
                  remote_home=None):
        script = ORIGINAL_SCRIPTS[dialect]
        if repaired:
            script = stage_remote_release.canonicalize_source_guard(script, dialect)
        env = {
            "PATH": os.environ.get("PATH", os.defpath),
            "HOME": str(home if home is not None else self.user),
            "LANG": "C",
            "LC_ALL": "C",
        }
        if remote_home is not None:
            env["CHERRI_REMOTE_HOME"] = str(remote_home)
        # In particular, no caller BASH_ENV, credentials or CHERRI_* overrides
        # enter this shell. The only optional override is the owned candidate.
        return subprocess.run(
            [self.bash, "--noprofile", "--norc", "-s", "--", str(source)],
            input=script, text=True, capture_output=True, cwd=self.root,
            env=env, timeout=5,
        )

    def assert_accepted(self, result, source):
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, str(source.resolve()) + "\n")
        self.assertEqual(result.stderr, "")

    def assert_rejected(self, result, dialect, diagnostic=None):
        expected = diagnostic or {
            "v2": "Source must be staged under ",
            "legacy": "source must be .cherri inside jobs",
        }[dialect]
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertEqual(result.stdout, "")
        self.assertIn(expected, result.stderr)

    def test_symlinked_default_home_accepts_lexical_and_resolved_inputs(self):
        lexical = self.alias / "jobs" / self.source.name
        self.assertNotEqual(str(lexical), str(self.source))
        self.assertEqual(lexical.resolve(), self.source)
        for dialect in ORIGINAL_SCRIPTS:
            for spelling, source in (("lexical", lexical), ("resolved", self.source)):
                with self.subTest(dialect=dialect, spelling=spelling):
                    self.assert_accepted(self.run_guard(dialect, source), self.source)

    def test_ordinary_default_and_explicit_candidate_homes_still_accept(self):
        ordinary_user = self.root / "ordinary-user"
        ordinary = ordinary_user / "workspaces/cherri-remote"
        candidate = self.root / "inactive-candidate"
        for base in (ordinary, candidate):
            (base / "jobs").mkdir(parents=True)
            (base / "jobs/input.cherri").write_text("public fixture\n", encoding="utf-8")
        for dialect in ORIGINAL_SCRIPTS:
            with self.subTest(dialect=dialect, topology="ordinary-default"):
                source = ordinary / "jobs/input.cherri"
                self.assert_accepted(self.run_guard(dialect, source, home=ordinary_user),
                                     source)
            with self.subTest(dialect=dialect, topology="explicit-candidate"):
                source = candidate / "jobs/input.cherri"
                self.assert_accepted(self.run_guard(dialect, source,
                                                   remote_home=candidate), source)

    def test_original_guard_reproduces_symlinked_default_rejection(self):
        lexical = self.alias / "jobs" / self.source.name
        for dialect in ORIGINAL_SCRIPTS:
            for spelling, source in (("lexical", lexical), ("resolved", self.source)):
                with self.subTest(dialect=dialect, spelling=spelling):
                    self.assert_rejected(
                        self.run_guard(dialect, source, repaired=False), dialect)

    def test_escape_paths_are_rejected(self):
        sibling = self.actual / "jobs-sibling/input.cherri"
        sibling.parent.mkdir()
        sibling.write_text("public outside fixture\n", encoding="utf-8")
        outside = self.actual / "outside/input.cherri"
        outside.parent.mkdir()
        outside.write_text("public outside fixture\n", encoding="utf-8")
        (self.actual / "jobs/file-link.cherri").symlink_to(outside)
        (self.actual / "jobs/directory-link").symlink_to(outside.parent,
                                                        target_is_directory=True)
        paths = {
            "sibling-prefix": sibling,
            "dotdot": self.alias / "jobs/../outside/input.cherri",
            "file-symlink": self.alias / "jobs/file-link.cherri",
            "directory-symlink": self.alias / "jobs/directory-link/input.cherri",
        }
        for name, source in paths.items():
            self.assertTrue(source.is_file(), name)
            self.assertFalse(source.resolve().is_relative_to(self.actual / "jobs"), name)
        for dialect in ORIGINAL_SCRIPTS:
            for name, source in paths.items():
                with self.subTest(dialect=dialect, escape=name):
                    self.assert_rejected(self.run_guard(dialect, source), dialect)

    def test_jobs_directory_symlink_does_not_expand_boundary(self):
        (self.actual / "jobs").rename(self.root / "retained-original-jobs")
        outside = self.root / "outside-jobs"
        outside.mkdir()
        source = outside / "input.cherri"
        source.write_text("public outside fixture\n", encoding="utf-8")
        (self.actual / "jobs").symlink_to(outside, target_is_directory=True)
        lexical = self.alias / "jobs/input.cherri"
        self.assertTrue(lexical.is_file())
        self.assertEqual(lexical.resolve(), source)
        for dialect in ORIGINAL_SCRIPTS:
            for spelling, path in (("lexical", lexical), ("resolved", source)):
                with self.subTest(dialect=dialect, spelling=spelling):
                    self.assert_rejected(self.run_guard(dialect, path), dialect)

    def test_original_extension_and_regular_file_rules_remain(self):
        other_extension = self.actual / "jobs/input.txt"
        other_extension.write_text("public fixture\n", encoding="utf-8")
        self.assert_accepted(self.run_guard("v2", other_extension), other_extension)
        self.assert_rejected(self.run_guard("legacy", other_extension), "legacy")
        directory = self.actual / "jobs/directory.cherri"
        directory.mkdir()
        for dialect, diagnostic in (("v2", "Input does not exist:"),
                                    ("legacy", "source file missing")):
            with self.subTest(dialect=dialect, rejected="directory"):
                self.assert_rejected(self.run_guard(dialect, directory), dialect,
                                     diagnostic)

    def test_unresolvable_base_fails_closed(self):
        self.alias.unlink()
        self.alias.symlink_to(self.alias.name, target_is_directory=True)
        self.assertTrue(self.source.is_file())
        for dialect, diagnostic in (("v2", "Cannot resolve Remote Builder home"),
                                    ("legacy", "cannot resolve Remote Builder home")):
            with self.subTest(dialect=dialect):
                self.assert_rejected(self.run_guard(dialect, self.source), dialect,
                                     diagnostic)

    def test_rewriter_rejects_missing_duplicate_or_changed_guards(self):
        for dialect, original in ORIGINAL_SCRIPTS.items():
            repaired = stage_remote_release.canonicalize_source_guard(original, dialect)
            cases = {
                "missing": "",
                "duplicate": original + "\n" + original,
                "changed-whitespace": original.replace('case "', 'case  "', 1),
                "other-route": ORIGINAL_SCRIPTS["legacy" if dialect == "v2" else "v2"],
                "already-repaired": repaired,
            }
            for name, script in cases.items():
                with self.subTest(dialect=dialect, shape=name):
                    with self.assertRaises(ValueError):
                        stage_remote_release.canonicalize_source_guard(script, dialect)
        for dialect in ("", "V2", "unknown"):
            with self.subTest(unsupported=dialect):
                with self.assertRaises(ValueError):
                    stage_remote_release.canonicalize_source_guard(
                        ORIGINAL_SCRIPTS["v2"], dialect)


if __name__ == "__main__":
    unittest.main()
