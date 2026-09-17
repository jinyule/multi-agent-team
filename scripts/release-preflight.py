"""Reject accidental release from a branch, wrong version, or unmerged commit."""
import json
import os
import re
import subprocess
from quality import ROOT

version = json.loads((ROOT / "desktop/package.json").read_text())["version"]
ref = os.environ.get("GITHUB_REF", "")
if not re.fullmatch(r"refs/tags/v\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?", ref) or ref != f"refs/tags/v{version}":
    raise SystemExit("release requires an explicit v<package-version> tag")
subprocess.run(["git", "merge-base", "--is-ancestor", "HEAD", "origin/main"], cwd=ROOT, check=True)
if subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT).strip():
    raise SystemExit("release requires a clean checkout")
print(f"Verified {ref} on main; this workflow only ships unsigned development prereleases")
