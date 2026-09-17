import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { createRoot } from "react-dom/client";
import type { Snapshot } from "./types";
import "./style.css";

const empty: Snapshot = {
  protocol_version: 1,
  projects: [],
  members: [],
  goals: [],
  cursor: 0,
};
const duties: Record<string, string> = {
  lead: "协调与交付",
  product: "需求与验收标准",
  architecture: "方案与技术边界",
  development: "测试先行开发",
  testing: "独立验证成果",
  review: "交叉检视变更",
};
const initials: Record<string, string> = {
  lead: "领",
  product: "产",
  architecture: "架",
  development: "开",
  testing: "测",
  review: "审",
};

function errorMessage(error: unknown) {
  const message = error instanceof Error ? error.message : String(error);
  if (message.includes("request_conflict"))
    return "请求内容有冲突，或这个目录已经注册。请检查后重试。";
  if (message.includes("invalid_input"))
    return "请检查名称、说明和项目目录，目录必须真实存在。";
  if (message.includes("service_unavailable"))
    return "本机服务未连接，请先启动服务。已保存的目标仍会保留。";
  if (message.includes("service_timeout"))
    return "服务暂未响应。再次保存会重试同一个请求，不会重复创建。";
  if (message.includes("protocol_mismatch"))
    return "桌面与本机服务版本不兼容，请使用配套版本。";
  return "操作未完成，请检查本机服务状态后重试。";
}

function useRequestID() {
  const pending = useRef<{ fingerprint: string; id: string } | null>(null);
  const next = (input: unknown) => {
    const fingerprint = JSON.stringify(input);
    if (!pending.current || pending.current.fingerprint !== fingerprint)
      pending.current = { fingerprint, id: crypto.randomUUID() };
    return pending.current.id;
  };
  return [
    next,
    () => {
      pending.current = null;
    },
  ] as const;
}

function App() {
  const [snapshot, setSnapshot] = useState(empty);
  const [connection, setConnection] = useState<
    "connecting" | "connected" | "disconnected"
  >("connecting");
  const [connectionError, setConnectionError] = useState("");
  const [selected, setSelected] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [projectName, setProjectName] = useState("");
  const [projectPath, setProjectPath] = useState("");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const projectDialog = useRef<HTMLDialogElement>(null);
  const [projectRequestID, clearProjectRequest] = useRequestID();
  const [goalRequestID, clearGoalRequest] = useRequestID();
  const project =
    snapshot.projects.find((p) => p.id === selected) || snapshot.projects[0];
  const goals = snapshot.goals.filter((g) => g.project_id === project?.id);

  const refresh = useCallback(async () => {
    try {
      const value = await window.team.snapshot();
      if (value.protocol_version !== 1) throw new Error("protocol_mismatch");
      setSnapshot((current) =>
        value.cursor >= current.cursor ? value : current,
      );
      setConnection("connected");
      setConnectionError("");
    } catch (error) {
      setConnection("disconnected");
      setConnectionError(errorMessage(error));
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout>;
    async function poll() {
      await refresh();
      if (!cancelled) timer = setTimeout(poll, 2000);
    }
    void poll();
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [refresh]);

  async function saveProject(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    const input = { name: projectName.trim(), path: projectPath.trim() };
    try {
      const value = await window.team.createProject({
        ...input,
        request_id: projectRequestID(input),
      });
      await refresh();
      clearProjectRequest();
      setSelected(value.id);
      projectDialog.current?.close();
      setProjectName("");
      setProjectPath("");
    } catch (error) {
      setError(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function saveGoal(event: FormEvent) {
    event.preventDefault();
    if (!project) return;
    setBusy(true);
    setError("");
    const input = { project_id: project.id, title: title.trim(), description };
    try {
      await window.team.createGoal({
        ...input,
        request_id: goalRequestID(input),
      });
      await refresh();
      clearGoalRequest();
      setTitle("");
      setDescription("");
    } catch (error) {
      setError(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function chooseDirectory() {
    try {
      const value = await window.team.selectDirectory();
      if (value) setProjectPath(value);
    } catch (error) {
      setError(errorMessage(error));
    }
  }

  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">协</span>
          <div>
            <strong>协作团队</strong>
            <small>个人工作空间</small>
          </div>
        </div>
        <div className="workspace-label">
          工作台 <span>开发预览</span>
        </div>
        <div className="sidebar-heading">
          我的项目 <span>{snapshot.projects.length}</span>
        </div>
        <nav aria-label="项目列表">
          {snapshot.projects.map((p) => (
            <button
              key={p.id}
              className={`project-button ${p.id === project?.id ? "selected" : ""}`}
              onClick={() => setSelected(p.id)}
            >
              <span className="project-icon">▧</span>
              <span>{p.name}</span>
            </button>
          ))}
          {snapshot.projects.length === 0 && (
            <p className="sidebar-empty">
              添加一个本地项目，开始记录你的目标。
            </p>
          )}
        </nav>
        <button
          className="add-project"
          aria-label="添加项目"
          onClick={() => {
            setError("");
            projectDialog.current?.showModal();
          }}
        >
          ＋ 添加项目
        </button>
        <div className="sidebar-bottom">
          <span className="eyebrow">首轮引擎</span>
          <p>
            Codex <span>待接入</span>
          </p>
          <p>
            Claude Code <span>待接入</span>
          </p>
          <div className="sidebar-note">
            在本机工作
            <br />
            由你决定方向与最终交付
          </div>
        </div>
      </aside>

      <main className="content">
        <header>
          <div>
            <span className="eyebrow">你的方向 · 团队的工作</span>
            <h1>目标与团队</h1>
          </div>
          <div className={`connection ${connection}`}>
            <span />
            {connection === "connected"
              ? "本机服务已连接"
              : connection === "connecting"
                ? "正在连接本机服务"
                : "本机服务连接中断"}
          </div>
        </header>
        {connectionError && (
          <div role="status" className="notice warning">
            {connectionError}
            <button onClick={() => void refresh()}>重新连接</button>
          </div>
        )}
        <section className="overview">
          <div>
            <span className="eyebrow">当前项目</span>
            <h2>{project?.name || "从一个项目开始"}</h2>
            <p className="project-path">
              {project?.path || "连接本地目录，为团队建立共同的工作上下文。"}
            </p>
          </div>
          <div className="count">
            <strong>{goals.length}</strong>
            <span>已保存目标</span>
          </div>
        </section>

        <section className="team-section" aria-labelledby="team-title">
          <div className="section-heading">
            <h2 id="team-title">你的固定团队</h2>
            <span>身份持续保留 · 按需执行</span>
          </div>
          <div className="members">
            {snapshot.members.map((member) => (
              <article className="member" data-testid="member" key={member.id}>
                <div className={`avatar ${member.role}`}>
                  {initials[member.role]}
                </div>
                <h3>{member.name}</h3>
                <p>{duties[member.role]}</p>
                <span className="member-state">尚未连接引擎</span>
              </article>
            ))}
          </div>
        </section>

        <section className="compose-grid">
          <form className="composer" onSubmit={saveGoal}>
            <div className="section-heading">
              <h2>给团队一个目标</h2>
              <span>先描述你想达成什么</span>
            </div>
            <label htmlFor="goal-title">目标标题</label>
            <input
              id="goal-title"
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              placeholder="例如：为项目增加外部程序控制接口"
              maxLength={80}
              required
              disabled={!project}
            />
            <label htmlFor="goal-description">目标说明</label>
            <textarea
              id="goal-description"
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder="期望的结果、已有约束，以及你在意的取舍。"
              maxLength={8000}
              rows={3}
              disabled={!project}
            />
            <div className="form-footer">
              <span>
                {project ? "目标保存到本机服务" : "先添加或选择一个项目"}
              </span>
              <button
                className="primary"
                disabled={!project || busy || connection !== "connected"}
              >
                {busy ? "正在保存…" : "保存目标"}
              </button>
            </div>
          </form>
          <aside className="stage-card">
            <span className="eyebrow">现在可以做什么</span>
            <h2>方向先留在这里</h2>
            <p>注册项目、记录目标，并保留团队的固定身份。</p>
            <p>
              Codex 与 Claude Code
              接入后，目标才能进入规划和执行。当前不会自动修改代码。
            </p>
            <div className="stage-footer">关闭桌面，已保存的内容仍会保留。</div>
          </aside>
        </section>
        {error && !projectDialog.current?.open && (
          <div role="alert" className="notice warning">
            {error}
          </div>
        )}

        <section className="goals-section" aria-labelledby="goals-title">
          <div className="section-heading">
            <h2 id="goals-title">
              已保存的目标 <span className="number">{goals.length}</span>
            </h2>
            <span>所有状态来自本机服务</span>
          </div>
          {goals.length ? (
            <div className="goal-list">
              {goals.map((goal) => (
                <article className="goal" key={goal.id} data-testid="goal">
                  <div className="goal-marker" />
                  <div className="goal-copy">
                    <h3>{goal.title}</h3>
                    <p>{goal.description || "暂未填写补充说明。"}</p>
                  </div>
                  <span className="goal-status">等待引擎接入</span>
                </article>
              ))}
            </div>
          ) : (
            <div className="empty-goals">
              <span>○</span>
              <div>
                <strong>还没有目标</strong>
                <p>保存第一个目标，让团队知道接下来要做什么。</p>
              </div>
            </div>
          )}
        </section>
      </main>

      <dialog ref={projectDialog} onCancel={() => setError("")}>
        <form onSubmit={saveProject}>
          <span className="eyebrow">本地项目</span>
          <h2>添加项目</h2>
          <p className="dialog-description">
            选择代码所在目录。注册项目不会修改目录中的文件。
          </p>
          <label htmlFor="project-name">项目名称</label>
          <input
            id="project-name"
            value={projectName}
            onChange={(event) => setProjectName(event.target.value)}
            maxLength={50}
            required
            autoFocus
          />
          <label htmlFor="project-path">项目目录</label>
          <div className="path-picker">
            <input
              id="project-path"
              value={projectPath}
              onChange={(event) => setProjectPath(event.target.value)}
              placeholder="/absolute/path/to/project"
              required
            />
            <button type="button" onClick={() => void chooseDirectory()}>
              选择目录
            </button>
          </div>
          {error && (
            <p role="alert" className="field-error">
              {error}
            </p>
          )}
          <div className="dialog-actions">
            <button
              type="button"
              onClick={() => {
                projectDialog.current?.close();
                setError("");
              }}
            >
              取消
            </button>
            <button
              className="primary"
              disabled={busy || connection !== "connected"}
            >
              {busy ? "正在保存…" : "保存项目"}
            </button>
          </div>
        </form>
      </dialog>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
