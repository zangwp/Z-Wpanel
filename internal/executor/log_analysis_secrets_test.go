package executor

import (
	"strings"
	"testing"
)

func TestLogAnalysisRedactsCompleteCredentialValuesBeforeAI(t *testing.T) {
	for _, input := range []string{
		`Authorization: Bearer bearer-secret`,
		`Authorization: Basic basic-secret`,
		`Proxy-Authorization: Digest username=digest-user response=digest-secret`,
		`Cookie: session=cookie-secret; other=second-secret`,
		`Set-Cookie: session=cookie-secret; Path=/; HttpOnly`,
		`{"password":"quoted secret with spaces","token":"escaped \"secret\""}`,
		`{'api_key':'quoted secret with spaces','authorization':'Bearer json-secret'}`,
		`/path?token=query-secret&api-key=key-secret`,
	} {
		t.Run(input, func(t *testing.T) {
			got := sanitizeLogAnalysisAITextKeepingIPs(input)
			for _, secret := range []string{"bearer-secret", "basic-secret", "digest-user", "digest-secret", "cookie-secret", "second-secret", "quoted secret", "escaped", "json-secret", "query-secret", "key-secret"} {
				if strings.Contains(got, secret) {
					t.Fatalf("credential value remained in AI evidence: %q", got)
				}
			}
			if !strings.Contains(got, "[redacted]") {
				t.Fatalf("missing redaction: %q", got)
			}
			if again := sanitizeLogAnalysisAITextKeepingIPs(got); again != got {
				t.Fatalf("redaction is not stable: %q -> %q", got, again)
			}
		})
	}
	plain := "203.0.113.10 /wp-login.php PHP warning"
	if got := sanitizeLogAnalysisAITextKeepingIPs(plain); got != plain {
		t.Fatalf("non-credential evidence changed: %q", got)
	}
}
