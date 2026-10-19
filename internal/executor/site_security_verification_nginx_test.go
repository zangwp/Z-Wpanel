package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebsiteSecurityFPMContentAndNginxIncludesHaveScopedFingerprints(t *testing.T) {
	site := securityVerificationWebsite(t)
	read := func(key string) string {
		t.Helper()
		value, ok := websiteSecurityFingerprint(site, key)
		if !ok {
			t.Fatalf("%s fingerprint unavailable", key)
		}
		return value
	}
	core, tls := read("debug_display"), read("https")
	if err := os.WriteFile(site.PHPPoolPath, []byte("[site]\nuser = wp_other\ngroup = wp_other\nphp_admin_flag[display_errors] = on\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if read("debug_display") == core || read("https") != tls {
		t.Fatal("FPM content change was not scoped to the PHP execution environment")
	}
	core = read("debug_display")
	root := filepath.Dir(site.NginxConfPath)
	first, second := filepath.Join(root, "first.conf"), filepath.Join(root, "second.conf")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("# initial included configuration\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Includes on one line must both be read. Comment examples are not directives.
	content := fmt.Sprintf("# include /never/read/this.conf;\ninclude %q; include %q;\n", first, second)
	if err := os.WriteFile(site.NginxConfPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	request := read("sensitive_files")
	if err := os.WriteFile(second, []byte("location = /private { return 404; }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if read("sensitive_files") == request || read("debug_display") != core {
		t.Fatal("included routing changes were not scoped to Nginx request checks")
	}
	if err := os.Remove(second); err != nil {
		t.Fatal(err)
	}
	if _, ok := websiteSecurityFingerprint(site, "sensitive_files"); ok {
		t.Fatal("missing required include was represented as unchanged")
	}
}

func TestWebsiteSecurityNginxGlobChangesAndIncludeBudgets(t *testing.T) {
	site := securityVerificationWebsite(t)
	root := filepath.Dir(site.NginxConfPath)
	includeDir := filepath.Join(root, "snippets")
	if err := os.Mkdir(includeDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(site.NginxConfPath, []byte(fmt.Sprintf("include %q;\n", filepath.Join(includeDir, "*.conf"))), 0600); err != nil {
		t.Fatal(err)
	}
	before, ok := websiteSecurityFingerprint(site, "https")
	if !ok {
		t.Fatal("empty include glob was not accepted")
	}
	if err := os.WriteFile(filepath.Join(includeDir, "new.conf"), []byte("# newly matching include\n"), 0600); err != nil {
		t.Fatal(err)
	}
	after, ok := websiteSecurityFingerprint(site, "https")
	if !ok || before == after {
		t.Fatal("new glob match did not invalidate the historical fingerprint")
	}
	for n := 0; n < 6; n++ {
		content := "# end\n"
		if n < 5 {
			content = fmt.Sprintf("include %q;\n", filepath.Join(root, fmt.Sprintf("depth-%d.conf", n+1)))
		}
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("depth-%d.conf", n)), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(site.NginxConfPath, []byte(fmt.Sprintf("include %q;\n", filepath.Join(root, "depth-0.conf"))), 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok := websiteSecurityFingerprint(site, "https"); ok {
		t.Fatal("unbounded include recursion was accepted")
	}
	if err := os.WriteFile(site.NginxConfPath, []byte(strings.Repeat("#", 256*1024+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok := websiteSecurityFingerprint(site, "https"); ok {
		t.Fatal("oversized configuration was accepted")
	}
}
