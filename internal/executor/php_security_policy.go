package executor

import (
	"github.com/zangwp/Z-Wpanel/internal/config"
	"path"
	"path/filepath"
	"strings"
)

const (
	phpDisabledFunctionsSetting = "exec,passthru,shell_exec,system,proc_open,popen,show_source"
)

func sitePHPOpenBaseDir(webRoot, domain string) string {
	return strings.Join([]string{
		webRoot,
		"/tmp",
		"/usr/share/php",
		path.Join(siteSecretsRoot, domain),
		sitePHPLogDir(domain),
	}, ":")
}

func sitePHPDisabledFunctions() string {
	return phpDisabledFunctionsSetting
}

func sitePHPRunnerOpenBaseDir(webRoot, domain, runnerDir string) string {
	return strings.Join([]string{
		sitePHPOpenBaseDir(webRoot, domain),
		runnerDir,
	}, ":")
}

func sitePHPLogDir(domain string) string {
	base := "/www/wwwlogs"
	if config.AppConfig != nil && config.AppConfig.Paths.WWWLogs != "" {
		base = config.AppConfig.Paths.WWWLogs
	}
	return filepath.Join(base, domain)
}
