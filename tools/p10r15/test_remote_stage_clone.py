"""Exercise immutable staging clones with real Git objects on two filesystems."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

from stage_remote_release import checked_clone, clean_head

ORIGIN = "https://github.com/davidpovarsky/cherri.git"


class ImmutableCloneTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(
            prefix="cherri-stage-clone-",
            dir=os.environ.get("P15_CLONE_TEST_SOURCE_ROOT"))
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)

    def git(self, repo, *args):
        result = subprocess.run(
            ["git", "-c", "gc.auto=0", "-c", "maintenance.auto=false",
             "-c", "core.hooksPath=/dev/null", *args],
            cwd=repo, check=True, text=True, capture_output=True, timeout=15)
        return result.stdout.strip()

    def source(self):
        source = self.root / "source"
        source.mkdir()
        self.git(source, "init", "--quiet")
        (source / "fixture.txt").write_text("requested immutable revision\n")
        self.git(source, "add", "fixture.txt")
        self.git(source, "-c", "user.name=Clone fixture",
                 "-c", "user.email=clone-fixture@example.invalid",
                 "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "First")
        revision = clean_head(source)
        (source / "fixture.txt").write_text("later source revision\n")
        self.git(source, "add", "fixture.txt")
        self.git(source, "-c", "user.name=Clone fixture",
                 "-c", "user.email=clone-fixture@example.invalid",
                 "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "Later")
        return source, revision

    def assert_independent_clone(self, source, destination, revision):
        source_head = clean_head(source)
        self.assertNotEqual(source_head, revision)
        checked_clone(source, destination, revision, ORIGIN)
        self.assertEqual(clean_head(source), source_head)
        self.assertEqual(clean_head(destination), revision)
        self.assertEqual(self.git(destination, "rev-parse", "--abbrev-ref", "HEAD"), "HEAD")
        self.assertEqual((destination / "fixture.txt").read_text(),
                         "requested immutable revision\n")
        self.assertEqual(self.git(destination, "remote", "get-url", "origin"), ORIGIN)
        self.assertFalse((destination / ".git/objects/info/alternates").exists())
        devices = {"sourceDevice": (source / ".git/objects").stat().st_dev,
                   "destinationDevice": (destination / ".git/objects").stat().st_dev}
        # Removing only this test's synthetic input proves no borrowed store remains.
        shutil.rmtree(source)
        self.git(destination, "fsck", "--full", "--strict", "--no-reflogs")
        self.assertEqual(clean_head(destination), revision)
        self.assertEqual(self.git(destination, "show", revision + ":fixture.txt"),
                         "requested immutable revision")
        print(json.dumps(dict(case=self.id(), sourceRemoved=True,
                              independentObjects=True, **devices), sort_keys=True))

    def test_same_filesystem_clone_remains_independent(self):
        source, revision = self.source()
        destination = self.root / "release"
        self.assertEqual(source.stat().st_dev, destination.parent.stat().st_dev)
        self.assert_independent_clone(source, destination, revision)

    def test_cross_filesystem_clone_remains_independent(self):
        source, revision = self.source()
        selected = os.environ.get("P15_CLONE_TEST_CROSS_ROOT")
        candidates = [Path(selected)] if selected else [Path("/dev/shm")]
        parent = next((path for path in candidates
                       if path.is_dir() and os.access(path, os.W_OK)
                       and path.stat().st_dev != source.stat().st_dev), None)
        if parent is None:
            if selected:
                self.fail("Configured cross-filesystem test root is not usable")
            self.skipTest("No writable second filesystem is available")
        directory = tempfile.TemporaryDirectory(prefix="cherri-stage-clone-", dir=parent)
        self.addCleanup(directory.cleanup)
        destination = Path(directory.name) / "release"
        self.assertNotEqual(source.stat().st_dev, destination.parent.stat().st_dev)
        self.assert_independent_clone(source, destination, revision)

    def test_unexpected_origin_is_rejected_before_clone(self):
        source, revision = self.source()
        destination = self.root / "release"
        with self.assertRaisesRegex(ValueError, "Unexpected immutable clone origin"):
            checked_clone(source, destination, revision, "https://example.invalid/cherri.git")
        self.assertFalse(destination.exists())
        self.assertNotEqual(clean_head(source), revision)


if __name__ == "__main__":
    unittest.main()
