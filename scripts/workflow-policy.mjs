/** Structural policies supplement actionlint with this repository's required gates. */
import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import YAML from "../desktop/node_modules/yaml/dist/index.js";

export function validateWorkflow(name, workflow) {
  const errors = [];
  if (workflow.permissions?.contents !== "read")
    errors.push("workflow must default to contents: read");
  if (workflow.on?.pull_request_target !== undefined)
    errors.push("privileged pull_request_target is forbidden");
  for (const [id, job] of Object.entries(workflow.jobs || {})) {
    if (!job["timeout-minutes"]) errors.push(`${id}: missing timeout`);
    if (job["continue-on-error"])
      errors.push(`${id}: continue-on-error forbidden`);
    for (const step of job.steps || []) {
      if (step["continue-on-error"])
        errors.push(`${id}: step continue-on-error forbidden`);
      if (step.uses && !/^[\w.-]+\/[\w./-]+@[a-f0-9]{40}$/.test(step.uses))
        errors.push(`${id}: action must pin a commit`);
      if (
        step.uses?.startsWith("actions/checkout@") &&
        step.with?.["persist-credentials"] !== false
      )
        errors.push(`${id}: checkout retains credentials`);
      if (
        step.uses?.startsWith("actions/upload-artifact@") &&
        step.with?.["if-no-files-found"] !== "error"
      )
        errors.push(`${id}: missing artifact can pass`);
      if (step.run?.includes("TEAM_UPDATE_SNAPSHOTS=1"))
        errors.push(`${id}: CI cannot update expected snapshots`);
    }
  }
  if (name === "ci.yml") {
    const verdict = workflow.jobs?.["all-checks-passed"];
    const expected = Object.keys(workflow.jobs || {})
      .filter((id) => id !== "all-checks-passed")
      .sort();
    if (
      !verdict ||
      JSON.stringify([...(verdict.needs || [])].sort()) !==
        JSON.stringify(expected)
    )
      errors.push("aggregate must require every CI job");
    if (verdict?.if !== "always()")
      errors.push("aggregate must run even after failure");
    if (
      !verdict?.steps?.some((step) => {
        const prefix = "python3 scripts/quality.py jobs ";
        if (!step.run?.startsWith(prefix)) return false;
        const required = step.run
          .slice(prefix.length)
          .trim()
          .split(/\s+/)
          .sort();
        return JSON.stringify(required) === JSON.stringify(expected);
      })
    )
      errors.push("aggregate must execute result verifier");
    for (const job of Object.values(workflow.jobs || {})) {
      if (job.permissions?.contents === "write")
        errors.push("CI cannot write repository contents");
      if (JSON.stringify(job).includes("secrets."))
        errors.push("required CI must be keyless");
    }
  }
  if (name === "release.yml") {
    if (Object.keys(workflow.on || {}).join() !== "workflow_dispatch")
      errors.push("release must be manual-only");
    const publish = workflow.jobs?.publish;
    if (
      publish?.environment !== "development-release" ||
      publish?.needs !== "candidate" ||
      publish?.if !== "inputs.publish"
    )
      errors.push(
        "release must require candidate and explicit environment approval",
      );
    if (publish?.permissions?.contents !== "write")
      errors.push("publish permission must be scoped to publish job");
    if (
      !(publish?.steps || []).some(
        (step) =>
          step.run === "python3 scripts/release_approval.py" &&
          step.env?.RELEASE_APPROVAL_CONFIGURED ===
            "${{ vars.RELEASE_APPROVAL_CONFIGURED }}",
      )
    )
      errors.push(
        "publish must fail closed until environment protection is verified",
      );
    if (
      (publish?.steps || []).some((step) =>
        /npm .*ci|make |go build|package-desktop/.test(step.run || ""),
      )
    )
      errors.push("publish cannot rebuild or install candidate code");
    if (
      !(publish?.steps || []).some(
        (step) => step.run === "python3 scripts/release.py verify",
      )
    )
      errors.push("publish must verify artifact bytes");
  }
  return errors;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const folder = path.resolve(
    path.dirname(fileURLToPath(import.meta.url)),
    "../.github/workflows",
  );
  const errors = [];
  for (const name of readdirSync(folder).filter((name) =>
    name.endsWith(".yml"),
  )) {
    const workflow = YAML.parse(readFileSync(path.join(folder, name), "utf8"));
    errors.push(
      ...validateWorkflow(name, workflow).map((error) => `${name}: ${error}`),
    );
  }
  if (errors.length) {
    console.error(errors.join("\n"));
    process.exitCode = 1;
  } else console.log("Workflow policies passed");
}
