from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import release


class ReleaseTests(unittest.TestCase):
    def test_matching_manifest_requires_application_license(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            names = ["teamd", "MultiAgentTeam.app/Contents/MacOS/MultiAgentTeam", "MultiAgentTeam.app/Contents/Resources/app.asar", "README.txt", "THIRD_PARTY_NOTICES.txt", "LICENSE", "LICENSES.chromium.html"]
            for name in names:
                target = root / name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text("original")
            manifest = {"format_version": 1, "channel": "unsigned-development", "files": release.inventory(root)}
            with self.assertRaises(ValueError):
                release.verify_payload(root, manifest)

    def test_go_license_supports_distribution_and_homebrew_layouts(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "LICENSE").write_text("Go license\n")
            self.assertEqual(release.go_license(root), "Go license\n")
            (root / "libexec").mkdir()
            self.assertEqual(release.go_license(root / "libexec"), "Go license\n")
            (root / "LICENSE").unlink()
            with self.assertRaises(FileNotFoundError):
                release.go_license(root / "libexec")

    def test_source_edit_during_packaging_invalidates_candidate(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "main.go").write_text("original\n")
            before = release.source_manifest(root)
            release.verify_source_snapshot(before, root)
            (root / "main.go").write_text("changed\n")
            with self.assertRaises(ValueError):
                release.verify_source_snapshot(before, root)

    def test_matching_manifest_cannot_omit_runtime_licenses(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ["teamd", "MultiAgentTeam.app/Contents/MacOS/MultiAgentTeam", "MultiAgentTeam.app/Contents/Resources/app.asar", "README.txt", "THIRD_PARTY_NOTICES.txt"]:
                target = root / name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text("original")
            manifest = {"format_version": 1, "channel": "unsigned-development", "files": release.inventory(root)}
            with self.assertRaises(ValueError):
                release.verify_payload(root, manifest)

    def test_manifest_rejects_modified_missing_and_extra_files(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            names = ["teamd", "MultiAgentTeam.app/Contents/MacOS/MultiAgentTeam", "MultiAgentTeam.app/Contents/Resources/app.asar", "README.txt", "APPLICATION_LICENSE.txt", "THIRD_PARTY_NOTICES.txt", "LICENSE", "LICENSES.chromium.html"]
            for name in names:
                target = root / name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text("original")
            manifest = {"format_version": 1, "channel": "unsigned-development", "files": release.inventory(root)}
            release.verify_payload(root, manifest)
            (root / "teamd").write_text("changed")
            with self.assertRaises(ValueError):
                release.verify_payload(root, manifest)
            (root / "teamd").write_text("original")
            (root / "unexpected").write_text("extra")
            with self.assertRaises(ValueError):
                release.verify_payload(root, manifest)
            (root / "unexpected").unlink()
            (root / "README.txt").unlink()
            with self.assertRaises(ValueError):
                release.verify_payload(root, manifest)

    def test_symlinks_must_stay_in_payload(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "outside").symlink_to("/etc/hosts")
            with self.assertRaises(ValueError):
                release.inventory(root)

    def test_unknown_manifest_is_not_accepted(self):
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(ValueError):
                release.verify_payload(Path(directory), {"format_version": 2, "channel": "unsigned-development"})


if __name__ == "__main__":
    unittest.main()
