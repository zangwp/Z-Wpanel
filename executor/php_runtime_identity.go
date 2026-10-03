package executor

import (
	"github.com/zangwp/Z-Wpanel/config"
	"path/filepath"
	"regexp"
)

var managedPHPVersion = regexp.MustCompile(`(?:^|/)php/(8\.[0-9]+)/fpm(?:/|$)`)

// Existing installations retain their configured FPM series. New installations
// select PHP 8.5 in the signed installer; an absent config preserves the old
// 8.3 fallback for legacy tools and tests.
func PHPVersion() string {
	if config.AppConfig != nil {
		if m := managedPHPVersion.FindStringSubmatch(filepath.ToSlash(config.AppConfig.Paths.PHPFPMPool)); len(m) == 2 {
			return m[1]
		}
	}
	return "8.3"
}
func PHPFPMService() string { return "php" + PHPVersion() + "-fpm" }
func PHPFPMBinary() string  { return "php-fpm" + PHPVersion() }
func PHPCLIBinary() string  { return "php" + PHPVersion() }

func wpInventoryPHPPath() string { return "/usr/bin/" + PHPCLIBinary() }
