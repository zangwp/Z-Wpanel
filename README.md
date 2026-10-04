# YUB WPanel

<p><img src="static/logo.png" alt="YUB WPanel" width="100"></p>

轻量的 VPS 与 WordPress 管理面板，使用 **Nginx、PHP-FPM、MariaDB 和 Redis**。集中管理网站、数据库、SSL、缓存、备份和服务器维护。

[English](README.en.md) · [使用文档](docs/project-guide.zh-CN.md) · [版本发布](https://github.com/zangwp/Z-Wpanel/releases) · [问题反馈](https://github.com/zangwp/Z-Wpanel/issues)

## 安装

支持全新的 **Debian 13、Ubuntu 24.04 LTS、Ubuntu 26.04 LTS**，架构为 **amd64 / arm64**。最低 1 核 CPU、1 GiB 内存。使用 root 执行：

```bash
curl -fsSL https://wpanel.zangyubin.top/install | bash
```

安装入口固定到已发布的稳定版本，验证 Ed25519 签名与 SHA-256 后安装。

没有 curl 的精简系统先执行：

```bash
apt-get update && apt-get install -y --no-install-recommends curl wget ca-certificates openssl
```

手动验签、国内网络与离线安装见[安装指南](docs/verified-install.md)。其他系统版本和已有生产环境不在自动安装支持范围内。

## 可以做什么

| 功能 | 内容 |
|---|---|
| 网站与 WordPress | 创建网站、搬家、域名管理、核心/插件/主题更新、文件与数据库管理 |
| 证书与加速 | 自动申请与续期证书、Nginx FastCGI 缓存、Redis 对象缓存、图片优化 |
| 备份与任务 | 网站与面板备份、SFTP/S3 异地保存；网站详情管理 WP-Cron 与证书续期 |
| VPS 维护 | 资源与服务状态、后台系统更新、DNS、IPv4/IPv6 优先级、Swap、系统语言和校时 |
| 安全与访问 | 登录保护、Fail2ban、独立白名单、可恢复防火墙策略、自定义 SSH 端口、日志与告警 |

全新安装使用官方稳定软件源和 PHP 8.5；Ubuntu 26.04 使用原生 PHP 8.5 安全更新包。现有安装保留 PHP 与数据库主版本，不自动跨系列升级。详见[软件与平台策略](docs/software-platforms.md)。

## SSH 快捷命令

`b` 与大写 `B` 等价。无参数进入 VPS 日常维护菜单。

```text
b info      查看面板信息
b status    诊断运行状态
b log [N]   查看日志
b restart   重启面板
b update    通过签名入口更新或修复面板
```

[详细说明](docs/project-guide.zh-CN.md) · [升级兼容性](docs/upgrade-compatibility.md) · [仓库结构](docs/repository-layout.md)

---

[GPL-3.0-only](LICENSE) · [项目声明](NOTICE.md) · [第三方许可](THIRD_PARTY_NOTICES.md) · [zangwp](https://github.com/zangwp)
