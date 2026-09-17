# 工程门禁验证记录

日期：2026-09-12。范围：deepseek-harness 工程规则适配、本仓库质量与 CI/CD、真实 service/Electron/开发候选 E2E。线上 CI 和发布环境未验证。

参考仓库 `/opt/code/open-source/agent/deepseek-harness` 的提交为 `52b21429d0560e137c04746f7a3cea09822a8120`，研究和实现后工作树均干净。没有复制参考源码、运行其完整测试矩阵或修改 nano-harness。工程分析与适配理由见[分析](../reference/deepseek-harness-engineering.md)。

## 本机环境与结果

环境：Darwin arm64、Go 1.27.0、Node.js 24.16.0、Python 3.14.7。工作分支 `codex/engineering-gates`；当前没有初始 commit/remote，本地 hook 已设置为 `.githooks`，没有全局 Git 配置修改。

| 检查 | 本地结果与证据边界 |
| --- | --- |
| `make check` | 格式、依赖锁/链接、workflow 策略、vet、strict TS、入口语法、规则测试、race/覆盖率、真实进程与窗口全部通过 |
| 规则负向测试 | 12 条 Python、3 条 Node；拒绝缺失/跳过/失败结果、漏列 CI job、未固定 Actions、自动发布或发布重建、产物缺失/篡改、越界链接及构建期间源码变动；检查 Go 许可目录兼容性 |
| Go 单元/集成 | 21 个实际测试及子测试、3 个必需包，无 skip；使用真实 SQLite、HTTP 与本机生命周期 |
| Go 逐文件覆盖率 | api 69/80（86.2%）；daemon 58/72（80.6%）；store 130/162（80.2%）；门禁 ≥80%，不包含 renderer/JS |
| service E2E | 1 条真实编译入口测试通过；进程恢复、去重、第二实例拒绝、正常退出 |
| 开发入口窗口 E2E | 3 条通过，无 skip，检查真实 API 状态与项目哨兵文件 |
| `.app` 候选窗口 E2E | 同样 3 条从包内 executable 与 teamd 启动并通过，没有回退到开发入口 |
| 工作流语法 | `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 -shellcheck=` 通过；shellcheck 未作为本项目工具，workflow 的 run 步骤为直接命令 |
| 产物一致性 | zip、manifest、SHA256SUMS；解包核对每个文件内容、权限与链接；核对源码摘要。当前 candidate 为 dirty、commit=null，禁止视为可发布候选 |
| 实际窗口检查 | 1240×860、920×700；页面非空、无横向溢出、无错误 overlay，正常/离线/错误反馈可辨认；开发和候选 QA 中 renderer errors/warnings 均为空 |

最终完整执行命令为 `TEAM_QA_DIR=/tmp/multi-agent-team-qa/engineering-final python3 scripts/capture-check.py green make check package`。最新完整输出保存在 `/tmp/multi-agent-team-qa/engineering-final-run.log`；其末尾给出本轮源码归档与 manifest 所在的唯一 evidence 目录。每条执行记录的退出码才是结论，目录名 green 不等于成功。

最终截图、QA JSON、Go JSONL/覆盖率、JUnit 在 `/tmp/multi-agent-team-qa/engineering-final`，包内验证在其 `packaged` 子目录。开发包在 `release/multi-agent-team-0.1.0-macos-arm64-dev.zip`，对应 `release/manifest.json` 与 `release/SHA256SUMS`。这些是本机生成产物，不要求新 clone 存在，也不作为 CI 静态放行依据；CI 总是执行并生成本轮证据。

## 保留的失败与修复

- [最初规则断言 RED](../../artifacts/verification/20260912T103632788754Z-red/output.log)：有效行为失败。更早的缺模块失败属于环境/装配缺口，不算 TDD RED。
- [CI 新增 job 漏验 RED](../../artifacts/verification/20260912T131904595841Z-red/output.log)与[完整 GREEN](../../artifacts/verification/20260912T131917453112Z-green/output.log)：needs 之外，结果校验参数也必须覆盖全部必需 job。
- [Go 空结果/skip 拒绝 RED](../../artifacts/verification/20260912T132144209594Z-red/output.log)与[规则及真实 Go GREEN](../../artifacts/verification/20260912T132224045144Z-green/output.log)：Go 原生命令成功不再绕过必需测试执行判断。
- [源码变动与许可证缺失 RED](../../artifacts/verification/20260912T132321126809Z-red/output.log)：修复后规则测试通过，最终全量记录包含复验。
- [打包装配失败](../../artifacts/verification/20260912T131938678181Z-green/output.log)：Packager 实际使用具名导出，原默认导入无法启动。此为接口装配失败，不算行为 RED；[修复后的真实包验证](../../artifacts/verification/20260912T131947843536Z-green/output.log)通过。
- actionlint 曾拒绝不允许位置的 `runner.temp` context，改为明确的临时证据目录后语法校验通过。
- [完整检查后 Go 许可装配失败](../../artifacts/verification/20260912T132749544739Z-green/output.log)：本机 Homebrew 将 Go LICENSE 放在 `libexec` 上一层，已用完整临时目录结构复现并修复；没有将许可证缺失改成忽略。运行时模块清单改为 teamd 实际导入的模块，空 Go 模块缓存验证输出保存在 `/tmp/multi-agent-team-qa/fresh-module-cache.log`。

## 尚不能声称完成的事项

没有远端，未运行线上 CI、配置 main 分支保护、required reviewers 或 development-release 环境；GitHub YAML 不代表 Gitea 已具有等价保护。未提交、推送、合并或发布，也没有非作者 PR review 结果。实际发布仍需人的明确决定。

当前包未签名、未公证，没有安装器/自动升级/登录项。项目于 2026-09-17 采用 MIT License；这不改变开发包的未签名状态。macOS 原生目录选择器、Intel Mac、真实模型引擎、Agent 自动编程与交叉检视、受托项目 CI/CD 不在这次已通过的验收范围。双引擎仍按[接入契约](../engine-contract.md)在 M2 同轮交付。
