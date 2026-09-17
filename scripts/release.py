"""Build and verify a macOS development archive; never publish or alter signing policy."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import tempfile
from quality import ROOT, source_digest, source_manifest

RELEASE = ROOT / "release"
PAYLOAD = RELEASE / "payload"


def verify_source_snapshot(snapshot, root=ROOT):
    if source_manifest(root) != snapshot:
        raise ValueError("source changed during candidate build or validation; rebuild from a stable snapshot")


def go_license(goroot):
    try:
        return (goroot / "LICENSE").read_text()
    except FileNotFoundError:
        if goroot.name != "libexec":
            raise
        return (goroot.parent / "LICENSE").read_text()


def run(command, **kwargs):
    subprocess.run(command, cwd=ROOT, check=True, **kwargs)


def sha(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def inventory(root):
    entries = {}
    for path in sorted(root.rglob("*")):
        key = path.relative_to(root).as_posix()
        if path.is_symlink():
            if not path.resolve().is_relative_to(root.resolve()):
                raise ValueError(f"artifact symlink escapes payload: {key}")
            entries[key] = {"link": os.readlink(path)}
        elif path.is_file():
            entries[key] = {"sha256": sha(path), "mode": path.stat().st_mode & 0o777}
    return entries


def verify_payload(payload, manifest):
    if manifest.get("format_version") != 1 or manifest.get("channel") != "unsigned-development":
        raise ValueError("unsupported release manifest or channel")
    actual = inventory(payload)
    if not actual or actual != manifest.get("files"):
        raise ValueError("release payload differs from verified manifest")
    for name in ["teamd", "MultiAgentTeam.app/Contents/MacOS/MultiAgentTeam", "MultiAgentTeam.app/Contents/Resources/app.asar", "README.txt", "THIRD_PARTY_NOTICES.txt", "LICENSE", "LICENSES.chromium.html"]:
        if name not in actual:
            raise ValueError(f"required release artifact missing: {name}")


def write_notices():
    # Preserve upstream notices from the actual module cache and Electron package.
    lines = ["Multi Agent Team: development preview\n", "Application licensing is not yet selected; this is not a redistribution license.\n"]
    goroot = Path(subprocess.check_output(["go", "env", "GOROOT"], cwd=ROOT, text=True).strip())
    lines.append("\n--- Go runtime and standard library ---\n" + go_license(goroot) + "\n")
    decoder = json.JSONDecoder()
    text = subprocess.check_output(["go", "list", "-deps", "-json", "./cmd/teamd"], cwd=ROOT, text=True)
    modules = {}
    while text.strip():
        value, end = decoder.raw_decode(text.lstrip())
        text = text.lstrip()[end:]
        module = value.get("Module")
        if module and not module.get("Main"):
            modules[module["Path"]] = module
    for value in sorted(modules.values(), key=lambda module: module["Path"]):
        if value.get("Replace") or not value.get("Dir") or not value.get("Version"):
            raise ValueError(f"release dependency must resolve to a versioned module: {value['Path']}")
        directory = Path(value["Dir"])
        licenses = sorted(p for p in directory.iterdir() if p.is_file() and p.name.lower().startswith(("license", "copying", "notice")))
        if not licenses:
            raise ValueError(f"license notice missing for {value['Path']}")
        lines.append(f"\n--- {value['Path']} {value['Version']} ---\n")
        for license_file in licenses:
            lines.append(license_file.read_text(errors="strict") + "\n")
    for dependency in ["react", "react-dom", "scheduler"]:
        directory = ROOT / "desktop/node_modules" / dependency
        metadata = json.loads((directory / "package.json").read_text())
        lines.append(f"\n--- {dependency} {metadata['version']} ---\n" + (directory / "LICENSE").read_text() + "\n")
    (PAYLOAD / "THIRD_PARTY_NOTICES.txt").write_text("".join(lines))


def pack():
    if platform.system() != "Darwin" or platform.machine() != "arm64":
        raise ValueError("candidate packaging requires native macOS arm64")
    # These are owned output directories only; never remove an arbitrary user path.
    if RELEASE.is_symlink():
        raise ValueError("release directory may not be a symlink")
    if RELEASE.exists():
        shutil.rmtree(RELEASE)
    sources = source_manifest(ROOT)
    run(["make", "build", "desktop-build"])
    run(["node", "scripts/package-desktop.mjs"])
    PAYLOAD.mkdir(parents=True)
    built = RELEASE / "packaged/MultiAgentTeam-darwin-arm64"
    for item in built.iterdir():
        destination = PAYLOAD / item.name
        if item.is_dir():
            shutil.copytree(item, destination, symlinks=True)
        else:
            shutil.copy2(item, destination)
    shutil.copy2(ROOT / "bin/teamd", PAYLOAD / "teamd")
    (PAYLOAD / "README.txt").write_text(
        "Multi Agent Team — unsigned macOS arm64 development preview\n\n"
        "This is not signed or notarized for public distribution. Do not disable Gatekeeper.\n"
        "In a terminal, run the included ./teamd, then launch MultiAgentTeam.app.\n"
        "Quit the terminal service with Ctrl+C; quitting the desktop leaves it running.\n"
        "No login item is installed. Agent execution and Git delivery are not available yet.\n"
    )
    write_notices()
    environment = {**os.environ, "TEAM_E2E_APP_EXECUTABLE": str(PAYLOAD / "MultiAgentTeam.app/Contents/MacOS/MultiAgentTeam"),
                   "TEAM_E2E_DAEMON": str(PAYLOAD / "teamd"),
                   "TEAM_QA_DIR": str(Path(os.environ.get("TEAM_QA_DIR", "/tmp/multi-agent-team-qa")) / "packaged")}
    run([sys.executable, "scripts/run-desktop-e2e.py"], env=environment)
    try:
        commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True, stderr=subprocess.DEVNULL).strip()
    except subprocess.CalledProcessError:
        commit = None
    dirty = bool(subprocess.check_output(["git", "status", "--porcelain", "--untracked-files=all"], cwd=ROOT, text=True).strip())
    verify_source_snapshot(sources)
    manifest = {"format_version": 1, "channel": "unsigned-development", "version": json.loads((ROOT / "desktop/package.json").read_text())["version"],
                "commit": commit, "dirty": dirty, "source_digest": hashlib.sha256(json.dumps(sources, sort_keys=True).encode()).hexdigest(), "sources": sources,
                "platform": "darwin-arm64", "go": subprocess.check_output(["go", "version"],text=True).strip(),
                "node": subprocess.check_output(["node", "--version"],text=True).strip(), "files": inventory(PAYLOAD)}
    (RELEASE / "manifest.json").write_text(json.dumps(manifest,indent=2)+"\n")
    verify_payload(PAYLOAD, manifest)
    archive = RELEASE / f"multi-agent-team-{manifest['version']}-macos-arm64-dev.zip"
    run(["ditto", "-c", "-k", "--sequesterRsrc", "--keepParent", str(PAYLOAD), str(archive)])
    (RELEASE / "SHA256SUMS").write_text(f"{sha(archive)}  {archive.name}\n{sha(RELEASE / 'manifest.json')}  manifest.json\n")
    verify(allow_dirty=True)
    print(f"Verified development candidate: {archive}")


def verify(allow_dirty=False):
    manifest = json.loads((RELEASE / "manifest.json").read_text())
    archives = list(RELEASE.glob("*.zip"))
    if len(archives) != 1:
        raise ValueError("expected exactly one release archive")
    expected = f"{sha(archives[0])}  {archives[0].name}\n{sha(RELEASE / 'manifest.json')}  manifest.json\n"
    if (RELEASE / "SHA256SUMS").read_text() != expected:
        raise ValueError("checksum mismatch")
    if manifest["source_digest"] != source_digest(ROOT):
        raise ValueError("source changed after candidate validation")
    if not allow_dirty:
        commit = subprocess.check_output(["git", "rev-parse", "HEAD"],cwd=ROOT,text=True).strip()
        if manifest["dirty"] or not manifest["commit"] or commit != manifest["commit"]:
            raise ValueError("publishing requires a clean candidate bound to the checked-out commit")
    with tempfile.TemporaryDirectory(prefix="team-archive-") as directory:
        run(["ditto", "-x", "-k", str(archives[0]), directory])
        verify_payload(Path(directory) / "payload", manifest)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["pack", "verify"])
    parser.add_argument("--allow-dirty", action="store_true")
    args = parser.parse_args()
    try:
        pack() if args.command == "pack" else verify(args.allow_dirty)
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        print(error, file=sys.stderr)
        sys.exit(1)
