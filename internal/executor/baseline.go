package executor

import (
	"github.com/zangwp/Z-Wpanel/internal/database"

	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func EnsureWordPressBaseline() {
	ensurePHPBaseline()
	ensureNginxBaseline()
	ensureNginxCacheBypassMap()
	ensureNginxSSLDefaultServer()
	ensureMariaDBBaseline()
	ensureRedisBaseline()
	ensureWordPressLoginAuditBaseline()
}

func ensurePHPBaseline() {
	changed, err := EnsurePHPRuntimeConfigFile()
	if err == nil && changed {
		exec.Command("systemctl", "reload", PHPFPMService()).Run()
	}
}

func ensureNginxBaseline() {
	path := "/etc/nginx/conf.d/yubwpanel.conf"
	data, err := os.ReadFile(path)

	if err != nil {
		// 文件不存在，创建完整配置
		content := `# YUB WPanel — WordPress 安全基线 (安装时自动生成)
client_max_body_size 64m;
server_names_hash_bucket_size 128;
`
		os.WriteFile(path, []byte(content), 0644)
		exec.Command("nginx", "-s", "reload").Run()
		return
	}

	// 文件已存在，检查是否缺少 server_names_hash_bucket_size
	content := string(data)
	if !strings.Contains(content, "server_names_hash_bucket_size") {
		content = strings.TrimRight(content, "\n") + "\nserver_names_hash_bucket_size 128;\n"
		os.WriteFile(path, []byte(content), 0644)
		exec.Command("nginx", "-s", "reload").Run()
	}
}

// ensureNginxCacheBypassMap 定义 $wp_cache_skip_args，供每个站点的 FastCGI 缓存绕过
// 判断复用（站点模板里引用了这个全局变量，不能在每个站点自己的配置文件里各定义一份——
// map 指令只能在 http{} 顶层出现一次，两个站点各定义一份同名变量会导致 nginx -t 报重复定义）。
//
// 语义：只要查询字符串完全由已知的营销/统计追踪参数组成（utm_*/fbclid/gclid 等），就不算
// "真实查询参数"，仍然允许命中缓存；只要出现任何一个不在名单里的参数，$wp_cache_skip_args
// 就会保留原始 $args（非空），触发跳过缓存——这样从广告点击进来的流量不会把缓存打穿。
func ensureNginxCacheBypassMap() {
	path := "/etc/nginx/conf.d/yubwpanel-cache-bypass.conf"
	if _, err := os.Stat(path); err == nil {
		return
	}
	content := `# YUB WPanel — FastCGI 缓存例外：营销追踪参数不算"真实查询参数"
map $args $wp_cache_skip_args {
    default $args;
    "~^(?:(?:utm_source|utm_medium|utm_campaign|utm_term|utm_content|fbclid|gclid|msclkid|mc_cid|mc_eid)=[^&]*&?)+$" "";
}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		log.Printf("[YUB-WPanel] 写入缓存例外配置失败 %s: %v", path, err)
		return
	}
	if out, err := exec.Command("nginx", "-t").CombinedOutput(); err != nil {
		log.Printf("[YUB-WPanel] Nginx 配置语法错误，跳过重载: %s", string(out))
		return
	}
	exec.Command("nginx", "-s", "reload").Run()
}

func ensureNginxSSLDefaultServer() {
	confPath := "/etc/nginx/conf.d/yubwpanel-ssl-default.conf"
	content := `# YUB WPanel — 默认 SSL 服务器，拒绝未知域名的 TLS 握手，防止证书跨站泄露
server {
    listen 443 ssl default_server;
    listen [::]:443 ssl default_server;
    http2 on;
    ssl_reject_handshake on;
}
`
	os.WriteFile(confPath, []byte(content), 0644)

	if out, err := exec.Command("nginx", "-t").CombinedOutput(); err != nil {
		fmt.Printf("[YUB-WPanel] Nginx 配置语法错误，跳过重载: %s\n", string(out))
		return
	}
	exec.Command("nginx", "-s", "reload").Run()
}

func ensureMariaDBBaseline() {
	path := "/etc/mysql/mariadb.conf.d/99-yubwpanel.cnf"
	if _, err := os.Stat(path); err == nil {
		return
	}
	poolSize := fmt.Sprintf("%dM", RecommendInnoDBBufferPoolSizeMB(CollectSystemFacts()))
	content := fmt.Sprintf(`# YUB WPanel — WordPress 安全基线 (安装时自动生成)
[mysqld]
innodb_buffer_pool_size = %s
`, poolSize)
	os.WriteFile(path, []byte(content), 0644)
	exec.Command("systemctl", "restart", "mariadb").Run()
}

func ensureRedisBaseline() {
	path := "/etc/redis/redis.conf"
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[YUB-WPanel] 读取 Redis 基线配置失败: %v", err)
		return
	}

	content := string(data)
	if FindRedisConfigValue(content, "maxmemory-policy") == "" && FindRedisConfigValue(content, "include") != "" {
		log.Printf("[YUB-WPanel] Redis 配置包含生效的 include 指令，跳过自动补充 maxmemory-policy，请管理员确认实际淘汰策略")
	}
	maxmem := fmt.Sprintf("%dmb", RecommendRedisMaxmemoryMB(CollectSystemFacts()))
	next, changed := BuildRedisBaselineConfig(content, maxmem)
	if !changed {
		return
	}

	result := SafeApplyRestartConfig(path, next, content, "redis-server", RedisReady)
	switch {
	case result.Applied:
		log.Printf("[YUB-WPanel] Redis 对象缓存基线已应用")
	case result.RolledBack && result.RollbackSucceeded:
		log.Printf("[YUB-WPanel] Redis 对象缓存基线应用失败，已恢复原配置: %v", result.Err)
	case result.RolledBack:
		log.Printf("[YUB-WPanel] Redis 对象缓存基线应用及回滚均失败，需要管理员立即检查 Redis: %v", result.Err)
	default:
		log.Printf("[YUB-WPanel] Redis 对象缓存基线应用失败，配置未改动: %v", result.Err)
	}
}

func getTotalMemoryKB() int64 {
	out, err := exec.Command("bash", "-c", "grep MemTotal /proc/meminfo | awk '{print $2}'").CombinedOutput()
	if err != nil {
		return 2097152 // default 2GB fallback
	}
	var kb int64
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &kb)
	return kb
}

func ensureWordPressLoginAuditBaseline() {
	db := database.GetDB()
	if db == nil {
		return
	}
	rows, err := db.Query(`SELECT id, domain, web_root, log_dir, system_user FROM websites WHERE site_type != 'php'`)
	if err != nil {
		log.Printf("[Z-Wpanel] 读取 WordPress 登录审计基线失败: %v", err)
		return
	}
	type auditSite struct {
		id                            int
		domain, webRoot, logDir, user string
	}
	var sites []auditSite
	for rows.Next() {
		var site auditSite
		if err := rows.Scan(&site.id, &site.domain, &site.webRoot, &site.logDir, &site.user); err != nil {
			log.Printf("[Z-Wpanel] 读取 WordPress 登录审计站点失败: %v", err)
			continue
		}
		sites = append(sites, site)
	}
	readErr := rows.Err()
	rows.Close() // PHP-runtime lookup may use the same SQLite connection.
	if readErr != nil {
		log.Printf("[Z-Wpanel] WordPress 登录审计基线扫描失败: %v", readErr)
	}
	for _, site := range sites {
		if !TryAcquireSiteOpLock(site.id, "login-audit-baseline") {
			log.Printf("[Z-Wpanel] 网站维护中，跳过登录审计基线 site=%d", site.id)
			continue
		}
		err := ConfigureWPLoginFailureLogging(site.webRoot, site.logDir, site.user)
		ReleaseSiteOpLock(site.id)
		if err != nil {
			log.Printf("[Z-Wpanel] WordPress 登录审计基线未启用 domain=%s: %v", site.domain, err)
		}
	}
}
