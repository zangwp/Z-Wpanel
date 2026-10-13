package executor

import "strings"

// A saved configuration is reported separately from a loopback request result.
// Match the existing Nginx guards without importing a LiteSpeed rule renderer.
func nginxBackupSecurityRulesMatch(content string, data *NginxSiteData) bool {
	if !strings.Contains(content, "if ($wp_sensitive_path_blocked) { return 404; }") {
		return false
	}
	if data.SiteType == "wordpress" {
		if !strings.Contains(content, "wp-content/uploads") {
			return false
		}
		if !data.XMLRPCEnabled && !strings.Contains(content, "location = /xmlrpc.php") {
			return false
		}
		if data.SQLiBlockEnabled && !strings.Contains(content, "if ($wp_sqli_block_hit) { return 403; }") {
			return false
		}
	}
	return true
}
