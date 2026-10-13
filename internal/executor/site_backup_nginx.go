package executor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/zangwp/Z-Wpanel/internal/config"
)

var siteBackupReloadFPM = reloadSiteBackupFPM
var siteBackupSyncConfigDir = syncSiteBackupConfigDir
var siteBackupSafeUserPattern = regexp.MustCompile(`^wp_[a-zA-Z0-9_]+$`)

func normalizeSiteBackupDomains(domain string, aliases []string) ([]string, error) {
	domains := append([]string{domain}, aliases...)
	seen := map[string]bool{}
	result := []string{}
	for _, name := range domains {
		if !IsValidDomain(name) {
			return nil, errors.New("网站域名无效")
		}
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	return result, nil
}
func safeSiteBackupNginxPath(path string) bool {
	return filepath.IsAbs(path) && !strings.ContainsAny(path, " \t\r\n;{}\"'\\$")
}
func siteBackupIPv6Available() bool {
	c, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		return false
	}
	c.Close()
	return true
}
func writeSiteBackupNginxAtomic(path string, data []byte, mode os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".backup-config-")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err = file.Chmod(mode); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	return siteBackupSyncConfigDir(filepath.Dir(path))
}
func testAndReloadSiteBackupNginx() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if out, err := boundedBackupRuntimeCommand(ctx, "nginx", "-t"); err != nil {
		return out, err
	}
	return boundedBackupRuntimeCommand(ctx, "nginx", "-s", "reload")
}
func (f *siteBackupFreeze) fpmMaintenance() string {
	return "; YUB WPanel backup maintenance: " + f.manifest.Marker + "\n"
}
func (f *siteBackupFreeze) disableFPM(ctx context.Context) error {
	path, err := managedSubpath(config.AppConfig.Paths.PHPFPMPool, f.site.PHPPoolPath, "PHP-FPM pool")
	if err != nil {
		return err
	}
	if err = fileRestoreNoSymlinkDirectories(filepath.Dir(path)); err != nil {
		return err
	}
	if f.manifest.PHPPoolPath != "" && f.manifest.PHPPoolPath != path {
		return errors.New("PHP-FPM pool 绑定已变化")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return errors.New("PHP-FPM pool 配置身份不安全")
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(original) == f.fpmMaintenance() && f.manifest.FPMDisabled {
		return siteBackupReloadFPM(ctx)
	}
	if f.manifest.OriginalPHPPool != "" && string(original) != f.manifest.OriginalPHPPool {
		return errors.New("PHP-FPM pool 已被外部修改")
	}
	sum := sha256.Sum256(original)
	f.manifest.PHPPoolPath = path
	f.manifest.OriginalPHPPool = string(original)
	f.manifest.PHPPoolSHA256 = hex.EncodeToString(sum[:])
	f.manifest.FPMDisabled = true
	if err = f.save("fpm_disable_intent"); err != nil {
		return err
	}
	if err = writeSiteBackupNginxAtomic(path, []byte(f.fpmMaintenance()), 0644); err != nil {
		return err
	}
	return siteBackupReloadFPM(ctx)
}
func (f *siteBackupFreeze) restoreFPM(ctx context.Context) error {
	if !f.manifest.FPMDisabled {
		return nil
	}
	path, err := managedSubpath(config.AppConfig.Paths.PHPFPMPool, f.manifest.PHPPoolPath, "PHP-FPM pool")
	if err != nil {
		return err
	}
	if err = fileRestoreNoSymlinkDirectories(filepath.Dir(path)); err != nil {
		return err
	}
	if path != f.site.PHPPoolPath || f.manifest.OriginalPHPPool == "" {
		return errors.New("PHP-FPM 原配置或绑定无法确认")
	}
	sum := sha256.Sum256([]byte(f.manifest.OriginalPHPPool))
	if hex.EncodeToString(sum[:]) != f.manifest.PHPPoolSHA256 {
		return errors.New("PHP-FPM 原配置摘要不匹配")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("PHP-FPM 维护配置身份不安全")
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(current) != f.fpmMaintenance() && string(current) != f.manifest.OriginalPHPPool {
		return errors.New("PHP-FPM 维护配置被外部修改")
	}
	if err = writeSiteBackupNginxAtomic(path, []byte(f.manifest.OriginalPHPPool), 0644); err != nil {
		return err
	}
	if err = siteBackupReloadFPM(ctx); err != nil {
		return err
	}
	f.manifest.FPMDisabled = false
	return f.save("fpm_restored")
}
func reloadSiteBackupFPM(ctx context.Context) error {
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if _, err := boundedBackupRuntimeCommand(bounded, PHPFPMBinary(), "-t"); err != nil {
		return err
	}
	_, err := boundedBackupRuntimeCommand(bounded, "systemctl", "reload", PHPFPMService())
	return err
}

func boundedBackupRuntimeCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = time.Second
	output := newBoundedCronOutput(256 << 10)
	cmd.Stdout = output
	cmd.Stderr = io.Discard
	err := cmd.Run()
	return output.String(), err
}

func syncSiteBackupConfigDir(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}
