# Agent Note: 工程门禁、真实桌面 E2E 与开发候选发布

Status: implemented

## Problem

基础仓库只有 make test/build 和单条窗口正常流程，没有可执行覆盖率、CI 汇总、发布字节校验或工程规则。现有 capture 脚本可把任意本机文件加入源码归档；窗口关闭后的清理也会重新访问失效的 Playwright 对象。

## Decision

参考 deepseek-harness 的真实入口验证、生命周期所有权、分层门禁和 artifact-first 发布方式，为当前 Go/Electron 仓库建立 AGENTS、工程/测试/发布文档及共享 Makefile。规则程序有负向测试，CI 缺失、失败、取消、跳过都不能形成通过。

以真实 SQLite/HTTP 和 Electron 进程验证项目/目标持久性、断线恢复、项目隔离、无 Node renderer、语义输出与窗口布局。测试保存启动时的进程句柄、等待退出，并把临时证据放在可配置的外部目录。归档只纳入显式源码类型，排除秘密与符号链接。

macOS 开发 zip 使用实际 `.app` 和独立 teamd 重跑相同 E2E，源码和产物清单/hash 一同记录。发布仅手动、校验 tag/main 与清洁 commit，受保护环境通过后上传同一次构建字节；当前未签名/未公证，只允许开发 prerelease。

收尾审计补齐 Go JSON 结果的空集合/skip/缺失拒绝；CI 新增 job 必须同时进入 needs 和结果校验参数。打包开始和验证完成时核对源码清单，缺少必需运行时许可证也拒绝候选。各缺口先保留断言失败，再实现修复；实际证据见[验证记录](../../docs/verification/engineering-gates.md)。

## Alternatives considered

**完整照搬参考仓库**：Cordis、TypeScript 多包、双语生成站点、Windows/Wine 与企业 runner 不符合本项目已选 Go 服务 + Electron 结构，未引入。

**所有文件立刻逐行 100%**：Go 语句、JS 分支和真实 GUI 的指标不等价；先强制 internal Go 每文件 80% 以及实际拒绝/恢复场景。没有掩盖未覆盖底层错误分支，也不为达标加生产测试开关；提高目标以有价值的回归覆盖推进。

**只写规范不执行**：不能证明规则拒绝错误，因此新增 rules tests、workflow lint、JUnit 反空/反跳过、快照和产物篡改检查。

**直接发布开发目录或发布时重建**：会脱离测试过的对象。显式打包、从包启动、清单比较和不重建的发布 job 保持证据对象一致。

## Consequences

每次完整门禁成本增加，但正常流程、恢复和产物入口都受检查。macOS arm64 本机候选和 GitHub Actions 已有真实运行证据；正式签名、公证、真实双引擎仍需后续里程碑。当前私有仓库方案不支持所需的分支保护和 environment reviewer，手动审批要求虽已写入规则和 workflow，仍不能称平台保护生效。
