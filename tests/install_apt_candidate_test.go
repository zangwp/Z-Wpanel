package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallAPTCandidateLargePolicy(t *testing.T) {
	script := readInstallScript(t, installScriptPath)
	helper := extractShellFunction(t, script, "apt_candidate_version", "install_verified_apt_key")
	// Real nginx.org policy output grows as old package versions accumulate.
	// Write far beyond pipe capacity to detect consumers that exit too early.
	policy := filepath.Join(t.TempDir(), "policy.txt")
	data := "nginx:\n  Candidate: 1.30.5-1~trixie\n" + strings.Repeat("     1.30.4-1~trixie 900 https://nginx.org/packages/debian trixie/nginx amd64 Packages\n", 100000)
	if err := os.WriteFile(policy, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	shell := `set -e -o pipefail
apt-cache() { cat "$APT_POLICY_FIXTURE"; }
` + helper + `
candidate=$(apt_candidate_version nginx)
[[ "$candidate" == "1.30.5-1~trixie" ]]
printf 'candidate-ok\n'
`
	cmd := exec.Command("bash", "-c", shell)
	cmd.Env = append(os.Environ(), "APT_POLICY_FIXTURE="+filepath.ToSlash(policy))
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "candidate-ok") {
		t.Fatalf("large APT policy must not abort installation: %v\n%s", err, output)
	}
}

func TestInstallAPTCandidatePreservesFailure(t *testing.T) {
	script := readInstallScript(t, installScriptPath)
	helper := extractShellFunction(t, script, "apt_candidate_version", "install_verified_apt_key")
	shell := `set -e -o pipefail
apt-cache() { printf '  Candidate: 1.30.5-1~trixie\n'; return 42; }
` + helper + `
status=0
candidate=$(apt_candidate_version nginx) || status=$?
[[ "$status" == 42 ]]
`
	if output, err := exec.Command("bash", "-c", shell).CombinedOutput(); err != nil {
		t.Fatalf("genuine APT failures must propagate: %v\n%s", err, output)
	}
}

func TestInstallFailureReportsStageAndExitCode(t *testing.T) {
	script := readInstallScript(t, installScriptPath)
	exitHandler := extractShellFunction(t, script, "installer_exit", "systemctl_enable_best_effort")
	shell := `set -e -o pipefail
INSTALL_STAGE='配置 Nginx APT 源与检查候选版本'
APT_SOURCES_MUTATED=false
repair_rollback() { :; }
cleanup_failed_fresh_panel_service() { :; }
cleanup_atomic_stage_file() { :; }
cleanup_install_workdir() { :; }
` + exitHandler + `
trap installer_exit EXIT
(exit 42)
`
	output, err := exec.Command("bash", "-c", shell).CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 42 || !strings.Contains(string(output), "失败阶段: 配置 Nginx APT 源与检查候选版本；退出码: 42") {
		t.Fatalf("failure must retain exit code and report stage: %v\n%s", err, output)
	}
}

func TestInstallNginxCandidateDiagnostics(t *testing.T) {
	script := readInstallScript(t, installScriptPath)
	configure := extractShellFunction(t, script, "configure_nginx_repository", "preserve_existing_php_series")
	for _, tc := range []struct{ name, candidate, aptStatus, lookupStatus, versionStatus, want string }{
		{"refresh", "", "31", "0", "0", "刷新 Nginx APT 源失败"},
		{"lookup", "", "0", "42", "0", "读取 Nginx APT 候选版本失败"},
		{"missing", "(none)", "0", "0", "0", "未提供候选包"},
		{"empty", "", "0", "0", "0", "未提供候选包"},
		{"old", "1.28.0", "0", "0", "1", "1.28.0 低于所需版本 1.30.5"},
		{"available", "1.30.5", "0", "0", "0", "Nginx 官方稳定候选包可用: 1.30.5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range []string{"sources.list.d", "preferences.d"} {
				if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			// Redirect every repository write to a disposable fixture; no APT or system mutations.
			helper := strings.ReplaceAll(configure, "/etc/apt/", "${FIXTURE_ROOT}/")
			shell := `set -e -o pipefail
PLATFORM_ID=debian
PLATFORM_CODENAME=trixie
PLATFORM_ARCH=amd64
assert_managed_source_target() { :; }
install_verified_apt_key() { :; }
apt-get() { return "$APT_STATUS"; }
apt_candidate_version() { printf '%s\n' "$CANDIDATE"; return "$LOOKUP_STATUS"; }
dpkg() { return "$VERSION_STATUS"; }
log_error() { printf '%s\n' "$*"; exit 1; }
log_info() { printf '%s\n' "$*"; }
` + helper + "\nconfigure_nginx_repository\n"
			cmd := exec.Command("bash", "-c", shell)
			cmd.Env = append(os.Environ(), "FIXTURE_ROOT="+filepath.ToSlash(root), "APT_STATUS="+tc.aptStatus, "LOOKUP_STATUS="+tc.lookupStatus, "VERSION_STATUS="+tc.versionStatus, "CANDIDATE="+tc.candidate)
			output, err := cmd.CombinedOutput()
			if (err == nil) != (tc.name == "available") || !strings.Contains(string(output), tc.want) {
				t.Fatalf("unexpected candidate result: %v\n%s", err, output)
			}
		})
	}
}
