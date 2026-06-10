package auth

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"zapgov/backend/internal/tenantctx"
)

type Handler struct {
	svc  *Service
	repo *Repository
	// resolveTenant mapeia subdomínio -> tenant_id (padrão 1 se nil/desconhecido).
	resolveTenant func(c *fiber.Ctx, sub string) int64
}

func NewHandler(svc *Service, repo *Repository) *Handler {
	return &Handler{svc: svc, repo: repo}
}

// SetTenantResolver liga o resolvedor de subdomínio (vindo do pacote tenants).
func (h *Handler) SetTenantResolver(fn func(c *fiber.Ctx, sub string) int64) {
	h.resolveTenant = fn
}

// tenantOf descobre o tenant da request de login pelo subdomínio (campo do
// corpo, header X-Tenant ou Host). Default = tenant 1.
func (h *Handler) tenantOf(c *fiber.Ctx, sub string) int64 {
	if sub == "" {
		sub = c.Get("X-Tenant")
	}
	if sub == "" {
		// Extrai o primeiro rótulo do Host: prefeitura.zapgov.com.br -> prefeitura
		host := c.Hostname()
		if i := strings.IndexByte(host, '.'); i > 0 {
			label := host[:i]
			if label != "www" && label != "localhost" {
				sub = label
			}
		}
	}
	if h.resolveTenant == nil {
		return 1
	}
	return h.resolveTenant(c, sub)
}

// RegisterPublic registra rotas sem autenticação (login).
func (h *Handler) RegisterPublic(r fiber.Router) {
	r.Post("/auth/login", h.login)
}

// RegisterProtected registra rotas que exigem JWT. A gestão de usuários é admin-only.
func (h *Handler) RegisterProtected(r fiber.Router) {
	r.Get("/auth/me", h.me)
	r.Post("/auth/password", h.changePassword)
	r.Get("/users", RequireRole("admin", "supervisor"), h.listUsers)
	r.Post("/users", RequireRole("admin"), h.createUser)
	r.Put("/users/:id", RequireRole("admin"), h.updateUser)
	r.Post("/users/:id/reset-password", RequireRole("admin"), h.resetPassword)
}

// resetPassword (admin): define uma senha provisória para o usuário, que será
// obrigado a trocá-la no próximo login.
func (h *Handler) resetPassword(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var body struct {
		Senha string `json:"senha"`
	}
	if err := c.BodyParser(&body); err != nil || len(body.Senha) < 6 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "senha provisória precisa ter ao menos 6 caracteres"})
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Senha), bcrypt.DefaultCost)
	if err := h.repo.adminSetPassword(c.Context(), int64(id), string(hash)); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) changePassword(c *fiber.Ctx) error {
	u := UserFromCtx(c)
	var body struct {
		SenhaAtual string `json:"senha_atual"`
		NovaSenha  string `json:"nova_senha"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	if err := h.svc.ChangePassword(c.Context(), u.UserID, body.SenhaAtual, body.NovaSenha); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) login(c *fiber.Ctx) error {
	var body struct {
		Email      string `json:"email"`
		Senha      string `json:"senha"`
		Subdominio string `json:"subdominio"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JSON inválido"})
	}
	tenantID := h.tenantOf(c, body.Subdominio)
	tok, u, err := h.svc.Login(c.Context(), tenantID, body.Email, body.Senha)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "e-mail ou senha incorretos"})
	}
	return c.JSON(fiber.Map{"token": tok, "user": u})
}

func (h *Handler) me(c *fiber.Ctx) error {
	u := UserFromCtx(c)
	return c.JSON(fiber.Map{
		"id": u.UserID, "nome": u.Nome, "email": u.Email, "papel": u.Papel,
	})
}

func (h *Handler) listUsers(c *fiber.Ctx) error {
	users, err := h.repo.List(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"users": users})
}

func (h *Handler) createUser(c *fiber.Ctx) error {
	var body struct {
		Nome  string `json:"nome"`
		Email string `json:"email"`
		Senha string `json:"senha"`
		Papel string `json:"papel"`
		Setor string `json:"setor"`
	}
	if err := c.BodyParser(&body); err != nil || body.Nome == "" || body.Email == "" || body.Senha == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "nome, e-mail e senha são obrigatórios"})
	}
	if body.Papel == "" {
		body.Papel = "atendente"
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Senha), bcrypt.DefaultCost)
	u, err := h.repo.Create(c.Context(), tenantctx.From(c.Context()), body.Nome, body.Email, string(hash), body.Papel, body.Setor)
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(u)
}

func (h *Handler) updateUser(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id inválido"})
	}
	var body struct {
		Nome  string `json:"nome"`
		Email string `json:"email"`
		Papel string `json:"papel"`
		Setor string `json:"setor"`
		Ativo bool   `json:"ativo"`
	}
	if err := c.BodyParser(&body); err != nil || body.Nome == "" || body.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "nome e e-mail são obrigatórios"})
	}
	if err := h.repo.Update(c.Context(), int64(id), body.Nome, body.Email, body.Papel, body.Setor, body.Ativo); err != nil {
		if errors.Is(err, ErrEmailExists) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}
