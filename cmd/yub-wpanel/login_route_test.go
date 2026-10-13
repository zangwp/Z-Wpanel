package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/zangwp/Z-Wpanel/internal/config"
	"github.com/zangwp/Z-Wpanel/internal/database"
	"github.com/zangwp/Z-Wpanel/internal/middleware"
	"github.com/zangwp/Z-Wpanel/internal/router"
	"github.com/zangwp/Z-Wpanel/web"
	"golang.org/x/crypto/bcrypt"
)

func TestLoginTokenRefreshWorksBeforeSessionThroughRealRouter(t *testing.T) {
	dir := t.TempDir()
	oldDB, oldConfig := database.DB, config.AppConfig
	if err := database.Open(filepath.Join(dir, "panel.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close(); database.DB = oldDB; config.AppConfig = oldConfig })
	if err := database.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("fixture-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.GetDB().Exec("INSERT INTO admin_users(id,username,password_hash) VALUES(1,'wpadmin',?)", string(hash)); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{BasicAuth: config.BasicAuthConfig{Username: "gateway", PasswordHash: string(hash)}}
	cfg.Panel.RandomSuffix = "login-regression"
	cfg.Panel.DataDir = dir
	cfg.Panel.TLSCertPath = filepath.Join(dir, "unused.crt")
	cfg.SQLite.Path = filepath.Join(dir, "panel.db")
	cfg.Security.MaxLoginAttempts, cfg.Security.AttemptWindowMinutes, cfg.Security.BanDurationHours = 100, 5, 1
	config.AppConfig = cfg
	middleware.GlobalSessionStore.DeleteAll()
	t.Cleanup(middleware.GlobalSessionStore.DeleteAll)
	engine, cleanup := router.SetupRouterWithCleanup(cfg, web.TemplatesFS, web.StaticFS, "login-regression", "")
	t.Cleanup(cleanup)
	server := httptest.NewTLSServer(engine)
	t.Cleanup(server.Close)
	client := server.Client()
	client.Jar, err = cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, body []byte, token string, gateway bool) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+"/login-regression"+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0 LoginRegression")
		if gateway {
			req.SetBasicAuth("gateway", "fixture-password")
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("X-CSRF-Token", token)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	read := func(resp *http.Response) []byte {
		t.Helper()
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	resp := request("GET", "/api/auth/csrf-token", nil, "", false)
	read(resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("token endpoint bypassed gateway: %d", resp.StatusCode)
	}
	resp = request("GET", "/login", nil, "", true)
	if body := read(resp); resp.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("loginForm()")) {
		t.Fatalf("login page status=%d", resp.StatusCode)
	}
	resp = request("GET", "/api/auth/csrf-token", nil, "", true)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(read(resp), &payload); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || !payload.Success || payload.Data.Token == "" || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("pre-login token status=%d payload=%+v", resp.StatusCode, payload)
	}
	for _, cookie := range client.Jar.Cookies(resp.Request.URL) {
		if cookie.Name == "wp_session" {
			t.Fatal("token refresh created an authenticated session")
		}
	}
	resp = request("GET", "/api/auth/check", nil, "", true)
	read(resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatal("token refresh unlocked protected API")
	}
	body := []byte(`{"username":"wpadmin","password":"fixture-password"}`)
	for _, token := range []string{"", "wrong-token"} {
		resp = request("POST", "/api/auth/login", body, token, true)
		read(resp)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("invalid CSRF accepted: %d", resp.StatusCode)
		}
	}
	resp = request("POST", "/api/auth/login", body, payload.Data.Token, true)
	if data := read(resp); resp.StatusCode != http.StatusOK || !bytes.Contains(data, []byte(`"success":true`)) {
		t.Fatalf("login failed: %d %s", resp.StatusCode, data)
	}
	resp = request("GET", "/api/auth/check", nil, "", true)
	if data := read(resp); resp.StatusCode != http.StatusOK || !bytes.Contains(data, []byte("wpadmin")) {
		t.Fatalf("session unusable after login: %d %s", resp.StatusCode, data)
	}
	resp = request("GET", "/api/security/settings", nil, "", true)
	data := read(resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("security settings status=%d", resp.StatusCode)
	}
	fixture := filepath.Join(dir, "security-settings.json")
	if err := os.WriteFile(fixture, data, 0600); err != nil {
		t.Fatal(err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node is required for the frontend API contract test:", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "../../tests/js/security_center_state.cjs")
	cmd.Env = append(os.Environ(), "YUB_SECURITY_SETTINGS_FIXTURE="+fixture)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("real security API rejected by frontend: %v\n%s", err, output)
	}
}
