package executor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	phpFPMHealthTimeout     = 4 * time.Second
	phpFPMHealthOutputLimit = 512 * 1024
)

// PHPFPMExtensionHealth reports a module loaded by a fresh PHP CLI invocation
// using the runtime's server configuration. It is not a per-site worker probe.
type PHPFPMExtensionHealth struct {
	Name       string `json:"name"`
	Importance string `json:"importance"`
	State      string `json:"state"`
}

// PHPFPMOPCacheHealth deliberately contains configuration only. A command-line
// invocation cannot observe the cache instance used by existing web workers.
type PHPFPMOPCacheHealth struct {
	ConfigurationState string `json:"configuration_state"`
	Enabled            *bool  `json:"enabled"`
	MemoryMB           *int   `json:"memory_mb"`
	MaxFiles           *int   `json:"max_files"`
	MetricsState       string `json:"metrics_state"`
	MetricsReason      string `json:"metrics_reason"`
}

type PHPFPMRuntimeHealth struct {
	Version            string                  `json:"version"`
	State              string                  `json:"state"`
	Reason             string                  `json:"reason,omitempty"`
	Extensions         []PHPFPMExtensionHealth `json:"extensions"`
	MissingRequired    int                     `json:"missing_required"`
	MissingRecommended int                     `json:"missing_recommended"`
	OPcache            PHPFPMOPCacheHealth     `json:"opcache"`
}

// This list follows the WordPress Hosting Handbook's required and highly
// recommended modules. Redis and OPcache match the panel's cache baseline.
// https://make.wordpress.org/hosting/handbook/server-environment/#php-extensions
var phpFPMHealthExtensions = []PHPFPMExtensionHealth{
	{Name: "json", Importance: "required"},
	{Name: "hash", Importance: "required"},
	{Name: "mysqli", Importance: "required"},
	{Name: "curl", Importance: "recommended"},
	{Name: "dom", Importance: "recommended"},
	{Name: "exif", Importance: "recommended"},
	{Name: "fileinfo", Importance: "recommended"},
	{Name: "igbinary", Importance: "recommended"},
	{Name: "imagick", Importance: "recommended"},
	{Name: "intl", Importance: "recommended"},
	{Name: "mbstring", Importance: "recommended"},
	{Name: "openssl", Importance: "recommended"},
	{Name: "xml", Importance: "recommended"},
	{Name: "zip", Importance: "recommended"},
	{Name: "redis", Importance: "recommended"},
	{Name: "opcache", Importance: "recommended"},
}

func newPHPFPMRuntimeHealth(version string) PHPFPMRuntimeHealth {
	health := PHPFPMRuntimeHealth{
		Version: version, State: "unknown",
		Extensions: append([]PHPFPMExtensionHealth(nil), phpFPMHealthExtensions...),
		OPcache: PHPFPMOPCacheHealth{
			ConfigurationState: "unknown", MetricsState: "unavailable",
			MetricsReason: "web_worker_probe_unavailable",
		},
	}
	for i := range health.Extensions {
		health.Extensions[i].State = "unknown"
	}
	return health
}

// ReadPHPFPMRuntimeHealth performs a bounded read-only check. Unsupported or
// untrusted installations and failed checks retain unknown extension states.
func ReadPHPFPMRuntimeHealth(ctx context.Context, runtime PHPFPMRuntime) PHPFPMRuntimeHealth {
	return ReadPHPFPMRuntimeHealthWithProbe(ctx, runtime, boundedPHPFPMHealthRead)
}

// ReadPHPFPMRuntimeHealthWithProbe lets the software handler coalesce/cache the
// raw command result. The probe must enforce its own output bound while reading;
// oversized results are also rejected here before parsing. Trust checks always
// run before the supplied probe, including on cache hits.
func ReadPHPFPMRuntimeHealthWithProbe(ctx context.Context, runtime PHPFPMRuntime, probe func(context.Context, string, ...string) ([]byte, error)) PHPFPMRuntimeHealth {
	return readPHPFPMRuntimeHealth(ctx, runtime, probe, validatePHPFPMHealthRuntime)
}

func readPHPFPMRuntimeHealth(ctx context.Context, runtime PHPFPMRuntime, probe func(context.Context, string, ...string) ([]byte, error), validate func(PHPFPMRuntime) (string, string, string)) PHPFPMRuntimeHealth {
	health := newPHPFPMRuntimeHealth(runtime.Version)
	if runtime.Version == "" {
		health.Reason = "unsupported_runtime"
		return health
	}
	if version, err := normalizePHPFPMVersion(runtime.Version); err != nil || version != runtime.Version {
		health.Reason = "unsupported_runtime"
		return health
	}
	if !runtime.Installed {
		health.State, health.Reason = "not_installed", "not_installed"
		return health
	}
	if ctx.Err() != nil {
		health.Reason = phpFPMHealthProbeErrorReason(ctx.Err())
		return health
	}
	iniPath, scanDir, reason := validate(runtime)
	if reason != "" {
		health.Reason = reason
		return health
	}
	if probe == nil {
		probe = boundedPHPFPMHealthRead
	}
	ctx, cancel := context.WithTimeout(ctx, phpFPMHealthTimeout)
	defer cancel()
	// PHP CLI loads the configured FPM ini and extension scan directory.
	// -i does not execute a PHP script (including auto_prepend/auto_append).
	// A cleared environment prevents inherited PHPRC, scan directories,
	// loader variables or PHP CLI socket settings from changing this invocation.
	output, err := probe(ctx, "/usr/bin/env", "-i", "PATH=/usr/bin:/bin", "PHP_INI_SCAN_DIR="+scanDir,
		runtime.CLIBinary, "-c", iniPath, "-i")
	if err != nil || ctx.Err() != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		health.Reason = phpFPMHealthProbeErrorReason(err)
		return health
	}
	return parsePHPFPMHealthOutput(runtime.Version, iniPath, scanDir, output)
}

func validatePHPFPMHealthRuntime(runtime PHPFPMRuntime) (string, string, string) {
	if runtime.Version != PHPVersion() || runtime.CLIBinary != "/usr/bin/"+PHPCLIBinary() {
		return "", "", "untrusted_runtime"
	}
	root := filepath.Join("/etc/php", runtime.Version, "fpm")
	ini := filepath.Join(root, "php.ini")
	scan := filepath.Join(root, "conf.d")
	if !phpFPMHealthTrustedPath(runtime.CLIBinary, "/usr", "executable", fileOwnerIDs) || !phpFPMHealthTrustedPath("/usr/bin/env", "/usr", "executable", fileOwnerIDs) {
		return "", "", "untrusted_runtime"
	}
	// Debian uses root-owned symlinks from fpm/conf.d to mods-available.
	if !phpFPMHealthTrustedPath(ini, "/etc/php", "file", fileOwnerIDs) || !phpFPMHealthTrustedPath(scan, "/etc/php", "directory", fileOwnerIDs) {
		return "", "", "configuration_unavailable"
	}
	entries, err := os.ReadDir(scan)
	if err != nil {
		return "", "", "configuration_unavailable"
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".ini") && !phpFPMHealthTrustedPath(filepath.Join(scan, e.Name()), "/etc/php", "file", fileOwnerIDs) {
			return "", "", "configuration_unavailable"
		}
	}
	return ini, scan, ""
}

// Validate both the supplied pathname's ancestor chain and its canonical
// target. Root-owned package symlinks are allowed; a writable ancestor or an
// escape from this PHP branch is not. Ownership is mandatory on Linux.
func phpFPMHealthTrustedPath(path, scope, kind string, owner func(os.FileInfo) (int, int, error)) bool {
	path, scope = filepath.Clean(path), filepath.Clean(scope)
	if !filepath.IsAbs(path) || !filepath.IsAbs(scope) || !phpFPMHealthPathWithin(path, scope) {
		return false
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil || !phpFPMHealthPathWithin(real, scope) {
		return false
	}
	for _, candidate := range []string{path, real} {
		for current := candidate; ; current = filepath.Dir(current) {
			info, err := os.Lstat(current)
			if err != nil {
				return false
			}
			uid, _, ownerErr := owner(info)
			checkKind := "directory"
			if current == candidate {
				checkKind = kind
			}
			if !phpFPMHealthSafeInfo(info, checkKind, uid, ownerErr) {
				return false
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
		}
	}
	return true
}

func phpFPMHealthPathWithin(path, scope string) bool {
	relative, err := filepath.Rel(scope, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func phpFPMHealthSafeInfo(info os.FileInfo, kind string, uid int, ownerErr error) bool {
	if info == nil || ownerErr != nil || uid != 0 {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return true // Parent and canonical target receive their own checks.
	}
	if info.Mode().Perm()&0022 != 0 {
		return false
	}
	switch kind {
	case "directory":
		return info.IsDir()
	case "executable":
		return info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
	default:
		return info.Mode().IsRegular()
	}
}

func parsePHPFPMHealthOutput(version, iniPath, scanDir string, output []byte) PHPFPMRuntimeHealth {
	health := newPHPFPMRuntimeHealth(version)
	health.Reason = "invalid_output"
	if len(output) == 0 || len(output) > phpFPMHealthOutputLimit || bytes.IndexByte(output, 0) >= 0 {
		return health
	}
	text := strings.ReplaceAll(string(output), "\r\n", "\n")
	if !strings.HasPrefix(strings.TrimSpace(text), "phpinfo()\n") || !strings.Contains(text, "\nPHP License\n") {
		return health
	}
	// PHP can return zero even when an extension failed during startup. Such a
	// result is not sufficient evidence that the remaining modules are missing.
	for _, marker := range []string{"PHP Warning:", "PHP Fatal error:", "PHP Parse error:", "Unable to load dynamic library"} {
		if strings.Contains(text, marker) {
			health.Reason = "probe_failed"
			return health
		}
	}
	values := make(map[string][]string)
	loaded := make(map[string]bool)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		parts := strings.Split(line, " => ")
		if len(parts) >= 2 {
			if _, duplicate := values[parts[0]]; !duplicate {
				values[parts[0]] = parts[1:]
			}
			continue
		}
		name := strings.ToLower(line)
		if name == "zend opcache" {
			name = "opcache"
		}
		loaded[name] = true
	}
	value := func(key string) string {
		if parts := values[key]; len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
		return ""
	}
	if !strings.HasPrefix(value("PHP Version"), version+".") || !strings.HasPrefix(value("Server API"), "Command Line Interface") {
		health.Reason = "runtime_mismatch"
		return health
	}
	if filepath.Clean(value("Loaded Configuration File")) != filepath.Clean(iniPath) ||
		filepath.Clean(value("Scan this dir for additional .ini files")) != filepath.Clean(scanDir) {
		health.Reason = "configuration_mismatch"
		return health
	}
	if failure := value("Startup Failed"); failure != "" {
		cliEnabled := phpFPMHealthBool(value("opcache.enable_cli"))
		// A CLI probe can load OPcache while its CLI cache is intentionally off.
		// This says nothing about the cache used by running FPM workers.
		if failure != "Opcode Caching is disabled for CLI" || cliEnabled == nil || *cliEnabled {
			health.Reason = "probe_failed"
			return health
		}
	}
	// Modules must appear in phpinfo's module section, rather than in a
	// Configure Command or an ini directive naming an unloaded extension.
	moduleStart := strings.Index(text, "\nConfiguration\n")
	moduleEnd := strings.Index(text, "\nPHP Variables\n")
	if moduleStart < 0 || moduleEnd <= moduleStart || !loaded["core"] || !loaded["standard"] {
		return health
	}
	loaded = make(map[string]bool)
	for _, line := range strings.Split(text[moduleStart:moduleEnd], "\n") {
		name := strings.ToLower(strings.TrimSpace(line))
		if name == "zend opcache" {
			name = "opcache"
		}
		loaded[name] = true
	}
	health.State, health.Reason = "ok", ""
	for i := range health.Extensions {
		extension := &health.Extensions[i]
		extension.State = "loaded"
		if !loaded[extension.Name] {
			extension.State = "missing"
			if extension.Importance == "required" {
				health.MissingRequired++
			} else {
				health.MissingRecommended++
			}
		}
	}
	if health.MissingRequired+health.MissingRecommended > 0 {
		health.State, health.Reason = "warning", "extensions_missing"
	}
	if loaded["opcache"] {
		health.OPcache.Enabled = phpFPMHealthBool(value("opcache.enable"))
		health.OPcache.MemoryMB = phpFPMHealthNonnegativeInt(value("opcache.memory_consumption"))
		health.OPcache.MaxFiles = phpFPMHealthNonnegativeInt(value("opcache.max_accelerated_files"))
		if health.OPcache.Enabled != nil && health.OPcache.MemoryMB != nil && health.OPcache.MaxFiles != nil {
			health.OPcache.ConfigurationState = "known"
		}
		if health.OPcache.Enabled != nil && !*health.OPcache.Enabled && health.State == "ok" {
			health.State, health.Reason = "warning", "opcache_disabled"
		}
	}
	return health
}

func phpFPMHealthBool(value string) *bool {
	var result bool
	switch strings.ToLower(value) {
	case "1", "on", "true", "yes":
		result = true
	case "0", "off", "false", "no":
		result = false
	default:
		return nil
	}
	return &result
}

func phpFPMHealthNonnegativeInt(value string) *int {
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed < 0 {
		return nil
	}
	result := int(parsed)
	return &result
}

func phpFPMHealthProbeErrorReason(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "probe_timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "probe_canceled"
	}
	return "probe_failed"
}

type phpFPMHealthBoundedWriter struct {
	buffer bytes.Buffer
	limit  int
}

func (writer *phpFPMHealthBoundedWriter) Write(data []byte) (int, error) {
	if len(data) > writer.limit-writer.buffer.Len() {
		return 0, errors.New("PHP health probe output limit exceeded")
	}
	return writer.buffer.Write(data)
}

func boundedPHPFPMHealthRead(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, phpFPMHealthTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	cmd.WaitDelay = time.Second
	stdout := &phpFPMHealthBoundedWriter{limit: phpFPMHealthOutputLimit}
	stderr := &phpFPMHealthBoundedWriter{limit: 16 * 1024}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if len(bytes.TrimSpace(stderr.buffer.Bytes())) != 0 {
		return nil, errors.New("PHP health probe startup diagnostic")
	}
	return append([]byte(nil), stdout.buffer.Bytes()...), nil
}

type PHPFPMRuntime struct {
	Version   string `json:"version"`
	Installed bool   `json:"installed"`
	CLIBinary string `json:"-"`
}

func ListPHPFPMRuntimesContext(ctx context.Context) []PHPFPMRuntime {
	runtime := PHPFPMRuntime{Version: PHPVersion(), CLIBinary: "/usr/bin/" + PHPCLIBinary()}
	info, err := os.Stat(runtime.CLIBinary)
	runtime.Installed = err == nil && info.Mode().IsRegular()
	return []PHPFPMRuntime{runtime}
}
func normalizePHPFPMVersion(version string) (string, error) {
	if !regexp.MustCompile(`^8\.[0-9]+$`).MatchString(version) {
		return "", errors.New("unsupported runtime")
	}
	return version, nil
}
