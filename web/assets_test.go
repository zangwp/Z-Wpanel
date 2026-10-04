package web

import (
	"errors"
	"io/fs"
	"testing"
)

func TestEmbeddedAssetsKeepPublicAndPrivateBoundaries(t *testing.T) {
	for _, name := range []string{"static/css/main.css", "static/css/theme.css", "static/js/app.js", "static/logo.png"} {
		if data, err := fs.ReadFile(StaticFS, name); err != nil || len(data) == 0 {
			t.Fatalf("public asset %s is missing: %v", name, err)
		}
	}
	for _, name := range []string{"templates/base.html", "source/frontend/input.css", "plugins/yub-wpanel-optimizer/yub-wpanel-optimizer.php"} {
		if _, err := fs.Stat(StaticFS, name); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("private asset %s is exposed by the public filesystem: %v", name, err)
		}
	}
	if data, err := fs.ReadFile(TemplatesFS, "templates/base.html"); err != nil || len(data) == 0 {
		t.Fatalf("embedded templates missing: %v", err)
	}
	for _, name := range []string{"yub-wpanel-optimizer/yub-wpanel-optimizer.php", "yub-wpanel-optimizer/assets/maintenance.js", "yub-wpanel-optimizer/includes/trait-maintenance.php"} {
		if data, err := fs.ReadFile(PluginFS, name); err != nil || len(data) == 0 {
			t.Fatalf("installed plugin path %s changed: %v", name, err)
		}
	}
}
