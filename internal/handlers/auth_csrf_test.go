package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	_ "modernc.org/sqlite"
)

func setupLoginCSRFTest(t *testing.T) (*gin.Engine, *sql.DB) {
	t.Helper()
	handler, router := setupMFAHandler(t)
	return router, handler.DB
}

func performLoginCSRFTest(t *testing.T, router http.Handler, cookieToken, headerToken string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"admin","password":"correct-password"}`))
	req.Header.Set("Content-Type", "application/json")
	if cookieToken != "" {
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: cookieToken})
	}
	if headerToken != "" {
		req.Header.Set("X-CSRF-Token", headerToken)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestLoginRequiresMatchingCSRFToken(t *testing.T) {
	router, _ := setupLoginCSRFTest(t)

	if recorder := performLoginCSRFTest(t, router, "", ""); recorder.Code != http.StatusForbidden {
		t.Fatalf("missing token status=%d, want %d", recorder.Code, http.StatusForbidden)
	}
	if recorder := performLoginCSRFTest(t, router, "cookie-token", "other-token"); recorder.Code != http.StatusForbidden {
		t.Fatalf("mismatched token status=%d, want %d", recorder.Code, http.StatusForbidden)
	}
	recorder := performLoginCSRFTest(t, router, "matching-token", "matching-token")
	if recorder.Code != http.StatusOK {
		t.Fatalf("matching token status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(recorder.Result().Cookies()) == 0 {
		t.Fatal("successful login did not set a session cookie")
	}
}
