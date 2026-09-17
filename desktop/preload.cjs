const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("team", {
  snapshot: () => ipcRenderer.invoke("team:snapshot"),
  createProject: (input) => ipcRenderer.invoke("team:create-project", input),
  createGoal: (input) => ipcRenderer.invoke("team:create-goal", input),
  selectDirectory: () => ipcRenderer.invoke("team:select-directory"),
});
