package handlers

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

var timeSyncCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// StartSystemTimeSync shares the panel's provider-aware synchronization with the CLI.
func StartSystemTimeSync() error { return startSystemTimeSync() }

func startSystemTimeSync() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	unit := ntpTimeSyncUnit()
	if unit == "" {
		// Do not silently replace a deliberately masked or unsupported provider.
		for _, candidate := range []string{"chrony.service", "systemd-timesyncd.service", "ntpsec.service", "ntp.service"} {
			out, err := timeSyncCommand(ctx, "systemctl", "show", candidate, "--property=LoadState", "--value")
			if err == nil && strings.TrimSpace(string(out)) == "masked" {
				return fmt.Errorf("%s 已被管理员屏蔽，请先检查服务策略", candidate)
			}
		}
		out, err := timeSyncCommand(ctx, "apt-get", "install", "-y", "--no-install-recommends", "systemd-timesyncd")
		if err != nil {
			log.Printf("安装时间同步服务失败: %s", out)
			return fmt.Errorf("缺少时间同步服务，安装 systemd-timesyncd 失败；请检查 APT 与网络")
		}
		unit = "systemd-timesyncd.service"
	}
	if out, err := timeSyncCommand(ctx, "systemctl", "enable", "--now", unit); err != nil {
		return fmt.Errorf("启动 %s 失败: %s", unit, strings.TrimSpace(string(out)))
	}
	if out, err := timeSyncCommand(ctx, "timedatectl", "set-ntp", "true"); err != nil {
		return fmt.Errorf("启用自动校时失败: %s", strings.TrimSpace(string(out)))
	}
	if out, err := timeSyncCommand(ctx, "systemctl", "restart", unit); err != nil {
		return fmt.Errorf("重启 %s 失败: %s", unit, strings.TrimSpace(string(out)))
	}
	return nil
}
