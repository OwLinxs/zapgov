// Package auth faz login (JWT) e RBAC (admin/supervisor/atendente).
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

type User struct {
	ID         int64     `json:"id"`
	TenantID   int64     `json:"-"`
	Nome       string    `json:"nome"`
	Email      string    `json:"email"`
	Papel      string    `json:"papel"`
	Setor      string    `json:"setor"`
	Ativo      bool      `json:"ativo"`
	MustChange bool      `json:"must_change_password"`
	CriadoEm   time.Time `json:"criado_em"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// byEmail traz o usuário + hash da senha (uso interno no login). O tenant é
// explícito (resolvido a partir do subdomínio antes do login).
func (r *Repository) byEmail(ctx context.Context, tenantID int64, email string) (*User, string, error) {
	var u User
	var hash string
	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, nome, email, senha_hash, papel, setor, ativo, must_change_password, criado_em
		FROM users WHERE tenant_id = $1 AND lower(email) = lower($2)`, tenantID, email).
		Scan(&u.ID, &u.TenantID, &u.Nome, &u.Email, &hash, &u.Papel, &u.Setor, &u.Ativo, &u.MustChange, &u.CriadoEm)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", nil
	}
	return &u, hash, err
}

// hashByID traz o hash atual (para validar a senha na troca).
func (r *Repository) hashByID(ctx context.Context, id int64) (string, error) {
	var hash string
	err := r.pool.QueryRow(ctx, `SELECT senha_hash FROM users WHERE id = $1`, id).Scan(&hash)
	return hash, err
}

// setPassword grava nova senha e limpa a flag de troca obrigatória.
func (r *Repository) setPassword(ctx context.Context, id int64, hash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET senha_hash = $2, must_change_password = false WHERE id = $1`, id, hash)
	return err
}

// markMustChange marca o usuário para trocar a senha no próximo acesso.
func (r *Repository) markMustChange(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET must_change_password = true WHERE id = $1`, id)
	return err
}

// adminSetPassword (reset pelo admin): grava a senha provisória e força a troca
// no próximo login. Escopo ao tenant para impedir reset cross-tenant.
func (r *Repository) adminSetPassword(ctx context.Context, id int64, hash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET senha_hash = $2, must_change_password = true WHERE id = $1 AND tenant_id = $3`,
		id, hash, tenantctx.From(ctx))
	return err
}

// ErrEmailExists: já há um usuário com esse e-mail no tenant.
var ErrEmailExists = errors.New("e-mail já cadastrado")

// isUniqueViolation detecta violação de UNIQUE (e-mail duplicado) do Postgres.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// Create insere o usuário já com troca de senha obrigatória no 1º acesso
// (senha inicial é provisória — admin define, usuário troca ao entrar).
// O tenant é explícito (seed/provisionamento) ou vem do ctx (admin logado).
func (r *Repository) Create(ctx context.Context, tenantID int64, nome, email, hash, papel, setor string) (*User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (tenant_id, nome, email, senha_hash, papel, setor, must_change_password)
		VALUES ($1, $2, lower($3), $4, $5, $6, true)
		RETURNING id, tenant_id, nome, email, papel, setor, ativo, must_change_password, criado_em`,
		tenantID, nome, email, hash, papel, setor).
		Scan(&u.ID, &u.TenantID, &u.Nome, &u.Email, &u.Papel, &u.Setor, &u.Ativo, &u.MustChange, &u.CriadoEm)
	if isUniqueViolation(err) {
		return nil, ErrEmailExists
	}
	return &u, err
}

func (r *Repository) List(ctx context.Context) ([]User, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, nome, email, papel, setor, ativo, criado_em FROM users WHERE tenant_id = $1 ORDER BY nome`, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (User, error) {
		var u User
		err := row.Scan(&u.ID, &u.Nome, &u.Email, &u.Papel, &u.Setor, &u.Ativo, &u.CriadoEm)
		return u, err
	})
}

func (r *Repository) Update(ctx context.Context, id int64, nome, email, papel, setor string, ativo bool) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET nome = $2, email = lower($3), papel = $4, setor = $5, ativo = $6 WHERE id = $1 AND tenant_id = $7`,
		id, nome, email, papel, setor, ativo, tenantctx.From(ctx))
	if isUniqueViolation(err) {
		return ErrEmailExists
	}
	return err
}

// countTenant conta usuários de um tenant (uso no seed do admin por tenant).
func (r *Repository) countTenant(ctx context.Context, tenantID int64) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE tenant_id = $1`, tenantID).Scan(&n)
	return n, err
}
