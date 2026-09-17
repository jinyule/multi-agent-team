# CI、CD 与发布规则

## 当前交付形态

当前只构建 **macOS arm64 unsigned-development** 候选：一个 Electron `.app`、独立 teamd、运行说明和实际分发依赖许可证。服务仍手动启动，没有安装登录项；未接入真实 Agent。未完成 Developer ID 签名、公证、安装器、产品许可选择，不能把该 zip 当成面向公众的正式安装包，也不要求用户关闭 Gatekeeper。

代码规则和本地产物可验证。当前 Git 仓库还没有 commit 或 remote，因此没有线上 CI 运行、分支保护或 environment 审批生效的证据。这些部署前提要在实际 GitHub/Gitea 仓库配置后复核，不能仅凭 YAML 声称完成。

## PR 与主干 CI

`.github/workflows/ci.yml` 对所有 PR、main push 和手动运行触发，不做可能跳过必需检查的 paths filter。三个独立 macOS arm64 job：static（格式/类型/vet/规则及 actionlint）、go（race/覆盖率/真实 service E2E）、desktop（真实窗口及打包 E2E）。最后 `all checks passed` 以 always() 执行，只有所有必需 job 均 success 才通过；failure/cancelled/skipped/missing 都失败。

workflow 默认 contents:read，checkout 不持久化凭据，Actions 固定提交 SHA，依赖 immutable install；每个 job 有超时。所有必需测试无密钥。不配置 self-hosted runner 执行不受信任 PR。结果 JSON、JUnit 和截图保留 14 天，开发包 7 天，缺少产物立即失败；实际过期后需重新构建验证。

本地 `make check` 与 CI 使用同一命令，CI 另有工作流语法校验与 package 验证。`make package` 可独立检查产物，但不单独证明源码门禁已通过。

## 从构建到发布

1. 本地 `make check`；`make package` 从显式 app 文件集合构建，不带 node_modules、源码仓库、测试、凭据或本地数据库。Electron 自带运行时，当前没有 SDK worker 的额外 Node 运行时。
2. 使用打包 `.app` 和 teamd 重跑三条真实桌面 E2E。构建前保存源码清单，验证后核对未变化；有变化则失败。通过后记录源码清单、源码摘要、commit、dirty、版本、平台、工具链与所有产物文件 hash、权限和符号链接。
3. 生成 zip、manifest 和 SHA256SUMS，解包并逐项核验，拒绝缺失/增加/改动文件或越界链接。`make release-verify` 允许本地未提交预览；CI 发布校验禁止 dirty/无 commit 候选。
4. `.github/workflows/release.yml` 仅 workflow_dispatch。选择已合入 main 的 `v<desktop/package.json version>` tag；preflight 核对 tag、版本、main 祖先关系、干净工作树。candidate job 跑完整 check、package 和严格 verify。
5. `publish=false`（默认）仅准备候选。需要实际发布时由人手动选择 `publish=true`，publish job 还需 `development-release` 环境审批。它下载同一次运行中的候选，只校验并上传，不安装依赖、不重建、不读取模型凭据。
6. 写权限仅授予 publish job，GH_TOKEN 仅传入发布步骤。生成 release notes 标明 unsigned development 和能力限制，以 prerelease 发布。相同 tag 的已有 Release 不覆盖；远端结果不确定时先人工核对资产 hash，再处理重试。

源码变化使 source_digest 失效；重新构建后的候选是新产物，不能沿用旧审批。签名/公证会改变字节，未来需在校验前加入签名、公证、staple、Gatekeeper 验证并重新做产物 E2E。稳定发布在这些前提完成前不开放。

## 托管平台配置与审计

上线 GitHub 时逐项检查实际状态：

- main 禁止直接推送/force push/删除；必需状态为精确名称 `all checks passed`，严格要求最新 base；适用时使用 merge queue，并核对 merge_group 触发后再启用队列。
- 需要非作者 review，dismiss stale approvals；最新提交再次确认。人类和创建 PR 的机器人身份分离，避免同一账户不能自批。管理员也不默认绕过规则。
- 本平台规定每次合并由人决定；GitHub 原生 approval 与平台 Agent 内部交叉检视分别记录，不能互相冒充。
- `development-release` 配置 required reviewers、禁止自批/绕过、限定版本 tag；缺少 required reviewers 的环境名本身不会提供审批。根据账户方案支持能力检查，不能假定所有仓库都有相同环境保护。
- 发布 tag 保护与维护者权限、Actions 权限、工件留存和默认分支应实查；记录 repository、检查日期与结果。不要让未审核 workflow 变更自行扩大写权限。
- 本仓库尚未配置特定 CODEOWNERS 身份；设置远端时将 workflow、质量脚本、权限/数据库与 release 文件交给实际负责人检视，不创建虚假用户条目。

Gitea 的 Actions、environment、分支规则不保证与 GitHub 等价。当前交付 GitHub workflow 和独立 Makefile 门禁；Gitea 上可调用同样的检查命令，但它的 runner、审批和发布授权需按安装版本单独配置验证。不将 GitHub environment 直接视为 Gitea 已有人审保障。

## 当前证据与外部依据

当前本地检查和产物验证记录在[工程门禁验证记录](verification/engineering-gates.md)。参考项目的规则和差异见[分析](reference/deepseek-harness-engineering.md)。

macOS runner 选择参考 [GitHub hosted runners](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)；环境审查需参考 [GitHub environments](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/manage-environments) 并检查实际仓库；Electron 包装使用 [官方 Packager](https://github.com/electron/packager)。
