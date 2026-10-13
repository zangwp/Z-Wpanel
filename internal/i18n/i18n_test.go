package i18n

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestAllMessageKeysIncludesLeavesAndPageMessages(t *testing.T) {
	keys := AllMessageKeys()
	found := map[string]bool{}
	for _, key := range keys {
		found[key] = true
	}
	for _, key := range []string{"common.request_failed", "nav.dashboard", "site_security.verifying", "settings.saving"} {
		if !found[key] {
			t.Fatalf("missing leaf key %s", key)
		}
	}
	if found["common"] || found["settings"] {
		t.Fatal("translation namespace exposed as a leaf")
	}
	catalog, err := NewTemplateMessages(os.DirFS("../../web"), keys)
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{DefaultLang, English} {
		var values map[string]string
		if err := json.Unmarshal([]byte(catalog.JSON(lang, "settings")), &values); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"common.request_failed", "settings.saving"} {
			if values[key] == "" || strings.TrimSpace(values[key]) == key {
				t.Fatalf("%s/%s has no actual translation", lang, key)
			}
		}
	}
}

func TestNormalizeLang(t *testing.T) {
	tests := map[string]string{
		"":      DefaultLang,
		"zh":    DefaultLang,
		"zh_CN": DefaultLang,
		"en":    English,
		"en_us": English,
	}
	for input, want := range tests {
		if got := NormalizeLang(input); got != want {
			t.Fatalf("NormalizeLang(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTranslateAndFallback(t *testing.T) {
	if got := T(English, "nav.dashboard"); got != "Dashboard" {
		t.Fatalf("English dashboard = %q", got)
	}
	if got := T("bad", "nav.dashboard"); got != "控制台" {
		t.Fatalf("default language dashboard = %q", got)
	}

	originalMessages := messages
	messages = map[string]map[string]any{
		DefaultLang: {"sample": map[string]any{"fallback": "中文回退"}},
		English:     {"sample": map[string]any{}},
	}
	t.Cleanup(func() {
		messages = originalMessages
	})
	if got := T(English, "sample.fallback"); got != "中文回退" {
		t.Fatalf("English missing key fallback = %q", got)
	}
	if got := T(English, "missing.key"); got != "missing.key" {
		t.Fatalf("missing key = %q", got)
	}
}

func TestLangFromRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "/panel?lang=en-US", nil)
	if got := LangFromRequest(req); got != English {
		t.Fatalf("query lang = %q", got)
	}

	req = httptest.NewRequest("GET", "/panel", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: English})
	if got := LangFromRequest(req); got != English {
		t.Fatalf("cookie lang = %q", got)
	}
}

func TestMaybeSetLanguageCookieUsesHardenedAttributes(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panel?lang=en-US", nil)
	MaybeSetLanguageCookie(recorder, req)

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies=%d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != CookieName || cookie.Value != English || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie attributes are not hardened: %+v", cookie)
	}
}
