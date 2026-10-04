package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zangwp/Z-Wpanel/internal/config"
)

func deployWordPress(ctx context.Context, cfg *config.Config, webRoot, tmpDir string) error {
	os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "wordpress.zip")

	if err := downloadWP(ctx, cfg, zipPath); err != nil {
		return err
	}

	extractDir := filepath.Join(tmpDir, "wp_extract")
	if _, err := executeCommand("unzip", "-q", "-o", zipPath, "-d", extractDir); err != nil {
		return fmt.Errorf("解压失败: %w", err)
	}

	srcDir := extractDir
	if info, err := os.Stat(filepath.Join(extractDir, "wordpress")); err == nil && info.IsDir() {
		srcDir = filepath.Join(extractDir, "wordpress")
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("读取WordPress文件失败: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(webRoot, entry.Name())
		if err := copyPath(srcPath, dstPath); err != nil {
			return fmt.Errorf("移动文件 %s 失败: %w", entry.Name(), err)
		}
	}

	return nil
}

func copyPath(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()

	if _, err := io.Copy(d, s); err != nil {
		return err
	}

	return os.Chmod(dst, 0644)
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if err := copyPath(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func downloadWP(ctx context.Context, cfg *config.Config, destPath string) error {
	_, _, err := AcquireCorePackage(ctx, cfg.Paths.WordPressPackage, destPath, "", defaultCorePackageRefresher(cfg))
	return err
}

func removeDefaultPlugins(webRoot string) {
	os.RemoveAll(filepath.Join(webRoot, "wp-content", "plugins", "akismet"))
	os.Remove(filepath.Join(webRoot, "wp-content", "plugins", "hello.php"))
}

func removeUnusedThemes(webRoot string) {
	for _, slug := range []string{"twentytwentyfour", "twentytwentythree"} {
		os.RemoveAll(filepath.Join(webRoot, "wp-content", "themes", slug))
	}
}

func installRedisCachePlugin(webRoot, systemUser string) error {
	const packageURL = "https://downloads.wordpress.org/plugin/redis-cache.latest-stable.zip"
	destDir := filepath.Join(webRoot, "wp-content", "plugins")
	tmp, err := os.CreateTemp("", "yub-wpanel-redis-cache-*.zip")
	if err != nil {
		return fmt.Errorf("创建 Redis 插件临时文件失败: %w", err)
	}
	zipPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(zipPath)
		return fmt.Errorf("关闭 Redis 插件临时文件失败: %w", err)
	}
	defer os.Remove(zipPath)

	if _, err := executeCommand("wget", "--no-config", "-q", "--https-only", "--no-hsts", "-T", "30", "-O", zipPath, packageURL); err != nil {
		return fmt.Errorf("下载 Redis Object Cache 插件失败: %w", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("创建 WordPress 插件目录失败: %w", err)
	}
	if _, err := executeCommand("unzip", "-q", "-o", zipPath, "-d", destDir); err != nil {
		return fmt.Errorf("解压 Redis Object Cache 插件失败: %w", err)
	}
	if _, err := executeCommand("chown", "-R", siteOwner(systemUser), filepath.Join(destDir, "redis-cache")); err != nil {
		return fmt.Errorf("设置 Redis Object Cache 插件权限失败: %w", err)
	}
	return nil
}
