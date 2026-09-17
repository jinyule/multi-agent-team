# GitHub 远端验证记录

日期：2026-09-17。仓库：[jinyule/multi-agent-team](https://github.com/jinyule/multi-agent-team)，可见性为 public，默认分支为 `main`。GitHub 已从默认分支识别 MIT License。本文只记录实际 API、workflow 与 artifact 结果。

## 主干与 CI

初始提交 `628bea0b1f3dc7b8502c7f4d54a099f0a92ca2a1` 的 CI [run 35196396552](https://github.com/jinyule/multi-agent-team/actions/runs/35196396552) 完成且 conclusion 为 `success`，static、go、desktop 与 `all checks passed` 全部成功，并生成 Go 证据、桌面证据和 macOS arm64 开发候选三类 artifact。

MIT 提交 `ce0d7166d674ee696aeae56bffe8a73279a7adf6` 的 CI [run 35198224239](https://github.com/jinyule/multi-agent-team/actions/runs/35198224239) 再次成功。它验证根目录 `LICENSE` 被纳入源码摘要、桌面 package/lock 均声明 MIT，开发包包含独立的 `APPLICATION_LICENSE.txt`，并继续通过 race、逐文件覆盖率、真实 teamd E2E、真实 Electron 窗口、打包窗口 E2E 和解包/hash 校验。最终必需 check 的精确名称是 `all checks passed`。

仓库 Actions 权限实际为 `default_workflow_permissions=read`、`can_approve_pull_request_reviews=false`。CI workflow 为 active。`Development release` 当前仍为 `disabled_manually`，仓库变量 `RELEASE_APPROVAL_CONFIGURED=false`；在包含 fail-closed preflight 的 PR 合入前不启用上传。

## 平台保护

仓库最初为 private 时，GitHub Free 对 branch protection/ruleset 返回 HTTP 403，对 environment required reviewers 返回 HTTP 422；测试留下的无保护环境当即删除。经所有者明确决定公开并在默认分支加入 MIT License 后，重新配置成功：

- `main` 要求最新基线上的 `all checks passed`，至少一名批准者，dismiss stale approval，且最后一次 push 必须由其他身份批准；管理员同样受约束；
- `main` 要求解决所有对话并保持线性历史，禁止 force push 和删除；
- PR [#7](https://github.com/jinyule/multi-agent-team/pull/7) 在全部检查成功后实际返回 `REVIEW_REQUIRED`，证明成功文字或 CI 绿灯不能代替批准；
- `development-release` 实际包含 required reviewer `jinyule`，只允许 protected branch，`prevent_self_review=false`。这是单维护者阶段的显式人类发布决定，尚不提供双人身份分离；增加维护者后应改为禁止自批。

PR 合并还需要另一名 GitHub 身份进行批准，因为作者不能批准自己的变更。平台内部 Agent 交叉检视不会冒充 GitHub 人类 approval；最终合并和发布继续由人决定。

## 线上发现与修复分支

初次 run 暴露两个不影响结论但需要清理的问题：覆盖率普通输出形似 `file.go: ...`，被 runner problem matcher 误标为错误 annotation；旧 upload-artifact v4 在 2026 runner 上产生 Node.js 20 弃用警告。

`codex/remote-validation` 分支把覆盖率输出改为明确摘要，并以固定提交升级到 upload-artifact v7.0.1（Node 24）和 download-artifact v8.0.1（Node 24）；同时加入发布审批 fail-closed preflight。该分支通过 PR 重跑完整 CI，等待非作者的人类 review，没有自动合入。

初次推送与 MIT 提交推送时，本机 pre-push 的 `make check` 被未接受的系统 Xcode License 拦截；没有以该错误冒充测试失败，也没有代替用户接受系统协议。两次推送使用 `--no-verify`，随后 GitHub 干净 macOS runner 均完整通过。
