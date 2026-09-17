"""Repository policy checks shared by local commands and CI.

Coverage counts Go statements per internal source file. External process entry
points are verified by built-binary E2E; this is not a whole-product coverage claim.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[1]
EXCLUDED = {".git", "node_modules", "artifacts", "dist", "bin", "release", "__pycache__", ".cache"}
SUFFIXES = {".go", ".mod", ".sum", ".md", ".json", ".yml", ".yaml", ".py", ".ts", ".tsx", ".cjs", ".mjs", ".css", ".html", ".sh", ".txt"}


def source_files(root):
    files = []
    for base, dirs, names in os.walk(root, followlinks=False):
        dirs[:] = sorted(d for d in dirs if d not in EXCLUDED and not (Path(base) / d).is_symlink())
        for name in names:
            p = Path(base) / name
            if p.is_symlink() or name.startswith(".env") or p.suffix in {".pem", ".key", ".p12"}:
                continue
            if p.suffix in SUFFIXES or name in {"LICENSE", "Makefile", ".gitignore", ".editorconfig", ".prettierignore", ".nvmrc", "pre-commit", "pre-push"}:
                files.append(p)
    return sorted(files)


def source_manifest(root):
    return {p.relative_to(root).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest() for p in source_files(root)}


def source_digest(root):
    return hashlib.sha256(json.dumps(source_manifest(root), sort_keys=True).encode()).hexdigest()


def job_failures(required, results):
    return [f"{name}: {results.get(name, {}).get('result', 'missing')}" for name in required
            if results.get(name, {}).get("result") != "success"]


def verify_go_tests(text, required):
    tests, packages = set(), set()
    for line in text.splitlines():
        event = json.loads(line)
        if not isinstance(event, dict):
            raise ValueError("invalid Go test event")
        action, package = event.get("Action"), event.get("Package")
        if action in {"fail", "skip", "build-fail"}:
            raise ValueError(f"required Go test failed or skipped: {package} {event.get('Test', '')}")
        if action == "pass":
            if event.get("Test"):
                tests.add((package, event["Test"]))
            else:
                packages.add(package)
    if not required or any(package not in packages or not any(p == package for p, _ in tests) for package in required):
        raise ValueError("required Go package lacks executed tests or a successful final result")
    return len(tests)


def verify_junit(text, minimum):
    try:
        document = ET.fromstring(text)
    except ET.ParseError as error:
        raise ValueError("invalid JUnit document") from error
    cases = list(document.iter("testcase"))
    if len(cases) < minimum:
        raise ValueError(f"expected at least {minimum} real tests, found {len(cases)}")
    if any(list(document.iter(tag)) for tag in ["failure", "error", "skipped"]):
        raise ValueError("required test failed, errored, or skipped")
    return len(cases)


def parse_go_coverage(text):
    lines = text.splitlines()
    if not lines or lines[0] not in {"mode: atomic", "mode: count", "mode: set"}:
        raise ValueError("missing coverage mode")
    counts = {}
    for line in lines[1:]:
        match = re.fullmatch(r"multi-agent-team/(.+\.go):\d+\.\d+,\d+\.\d+ (\d+) (\d+)", line)
        if not match:
            raise ValueError(f"invalid coverage block: {line}")
        filename, statements, count = match.groups()
        covered, total = counts.get(filename, (0, 0))
        counts[filename] = (covered + (int(statements) if int(count) else 0), total + int(statements))
    return counts


def coverage_failures(actual, sources, threshold):
    errors = []
    for filename in sources:
        covered, total = actual.get(filename, (0, 0))
        if not total or covered * 100 < total * threshold:
            errors.append(f"{filename}: {covered}/{total} statements, requires {threshold}%")
    return errors


def coverage_line(filename, covered, total):
    return f"coverage {filename} — {covered}/{total} statements ({covered * 100 / total if total else 0:.1f}%)"


def repository_errors(root):
    errors = []
    for path in source_files(root):
        name = path.relative_to(root).as_posix()
        text = path.read_text()
        if text and (not text.endswith("\n") or text.endswith("\n\n")):
            errors.append(f"{name}: must end in exactly one newline")
        if any(line.endswith((" ", "\t")) for line in text.splitlines()):
            errors.append(f"{name}: trailing whitespace")
        if re.search(r"^(<<<<<<< |=======|>>>>>>> )", text, re.M):
            errors.append(f"{name}: unresolved conflict marker")
        if path.suffix == ".md":
            for target in re.findall(r"\]\(([^)]+)\)", text):
                if target.startswith(("http:", "https:", "mailto:", "#", "/")):
                    continue
                local = target.split("#", 1)[0].split(":", 1)[0]
                resolved = path.parent / local
                # Historical local evidence is deliberately outside the source archive.
                if "artifacts" in resolved.parts:
                    if not name.startswith("docs/verification/"):
                        errors.append(f"{name}: generated evidence link belongs in docs/verification")
                    continue
                if local and not resolved.exists():
                    errors.append(f"{name}: broken relative link {target}")
    package = json.loads((root / "desktop/package.json").read_text())
    lock = json.loads((root / "desktop/package-lock.json").read_text())["packages"][""]
    if package.get("license") != "MIT" or lock.get("license") != "MIT":
        errors.append("desktop package and lockfile must declare MIT")
    for section in ["dependencies", "devDependencies"]:
        if lock.get(section) != package.get(section):
            errors.append(f"desktop lockfile differs: {section}")
        for name, version in package.get(section, {}).items():
            if not re.fullmatch(r"\d+\.\d+\.\d+(?:-[\w.-]+)?", version):
                errors.append(f"dependency {name} must pin an exact version")
    if lock["version"] != package["version"]:
        errors.append("desktop version differs from lockfile")
    return errors


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("repo")
    commands.add_parser("digest")
    cov = commands.add_parser("coverage")
    cov.add_argument("profile", type=Path)
    cov.add_argument("--minimum", type=int, default=80)
    junit = commands.add_parser("junit")
    junit.add_argument("report", type=Path)
    junit.add_argument("--minimum", type=int, default=1)
    jobs = commands.add_parser("jobs")
    jobs.add_argument("required", nargs="+")
    args = parser.parse_args()
    errors = []
    if args.command == "repo":
        errors = repository_errors(ROOT)
        gofiles = [str(p.relative_to(ROOT)) for p in source_files(ROOT) if p.suffix == ".go"]
        result = subprocess.run(["gofmt", "-l", *gofiles], cwd=ROOT, capture_output=True, text=True, check=True)
        if result.stdout.strip():
            errors.append("gofmt required:\n" + result.stdout)
    elif args.command == "coverage":
        actual = parse_go_coverage(args.profile.read_text())
        sources = sorted(p.relative_to(ROOT).as_posix() for p in (ROOT / "internal").rglob("*.go") if not p.name.endswith("_test.go"))
        if not sources:
            raise ValueError("no internal Go sources discovered")
        errors = coverage_failures(actual, sources, args.minimum)
        for filename in sources:
            covered, total = actual.get(filename, (0, 0))
            print(coverage_line(filename, covered, total))
    elif args.command == "junit":
        print(f"Verified {verify_junit(args.report.read_text(), args.minimum)} executed tests; no skipped/failing cases")
    elif args.command == "jobs":
        errors = job_failures(args.required, json.loads(os.environ["NEEDS_JSON"]))
    elif args.command == "digest":
        print(source_digest(ROOT))
    if errors:
        raise ValueError("\n".join(errors))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, KeyError) as error:
        print(error, file=sys.stderr)
        sys.exit(1)
