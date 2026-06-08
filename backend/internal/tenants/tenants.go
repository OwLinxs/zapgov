// Package tenants gerencia os órgãos (tenants) e suas credenciais Meta.
// Vive no escopo compartilhado (não é filtrado por tenant_id).
package tenants

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/messaging"
)

var ErrNotFound = errors.New("tenant não encontrado")

type Tenant struct {
	ID           int64     `json:"id"`
	Nome         string    `json:"nome"`
	Subdominio   string    `json:"subdominio"`
	Plano        string    `json:"plano"`
	LimiteMensal int       `json:"limite_mensal"`
	Ativo        bool      `json:"ativo"`
	CriadoEm     time.Time `json:"criado_em"`
}

// MetaCreds são as credenciais da Meta Cloud API de um tenant.
type MetaCreds struct {
	PhoneNumberID      string `json:"phone_number_id"`
	WABAID             string `json:"waba_id"`
	Token              string `json:"token"`
	AppSecret          string `json:"app_secret"`
	APIVersion         string `json:"api_version"`
	WebhookVerifyToken string `json:"webhook_verify_token"`
}

type Repository struct {
	pool *pgxpool.Pool
	// cache de providers Meta por tenant (invalidado em SetCreds).
	provMu    sync.RWMutex
	provCache map[int64]messaging.MessageProvider
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, provCache: map[int64]messaging.MessageProvider{}}
}

const cols = `id, nome, subdominio, plano, limite_mensal, ativo, criado_em`

func scan(row pgx.CollectableRow) (Tenant, error) {
	var t Tenant
	err := row.Scan(&t.ID, &t.Nome, &t.Subdominio, &t.Plano, &t.LimiteMensal, &t.Ativo, &t.CriadoEm)
	return t, err
}

// IDBySubdomain resolve o tenant a partir do subdomínio. Vazio/desconhecido cai
// no tenant padrão (1) para manter o comportamento single-tenant atual.
func (r *Repository) IDBySubdomain(ctx context.Context, sub string) int64 {
	if sub == "" {
		return 1
	}
	var id int64
	err := r.pool.QueryRow(ctx, `SELECT id FROM tenants WHERE subdominio = lower($1) AND ativo`, sub).Scan(&id)
	if err != nil {
		return 1
	}
	return id
}

// IDByPhoneNumberID resolve o tenant dono de um número da Meta (roteamento de
// webhook de entrada). 0 se nenhum tenant tem aquele número configurado.
func (r *Repository) IDByPhoneNumberID(ctx context.Context, phoneNumberID string) int64 {
	if phoneNumberID == "" {
		return 0
	}
	var id int64
	err := r.pool.QueryRow(ctx, `SELECT id FROM tenants WHERE wa_phone_number_id = $1`, phoneNumberID).Scan(&id)
	if err != nil {
		return 0
	}
	return id
}

// Creds devolve as credenciais Meta de um tenant (campos vazios = usar fallback).
func (r *Repository) Creds(ctx context.Context, tenantID int64) (MetaCreds, error) {
	var m MetaCreds
	err := r.pool.QueryRow(ctx, `
		SELECT wa_phone_number_id, wa_waba_id, wa_token, wa_app_secret, wa_api_version, webhook_verify_token
		FROM tenants WHERE id = $1`, tenantID).
		Scan(&m.PhoneNumberID, &m.WABAID, &m.Token, &m.AppSecret, &m.APIVersion, &m.WebhookVerifyToken)
	return m, err
}

func (r *Repository) List(ctx context.Context) ([]Tenant, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+cols+` FROM tenants ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, scan)
}

func (r *Repository) Get(ctx context.Context, id int64) (*Tenant, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+cols+` FROM tenants WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t, err := pgx.CollectOneRow(rows, scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &t, err
}

// Create provisiona um novo tenant.
func (r *Repository) Create(ctx context.Context, nome, subdominio, plano string) (*Tenant, error) {
	if plano == "" {
		plano = "Gov Pro"
	}
	rows, err := r.pool.Query(ctx, `
		INSERT INTO tenants (nome, subdominio, plano)
		VALUES ($1, lower($2), $3)
		RETURNING `+cols, nome, subdominio, plano)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t, err := pgx.CollectOneRow(rows, scan)
	return &t, err
}

// SetActive suspende/reativa um tenant.
func (r *Repository) SetActive(ctx context.Context, id int64, ativo bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE tenants SET ativo = $2 WHERE id = $1`, id, ativo)
	return err
}

// UpdatePlano altera plano e limite mensal.
func (r *Repository) UpdatePlano(ctx context.Context, id int64, plano string, limite int) error {
	_, err := r.pool.Exec(ctx, `UPDATE tenants SET plano = $2, limite_mensal = $3 WHERE id = $1`, id, plano, limite)
	return err
}

// SetCreds grava as credenciais Meta de um tenant (onboarding/config).
func (r *Repository) SetCreds(ctx context.Context, id int64, m MetaCreds) error {
	if m.APIVersion == "" {
		m.APIVersion = "v21.0"
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE tenants SET
			wa_phone_number_id = $2, wa_waba_id = $3, wa_token = $4,
			wa_app_secret = $5, wa_api_version = $6, webhook_verify_token = $7
		WHERE id = $1`,
		id, m.PhoneNumberID, m.WABAID, m.Token, m.AppSecret, m.APIVersion, m.WebhookVerifyToken)
	if err == nil {
		r.invalidateProvider(id) // força reconstruir o provider com as novas creds
	}
	return err
}
