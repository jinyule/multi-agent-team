# 依赖与分发清单

当前依赖事实由 `go.mod`/`go.sum` 和 `desktop/package.json`/`package-lock.json` 管理。直接依赖固定版本，传递依赖由 lockfile 或 Go 模块图固定；升级需说明原因并通过相同门禁。下表说明用途，不代替完整锁文件。

| 直接依赖 | 固定版本 | 用途与进入产物的方式 |
| --- | --- | --- |
| modernc.org/sqlite | v1.58.0 | 无 CGO SQLite 驱动，编入 teamd |
| react / react-dom | 19.3.0 | 桌面视图与 DOM 渲染，连同 scheduler 打入 renderer |
| electron | 44.3.0 | macOS 桌面运行时；其二进制和上游 notices 随 `.app` 分发 |
| @electron/packager | 20.3.0 | 将 main/preload/renderer 组装成 `.app`；仅构建期使用 |
| playwright | 1.63.0 | 控制真实 Electron 窗口并读取实际 UI；仅测试使用 |
| typescript | 7.0.2 | strict 类型检查；仅构建期使用 |
| vite | 8.3.0 | renderer 构建；仅构建期使用 |
| prettier | 3.9.6 | 格式门禁；仅开发/CI 使用 |
| yaml | 2.9.1 | 解析 workflow 后检查结构和拒绝规则；仅开发/CI 使用 |
| @types/node | 24.13.4 | Node 类型；仅类型检查 |
| @types/react / @types/react-dom | 19.3.0 | React 类型；仅类型检查 |

本次工程门禁新增 Packager 与 YAML 解析器；其余为已有桌面和测试依赖。没有复制 deepseek-harness 的源码、品牌或许可证，也没有引入 Cordis。

验证工具链固定为 Go 1.27.0、Node.js 24.16.0；actionlint 通过显式 `v1.7.12` Go 模块调用，GitHub Actions 固定在 workflow 中列出的提交 SHA。Python 使用 runner/本机 Python 3 标准库，最低需要 3.9；当前实际版本记录在验证报告。当前未引入 Python 第三方依赖。

## 开发包中的许可证材料

`scripts/release.py` 从实际依赖位置读取 notices，不用手写摘要代替上游正文：

- Electron 的 `LICENSE` 与 `LICENSES.chromium.html` 原样保留，缺失时产物验证失败。
- `THIRD_PARTY_NOTICES.txt` 包含 Go runtime/标准库许可、teamd 实际导入模块的 LICENSE/COPYING/NOTICE，以及 React、React DOM、scheduler 的版本和许可原文。Go 许可支持官方发行目录和 Homebrew 的 `libexec` 布局；不依赖未参与构建的工具模块缓存。缺失或读取失败会阻止打包。
- Packager、Playwright、类型和 lint 工具不进入应用运行时；完整开发依赖仍可从锁文件审计。

包内没有代码仓库、用户数据库或 `.env`，只复制指定入口与 renderer 构建目录；当前 renderer 保留构建 source map 用于开发预览诊断。项目本身的公开分发许可尚未选择，因此现有第三方 notices 不代表已经授予本项目公开分发许可。正式交付前需确定产品许可、签名和公证流程，见[发布规则](release.md)。
