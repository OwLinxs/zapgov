package messaging

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"zapgov/backend/internal/tenantctx"
	"zapgov/backend/internal/testdb"
)

func TestSectorBoundaryOnDirectRoutes(t *testing.T) {
	pool := testdb.New(t)
	ctx := tenantctx.With(context.Background(), 1)
	repo := NewRepository(pool)
	own, _ := repo.EnsureConversation(ctx, "5511999990001", "Own")
	other, _ := repo.EnsureConversation(ctx, "5511999990002", "Other")
	for id, sector := range map[int64]string{own: "Saúde", other: "Educação"} {
		if err := repo.SetSetor(ctx, id, sector); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.InsertInbound(ctx, id, fmt.Sprintf("in-%d", id), "privado", time.Now(), MediaInfo{}, ""); err != nil {
			t.Fatal(err)
		}
	}
	all, err := repo.ListConversations(ctx, 100, nil, "", "", false)
	if err != nil || len(all) != 2 {
		t.Fatalf("admin list: %d %v", len(all), err)
	}
	none, err := repo.ListConversations(ctx, 100, []string{}, "", "", false)
	if err != nil || len(none) != 0 {
		t.Fatalf("empty scope exposed conversations: %d %v", len(none), err)
	}
	h := NewHandler(NewService(repo, UnavailableProvider{}))
	h.RouteBySetor = func(context.Context) bool { return true }
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(tenantctx.Key, int64(1))
		c.Locals("papel", "atendente")
		c.Locals("setor", "Saúde")
		return c.Next()
	})
	h.Register(app)
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{{"GET", fmt.Sprintf("/conversations/%d/messages", own), "", 200}, {"GET", fmt.Sprintf("/conversations/%d/messages", other), "", 404}, {"PUT", fmt.Sprintf("/conversations/%d/claim", other), `{}`, 404}, {"POST", "/messages/send", `{"to":"5511999990002","body":"not allowed"}`, 404}, {"POST", "/conversations/resolve-bulk", fmt.Sprintf(`{"ids":[%d,%d]}`, own, other), 404}} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != tc.status {
			t.Errorf("%s %s: %d want %d", tc.method, tc.path, res.StatusCode, tc.status)
		}
	}
	// The sector boundary must not prevent template initiation. The unavailable
	// provider returns 502, proving this request reached the send handler.
	for _, blocked := range []bool{false, true} {
		h.TemplateAllowed = func(context.Context, string, string) bool { return !blocked }
		req := httptest.NewRequest("POST", "/messages/template", strings.NewReader(`{"to":"5511999990002","template":"teste","language":"pt_BR"}`))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		want := 502
		if blocked {
			want = 403
		}
		if res.StatusCode != want {
			t.Errorf("template blocked=%v: got %d want %d", blocked, res.StatusCode, want)
		}
	}
	msgs, err := repo.ListRecentScoped(ctx, 50, []string{"Saúde"})
	if err != nil || len(msgs) != 1 || msgs[0].ConversationID != own {
		t.Fatalf("list isolation: %+v %v", msgs, err)
	}
}
func TestStatusDoesNotRegress(t *testing.T) {
	for _, tc := range [][3]string{{"read", "sent", "read"}, {"read", "delivered", "read"}, {"delivered", "failed", "delivered"}, {"sent", "read", "read"}, {"sent", "failed", "failed"}} {
		if got := nextStatus(tc[0], tc[1]); got != tc[2] {
			t.Errorf("%v got %s", tc, got)
		}
	}
}

func TestStoredStatusProgression(t *testing.T) {
	pool := testdb.New(t)
	ctx := tenantctx.With(context.Background(), 1)
	repo := NewRepository(pool)
	conv, err := repo.EnsureConversation(ctx, "5511999990001", "Test")
	if err != nil {
		t.Fatal(err)
	}
	msg, err := repo.InsertOutbound(ctx, conv, "out-test", "test", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range []string{"failed", "sent"} {
		if _, _, err := repo.UpdateStatus(ctx, msg.WAMessageID, st, map[string]string{"failed": "provider refusal"}[st]); err != nil {
			t.Fatal(err)
		}
	}
	var status, detail string
	if err := pool.QueryRow(ctx, `SELECT status,error FROM messages WHERE id=$1`, msg.ID).Scan(&status, &detail); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || detail != "provider refusal" {
		t.Fatalf("regressed failure: %s %s", status, detail)
	}
	if _, _, err := repo.UpdateStatus(ctx, msg.WAMessageID, "read", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.UpdateStatus(ctx, msg.WAMessageID, "delivered", ""); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM messages WHERE id=$1`, msg.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "read" {
		t.Fatalf("regressed read: %s", status)
	}
}
