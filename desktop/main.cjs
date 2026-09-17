const { app, BrowserWindow, ipcMain, dialog } = require("electron");
const http = require("node:http");
const path = require("node:path");
const { pathToFileURL } = require("node:url");

const index = path.join(__dirname, "dist", "index.html");
let window;

function request(method, route, body) {
  const state =
    process.env.TEAM_STATE_DIR ||
    path.join(app.getPath("appData"), "multi-agent-team");
  const encoded =
    body === undefined ? undefined : Buffer.from(JSON.stringify(body));
  return new Promise((resolve, reject) => {
    const req = http.request(
      {
        socketPath: path.join(state, "teamd.sock"),
        path: route,
        method,
        headers: encoded
          ? {
              "Content-Type": "application/json",
              "Content-Length": encoded.length,
            }
          : {},
      },
      (res) => {
        let data = "";
        res.setEncoding("utf8");
        res.on("data", (chunk) => {
          data += chunk;
          if (data.length > 4 * 1024 * 1024)
            req.destroy(new Error("response_too_large"));
        });
        res.on("error", reject);
        res.on("end", () => {
          try {
            const value = JSON.parse(data);
            if (res.statusCode >= 400)
              reject(new Error(value.error || "service_error"));
            else resolve(value);
          } catch {
            reject(new Error("invalid_service_response"));
          }
        });
      },
    );
    req.setTimeout(5000, () => req.destroy(new Error("service_timeout")));
    req.on("error", (err) =>
      reject(
        new Error(
          err.code === "ENOENT" || err.code === "ECONNREFUSED"
            ? "service_unavailable"
            : err.message,
        ),
      ),
    );
    req.end(encoded);
  });
}

function handler(name, fn) {
  ipcMain.handle(name, (event, input) => {
    if (
      !window ||
      event.sender !== window.webContents ||
      event.senderFrame !== event.sender.mainFrame ||
      event.senderFrame.url !== pathToFileURL(index).href
    ) {
      throw new Error("unauthorized_frame");
    }
    return fn(input);
  });
}

function createWindow() {
  window = new BrowserWindow({
    width: 1240,
    height: 860,
    minWidth: 920,
    minHeight: 700,
    title: "协作团队",
    backgroundColor: "#f6f5f1",
    webPreferences: {
      preload: path.join(__dirname, "preload.cjs"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  });
  window.webContents.setWindowOpenHandler(() => ({ action: "deny" }));
  window.webContents.on("will-navigate", (event) => event.preventDefault());
  window.on("closed", () => {
    window = undefined;
  });
  return window.loadFile(index);
}

app.whenReady().then(async () => {
  handler("team:snapshot", () => request("GET", "/v1/snapshot"));
  handler("team:create-project", (input) =>
    request("POST", "/v1/projects", input),
  );
  handler("team:create-goal", (input) => {
    if (
      !input ||
      typeof input.project_id !== "string" ||
      !/^[a-f0-9]{32}$/.test(input.project_id)
    )
      throw new Error("invalid_input");
    const { project_id, ...body } = input;
    return request("POST", `/v1/projects/${project_id}/goals`, body);
  });
  handler("team:select-directory", async () => {
    const result = await dialog.showOpenDialog(window, {
      title: "选择项目目录",
      properties: ["openDirectory"],
    });
    return result.canceled ? null : result.filePaths[0];
  });
  await createWindow();
});

app.on("activate", () => {
  if (BrowserWindow.getAllWindows().length === 0) void createWindow();
});
app.on("window-all-closed", () => app.quit());
