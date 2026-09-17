# 协作团队

个人使用的多 Agent 编程平台。采用独立 macOS 桌面与本机任务服务：固定团队承担需求、开发、测试、检视和交付准备，用户负责方向及最终决定。

当前处于基础实现阶段。已选首轮引擎为 **Codex 与 Claude Code**，随后接入 nano-harness。真实引擎执行、团队调度及面向受托项目的质量门禁、PR/CI 和发布准备尚未交付；当前目标保持“等待引擎接入”，不会自动运行编程任务。本仓库自身已建立工程门禁、GitHub workflow 和开发包验证。

## 当前工作台

- 注册本地项目，提交并查看目标。
- 显示六名固定专业成员，身份保存在服务中。
- SQLite 事务持久化，重复请求去重，事件支持游标续读。
- 桌面与服务独立运行，关闭桌面后服务保留；服务重启后读取原状态。

## 本机运行

开发验证环境为 Apple Silicon macOS、Go 1.27、Node.js 24。模块声明的 Go 最低版本为 1.25；其他版本/系统尚未完成验证。

先安装桌面依赖并构建：

```sh
npm --prefix desktop ci
make build desktop-build
```

在独立终端启动本机服务：

```sh
./bin/teamd
```

再启动桌面：

```sh
npm --prefix desktop start
```

服务默认把状态放在 `~/Library/Application Support/multi-agent-team`。服务终端按 Ctrl+C 可正常停止；退出桌面不停止服务。当前没有自动注册登录启动或安装系统服务。

调试时可给服务传 `--state-dir /absolute/private/directory`，并为桌面设置同一路径的 `TEAM_STATE_DIR`。使用本产品专用的私有目录，不指向代码仓库或共享目录。

## 验证

```sh
make check
```

包括格式/类型/workflow 规则、门禁负向测试、Go race 与逐文件 ≥80% 覆盖率、SQLite/HTTP 集成、真实服务进程和 Electron 窗口 E2E。必需 Go/桌面测试拒绝空测试和跳过。桌面 E2E 使用临时项目与数据目录，不调用模型、不改用户项目。临时窗口会在测试结束后关闭。

`make hooks` 安装本仓库 hook，推送前执行 `make check`。`make package` 构建 macOS arm64 开发 zip，启动包内应用验证，再检查解包后的文件/hash；`make release-verify` 核对现有包。当前包未签名、未公证，服务须手动启动。线上 CI、分支保护与发布审批尚待配置远端后验证。

TDD 证据记录工具可保存命令输出、退出码、文件摘要和运行前源码归档：

```sh
python3 scripts/capture-check.py green go test -race -count=1 ./...
```

这些是当前开发阶段的验证记录；完整产品的受控证据存储与门禁执行器仍在后续里程碑中。

## 设计与进度

- [需求草案](docs/product-requirements-draft.md)
- [已接受的架构决定](docs/adr/001-local-service.md)
- [模块与接口设计](docs/architecture.md)
- [里程碑与能力状态](docs/implementation-plan.md)
- [nano-harness 验收案例](docs/nano-harness-acceptance-candidates.md)
- [仓库规则](AGENTS.md)与[工程规范](docs/development.md)
- [测试与 E2E](docs/testing.md)、[CI/CD 与发布](docs/release.md)
- [参考项目工程分析](docs/reference/deepseek-harness-engineering.md)
- [依赖与分发清单](docs/dependencies.md)、[本地验证记录](docs/verification/engineering-gates.md)
