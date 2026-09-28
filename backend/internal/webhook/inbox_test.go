package webhook

import (
	"context"
	"errors"
	"strings"
	"testing"
	"zapgov/backend/internal/tenantctx"
	"zapgov/backend/internal/testdb"
)

func TestSplitMixedPhoneNumbers(t *testing.T) {
	body := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"a","changes":[{"value":{"metadata":{"phone_number_id":"one"}}},{"value":{"metadata":{"phone_number_id":"two"}}}]}]}`)
	parts, err := splitPayload(body)
	if err != nil || len(parts) != 2 {
		t.Fatalf("split: %v", err)
	}
	if phoneNumberIDFromBody(parts[0]) != "one" || phoneNumberIDFromBody(parts[1]) != "two" {
		t.Fatal("mixed tenant payload")
	}
	if _, err := splitPayload([]byte(`{"entry":[]}`)); err == nil {
		t.Fatal("accepted empty payload")
	}
}
func TestInboxDurableRetry(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	i := NewInbox(pool)
	batch := []Envelope{{TenantID: 1, Body: []byte(`{"entry":[]}`)}}
	for n := 0; n < 2; n++ {
		if err := i.Accept(ctx, batch); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_inbox`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("dedup: %d %v", count, err)
	}
	if found, err := i.ProcessOne(ctx, func(c context.Context, b []byte) error {
		if tenantctx.From(c) != 1 {
			t.Fatal("wrong tenant")
		}
		return errors.New("database unavailable")
	}); !found || err == nil {
		t.Fatal("processing error lost")
	}
	if _, err := pool.Exec(ctx, `UPDATE webhook_inbox SET next_attempt_at=now()`); err != nil {
		t.Fatal(err)
	}
	if found, err := i.ProcessOne(ctx, func(context.Context, []byte) error { return nil }); !found || err != nil {
		t.Fatalf("retry: %v %v", found, err)
	}
	if found, err := i.ProcessOne(ctx, func(context.Context, []byte) error { t.Fatal("duplicate processing"); return nil }); found || err != nil {
		t.Fatalf("completed: %v %v", found, err)
	}
}

// Old failing statuses must yield to received messages, even when their retry
// is already due. No event is discarded to unblock the inbox.
func TestInboxRetryDoesNotStarveNewEvents(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	inbox := NewInbox(pool)
	if err := inbox.Accept(ctx, []Envelope{
		{TenantID: 1, Body: []byte(`{"kind":"unknown_status"}`)},
		{TenantID: 1, Body: []byte(`{"kind":"incoming_message"}`)},
	}); err != nil {
		t.Fatal(err)
	}
	if found, err := inbox.ProcessOne(ctx, func(context.Context, []byte) error { return errors.New("missing message") }); !found || err == nil {
		t.Fatal("expected persisted retry")
	}
	// Simulate enough elapsed time for the retry to be eligible, while keeping
	// the fresh event older in eligibility order than the rescheduled status.
	if _, err := pool.Exec(ctx, `UPDATE webhook_inbox SET next_attempt_at=CASE WHEN attempts=0 THEN now()-interval '2 minutes' ELSE now()-interval '1 minute' END`); err != nil {
		t.Fatal(err)
	}
	if found, err := inbox.ProcessOne(ctx, func(_ context.Context, body []byte) error {
		if !strings.Contains(string(body), "incoming_message") {
			t.Fatalf("retry starved fresh event: %s", body)
		}
		return nil
	}); !found || err != nil {
		t.Fatalf("fresh event: %v %v", found, err)
	}
	var pending, done int
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE processed_at IS NULL),count(*) FILTER (WHERE processed_at IS NOT NULL) FROM webhook_inbox`).Scan(&pending, &done); err != nil || pending != 1 || done != 1 {
		t.Fatalf("retention: %d %d %v", pending, done, err)
	}
}
