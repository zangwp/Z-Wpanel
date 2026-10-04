package executor

import (
	"context"
	"errors"
	"github.com/zangwp/Z-Wpanel/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPanelACMENginxPublishesOnlyChallengeAndRestoresOnFailure(t *testing.T) {
	previous, previousDir, previousRun := config.AppConfig, panelACMEConfigDir, runPanelACMECommand
	t.Cleanup(func() { config.AppConfig, panelACMEConfigDir, runPanelACMECommand = previous, previousDir, previousRun })
	root := t.TempDir()
	panelACMEConfigDir = t.TempDir()
	config.AppConfig = &config.Config{Paths: config.PathsConfig{WWWRoot: root}}
	var calls []string
	runPanelACMECommand = func(_ context.Context, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		return nil, nil
	}
	webroot, err := panelACMEWebRoot("Panel.Example.com")
	if err != nil {
		t.Fatal(err)
	}
	if webroot != filepath.Join(root, ".yub-panel-acme") {
		t.Fatalf("unsafe root: %s", webroot)
	}
	files, _ := filepath.Glob(filepath.Join(panelACMEConfigDir, "*.conf"))
	if len(files) != 1 {
		t.Fatalf("challenge configuration missing: %v", files)
	}
	content, _ := os.ReadFile(files[0])
	for _, want := range []string{"server_name panel.example.com;", "location ^~ /.well-known/acme-challenge/", "location / { return 404; }"} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("missing isolation %s", want)
		}
	}
	if strings.Join(calls, ";") != "nginx -t;systemctl reload nginx" {
		t.Fatal(calls)
	}
	if _, err := panelACMEWebRoot("panel.example.com"); err != nil || len(calls) != 2 {
		t.Fatal("unchanged configuration should be idempotent", err)
	}
	for _, failure := range []string{"nginx -t", "systemctl reload nginx"} {
		calls = nil
		failed := false
		runPanelACMECommand = func(_ context.Context, args ...string) ([]byte, error) {
			command := strings.Join(args, " ")
			calls = append(calls, command)
			if command == failure && !failed {
				failed = true
				return []byte("failure"), errors.New("rejected")
			}
			return nil, nil
		}
		if _, err := panelACMEWebRoot("other.example.com"); err == nil {
			t.Fatal("expected failure")
		}
		files, _ = filepath.Glob(filepath.Join(panelACMEConfigDir, "*.conf"))
		if len(files) != 1 {
			t.Fatal("failed challenge configuration was retained")
		}
		if failure == "systemctl reload nginx" && len(calls) != 4 {
			t.Fatal("previous configuration was not reloaded", calls)
		}
	}
}

func TestPanelACMERejectsSymlinkChallengeRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(root, ".well-known")); err != nil {
		t.Fatal(err)
	}
	if err := ensurePanelACMEChallengeDirectory(root); err == nil {
		t.Fatal("symlink challenge directory accepted")
	}
}
