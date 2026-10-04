package executor

import (
	"github.com/zangwp/Z-Wpanel/internal/config"
	"testing"
)

func TestPHPRuntimeFollowsConfiguredPool(t *testing.T) {
	previous := config.AppConfig
	t.Cleanup(func() { config.AppConfig = previous })
	for _, series := range []string{"8.3", "8.4", "8.5"} {
		config.AppConfig = &config.Config{Paths: config.PathsConfig{PHPFPMPool: "/etc/php/" + series + "/fpm/pool.d"}}
		if PHPVersion() != series || PHPFPMService() != "php"+series+"-fpm" || PHPFPMBinary() != "php-fpm"+series || wpInventoryPHPPath() != "/usr/bin/php"+series {
			t.Fatalf("runtime identity does not retain configured PHP %s", series)
		}
	}
	config.AppConfig = nil
	if PHPVersion() != "8.3" {
		t.Fatal("legacy fallback changed")
	}
}
