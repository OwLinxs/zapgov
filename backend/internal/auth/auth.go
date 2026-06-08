package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"zapgov/backend/internal/tenantctx"
)

var ErrInvalid = errors.New("credenciais inválidas")

type Service struct {
	repo   *Repository
	secret string
}

func NewService(repo *Repository, secret string) *Service {
	return &Service{repo: repo, secret: secret}
}

// Claims do JWT: identidade + papel para RBAC + tenant para isolamento.
type Claims struct {
	UserID   int64  `json:"uid"`
	TenantID int64  `json:"tid"`
	Nome     string `json:"nome"`
	Email    string `json:"email"`
	Papel    string `json:"papel"`
	Setor    string `json:"setor"`
	jwt.RegisteredClaims
}

func (s *Service) issue(u *User) (string, error) {
	claims := Claims{
		UserID:   u.ID,
		TenantID: u.TenantID,
		Nome:     u.Nome,
		Email:    u.Email,
		Papel:    u.Papel,
		Setor:    u.Setor,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprint(u.ID),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.secret))
}

func (s *Service) parse(tok string) (*Claims, error) {
	c := &Claims{}
	_, err := jwt.ParseWithClaims(tok, c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("método de assinatura inesperado")
		}
		return []byte(s.secret), nil
	})
	return c, err
}

// Login valida email+senha (no tenant resolvido) e devolve um JWT.
func (s *Service) Login(ctx context.Context, tenantID int64, email, senha string) (string, *User, error) {
	u, hash, err := s.repo.byEmail(ctx, tenantID, email)
	if err != nil {
		return "", nil, err
	}
	if u == nil || !u.Ativo {
		return "", nil, ErrInvalid
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(senha)) != nil {
		return "", nil, ErrInvalid
	}
	tok, err := s.issue(u)
	return tok, u, err
}

// ChangePassword troca a senha do usuário após validar a senha atual.
func (s *Service) ChangePassword(ctx context.Context, userID int64, atual, nova string) error {
	if len(nova) < 8 {
		return fmt.Errorf("a nova senha precisa de ao menos 8 caracteres")
	}
	hash, err := s.repo.hashByID(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(atual)) != nil {
		return fmt.Errorf("senha atual incorreta")
	}
	novoHash, err := bcrypt.GenerateFromPassword([]byte(nova), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.setPassword(ctx, userID, string(novoHash))
}

// EnsureSeedAdmin cria um admin padrão no tenant indicado se ele ainda não tiver
// usuários. Usado tanto no bootstrap do tenant 1 quanto ao provisionar um novo.
func (s *Service) EnsureSeedAdmin(ctx context.Context, tenantID int64, email, senha string) {
	n, err := s.repo.countTenant(ctx, tenantID)
	if err != nil || n > 0 {
		return
	}
	if email == "" {
		email = "admin@zapgov.local"
	}
	if senha == "" {
		senha = "admin123"
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(senha), bcrypt.DefaultCost)
	u, err := s.repo.Create(ctx, tenantID, "Administrador", email, string(hash), "admin", "")
	if err != nil {
		log.Printf("auth: falha ao criar admin seed (tenant %d): %v", tenantID, err)
		return
	}
	_ = s.repo.markMustChange(ctx, u.ID) // exige troca no primeiro acesso
	log.Printf("auth: admin seed criado (tenant %d) → %s / %s (TROCA OBRIGATÓRIA no 1º acesso)", tenantID, email, senha)
}

// ---- Middlewares ----

// Required exige um JWT válido e injeta o usuário no contexto.
func (s *Service) Required() fiber.Handler {
	return func(c *fiber.Ctx) error {
		h := c.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "não autenticado"})
		}
		claims, err := s.parse(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "sessão inválida ou expirada"})
		}
		// Tokens emitidos antes do multi-tenant não têm tid → trata como tenant 1
		// (plataforma), evitando tenant_id=0 (que quebra FK e some com os dados).
		tid := claims.TenantID
		if tid == 0 {
			tid = 1
		}
		c.Locals("user", claims)
		c.Locals("papel", claims.Papel) // atalhos p/ outros pacotes sem importar auth
		c.Locals("uid", claims.UserID)
		c.Locals("nome", claims.Nome)
		c.Locals("setor", claims.Setor)
		c.Locals(tenantctx.Key, tid) // isolamento: repos leem via c.Context().Value
		return c.Next()
	}
}

// RequireRole restringe a rota aos papéis informados.
func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u, _ := c.Locals("user").(*Claims)
		if u == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "não autenticado"})
		}
		for _, r := range roles {
			if u.Papel == r {
				return c.Next()
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "sem permissão para esta ação"})
	}
}

// RequirePlatformAdmin restringe a rota ao administrador da plataforma (admin do
// tenant 1). Usado no Super Admin, que opera cross-tenant.
func RequirePlatformAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		u, _ := c.Locals("user").(*Claims)
		if u == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "não autenticado"})
		}
		if u.Papel != "admin" || u.TenantID != 1 {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "acesso restrito ao administrador da plataforma"})
		}
		return c.Next()
	}
}

// ValidToken diz se o JWT é válido (uso no SSE, que autentica por query param).
func (s *Service) ValidToken(token string) bool {
	if token == "" {
		return false
	}
	_, err := s.parse(token)
	return err == nil
}

// TenantFromToken valida o JWT e devolve o tenant do usuário (uso no SSE).
// Tokens legados sem tid caem no tenant 1.
func (s *Service) TenantFromToken(token string) (int64, bool) {
	if token == "" {
		return 0, false
	}
	c, err := s.parse(token)
	if err != nil {
		return 0, false
	}
	tid := c.TenantID
	if tid == 0 {
		tid = 1
	}
	return tid, true
}

// UserFromCtx devolve o usuário autenticado da request.
func UserFromCtx(c *fiber.Ctx) *Claims {
	u, _ := c.Locals("user").(*Claims)
	return u
}
