const assert = require("node:assert/strict");
const { _electron: electron } = require("playwright");
const { spawn } = require("node:child_process");
const { mkdtemp, rm, mkdir, readFile, writeFile } = require("node:fs/promises");
const path = require("node:path");
const http = require("node:http");

const root = path.resolve(__dirname, "../..");
const output = process.env.TEAM_QA_DIR || "/tmp/multi-agent-team-qa";

function childEnvironment(extra = {}) {
  const env = {};
  for (const name of [
    "PATH",
    "HOME",
    "TMPDIR",
    "LANG",
    "LC_ALL",
    "DISPLAY",
    "XAUTHORITY",
    "SystemRoot",
  ]) {
    if (process.env[name]) env[name] = process.env[name];
  }
  return { ...env, ...extra };
}

function stopped(child) {
  return child.exitCode !== null || child.signalCode !== null;
}

async function stop(child, signal = "SIGTERM") {
  if (!child || stopped(child)) return;
  const done = new Promise((resolve) => child.once("exit", resolve));
  child.kill(signal);
  const escalation = setTimeout(() => child.kill("SIGKILL"), 3000);
  try {
    await done;
  } finally {
    clearTimeout(escalation);
  }
}

async function fixture(t, name) {
  const state = await mkdtemp("/tmp/team-e2e-");
  const project = await mkdtemp("/tmp/team-project-");
  await writeFile(
    path.join(project, "untouched.txt"),
    "unchanged project content\n",
  );
  const records = {
    name,
    packaged: !!process.env.TEAM_E2E_APP_EXECUTABLE,
    errors: [],
    warnings: [],
    checkpoints: [],
  };
  const applications = [];
  const services = [];
  await mkdir(output, { recursive: true });
  t.after(async () => {
    try {
      for (const entry of applications.reverse()) {
        if (!stopped(entry.process)) await entry.app.close();
      }
    } finally {
      for (const child of services.reverse()) await stop(child);
      await writeFile(
        path.join(output, `${name}-qa.json`),
        JSON.stringify(records, null, 2) + "\n",
      );
      await rm(state, { recursive: true, force: true });
      await rm(project, { recursive: true, force: true });
    }
  });

  async function request(route, method = "GET", payload) {
    return new Promise((resolve, reject) => {
      const data = payload === undefined ? undefined : JSON.stringify(payload);
      const req = http.request(
        {
          socketPath: path.join(state, "teamd.sock"),
          path: route,
          method,
          headers: data ? { "Content-Type": "application/json" } : {},
        },
        (res) => {
          let body = "";
          res.setEncoding("utf8");
          res.on("data", (chunk) => {
            body += chunk;
          });
          res.on("error", reject);
          res.on("end", () => {
            try {
              resolve({ status: res.statusCode, body: JSON.parse(body) });
            } catch (error) {
              reject(error);
            }
          });
        },
      );
      req.on("error", reject);
      req.setTimeout(1000, () => req.destroy(new Error("request timeout")));
      req.end(data);
    });
  }

  async function startService() {
    const child = spawn(
      process.env.TEAM_E2E_DAEMON || path.join(root, "bin/teamd"),
      ["--state-dir", state],
      { env: childEnvironment(), stdio: ["ignore", "pipe", "pipe"] },
    );
    services.push(child);
    let log = "";
    let spawnError;
    child.once("error", (error) => {
      spawnError = error;
    });
    child.stdout.on("data", (data) => {
      log += data;
    });
    child.stderr.on("data", (data) => {
      log += data;
    });
    const deadline = Date.now() + 10000;
    while (Date.now() < deadline) {
      if (spawnError) throw spawnError;
      if (stopped(child)) throw new Error(`service stopped: ${log}`);
      try {
        const response = await request("/healthz");
        if (response.status === 200) return child;
      } catch (error) {
        if (!["ENOENT", "ECONNREFUSED"].includes(error.code)) throw error;
      }
      await new Promise((resolve) => setTimeout(resolve, 30));
    }
    throw new Error(`service readiness timed out: ${log}`);
  }

  async function launch() {
    const packaged = process.env.TEAM_E2E_APP_EXECUTABLE;
    const app = await electron.launch({
      executablePath: packaged,
      args: packaged ? [] : [path.join(root, "desktop/main.cjs")],
      env: childEnvironment({ TEAM_STATE_DIR: state }),
    });
    applications.push({ app, process: app.process() });
    const page = await app.firstWindow();
    page.setDefaultTimeout(10000);
    page.on("pageerror", (error) => records.errors.push(error.message));
    page.on("console", (message) => {
      if (message.type() === "error") records.errors.push(message.text());
      if (message.type() === "warning") records.warnings.push(message.text());
    });
    await page
      .getByRole("heading", { name: "目标与团队", exact: true })
      .waitFor();
    assert.equal(await page.title(), "协作团队");
    assert.match(page.url(), /^file:.*\/dist\/index\.html$/);
    return { app, page };
  }

  async function checkpoint(page, label) {
    const rendered = await page.locator("body").innerText();
    assert.ok(rendered.includes("目标与团队"));
    assert.equal(await page.locator("vite-error-overlay").count(), 0);
    const geometry = await page.evaluate(() => ({
      viewport: innerWidth,
      scroll: document.documentElement.scrollWidth,
    }));
    assert.ok(
      geometry.scroll <= geometry.viewport,
      `horizontal overflow: ${JSON.stringify(geometry)}`,
    );
    assert.deepEqual(records.errors, [], "renderer errors");
    await page.screenshot({
      path: path.join(output, `${name}-${label}.png`),
      fullPage: false,
    });
    records.checkpoints.push({
      label,
      url: page.url(),
      title: await page.title(),
      geometry,
      rendered,
    });
  }

  async function register(page, projectName, projectPath = project) {
    await page.getByRole("button", { name: "添加项目", exact: true }).click();
    await page.getByLabel("项目名称", { exact: true }).fill(projectName);
    await page.getByLabel("项目目录", { exact: true }).fill(projectPath);
    await page.getByRole("button", { name: "保存项目", exact: true }).click();
    await page.locator("dialog").waitFor({ state: "hidden" });
    await page
      .locator(".overview h2")
      .filter({ hasText: projectName })
      .waitFor();
  }

  async function goal(page, title) {
    await page.getByLabel("目标标题", { exact: true }).fill(title);
    await page
      .getByLabel("目标说明", { exact: true })
      .fill("支持提交、中断与恢复；保持现有权限。");
    await page.getByRole("button", { name: "保存目标", exact: true }).click();
    await page
      .getByRole("heading", { name: title, exact: true })
      .first()
      .waitFor();
  }

  async function unchangedProject() {
    assert.equal(
      await readFile(path.join(project, "untouched.txt"), "utf8"),
      "unchanged project content\n",
    );
  }
  return {
    state,
    project,
    records,
    request,
    startService,
    launch,
    checkpoint,
    register,
    goal,
    unchangedProject,
  };
}

module.exports = { fixture, stop, output, root };
