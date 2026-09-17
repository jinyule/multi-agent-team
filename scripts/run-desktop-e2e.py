"""Run the actual Electron scenarios and reject missing/skipped JUnit evidence."""
import os
from pathlib import Path
import subprocess
import sys
from quality import ROOT, verify_junit

directory = Path(os.environ.get("TEAM_QA_DIR", "/tmp/multi-agent-team-qa"))
directory.mkdir(parents=True, exist_ok=True)
report = directory / "desktop-junit.xml"
report.unlink(missing_ok=True)
result = subprocess.run([
    "node", "--test", "--test-concurrency=1", "--test-reporter=spec", "--test-reporter-destination=stdout",
    "--test-reporter=junit", f"--test-reporter-destination={report}", "e2e/workspace.test.cjs",
], cwd=ROOT / "desktop")
if result.returncode:
    sys.exit(result.returncode)
print(f"Verified {verify_junit(report.read_text(), minimum=3)} real Electron cases, no skipped tests")
