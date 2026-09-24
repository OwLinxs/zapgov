// Package campaigns registra e lista disparos (histórico).
package campaigns

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

type Campaign struct {
	Puladas       int        `json:"puladas"`
	Revisao       int        `json:"revisao"`
	ID            int64      `json:"id"`
	TenantID      int64      `json:"-"`
	Template      string     `json:"template"`
	Language      string     `json:"language"`
	Setor         string     `json:"setor"`
	Total         int        `json:"total"`
	Enviadas      int        `json:"enviadas"`
	Entregues     int        `json:"entregues"`
	Lidas         int        `json:"lidas"`
	Falhas        int        `json:"falhas"`
	Status        string     `json:"status"`
	Operador      string     `json:"operador"`
	Personalizado bool       `json:"personalizado"`
	ScheduledAt   *time.Time `json:"scheduled_at"`
	CriadoEm      time.Time  `json:"criado_em"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const cols = `id, template, language, setor, total, enviadas, entregues, lidas, falhas, status, operador, personalizado, scheduled_at, criado_em, puladas, revisao`

func scan(row pgx.CollectableRow) (Campaign, error) {
	var c Campaign
	err := row.Scan(&c.ID, &c.Template, &c.Language, &c.Setor, &c.Total,
		&c.Enviadas, &c.Entregues, &c.Lidas, &c.Falhas, &c.Status, &c.Operador,
		&c.Personalizado, &c.ScheduledAt, &c.CriadoEm, &c.Puladas, &c.Revisao)
	return c, err
}

func (r *Repository) Create(ctx context.Context, c Campaign) (*Campaign, error) {
	rows, err := r.pool.Query(ctx, `
		INSERT INTO campaigns (tenant_id, template, language, setor, total, status, operador, personalizado, scheduled_at)
		VALUES ($9,$1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING `+cols,
		c.Template, c.Language, c.Setor, c.Total, c.Status, c.Operador, c.Personalizado, c.ScheduledAt, tenantctx.From(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := pgx.CollectOneRow(rows, scan)
	return &out, err
}

func (r *Repository) UpdateStats(ctx context.Context, id int64, enviadas, entregues, falhas int, status string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE campaigns SET enviadas=$2, entregues=$3, falhas=$4, status=$5 WHERE id=$1`,
		id, enviadas, entregues, falhas, status,
	)
	return err
}

// IncEnviadas/IncFalhas: contadores atômicos atualizados pelo worker da fila.
func (r *Repository) IncEnviadas(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE campaigns SET enviadas = enviadas + 1 WHERE id = $1`, id)
	return err
}

func (r *Repository) IncFalhas(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE campaigns SET falhas = falhas + 1 WHERE id = $1`, id)
	return err
}

// TagMessage liga uma mensagem enviada à campanha (worker, após o envio) —
// permite que o webhook de status atualize os contadores do disparo.
func (r *Repository) TagMessage(ctx context.Context, msgID, campID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE messages SET campaign_id = $2 WHERE id = $1 AND tenant_id = $3`,
		msgID, campID, tenantctx.From(ctx))
	return err
}

// MarkDoneIfComplete fecha a campanha quando todos os destinatários foram
// processados (enviadas + falhas >= total).
func (r *Repository) MarkDoneIfComplete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE campaigns SET status = 'enviado'
		WHERE id = $1 AND tenant_id = $2 AND status = 'enviando' AND enviadas + falhas >= total`,
		id, tenantctx.From(ctx))
	return err
}

// statusRank ordena a progressão de status da Meta (p/ não contar regressões
// de reentrega: read não volta a delivered).
func statusRank(s string) int {
	switch s {
	case "delivered":
		return 2
	case "read":
		return 3
	default: // pending/sent
		return 1
	}
}

// statusDeltas traduz uma transição de status em incrementos dos contadores
// (entregues, lidas, falhas). Função pura — testável sem banco.
func statusDeltas(oldStatus, newStatus string) (dEnt, dLidas, dFalhas int) {
	switch newStatus {
	case "delivered":
		if statusRank(oldStatus) < 2 {
			dEnt = 1
		}
	case "read":
		// read implica delivered (a Meta pode pular o evento de entrega).
		if statusRank(oldStatus) < 2 {
			dEnt = 1
		}
		if statusRank(oldStatus) < 3 {
			dLidas = 1
		}
	case "failed":
		if oldStatus != "failed" {
			dFalhas = 1
		}
	}
	return
}

// ApplyStatus aplica a transição de status de uma mensagem de campanha aos
// contadores do disparo. Chamado pelo webhook.
func (r *Repository) ApplyStatus(ctx context.Context, id int64, oldStatus, newStatus string) error {
	dEnt, dLidas, dFalhas := statusDeltas(oldStatus, newStatus)
	if dEnt == 0 && dLidas == 0 && dFalhas == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE campaigns SET entregues = entregues + $2, lidas = lidas + $3, falhas = falhas + $4
		WHERE id = $1 AND tenant_id = $5`,
		id, dEnt, dLidas, dFalhas, tenantctx.From(ctx))
	return err
}

func (r *Repository) List(ctx context.Context, setor, status string) ([]Campaign, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+cols+` FROM campaigns
		WHERE tenant_id = $3 AND ($1 = '' OR setor = $1) AND ($2 = '' OR status = $2)
		ORDER BY criado_em DESC`,
		setor, status, tenantctx.From(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, scan)
}

// Recipient é um destinatário persistido (com variáveis próprias).
type Recipient struct {
	Phone  string   `json:"phone"`
	Nome   string   `json:"nome"`
	Params []string `json:"params"`
}

// AddRecipients persiste os destinatários de uma campanha.
func (r *Repository) AddRecipients(ctx context.Context, campID int64, recs []Recipient) error {
	tid := tenantctx.From(ctx)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, rec := range recs {
		params := rec.Params
		if params == nil {
			params = []string{}
		}
		raw, _ := json.Marshal(params)
		if _, err := tx.Exec(ctx, `
			INSERT INTO campaign_recipients (tenant_id, campaign_id, phone, nome, params)
			VALUES ($1, $2, $3, $4, $5)`,
			tid, campID, rec.Phone, rec.Nome, raw); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Recipients devolve os destinatários persistidos de uma campanha.
func (r *Repository) Recipients(ctx context.Context, campID int64) ([]Recipient, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT phone, nome, params FROM campaign_recipients
		WHERE campaign_id = $1 AND tenant_id = $2 ORDER BY id`,
		campID, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Recipient, error) {
		var rec Recipient
		var raw []byte
		err := row.Scan(&rec.Phone, &rec.Nome, &raw)
		if err == nil {
			_ = json.Unmarshal(raw, &rec.Params)
		}
		return rec, err
	})
}

// SetStatus muda o status da campanha (agendado→enviando, agendado→cancelado).
func (r *Repository) SetStatus(ctx context.Context, id int64, de, para string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE campaigns SET status = $3 WHERE id = $1 AND tenant_id = $2 AND status = $4`,
		id, tenantctx.From(ctx), para, de)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// DueScheduled devolve campanhas agendadas vencidas (todos os tenants).
func (r *Repository) DueScheduled(ctx context.Context, limit int) ([]Campaign, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+cols+`, tenant_id FROM campaigns
		WHERE status = 'agendado' AND scheduled_at IS NOT NULL AND scheduled_at <= now()
		ORDER BY scheduled_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Campaign, error) {
		var c Campaign
		err := row.Scan(&c.ID, &c.Template, &c.Language, &c.Setor, &c.Total,
			&c.Enviadas, &c.Entregues, &c.Lidas, &c.Falhas, &c.Status, &c.Operador,
			&c.Personalizado, &c.ScheduledAt, &c.CriadoEm, &c.Puladas, &c.Revisao, &c.TenantID)
		return c, err
	})
}

func (r *Repository) Get(ctx context.Context, id int64) (*Campaign, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+cols+` FROM campaigns WHERE id=$1 AND tenant_id=$2`, id, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	c, err := pgx.CollectOneRow(rows, scan)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
