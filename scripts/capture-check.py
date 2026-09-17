"""Record a real command result and the source snapshot used for it."""
import datetime
import hashlib
import json
import pathlib
import subprocess
import sys
import tarfile
from quality import source_files

root = pathlib.Path(__file__).resolve().parents[1]
phase, *command = sys.argv[1:]
if phase not in ("red", "green") or not command:
    raise SystemExit("usage: capture-check.py red|green COMMAND [ARGS...]")
stamp = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%S%fZ")
target = root / "artifacts" / "verification" / f"{stamp}-{phase}"
target.mkdir(parents=True)
files = source_files(root)
manifest = {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest() for p in files}
with tarfile.open(target / "source.tar.gz", "w:gz") as archive:
    for p in files:
        archive.add(p, arcname=str(p.relative_to(root)), recursive=False)
result = subprocess.run(command, cwd=root, capture_output=True, text=True)
(target / "output.log").write_text(result.stdout + result.stderr)
(target / "manifest.json").write_text(json.dumps({"phase": phase, "command": command,
    "exit_code": result.returncode, "captured_at": stamp, "files": manifest}, indent=2) + "\n")
print(result.stdout + result.stderr, end="")
print(f"Evidence: {target}")
raise SystemExit(result.returncode)
