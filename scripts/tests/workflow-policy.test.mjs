import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import YAML from "../../desktop/node_modules/yaml/dist/index.js";
import { validateWorkflow } from "../workflow-policy.mjs";

const ci = YAML.parse(
  readFileSync(
    new URL("../../.github/workflows/ci.yml", import.meta.url),
    "utf8",
  ),
);
const release = YAML.parse(
  readFileSync(
    new URL("../../.github/workflows/release.yml", import.meta.url),
    "utf8",
  ),
);
test("actual workflows comply with required policies", () => {
  assert.deepEqual(validateWorkflow("ci.yml", ci), []);
  assert.deepEqual(validateWorkflow("release.yml", release), []);
});
test("missing aggregate needs, skip, unpinned action, secret and tolerated failure are rejected", () => {
  for (const mutate of [
    (w) => w.jobs["all-checks-passed"].needs.pop(),
    (w) => (w.jobs["all-checks-passed"].if = "success()"),
    (w) => (w.jobs.static.steps[0].uses = "actions/checkout@v6"),
    (w) => (w.jobs.static["continue-on-error"] = true),
    (w) => (w.jobs.go.env = { KEY: "${{ secrets.KEY }}" }),
    (w) => {
      w.jobs.additional = structuredClone(w.jobs.static);
      w.jobs["all-checks-passed"].needs.push("additional");
    },
  ]) {
    const changed = structuredClone(ci);
    mutate(changed);
    assert.ok(validateWorkflow("ci.yml", changed).length);
  }
});
test("automatic release, absent environment and rebuilding during publish are rejected", () => {
  for (const mutate of [
    (w) => (w.on.push = {}),
    (w) => delete w.jobs.publish.environment,
    (w) => w.jobs.publish.steps.push({ run: "make package" }),
    (w) => {
      w.jobs.publish.steps = w.jobs.publish.steps.filter(
        (step) => step.run !== "python3 scripts/release_approval.py",
      );
    },
  ]) {
    const changed = structuredClone(release);
    mutate(changed);
    assert.ok(validateWorkflow("release.yml", changed).length);
  }
});
