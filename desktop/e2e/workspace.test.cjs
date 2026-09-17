const { test } = require("node:test");
const assert = require("node:assert/strict");
const { readFile, writeFile } = require("node:fs/promises");
const path = require("node:path");
const { fixture, stop, root } = require("./harness.cjs");

test(
  "offline startup, service recovery, and desktop restart preserve real goals",
  { timeout: 60000 },
  async (t) => {
    const h = await fixture(t, "lifecycle");
    let { app, page } = await h.launch();
    await page.getByText("本机服务连接中断", { exact: true }).waitFor();
    await h.checkpoint(page, "offline");
    let service = await h.startService();
    await page.getByRole("button", { name: "重新连接", exact: true }).click();
    await page.getByText("本机服务已连接", { exact: true }).waitFor();
    await h.register(page, "验收项目");
    await h.goal(page, "外部程序控制接口");
    const before = await h.request("/v1/snapshot");
    assert.equal(before.body.goals.length, 1);
    assert.equal(before.body.goals[0].state, "awaiting_engine");
    await stop(service, "SIGKILL");
    await page.getByText("本机服务连接中断", { exact: true }).waitFor();
    assert.equal(
      await page
        .getByRole("button", { name: "保存目标", exact: true })
        .isDisabled(),
      true,
    );
    await page
      .getByRole("heading", { name: "外部程序控制接口", exact: true })
      .waitFor();
    service = await h.startService();
    await page.getByRole("button", { name: "重新连接", exact: true }).click();
    await page.getByText("本机服务已连接", { exact: true }).waitFor();
    assert.deepEqual((await h.request("/v1/snapshot")).body, before.body);
    await app.close();
    assert.equal((await h.request("/healthz")).status, 200);
    ({ app, page } = await h.launch());
    await page
      .getByRole("heading", { name: "外部程序控制接口", exact: true })
      .waitFor();
    await h.checkpoint(page, "recovered");
    await h.unchangedProject();
  },
);

test(
  "validation, duplicate registration and project switching do not mix work",
  { timeout: 60000 },
  async (t) => {
    const h = await fixture(t, "validation");
    await h.startService();
    const { page } = await h.launch();
    await page.getByText("本机服务已连接", { exact: true }).waitFor();
    await page.getByRole("button", { name: "添加项目", exact: true }).click();
    await page.getByLabel("项目名称", { exact: true }).fill("项目一");
    await page
      .getByLabel("项目目录", { exact: true })
      .fill(path.join(h.project, "missing"));
    await page.getByRole("button", { name: "保存项目", exact: true }).click();
    await page.getByRole("alert").waitFor();
    assert.equal((await h.request("/v1/snapshot")).body.projects.length, 0);
    await h.checkpoint(page, "invalid-path");
    await page.getByLabel("项目目录", { exact: true }).fill(h.project);
    await page.getByRole("button", { name: "保存项目", exact: true }).click();
    await page.locator("dialog").waitFor({ state: "hidden" });
    await h.goal(page, "项目一目标");
    await page.getByRole("button", { name: "添加项目", exact: true }).click();
    await page.getByLabel("项目名称", { exact: true }).fill("重复项目");
    await page.getByLabel("项目目录", { exact: true }).fill(h.project);
    await page.getByRole("button", { name: "保存项目", exact: true }).click();
    await page.getByRole("alert").waitFor();
    assert.equal((await h.request("/v1/snapshot")).body.projects.length, 1);
    await page.getByRole("button", { name: "取消", exact: true }).click();
    await h.register(page, "项目二", h.state);
    assert.equal(await page.locator('[data-testid="goal"]').count(), 0);
    await h.goal(page, "项目二目标");
    await page.getByRole("button", { name: "项目一", exact: false }).click();
    await page
      .getByRole("heading", { name: "项目一目标", exact: true })
      .waitFor();
    assert.equal(
      await page
        .getByRole("heading", { name: "项目二目标", exact: true })
        .count(),
      0,
    );
    assert.equal((await h.request("/v1/snapshot")).body.goals.length, 2);
    await h.unchangedProject();
    await h.checkpoint(page, "separate-projects");
  },
);

test(
  "real renderer matches reviewed semantics and stays isolated at supported sizes",
  { timeout: 60000 },
  async (t) => {
    const h = await fixture(t, "semantics");
    await h.startService();
    const { page } = await h.launch();
    await page.getByText("本机服务已连接", { exact: true }).waitFor();
    await h.register(page, "验收项目");
    await h.goal(page, "外部程序控制接口");
    assert.deepEqual(
      await page.evaluate(() => ({
        require: typeof window.require,
        process: typeof window.process,
        team: Object.keys(window.team).sort(),
      })),
      {
        require: "undefined",
        process: "undefined",
        team: ["createGoal", "createProject", "selectDirectory", "snapshot"],
      },
    );
    const semantic = {
      members: await page
        .locator('[data-testid="member"] h3')
        .allTextContents(),
      goals: await page.locator('[data-testid="goal"] h3').allTextContents(),
      states: await page.locator(".goal-status").allTextContents(),
      buttons: await page.locator("main button").allTextContents(),
    };
    const expected = path.join(root, "desktop/e2e/snapshots/workspace.json");
    if (process.env.TEAM_UPDATE_SNAPSHOTS === "1") {
      if (process.env.CI) throw new Error("CI cannot update snapshots");
      await writeFile(expected, JSON.stringify(semantic, null, 2) + "\n");
    }
    assert.deepEqual(semantic, JSON.parse(await readFile(expected, "utf8")));
    await page.setViewportSize({ width: 1240, height: 860 });
    await h.checkpoint(page, "desktop");
    await page.setViewportSize({ width: 920, height: 700 });
    await h.checkpoint(page, "compact");
    assert.equal(
      await page
        .getByRole("button", { name: "保存目标", exact: true })
        .isVisible(),
      true,
    );
    await h.unchangedProject();
  },
);
