# deepseek-harness 工程规范分析与本仓库适配

分析日期：2026-09-12。参考目录 `/opt/code/open-source/agent/deepseek-harness`，读取时提交 `52b21429d0560e137c04746f7a3cea09822a8120`，工作树干净。本文基于当前源码和 workflow，不将 proposed/archived Agent Notes 当作已经实施的规则。未修改参考仓库，也未运行其庞大测试矩阵。

## 分析范围与证据

已读取根 `AGENTS.md`、`CONTRIBUTING.md`、`docs/architecture.md`、`docs/development.md`、`docs/testing.md`、`docs/defensive-patterns.md`、`.agents/notes/README.md`、`lefthook.yml`；核对 `.github/AGENTS.md`、PR 模板、Dependabot，以及 `ci.yml`、`ci-master.yml`、`e2e.yml`、`release.yml`、`release-publish.yml` 的实际命令、权限、依赖和失败判定；检查 `scripts/run-gates.ts` 的任务依赖、失败/跳过汇总与源码/产物门禁职责。

这些路径相对于参考目录。该目录只是调查资料，不是当前 CI 的依赖；clone 当前仓库后不需要它存在。

## 1. 结构与生命周期规范

参考项目采用 Cordis 插件树，服务、事件和注册均作为可撤销 effect；它强调能力的定义、提供者和消费者完整对应。对本项目有价值的是明确所有权、组合入口与退出条件，而不是照搬插件框架。

本项目仍保持已选架构：Go 服务的组合入口、HTTP API、数据库模块分开；Electron main/preload/renderer 分开；引擎未来通过适配器接入。资源创建者负责关闭，清理须等待进程停止；超时、信号和退出码分别报告。对应 [AGENTS](../../AGENTS.md)、[架构](../architecture.md) 与 daemon/真实进程 E2E。

参考项目的 pre-release 规则允许重构并明确拒绝不认识的磁盘格式。本项目也通过 schema/protocol 版本显式拒绝不兼容状态，但保留已有用户目标，不能为了重构默认删库。当前对未来 schema、损坏 DB、普通文件占据 socket、非私有状态目录都有失败验证。

## 2. 测试分层与有效证据

参考项目将单元测试、逐文件覆盖率、真实 API、无密钥快照、真实浏览器快照和构建产物 smoke 分开。最关键的规则是只替换不确定的外部依赖，验证实际文件/持久状态，并走真正的应用入口；库测试不能替代组装后的产品。

本项目按同样原则分层：真实 SQLite/HTTP 集成；真实 teamd binary 重启/单实例 E2E；真实 Electron 窗口；读写无密钥语义快照；打包后的 `.app` 与独立 service 重跑同一组桌面场景。项目文件必须保持原字节。快照规范化不隐藏目标、状态、成员或按钮等语义；不通过自动更新快照把差异洗成通过。

参考项目自带服务商背景，明确不限制真实 API 测试投入；这项成本政策不适用于个人平台。当前无真实引擎实现，因此必需 E2E 无密钥、无模型调用。M2 要增加有界真实 Codex/Claude Code 接入验证；未来有密钥测试独立显式执行，缺少配置不能显示“已通过”。

## 3. 覆盖率：采用逐文件门禁，明确指标差异

参考项目要求核心 TypeScript 产品文件逐文件 100%。当前仓库是 Go + Electron，不能把 Go 语句覆盖率、TS 行/分支覆盖率和窗口 E2E 混成同一数字。

当前制定：`internal/**/*.go` 每个产品文件至少 **80% Go 语句覆盖率**，自动发现新增文件，缺失报告按失败。新权限/状态拒绝条件都要有成功和失败测试；覆盖率不能替代这些验收。`cmd/teamd/main.go` 为进程组合入口，由真实 binary 测试验证，明确不计入该数字。Electron 以 strict TypeScript、IPC 静态检查、真实窗口隔离/交互/语义快照与产物测试门禁，不宣称已有 JS 数值覆盖率。

选择 80% 是针对现阶段 Go 错误处理和资源管理的工程默认，不是把参考项目的 100% 偷换成已达成。新增了真实损坏数据、重试、分页、权限和退出测试，未为触达不可稳定制造的 OS/SQLite 底层错误而引入生产测试开关。将来提高阈值要以有意义的验证或删除死代码推进；降低现有阈值必须提交人的决定。

## 4. 本地检查、CI 与必需汇总

参考项目本地按修改表面选检查，CI 承担完整矩阵；多个独立 job 最后汇总为稳定的 `all checks passed`。汇总使用 `always()`，将失败、取消和跳过都视为失败，避免依赖失败导致汇总跳过而误放行。live API job 在需要凭据的场景先做 preflight，拒绝全跳过假绿。

当前仓库以 Makefile 为共享命令：quick、规则测试、coverage、服务 E2E、桌面 E2E、package。PR/main CI 拆静态、Go 和桌面/产物三条 macOS arm64 lane；稳定汇总包含全部必需 job。门禁自身有负向测试，拒绝漏列 job、未固定 Action、缺失/跳过测试及发布重建。当前仅正式支持 macOS，不搬入参考项目的 Windows/Wine、Python SDK 和私有 runner failover。

本地 pre-commit 只校验 staged whitespace，pre-push 跑 `make check`，符合用户要求的上库前门禁。hook 可绕过，最终必须由托管平台分支保护落实。参考项目的自动 staged 修复、双语 merge driver 和组织 Issue/label 规则未复制。

## 5. 发布：验证同一组字节

参考项目在无密钥 pack 阶段构建并验证 npm tarball，单独手动发布 workflow 只在环境审批后下载并上传产物，不在发布 job 重建。checkout 不保留凭据，秘密只放到真正需要的发布步骤。

当前交付物改为 Electron `.app`、独立 teamd 和许可证说明的 macOS arm64 开发 zip。先从显式文件集合构建，真实运行打包产物 E2E，再生成逐文件/符号链接清单、源码摘要、commit、版本、工具链、SHA256SUMS；解包再次核验。发布 job 无安装/构建，验证相同 bytes 后发布开发 prerelease；手动触发、tag/main 校验、独立环境审批和最小写权限。参考项目的 npm 多包 family/vendor 发布不适用。

本项目于 2026-09-17 独立选用 MIT License；当前仍没有 Developer ID 签名、公证或正式安装器，产物明确标为 unsigned-development。签名、公证后的文件摘要将不同，必须重新做产物验证，不能沿用未签名包的清单。见 [发布规则](../release.md)。

## 6. 文档、贡献和依赖

参考项目把当前事实与决策理由分开，非机械变更同 PR 写 Agent Note，审阅真实备选和代价；归档记录不是当前事实权威。本项目沿用 ADR + Agent Notes，但暂不复制双语 sidecar、生成站点和复杂分类目录。

贡献规则、测试、CI/CD、数据/权限规则各有一个事实归属文件；PR 模板引用它们。固定依赖、锁文件一致性、格式、文档相对链接、冲突标记、workflow 策略进入实际 quick gate。Dependabot 提案依旧经过相同门禁和人的合并决定，不能自动合并。

## 适配清单

| 参考原则 | 当前规则及证明入口 |
| --- | --- |
| effect 所有权/等待退出 | daemon 测试与 Electron harness 有界清理 |
| 真实入口、真实世界 | `make e2e`；检查 API、持久目标和 untouched 文件 |
| 无密钥回放、CI 禁止刷新 | desktop 语义 JSON、只比较测试与 workflow 策略 |
| 每文件覆盖率 | `make coverage`、缺失/低覆盖报告拒绝测试 |
| 汇总不接受 skip | quality.job_failures 与 workflow-policy 负向测试 |
| immutable install | `npm ci`、go.sum、依赖固定版本检查 |
| source/artifact 分离 | `make package`、真实打包应用 E2E、解包/hash 校验 |
| 手动审批后发布相同 bytes | release workflow candidate/publish 分离、环境保护配置清单 |
| 文档和决策随改动 | AGENTS、CONTRIBUTING、ADR/Agent Note、PR 模板 |

以上本地机制可以验证；线上分支保护、环境审批与 CI 运行结果必须以未来实际仓库配置为证。当前无远端，不能声称已生效。
