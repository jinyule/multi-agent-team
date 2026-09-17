/** Assemble an unsigned development app without installing or starting a service. */
import { packager } from "../desktop/node_modules/@electron/packager/dist/index.js";
import { mkdtemp, cp, writeFile, rm, readFile, mkdir } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
if (process.platform !== "darwin" || process.arch !== "arm64")
  throw new Error("Release candidate requires native macOS arm64");
const pkg = JSON.parse(
  await readFile(path.join(root, "desktop/package.json"), "utf8"),
);
const stage = await mkdtemp(path.join(tmpdir(), "team-package-"));
try {
  for (const name of ["main.cjs", "preload.cjs", "dist"])
    await cp(path.join(root, "desktop", name), path.join(stage, name), {
      recursive: true,
    });
  await writeFile(
    path.join(stage, "package.json"),
    JSON.stringify({
      name: "multi-agent-team",
      version: pkg.version,
      main: "main.cjs",
      private: true,
    }) + "\n",
  );
  const output = path.join(root, "release/packaged");
  await mkdir(output, { recursive: true });
  await packager({
    dir: stage,
    name: "MultiAgentTeam",
    out: output,
    overwrite: true,
    platform: "darwin",
    arch: "arm64",
    electronVersion: pkg.devDependencies.electron,
    appBundleId: "local.multiagent.team",
    appVersion: pkg.version,
    asar: true,
    prune: false,
    osxSign: false,
    osxNotarize: false,
  });
} finally {
  await rm(stage, { recursive: true, force: true });
}
