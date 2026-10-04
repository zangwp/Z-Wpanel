package tests

import (
	"os"
	"strings"
	"testing"
)

func TestRepositoryRootRemainsGrouped(t *testing.T) {
	entries, err := os.ReadDir("..")
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		".github": true, "cmd": true, "deploy": true, "docs": true,
		"internal": true, "tests": true, "third_party": true, "web": true,
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") && entry.Name() != ".github" {
				continue
			}
			if !allowed[entry.Name()] {
				t.Errorf("ungrouped root directory: %s", entry.Name())
			}
		} else if strings.HasSuffix(entry.Name(), ".go") {
			t.Errorf("Go source belongs under cmd, internal, web or tests: %s", entry.Name())
		}
	}
	for _, name := range []string{"../cmd/yub-wpanel/main.go", "../web/assets.go", "../internal/config/distribution.go", "../install.sh", "../install-cn.sh", "../LICENSE"} {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("required entry %s missing: %v", name, err)
		}
	}
}
