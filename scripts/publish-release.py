"""Publish a previously verified development candidate, without replacing releases."""
import json
import os
import re
import subprocess
import tempfile
from pathlib import Path
from release import RELEASE, verify

verify()
tag = os.environ["RELEASE_TAG"]
repository = os.environ["RELEASE_REPOSITORY"]
manifest = json.loads((RELEASE / "manifest.json").read_text())
if tag != "v" + manifest["version"] or not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository):
    raise SystemExit("release tag or repository mismatch")
# An existing tag is not overwritten; uncertain retries are resolved by inspecting
# the remote release and hashes before deciding whether any follow-up is necessary.
with tempfile.TemporaryDirectory(prefix="team-release-notes-") as directory:
    notes = Path(directory) / "notes.md"
    notes.write_text("Unsigned macOS arm64 development preview. Not signed or notarized for public distribution.\n\n"
                     "Includes an independent local service and desktop. Agent execution and Git delivery are not available yet.\n\n"
                     f"Source commit: {manifest['commit']}\nSource digest: {manifest['source_digest']}\n"
                     "Verify downloads with SHA256SUMS. See the included README for startup instructions.\n")
    subprocess.run(["gh", "release", "create", tag, *[str(p) for p in RELEASE.glob("*.zip")],
                    str(RELEASE / "manifest.json"), str(RELEASE / "SHA256SUMS"),
                    "--repo", repository, "--verify-tag", "--prerelease", "--title", f"{tag} — development preview", "--notes-file", str(notes)], check=True)
