// Package tenants gerencia os órgãos (tenants) e suas credenciais Meta.
// Vive no escopo compartilhado (não é filtrado por tenant_id).
package tenants

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("tenant não encontrado")

type Tenant struct {
	ID           int64           `json:"id"`
	Nome         string          `json:"nome"`
	Subdominio   string          `json:"subdominio"`
	Plano        string          `json:"plano"`
	LimiteMensal int             `json:"limite_mensal"`
	Ativo        bool            `json:"ativo"`
	Modulos      map[string]bool `json:"modulos"` // módulos habilitados (chamados/base/disparo…)
	CriadoEm     time.Time       `json:"criado_em"`
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
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const cols = `id, nome, subdominio, plano, limite_mensal, ativo, modulos, criado_em`

func scan(row pgx.CollectableRow) (Tenant, error) {
	var t Tenant
	var mods []byte
	err := row.Scan(&t.ID, &t.Nome, &t.Subdominio, &t.Plano, &t.LimiteMensal, &t.Ativo, &mods, &t.CriadoEm)
	if err == nil && len(mods) > 0 {
		_ = json.Unmarshal(mods, &t.Modulos)
	}
	if t.Modulos == nil {
		t.Modulos = map[string]bool{}
	}
	return t, err
}

// Option é um painel disponível na tela de login (sem dados sensíveis).
type Option struct {
	Nome       string `json:"nome"`
	Subdominio string `json:"subdominio"`
}

// PublicOptions lista os painéis ativos para o seletor de login (nome +
// subdomínio apenas). Permite acessar tenants sem subdomínio próprio no DNS.
func (r *Repository) PublicOptions(ctx context.Context) ([]Option, error) {
	rows, err := r.pool.Query(ctx, `SELECT nome, subdominio FROM tenants WHERE ativo ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Option, error) {
		var o Option
		err := row.Scan(&o.Nome, &o.Subdominio)
		return o, err
	})
	if out == nil {
		out = []Option{}
	}
	return out, err
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
		return 0
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
	err := r.pool.QueryRow(ctx, `SELECT id FROM tenants WHERE wa_phone_number_id = $1 AND ativo`, phoneNumberID).Scan(&id)
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
		FROM tenants WHERE id = $1 AND ativo`, tenantID).
		Scan(&m.PhoneNumberID, &m.WABAID, &m.Token, &m.AppSecret, &m.APIVersion, &m.WebhookVerifyToken)
	if err != nil {
		return MetaCreds{}, err
	}
	return transformSecrets(tenantID, m, openSecret)
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
	sealed, err := transformSecrets(id, m, sealSecret)
	if err != nil {
		return err
	}
	m = sealed
	_, err = r.pool.Exec(ctx, `
		UPDATE tenants SET
			wa_phone_number_id = $2, wa_waba_id = $3, wa_token = $4,
			wa_app_secret = $5, wa_api_version = $6, webhook_verify_token = $7
		WHERE id = $1`,
		id, m.PhoneNumberID, m.WABAID, m.Token, m.AppSecret, m.APIVersion, m.WebhookVerifyToken)
	return err
}

// Modulos devolve os módulos habilitados de um tenant (mapa vazio se nenhum).
func (r *Repository) Modulos(ctx context.Context, id int64) map[string]bool {
	var b []byte
	_ = r.pool.QueryRow(ctx, `SELECT modulos FROM tenants WHERE id = $1`, id).Scan(&b)
	m := map[string]bool{}
	if len(b) > 0 {
		_ = json.Unmarshal(b, &m)
	}
	delete(m, "chamados")
	return m
}

// SetModulos regrava o conjunto de módulos habilitados de um tenant.
func (r *Repository) SetModulos(ctx context.Context, id int64, m map[string]bool) error {
	if m == nil {
		m = map[string]bool{}
	}
	delete(m, "chamados")
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `UPDATE tenants SET modulos = $2::jsonb WHERE id = $1`, id, b)
	return err
}
