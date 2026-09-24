package webhook

import (
	"context"
	"errors"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
	"zapgov/backend/internal/messaging"
)

type signatureProvider struct {
	messaging.UnavailableProvider
	valid bool
}

func (p signatureProvider) VerifySignature([]byte, string) bool { return p.valid }
func TestWebhookAcknowledgesOnlyPersistedEvents(t *testing.T) {
	payload := `{"entry":[{"changes":[{"value":{"metadata":{"phone_number_id":"number"}}}]}]}`
	for _, tc := range []struct {
		name   string
		valid  bool
		tenant int64
		fail   bool
		status int
	}{{"stored", true, 2, false, 200}, {"storage failure", true, 2, true, 503}, {"unknown number", true, 0, false, 403}, {"invalid signature", false, 2, false, 403}} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandler(nil, signatureProvider{valid: tc.valid}, "verify")
			h.SetTenantResolver(func(string) int64 { return tc.tenant })
			called := false
			h.Accept = func(ctx context.Context, b []Envelope) error {
				called = true
				if len(b) != 1 || b[0].TenantID != 2 {
					t.Fatal("wrong tenant")
				}
				if tc.fail {
					return errors.New("unavailable")
				}
				return nil
			}
			app := fiber.New()
			h.Register(app)
			req := httptest.NewRequest("POST", "/webhook", strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			res, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != tc.status {
				t.Fatalf("got %d want %d", res.StatusCode, tc.status)
			}
			if called != (tc.valid && tc.tenant > 0) {
				t.Fatal("unverified payload persisted")
			}
		})
	}
}
