# 测试与 E2E 规范

## 验证分层

单元测试描述行为与边界；SQLite/HTTP 集成使用真实数据库和事务；服务 E2E 启动实际编译的 teamd；桌面 E2E 启动实际 Electron main/preload/renderer，通过可访问控件提交目标。打包 E2E 使用候选目录中的 `.app` executable 与 teamd，不回退源码或开发 binary。

只在模型、第三方网络、时钟等昂贵/不确定边界使用可控替身。当前没有真实 Agent adapter，因此所有必需 E2E 无密钥、不花模型额度。它们证明当前工作台而非 M2–M5 的 Agent 编程闭环。

## TDD 与证据

新增行为先编写能暴露缺口的断言，观察有效失败，再实现。缺依赖、编译环境未准备好不算有效 RED；新增符号尚未实现的编译失败需要补充运行到断言的行为失败才能作本仓库证据。`scripts/capture-check.py` 保存命令、退出码、源码摘要和运行前归档，排除 secrets、符号链接和产物；它是开发辅助，不能防止同权限进程伪造结果。失败和修复过程保留，禁止把 green 目录名等同于成功。

覆盖率门禁自动发现所有 `internal/**/*.go` 产品文件，每文件 ≥80% Go 语句；缺少报告或新文件未覆盖即失败。组合入口通过真实 binary E2E 验证；Go 数字不包含 renderer/JS，也不代表权限条件都被检验。新条件增加允许/拒绝、成功/失败或生命周期回归测试；不得为达标增加无意义 getter/assertion 或生产测试开关。

`scripts/run-go-tests.py` 保存真实 `go test -json` 的事件与 stderr，并检查每个必需包至少执行一个测试且得到最终成功结果。任何测试/包 skip、fail、缺失最终结果或空集合均失败；同时检查 Go 进程退出码。Go 测试设置 60 秒超时，CI job 另有总超时。

## 当前桌面验收矩阵

| 场景 | 操作 | 外部可观察结果 |
| --- | --- | --- |
| 离线启动和恢复 | 先开桌面，启动服务，再注册项目/保存目标 | 真实连接状态变化，API 中出现一份目标 |
| 服务异常终止 | SIGKILL 后保持窗口，再启动服务 | 保存禁用、旧目标保留、重连后持久数据一致 |
| 桌面退出/重开 | 退出 app，再开同一状态目录 | service health 仍正常、目标未丢失 |
| 无效/重复项目 | 不存在的目录、重复注册 | 有错误反馈，DB 无额外项目 |
| 多项目切换 | 不同目录各保存目标并切换 | UI 与服务各自归属正确，不串项目 |
| 渲染权限 | 查看 renderer 可用 API | 无 Node require/process，仅四个固定 bridge 方法 |
| 产品语义与布局 | 对照成员/目标/状态/按钮快照，1240×860 与 920×700 | 语义一致、无横向溢出、页面非空、无错误 overlay、无 renderer error |
| 文件不变 | 项目放置哨兵文件，流程后独立读取 | 文件字节未被注册/保存操作改变 |

首发是 macOS 桌面，920×700 为支持的最小窗口；没有移动端验收承诺。原生目录选择器、签名、公证、真实引擎、远端 forge 尚不在当前自动化覆盖范围。

## 执行与快照

`make e2e` 是可复现入口。底层 `python3 scripts/run-desktop-e2e.py` 生成新 JUnit，并强制至少三条实际用例，任何 skipped/error/failure 都失败。没有自动 retries 将不稳定测试变绿；重跑仍保留原失败。

语义基线位于 `desktop/e2e/snapshots/workspace.json`。正常运行只读。需要更新时在本机显式 `TEAM_UPDATE_SNAPSHOTS=1 make desktop-test`，审阅差异；CI 环境下禁止更新。不把随机路径/ID写入预期值，也不规范化真正的状态或内容。

E2E 每例拥有临时私有数据、代码哨兵、进程和窗口；以 health 请求等条件等待，少量有界轮询用于进程就绪，不能把固定 sleep 当成功证据。清理先请求停止，限时后终止，再等退出；不得删除用户目录或向全局进程名发送 kill。测试进程只继承需要的基础环境，模型 secrets 不透传。

`TEAM_QA_DIR` 可指定报告目录，默认 `/tmp/multi-agent-team-qa`。截图、QA JSON 和 JUnit 不提交源码；CI 作为 artifact 上传，缺文件即失败。历史 M1 报告不覆盖新验证；源码 snapshot 与失败日志按需要通过 capture-check 另存。

## 后续真实引擎测试

Codex 与 Claude Code 各自验证会话、工具审批、取消、恢复和用量，再验证平台完整开发任务。必要凭据的 trusted lane 先 preflight，缺失必须报“未验证/失败”；不得给 fork PR 发秘密、使用 pull_request_target 执行未审代码，或让无模型 E2E 冒充真实模型成功。工具写入以文件和执行事实为准。
