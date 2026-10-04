# YUB WPanel

<p><img src="static/logo.png" alt="YUB WPanel" width="100"></p>

A lightweight VPS and WordPress management panel powered by **Nginx, PHP-FPM, MariaDB, and Redis**. Manage sites, databases, SSL, caching, backups, and server maintenance.

[中文](README.md) · [User guide](docs/project-guide.en.md) · [Releases](https://github.com/zangwp/Z-Wpanel/releases) · [Issues](https://github.com/zangwp/Z-Wpanel/issues)

## Installation

Supported clean servers: **Debian 13, Ubuntu 24.04 LTS, Ubuntu 26.04 LTS**, on **amd64 / arm64**. Minimum: 1 CPU core and 1 GiB RAM. Run as root:

```bash
curl -fsSL https://wpanel.zangyubin.top/install | bash
```

The entry is pinned to a published stable release and verifies Ed25519 signatures and SHA-256 digests before installation. If curl is missing, first run:

```bash
apt-get update && apt-get install -y --no-install-recommends curl wget ca-certificates openssl
```

See [verified installation](docs/verified-install.md) for manual verification, China-friendly mirrors, and offline setup. Other distributions and existing production environments are outside automated installation support.

## Features

| Area | Features |
|---|---|
| Websites | Provisioning, migration, domains, WordPress core/plugin/theme updates, files and databases |
| Certificates and speed | Automatic certificate renewal, Nginx FastCGI cache, Redis object cache, image optimization |
| Backups and tasks | Site/panel backups, SFTP/S3 copies; per-site WP-Cron and certificate renewal controls |
| VPS maintenance | Service and resource status, background updates, DNS, IP priority, Swap, locales and time synchronization |
| Security | Login protection, Fail2ban, separate allowlists, recoverable firewall policies, custom SSH ports, logs and alerts |

New installations use official stable repositories and PHP 8.5; Ubuntu 26.04 uses its native PHP 8.5 security packages. Existing PHP and database series are retained. See [software and platform policy](docs/software-platforms.md).

## SSH commands

`b` and `B` are equivalent. Without arguments they open the VPS maintenance menu.

```text
b info      Panel information
b status    Runtime diagnostics
b log [N]   Logs
b restart   Restart panel
b update    Update or repair through the signed entry
```

[Full guide](docs/project-guide.en.md) · [Upgrade compatibility](docs/upgrade-compatibility.md) · [Repository layout](docs/repository-layout.md)

---

[GPL-3.0-only](LICENSE) · [Project notice](NOTICE.md) · [Third-party notices](THIRD_PARTY_NOTICES.md) · [zangwp](https://github.com/zangwp)
