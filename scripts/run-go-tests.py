"""Run real Go tests and reject empty or skipped required suites."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
from quality import ROOT, verify_go_tests

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("suite", choices=["test", "coverage", "e2e"])
args = parser.parse_args()
output = Path(os.environ.get("TEAM_QA_DIR", "/tmp/multi-agent-team-qa"))
output.mkdir(parents=True, exist_ok=True)
command = ["go", "test", "-json", "-count=1", "-timeout=60s"]
if args.suite == "e2e":
    command.append("./tests/e2e")
    required = ["multi-agent-team/tests/e2e"]
else:
    command.append("-race")
    if args.suite == "coverage":
        command.append(f"-coverprofile={output / 'go-coverage.out'}")
    command.append("./internal/...")
    required = sorted({"multi-agent-team/" + p.parent.relative_to(ROOT).as_posix()
                       for p in (ROOT / "internal").rglob("*.go") if not p.name.endswith("_test.go")})
result = subprocess.run(command, cwd=ROOT, capture_output=True, text=True)
(output / f"go-{args.suite}.jsonl").write_text(result.stdout)
(output / f"go-{args.suite}.stderr.log").write_text(result.stderr)
print(result.stderr, end="", file=sys.stderr)
try:
    for line in result.stdout.splitlines():
        print(json.loads(line).get("Output", ""), end="")
    if result.returncode:
        raise SystemExit(result.returncode)
    count = verify_go_tests(result.stdout, required)
    print(f"Verified {count} executed Go tests/subtests across {len(required)} required packages; no skips")
except (ValueError, TypeError, AttributeError) as error:
    print(error, file=sys.stderr)
    raise SystemExit(1)
