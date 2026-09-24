package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
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
	SessionVersion int64  `json:"sv"`
	MustChange     bool   `json:"-"`
	UserID         int64  `json:"uid"`
	TenantID       int64  `json:"tid"`
	Nome           string `json:"nome"`
	Email          string `json:"email"`
	Papel          string `json:"papel"`
	Setor          string `json:"setor"`
	jwt.RegisteredClaims
}

func (s *Service) issue(u *User) (string, error) {
	claims := Claims{
		SessionVersion: u.SessionVersion,
		UserID:         u.ID,
		TenantID:       u.TenantID,
		Nome:           u.Nome,
		Email:          u.Email,
		Papel:          u.Papel,
		Setor:          u.Setor,
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
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
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
	if len(nova) < 8 || len(nova) > 72 {
		return fmt.Errorf("a nova senha precisa ter entre 8 e 72 bytes")
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
func (s *Service) EnsureSeedAdmin(ctx context.Context, tenantID int64, email, senha string) error {
	n, err := s.repo.countTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if email == "" {
		email = "admin@zapgov.local"
	}
	if senha == "" {
		senha = os.Getenv("SEED_ADMIN_PASSWORD")
	}
	if len(senha) < 12 || len(senha) > 72 {
		return fmt.Errorf("configure SEED_ADMIN_PASSWORD com 12 a 72 bytes para criar o administrador inicial")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(senha), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if _, err := s.repo.Create(ctx, tenantID, "Administrador", email, string(hash), "admin", ""); err != nil {
		return err
	}
	log.Printf("auth: administrador inicial criado (tenant %d), troca obrigatória no primeiro acesso", tenantID)
	return nil
}

// ---- Middlewares ----

// Required exige um JWT válido e injeta o usuário no contexto.
func (s *Service) Required() fiber.Handler {
	return func(c *fiber.Ctx) error {
		h := c.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "não autenticado"})
		}
		claims, err := s.ValidateSession(c.Context(), strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "sessão inválida ou expirada"})
		}
		tid := claims.TenantID
		if claims.MustChange && c.Path() != "/auth/password" && c.Path() != "/auth/me" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "troca de senha obrigatória", "must_change_password": true})
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

// ValidateSession checks current user/tenant status, revocation and permissions.
func (s *Service) ValidateSession(ctx context.Context, token string) (*Claims, error) {
	c, err := s.parse(token)
	if err != nil {
		return nil, err
	}
	if c.TenantID <= 0 || c.SessionVersion <= 0 {
		return nil, ErrInvalid
	}
	u, err := s.repo.SessionUser(ctx, c.TenantID, c.UserID)
	if err != nil {
		return nil, err
	}
	if !sessionMatches(c, u) {
		return nil, ErrInvalid
	}
	c.Nome, c.Email, c.Papel, c.Setor = u.Nome, u.Email, u.Papel, u.Setor
	c.MustChange = u.MustChange
	return c, nil
}

func sessionMatches(c *Claims, u *User) bool {
	return u != nil && u.Ativo && c.TenantID == u.TenantID && c.UserID == u.ID && c.SessionVersion == u.SessionVersion
}

func (s *Service) TenantFromToken(token string) (int64, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := s.ValidateSession(ctx, token)
	if err != nil || c.MustChange {
		return 0, false
	}
	return c.TenantID, true
}

// UserFromCtx devolve o usuário autenticado da request.
func UserFromCtx(c *fiber.Ctx) *Claims {
	u, _ := c.Locals("user").(*Claims)
	return u
}
