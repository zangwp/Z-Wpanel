# 仓库结构 / Repository layout

根目录保留项目说明、安装入口、Go 模块信息和许可证。源码、网页资源和开发工具按用途归并；目录整理不改变 Nginx/PHP-FPM 架构、服务器安装路径、浏览器资源 URL 或发布附件名称。

The root contains the README, installers, Go module metadata and license. Implementation and assets are grouped by purpose. This retains Nginx/PHP-FPM, installed server paths, browser URLs and release asset names.

| 目录 | 内容 |
|---|---|
| `cmd/yub-wpanel/` | 面板程序入口，以及依赖主包实现的测试 |
| `internal/` | 配置、指标、数据库、请求处理、系统操作、模型、路由和翻译 |
| `web/templates/` | 页面模板，独立嵌入，不作为静态资源公开 |
| `web/static/` | 运行时 CSS、JavaScript 和图标 |
| `web/source/` | 前端构建源码、品牌素材和社区图片，不进入公开静态文件系统 |
| `web/plugins/yub-wpanel-optimizer/` | WordPress 配套插件；网站上仍使用原插件目录名 |
| `deploy/` | 短安装入口、统计 Worker 和 `tools/` 开发验证脚本 |
| `docs/` | 英文 README、安装与维护指南、迁移说明和安全文档 |
| `tests/` | 跨模块、安装器、前端和发布流程验证 |
| `third_party/` | 项目声明、第三方声明和固定版本许可材料 |
| `.github/workflows/` | CI、原生双架构验证、签名及发布 |

## 构建与验证

在仓库根目录执行；完整运行测试使用受支持的 Linux 环境。

```bash
go test ./...
go vet ./...
go build -o yub-wpanel ./cmd/yub-wpanel
bash deploy/tools/verify.sh
```

Backend packages use Go's `internal` boundary. Both release architectures build `./cmd/yub-wpanel`. Templates, public static files and plugin sources have separate embedded filesystems in `web/assets.go`.

## 必需文件和兼容性

- `LICENSE` 保留在根目录，GitHub 会据此自动显示许可证标签。项目声明和第三方声明归入 `third_party/`；发布归档里的文件名及 `/usr/share/doc/yub-wpanel` 安装位置保持原样。
- `install-cn.sh` 提供国内入口和全球 `bootstrap.sh` 的共享验签实现，主安装逻辑保留在 `install.sh`。
- 配套插件仍用于现有 WordPress 网站；它与面板图标各自打包，保留各自需要的资源。
- 旧版本升级文档的固定标签、附件名和兼容步骤保留，避免破坏历史升级路径。
- 根目录不保存测试输出或发布产物；`dist/` 被忽略，生成二进制不会使后续架构的源码状态标记变脏。
