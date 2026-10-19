package i18n

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestSecurityHistoryAndLoginTranslationsReachActualPageCatalog(t *testing.T) {
	catalog, err := NewTemplateMessages(os.DirFS("../../web/templates"), AllMessageKeys())
	if err != nil {
		t.Fatal(err)
	}
	pageKeys := map[string][]string{
		"websites_detail_content": {
			"site_security.verification_save_failed", "site_security.verification_history_unavailable",
			"site_security.last_verification", "site_security.last_verification_evidence", "site_security.verification_applicability",
			"site_security.history_never_checked", "site_security.history_passed", "site_security.history_ready",
			"site_security.history_failed", "site_security.history_incomplete", "site_security.history_configuration_changed",
			"site_security.history_configuration_unavailable", "site_security.verification_result_passed", "site_security.verification_result_ready",
			"site_security.verification_result_failed", "site_security.verification_result_incomplete", "site_security.recheck_short",
		},
		"security_content": {
			"security.ip_update_running", "security.refresh_status_unavailable", "security.refresh_completed",
			"security.refresh_completed_status_unavailable", "security.refresh_running", "security.refresh_check_result", "security.refresh_failed",
		},
		"login":            {"auth.login_timeout", "auth.gateway_auth_required"},
		"settings_content": {"settings.account_change_in_progress"},
	}
	for _, locale := range []string{DefaultLang, English} {
		for page, keys := range pageKeys {
			t.Run(locale+"/"+page, func(t *testing.T) {
				var messages map[string]string
				if err := json.Unmarshal([]byte(catalog.JSON(locale, page)), &messages); err != nil {
					t.Fatal(err)
				}
				for _, key := range keys {
					want := T(locale, key)
					if want == "" || want == key || strings.TrimSpace(messages[key]) != want {
						t.Errorf("missing translated %s in page catalog: got %q, want %q", key, messages[key], want)
					}
				}
			})
		}
	}
}
