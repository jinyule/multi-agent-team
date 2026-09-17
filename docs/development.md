# 工程规范

环境：macOS arm64，Go 1.27.0，Node.js 24.16.0（`.nvmrc`），npm lockfile 安装。`go.mod` 声明语言/依赖最低版本 1.25，当前 CI 只证明 1.27。应用采取[方案二](adr/001-local-service.md)。

## 初始化与日常开发

1. `npm --prefix desktop ci` 安装固定依赖，`go mod download` 下载 Go 依赖。
2. `make hooks` 安装仓库本地 hook；已有其他 hooksPath 时拒绝覆盖。没有全局 Git 修改或自动提交。
3. 阅读当前代码的 AGENTS 和架构；明确行为、验收、失败和退出条件。先跑相关 RED，再实现 GREEN。
4. 更新相关文档和 ADR/Agent Note，审阅差异、运行相关检查。非机械规则修改同时测试规则的拒绝路径。
5. 推送前 `make check`；PR CI 和非作者检视通过后，由人决定主干合并。实现这些项目规则本身不等于平台已支持 Agent 自动检视。

## 命令归属

| 命令 | 门禁内容 | 是否需要构建产物 |
| --- | --- | --- |
| `make quick` | gofmt/格式、固定依赖/文档、workflow 策略、go vet、strict TS、Node 入口语法 | 否 |
| `make test-rules` | Python/Node 规则负向测试 | 否 |
| `make test` | internal 的 Go race 单元/集成测试 | 否 |
| `make coverage` | race + 每个 internal 产品 Go 文件 ≥80% 语句覆盖 | 否 |
| `make service-e2e` | teamd 真实入口、SIGKILL 恢复、单实例、重复请求 | 先构建 teamd |
| `make desktop-test` | 真实 Electron 场景、语义快照、JUnit 非空/无跳过 | 先构建 service 与 renderer |
| `make e2e` | 服务及桌面 E2E | 是 |
| `make check` / `make ci` | 上述全部必需检查 | 是 |
| `make package` | 构建 `.app`/service、打包应用 E2E、zip 和 hash 清单、解包校验 | 是 |
| `make release-verify` | 重新核对已有候选字节与当前源码摘要，允许本地 dirty 预览 | 已有候选，不重建 |

当前必需矩阵很小，上库前完整 check 可接受。局部修改可先用具体 `go test -run` 或单个 E2E 定位；完整检查通过后不因“再保存一次”反复跑。打包后的源文件变化使候选验证失效，需要按影响重建。

## 编码与边界

- Go import 按标准库/项目组组织，gofmt、vet、race 强制；返回错误携带操作上下文，避免吞掉损坏/权限失败。导出接口说明前置条件、结果与生命周期。
- React strict TS，预加载只暴露固定方法；renderer 处理真实断线与未知状态，不展示未经验证的成功。Node 入口 CJS 是 Electron preload 沙箱所需，renderer 与脚本使用各自声明的模块模式，不套用参考仓库 ESM-only。
- 尽量用维护中的依赖减少自有实现；新增依赖固定直接版本，提交完整锁文件。发布保留实际分发组件许可证，不自行给参考项目源码改许可。
- 当前版本、引入目的与分发方式见[依赖清单](dependencies.md)。
- 同步修改 API、前后端类型、测试和文档；磁盘格式/协议变更写 ADR，拒绝未来版本并保留失败证据。不要默认删库恢复。
- 目标、权限、证据、预算和决策分离；未经实现的能力留在计划，不靠 UI 文案宣称完成。

## 文档与变更记录

当前行为放架构、测试、发布等归属文档；理由放 `docs/adr` 或 `.agents/notes`。非机械变更新增/更新 Agent Note（问题、决定、实际备选、代价、验证）。机械格式修改在 PR 中说明即可。历史证据可保留本机路径和限制；新 clone 无需存在这些历史文件。

仓库内部链接用相对路径，回答用户时使用绝对本机链接。源码引用、示例和所有命令应可从仓库根执行。研究材料、愿景和未实现计划明确标注，不升级为当前实现事实。
