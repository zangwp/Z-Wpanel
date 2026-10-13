package executor

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func phpFPMHealthFixture(startupFailure, cliEnabled string, exclude ...string) []byte {
	omit := make(map[string]bool)
	for _, name := range exclude {
		omit[name] = true
	}
	var text strings.Builder
	text.WriteString("phpinfo()\nPHP Version => 8.5.11\nServer API => Command Line Interface\nLoaded Configuration File => /etc/php/8.5/fpm/php.ini\nScan this dir for additional .ini files => /etc/php/8.5/fpm/conf.d\n\nConfiguration\n\nCore\n\nstandard\n")
	for _, extension := range phpFPMHealthExtensions {
		if omit[extension.Name] {
			continue
		}
		name := extension.Name
		if name == "opcache" {
			name = "Zend OPcache"
		}
		fmt.Fprintf(&text, "\n%s\n", name)
	}
	if startupFailure != "" {
		fmt.Fprintf(&text, "Startup Failed => %s\n", startupFailure)
	}
	fmt.Fprintf(&text, "opcache.enable => On => On\nopcache.enable_cli => %s => %s\nopcache.memory_consumption => 256 => 256\nopcache.max_accelerated_files => 20000 => 20000\n\nPHP Variables\n\nPHP License\n", cliEnabled, cliEnabled)
	return []byte(text.String())
}

func parsePHPFPMHealthFixture(output []byte) PHPFPMRuntimeHealth {
	return parsePHPFPMHealthOutput("8.5", "/etc/php/8.5/fpm/php.ini", "/etc/php/8.5/fpm/conf.d", output)
}

func TestPHPFPMHealthCLICacheDisabledStillReadsExtensions(t *testing.T) {
	for _, failure := range []string{"", "Opcode Caching is disabled for CLI"} {
		t.Run(failure, func(t *testing.T) {
			health := parsePHPFPMHealthFixture(phpFPMHealthFixture(failure, "Off", "imagick"))
			if health.State != "warning" || health.Reason != "extensions_missing" || health.MissingRequired != 0 || health.MissingRecommended != 1 {
				t.Fatalf("CLI cache setting hid loaded extensions: %+v", health)
			}
			for _, extension := range health.Extensions {
				want := "loaded"
				if extension.Name == "imagick" {
					want = "missing"
				}
				if extension.State != want {
					t.Errorf("%s=%s, want %s", extension.Name, extension.State, want)
				}
			}
			if health.OPcache.ConfigurationState != "known" || health.OPcache.Enabled == nil || !*health.OPcache.Enabled || health.OPcache.MemoryMB == nil || *health.OPcache.MemoryMB != 256 {
				t.Fatalf("FPM configuration was lost: %+v", health.OPcache)
			}
			if health.OPcache.MetricsState != "unavailable" {
				t.Fatal("CLI probe claimed to measure running FPM cache")
			}
		})
	}
}

func TestPHPFPMHealthRejectsRealFailuresAndMismatchedEvidence(t *testing.T) {
	valid := string(phpFPMHealthFixture("Opcode Caching is disabled for CLI", "Off"))
	for name, output := range map[string]string{
		"shared memory failure":    string(phpFPMHealthFixture("No available SHM backend", "Off")),
		"inconsistent CLI setting": string(phpFPMHealthFixture("Opcode Caching is disabled for CLI", "On")),
		"unconfirmed CLI setting":  strings.ReplaceAll(valid, "opcache.enable_cli => Off => Off\n", ""),
		"startup warning":          "PHP Warning: Unable to load dynamic library\n" + valid,
		"wrong version":            strings.ReplaceAll(valid, "8.5.11", "8.4.11"),
		"wrong SAPI":               strings.ReplaceAll(valid, "Command Line Interface", "FPM/FastCGI"),
		"wrong configuration":      strings.ReplaceAll(valid, "/fpm/php.ini", "/cli/php.ini"),
		"wrong scan directory":     strings.ReplaceAll(valid, "/fpm/conf.d", "/cli/conf.d"),
		"truncated output":         strings.Split(valid, "PHP Variables")[0],
		"oversized output":         strings.Repeat("x", phpFPMHealthOutputLimit+1),
		"NUL output":               valid + "\x00",
	} {
		t.Run(name, func(t *testing.T) {
			health := parsePHPFPMHealthFixture([]byte(output))
			if health.State != "unknown" || health.Reason == "" || health.MissingRequired != 0 || health.MissingRecommended != 0 {
				t.Fatalf("invalid evidence produced confirmed health: %+v", health)
			}
			for _, extension := range health.Extensions {
				if extension.State != "unknown" {
					t.Fatalf("%s should remain unconfirmed", extension.Name)
				}
			}
		})
	}
}

func TestPHPFPMHealthProbeUsesFPMConfigurationWithoutChangingSettings(t *testing.T) {
	const ini, scan = "/etc/php/8.5/fpm/php.ini", "/etc/php/8.5/fpm/conf.d"
	runtime := PHPFPMRuntime{Version: "8.5", Installed: true, CLIBinary: "/usr/bin/php8.5"}
	calls := 0
	health := readPHPFPMRuntimeHealth(context.Background(), runtime, func(ctx context.Context, name string, args ...string) ([]byte, error) {
		calls++
		want := []string{"-i", "PATH=/usr/bin:/bin", "PHP_INI_SCAN_DIR=" + scan, runtime.CLIBinary, "-c", ini, "-i"}
		if name != "/usr/bin/env" || !reflect.DeepEqual(args, want) {
			t.Fatalf("unexpected probe command: %s %v", name, args)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("probe has no deadline")
		}
		return phpFPMHealthFixture("Opcode Caching is disabled for CLI", "Off"), nil
	}, func(PHPFPMRuntime) (string, string, string) { return ini, scan, "" })
	if calls != 1 || health.State != "ok" {
		t.Fatalf("health=%+v, calls=%d", health, calls)
	}
}

func TestPHPFPMHealthRealCLIOutput(t *testing.T) {
	fixture := os.Getenv("YUB_PHP_HEALTH_REAL_FIXTURE")
	if fixture == "" {
		t.Skip("real PHP CLI fixture not supplied")
	}
	output, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	health := parsePHPFPMHealthOutput(os.Getenv("YUB_PHP_HEALTH_REAL_VERSION"), os.Getenv("YUB_PHP_HEALTH_REAL_INI"), os.Getenv("YUB_PHP_HEALTH_REAL_SCAN"), output)
	if health.State == "unknown" || health.OPcache.ConfigurationState != "known" {
		t.Fatalf("real PHP CLI output was rejected: %+v", health)
	}
}
