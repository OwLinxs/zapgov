package auth

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"net/http/httptest"
	"testing"
	"zapgov/backend/internal/tenantctx"
	"zapgov/backend/internal/testdb"
)

func TestSessionMatches(t *testing.T) {
	c := &Claims{UserID: 1, TenantID: 2, SessionVersion: 3}
	for _, u := range []*User{nil, {ID: 1, TenantID: 2, SessionVersion: 2, Ativo: true}, {ID: 1, TenantID: 3, SessionVersion: 3, Ativo: true}, {ID: 1, TenantID: 2, SessionVersion: 3}} {
		if sessionMatches(c, u) {
			t.Fatalf("accepted revoked or inactive identity: %+v", u)
		}
	}
	if !sessionMatches(c, &User{ID: 1, TenantID: 2, SessionVersion: 3, Ativo: true}) {
		t.Fatal("valid session rejected")
	}
}
func TestLiveAuthorization(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	repo := NewRepository(pool)
	svc := NewService(repo, "test-secret-that-is-only-for-tests")
	hash, _ := bcrypt.GenerateFromPassword([]byte("initial-password"), bcrypt.MinCost)
	u, err := repo.Create(ctx, 1, "Teste", "test@example.invalid", string(hash), "admin", "")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := svc.Login(ctx, 1, u.Email, "initial-password")
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(svc.Required())
	app.Get("/protected", func(c *fiber.Ctx) error { return c.SendStatus(200) })
	app.Get("/auth/me", func(c *fiber.Ctx) error { return c.SendStatus(200) })
	request := func(path string, want int) {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != want {
			t.Fatalf("%s: got %d want %d", path, res.StatusCode, want)
		}
	}
	request("/protected", 403)
	request("/auth/me", 200)
	if _, err := pool.Exec(ctx, `UPDATE users SET must_change_password=false,papel='atendente',setor='Saúde' WHERE id=$1`, u.ID); err != nil {
		t.Fatal(err)
	}
	c, err := svc.ValidateSession(ctx, token)
	if err != nil || c.Papel != "atendente" || c.Setor != "Saúde" {
		t.Fatalf("live claims: %+v %v", c, err)
	}
	request("/protected", 200)
	if err := repo.adminSetPassword(tenantctx.With(ctx, 1), u.ID, string(hash)); err != nil {
		t.Fatal(err)
	}
	request("/protected", 401)
	token, _, err = svc.Login(ctx, 1, u.Email, "initial-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE tenants SET ativo=false WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	request("/auth/me", 401)
}
