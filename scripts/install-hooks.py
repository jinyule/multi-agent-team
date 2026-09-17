"""Install repository-local hooks without overwriting unrelated hook configuration."""
import os
import subprocess
from quality import ROOT

result = subprocess.run(["git", "config", "--local", "--get", "core.hooksPath"], cwd=ROOT, capture_output=True, text=True)
if result.returncode not in (0, 1):
    raise SystemExit("could not inspect repository hook configuration")
if result.stdout.strip() not in ("", ".githooks"):
    raise SystemExit("existing core.hooksPath is not owned by this repository; preserve it and invoke make check manually")
for name in ["pre-commit", "pre-push"]:
    os.chmod(ROOT / ".githooks" / name, 0o755)
subprocess.run(["git", "config", "--local", "core.hooksPath", ".githooks"], cwd=ROOT, check=True)
print("Installed repository-local hooks; no global Git configuration changed")
