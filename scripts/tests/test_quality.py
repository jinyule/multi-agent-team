import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("quality", ROOT / "scripts/quality.py")
quality = importlib.util.module_from_spec(spec)
spec.loader.exec_module(quality)


class GateTests(unittest.TestCase):
    def test_go_results_require_tests_and_reject_skips_missing_and_failure(self):
        def report(events):
            return "\n".join(json.dumps(event) for event in events)
        case = {"Action": "pass", "Package": "team/api", "Test": "TestAPI"}
        package = {"Action": "pass", "Package": "team/api"}
        self.assertEqual(quality.verify_go_tests(report([case, package]), ["team/api"]), 1)
        for events in [[], [package], [case],
                       [{**case, "Action": "skip"}, package],
                       [case, {**case, "Test": "TestOther", "Action": "skip"}, package],
                       [{**case, "Action": "fail"}, package],
                       [case, {**package, "Action": "fail"}],
                       [{**case, "Package": "team/other"}, package]]:
            with self.subTest(events=events), self.assertRaises(ValueError):
                quality.verify_go_tests(report(events), ["team/api"])
        with self.assertRaises(ValueError):
            quality.verify_go_tests("not JSON", ["team/api"])

    def test_required_jobs_reject_every_non_success_and_missing(self):
        required = ["static", "go", "desktop"]
        self.assertEqual(quality.job_failures(required, {k: {"result": "success"} for k in required}), [])
        for result in ["failure", "cancelled", "skipped", "neutral", "unknown"]:
            with self.subTest(result=result):
                self.assertTrue(quality.job_failures(required, {k: {"result": result} for k in required}))
        self.assertTrue(quality.job_failures(required, {}))
        self.assertTrue(quality.job_failures(required, {"static": {"result": "success"}}))

    def test_junit_rejects_empty_skipped_failed_and_malformed(self):
        for value in ["<testsuites/>", '<testsuite><testcase><skipped/></testcase></testsuite>',
                      '<testsuite><testcase><failure/></testcase></testsuite>', "not XML"]:
            with self.subTest(value=value), self.assertRaises(ValueError):
                quality.verify_junit(value, minimum=1)
        quality.verify_junit('<testsuite><testcase name="actual"/></testsuite>', minimum=1)
        with self.assertRaises(ValueError):
            quality.verify_junit('<testsuite><testcase/></testsuite>', minimum=2)

    def test_coverage_discovers_missing_files_and_low_coverage(self):
        report = 'mode: atomic\nmulti-agent-team/internal/a/a.go:1.1,3.2 3 1\nmulti-agent-team/internal/a/a.go:4.1,5.2 1 0\n'
        actual = quality.parse_go_coverage(report)
        self.assertEqual(actual["internal/a/a.go"], (3, 4))
        self.assertEqual(quality.coverage_failures(actual, ["internal/a/a.go"], 75), [])
        self.assertTrue(quality.coverage_failures(actual, ["internal/a/a.go"], 80))
        self.assertTrue(quality.coverage_failures(actual, ["internal/new/new.go"], 80))
        with self.assertRaises(ValueError):
            quality.parse_go_coverage('mode: atomic\ngarbage\n')

    def test_coverage_output_is_not_parsed_as_a_github_error_annotation(self):
        line = quality.coverage_line("internal/api/api.go", 69, 80)
        self.assertEqual(line, "coverage internal/api/api.go — 69/80 statements (86.2%)")
        self.assertNotIn(".go:", line)

    def test_source_inventory_excludes_secrets_builds_and_external_symlinks(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ["main.go", "LICENSE", ".env", ".env.local", "private.pem", "node_modules/x.js", "artifacts/log.txt", "docs/readme.md"]:
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("sample\n")
            (root / "escape.md").symlink_to("/etc/hosts")
            self.assertEqual([p.relative_to(root).as_posix() for p in quality.source_files(root)], ["LICENSE", "docs/readme.md", "main.go"])

    def test_inventory_digest_changes_with_content(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "a.go").write_text("one\n")
            first = quality.source_digest(root)
            (root / "a.go").write_text("two\n")
            self.assertNotEqual(first, quality.source_digest(root))


if __name__ == "__main__":
    unittest.main()
