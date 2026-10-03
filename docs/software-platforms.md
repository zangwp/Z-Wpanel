# 软件与平台策略 / Software and platform policy

| 系统 | 架构 | 全新安装 PHP | 源 |
|---|---|---|---|
| Debian 13 | amd64 / arm64 | PHP 8.5 | Ondřej Surý 签名仓库 |
| Ubuntu 24.04 LTS | amd64 / arm64 | PHP 8.5 | Ondřej PHP PPA（noble） |
| Ubuntu 26.04 LTS | amd64 / arm64 | PHP 8.5 | Ubuntu 原生 main/universe 与安全更新 |

全新安装使用 [Nginx 官方稳定源](https://nginx.org/en/linux_packages.html)、[MariaDB 官方源](https://mariadb.org/download/?t=repo-config)、[Redis 官方源](https://redis.io/docs/latest/operate/oss_and_stack/install/install-stack/apt/)。此次验证的候选版本为 Nginx 1.30.5、MariaDB 13.0.2、Redis 8.10.2。Debian 13 与 Ubuntu 24.04 的 PHP 候选为 8.5.11；Ubuntu 26.04 使用原生 8.5 系列及 Ubuntu 回补的安全修复，版本字符串可能不同。绝不向 Ubuntu 26.04 混入 noble 的 PHP 软件包。

MariaDB 13.0 是此次全新安装的稳定滚动系列。既有 MariaDB 安装保持原系列；面板更新和普通系统更新不会自行改用 13.0。PHP-FPM 的服务、命令、Pool 与套接字从现有配置识别，保留 8.3/8.4/8.5 的网站配置。跨系列升级应单独备份并检查兼容性。

Fail2ban、nftables、Cron 与图片优化工具使用对应发行版的软件包；Node.js + npm 为可选构建工具，按需使用发行版维护版本。WP-CLI 从固定官方发行包安装并校验摘要，不代替 Cron 服务。

网站仍使用 Nginx FastCGI + PHP-FPM，不安装 OpenLiteSpeed 或 LSPHP。开放 UDP 443 只改变防火墙访问策略，不会自动给网站启用 HTTP/3。数据库与 Redis 保持本机访问。SSH 和面板端口可调整来源范围；更改 SSH 端口必须保留原连接、验证新连接，超时恢复。

New installs use PHP 8.5 and signed official Nginx/MariaDB/Redis repositories. Ubuntu 26.04 keeps native PHP packages with Ubuntu security backports. Existing runtime series and site configuration are retained. Optional tools use maintained distribution packages. Firewall changes do not enable a web protocol or change database bindings.
