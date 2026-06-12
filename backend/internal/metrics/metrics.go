// Package metrics serve os indicadores do dashboard (KPIs, volume e atividade)
// calculados direto das tabelas messages/campaigns/conversations.
package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// KPIs do topo do dashboard. Deltas comparam hoje vs. ontem.
type KPIs struct {
	EnviadasHoje  int     `json:"enviadasHoje"`
	EnviadasDelta float64 `json:"enviadasDelta"`
	TaxaEntrega   float64 `json:"taxaEntrega"`
	EntregaDelta  float64 `json:"entregaDelta"`
	TaxaLeitura   float64 `json:"taxaLeitura"`
	LeituraDelta  float64 `json:"leituraDelta"`
	Falhas        int     `json:"falhas"`
	FalhasDelta   float64 `json:"falhasDelta"`
}

type dayAgg struct {
	enviadas, entregues, lidas, falhas int
}

func (r *Repository) KPIs(ctx context.Context) (KPIs, error) {
	hoje, err := r.aggDay(ctx, 0)
	if err != nil {
		return KPIs{}, err
	}
	ontem, err := r.aggDay(ctx, 1)
	if err != nil {
		return KPIs{}, err
	}

	rate := func(num, den int) float64 {
		if den == 0 {
			return 0
		}
		return float64(num) / float64(den) * 100
	}
	pctDelta := func(novo, velho int) float64 {
		if velho == 0 {
			if novo == 0 {
				return 0
			}
			return 100
		}
		return (float64(novo) - float64(velho)) / float64(velho) * 100
	}

	entHoje, entOntem := rate(hoje.entregues, hoje.enviadas), rate(ontem.entregues, ontem.enviadas)
	leiHoje, leiOntem := rate(hoje.lidas, hoje.enviadas), rate(ontem.lidas, ontem.enviadas)

	return KPIs{
		EnviadasHoje:  hoje.enviadas,
		EnviadasDelta: round1(pctDelta(hoje.enviadas, ontem.enviadas)),
		TaxaEntrega:   round1(entHoje),
		EntregaDelta:  round1(entHoje - entOntem),
		TaxaLeitura:   round1(leiHoje),
		LeituraDelta:  round1(leiHoje - leiOntem),
		Falhas:        hoje.falhas,
		FalhasDelta:   round1(pctDelta(hoje.falhas, ontem.falhas)),
	}, nil
}

// aggDay agrega as mensagens de saída de um dia (offset 0 = hoje, 1 = ontem).
func (r *Repository) aggDay(ctx context.Context, offset int) (dayAgg, error) {
	var a dayAgg
	err := r.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE direction = 'out'),
			count(*) FILTER (WHERE direction = 'out' AND status IN ('delivered','read')),
			count(*) FILTER (WHERE direction = 'out' AND status = 'read'),
			count(*) FILTER (WHERE direction = 'out' AND status = 'failed')
		FROM messages
		WHERE tenant_id = $2
		  AND created_at >= date_trunc('day', now()) - ($1 * interval '1 day')
		  AND created_at <  date_trunc('day', now()) - (($1 - 1) * interval '1 day')`,
		offset, tenantctx.From(ctx)).
		Scan(&a.enviadas, &a.entregues, &a.lidas, &a.falhas)
	return a, err
}

// VolumePoint é um ponto do gráfico de volume (compatível com o front).
type VolumePoint struct {
	Label string `json:"label"`
	Value int    `json:"value"`
	Lidas int    `json:"lidas"`
}

func (r *Repository) Volume(ctx context.Context, rng string) ([]VolumePoint, error) {
	var sql string
	if rng == "dia" {
		// Últimos 7 dias.
		sql = `
			SELECT to_char(date_trunc('day', created_at), 'DD/MM') AS label,
				count(*) FILTER (WHERE direction = 'out') AS value,
				count(*) FILTER (WHERE direction = 'out' AND status = 'read') AS lidas
			FROM messages
			WHERE tenant_id = $1 AND created_at >= date_trunc('day', now()) - interval '6 days'
			GROUP BY date_trunc('day', created_at)
			ORDER BY date_trunc('day', created_at)`
	} else {
		// Por hora, no dia de hoje.
		sql = `
			SELECT to_char(date_trunc('hour', created_at), 'HH24') || 'h' AS label,
				count(*) FILTER (WHERE direction = 'out') AS value,
				count(*) FILTER (WHERE direction = 'out' AND status = 'read') AS lidas
			FROM messages
			WHERE tenant_id = $1 AND created_at >= date_trunc('day', now())
			GROUP BY date_trunc('hour', created_at)
			ORDER BY date_trunc('hour', created_at)`
	}
	rows, err := r.pool.Query(ctx, sql, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pts, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (VolumePoint, error) {
		var p VolumePoint
		err := row.Scan(&p.Label, &p.Value, &p.Lidas)
		return p, err
	})
	if err != nil {
		return nil, err
	}
	if pts == nil {
		pts = []VolumePoint{}
	}
	return pts, nil
}

// Atividade é um item do feed (compatível com o front).
type Atividade struct {
	Tipo     string `json:"tipo"` // disparo | resposta | falha
	Titulo   string `json:"titulo"`
	Meta     string `json:"meta"`
	Setor    string `json:"setor"`
	Status   string `json:"status,omitempty"`
	Tempo    string `json:"tempo"`
	Operador string `json:"operador,omitempty"`
}

type atividadeRow struct {
	at  time.Time
	ati Atividade
}

func (r *Repository) Atividade(ctx context.Context, limit int) ([]Atividade, error) {
	if limit <= 0 {
		limit = 8
	}
	tid := tenantctx.From(ctx)
	var rows []atividadeRow

	// Disparos (campanhas).
	cRows, err := r.pool.Query(ctx, `
		SELECT criado_em, template, total, setor, status, operador
		FROM campaigns WHERE tenant_id = $2 ORDER BY criado_em DESC LIMIT $1`, limit, tid)
	if err != nil {
		return nil, err
	}
	for cRows.Next() {
		var at time.Time
		var tmpl, setor, status, operador string
		var total int
		if err := cRows.Scan(&at, &tmpl, &total, &setor, &status, &operador); err != nil {
			cRows.Close()
			return nil, err
		}
		tipo := "disparo"
		if status == "falhou" {
			tipo = "falha"
		}
		rows = append(rows, atividadeRow{at, Atividade{
			Tipo:     tipo,
			Titulo:   fmt.Sprintf("Disparo %q", tmpl),
			Meta:     fmt.Sprintf("%d destinatários", total),
			Setor:    setor,
			Status:   visStatus(status),
			Tempo:    "",
			Operador: operador,
		}})
	}
	cRows.Close()

	// Respostas recebidas (mensagens de entrada recentes).
	mRows, err := r.pool.Query(ctx, `
		SELECT m.created_at, COALESCE(c.contact_name, c.contact_phone), m.body, COALESCE(c.setor, 'Geral')
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE m.direction = 'in' AND m.tenant_id = $2
		ORDER BY m.created_at DESC LIMIT $1`, limit, tid)
	if err != nil {
		return nil, err
	}
	for mRows.Next() {
		var at time.Time
		var nome, body, setor string
		if err := mRows.Scan(&at, &nome, &body, &setor); err != nil {
			mRows.Close()
			return nil, err
		}
		rows = append(rows, atividadeRow{at, Atividade{
			Tipo:   "resposta",
			Titulo: nome + " respondeu",
			Meta:   fmt.Sprintf("%q", trunc(body, 60)),
			Setor:  setor,
			Tempo:  "",
		}})
	}
	mRows.Close()

	// Ordena por tempo desc e corta no limite.
	for i := range rows {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].at.After(rows[i].at) {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	out := make([]Atividade, 0, limit)
	for i, rw := range rows {
		if i >= limit {
			break
		}
		rw.ati.Tempo = humanAgo(rw.at)
		out = append(out, rw.ati)
	}
	return out, nil
}

// HeatCell é uma célula do mapa de calor de horário de pico: total de mensagens
// RECEBIDAS por dia da semana (0=Dom..6=Sáb) e hora local, últimos 30 dias.
type HeatCell struct {
	Dia   int `json:"dia"`
	Hora  int `json:"hora"`
	Total int `json:"total"`
}

func (r *Repository) Heatmap(ctx context.Context) ([]HeatCell, error) {
	// Fuso fixo do município (created_at é timestamptz/UTC no banco).
	rows, err := r.pool.Query(ctx, `
		SELECT EXTRACT(dow  FROM created_at AT TIME ZONE 'America/Sao_Paulo')::int,
		       EXTRACT(hour FROM created_at AT TIME ZONE 'America/Sao_Paulo')::int,
		       count(*)::int
		FROM messages
		WHERE tenant_id = $1 AND direction = 'in'
		  AND created_at >= now() - interval '30 days'
		GROUP BY 1, 2`,
		tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (HeatCell, error) {
		var h HeatCell
		err := row.Scan(&h.Dia, &h.Hora, &h.Total)
		return h, err
	})
	if out == nil {
		out = []HeatCell{}
	}
	return out, err
}

// AtendenteStat é o desempenho de um atendente (últimos 30 dias).
type AtendenteStat struct {
	Nome          string  `json:"nome"`
	Atendimentos  int     `json:"atendimentos"`
	Mensagens     int     `json:"mensagens"`
	TempoMedioMin float64 `json:"tempo_medio_min"`
	Satisfacao    float64 `json:"satisfacao"` // média 1-5 (0 = sem respostas)
}

func (r *Repository) Atendentes(ctx context.Context) ([]AtendenteStat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.atendente,
		       count(DISTINCT s.id) FILTER (WHERE s.encerrada_em IS NOT NULL) AS atendimentos,
		       count(m.id) FILTER (WHERE m.direction = 'out') AS mensagens,
		       COALESCE(avg(EXTRACT(EPOCH FROM (s.encerrada_em - s.iniciada_em)))
		                FILTER (WHERE s.encerrada_em IS NOT NULL), 0) / 60 AS tempo_medio_min,
		       COALESCE((SELECT avg(sv.nota) FROM surveys sv
		                 WHERE sv.tenant_id = s.tenant_id AND sv.atendente = s.atendente
		                   AND sv.nota IS NOT NULL AND sv.enviada_em >= now() - interval '30 days'), 0) AS satisfacao
		FROM sessions s
		LEFT JOIN messages m ON m.session_id = s.id
		WHERE s.tenant_id = $1 AND s.atendente <> ''
		  AND s.iniciada_em >= now() - interval '30 days'
		GROUP BY s.tenant_id, s.atendente
		ORDER BY atendimentos DESC, mensagens DESC`,
		tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AtendenteStat, error) {
		var a AtendenteStat
		err := row.Scan(&a.Nome, &a.Atendimentos, &a.Mensagens, &a.TempoMedioMin, &a.Satisfacao)
		a.TempoMedioMin = round1(a.TempoMedioMin)
		a.Satisfacao = round1(a.Satisfacao)
		return a, err
	})
	if out == nil {
		out = []AtendenteStat{}
	}
	return out, err
}

// ---- helpers ----

func round1(f float64) float64 {
	return float64(int(f*10+sign(f)*0.5)) / 10
}
func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// visStatus mapeia o status interno da campanha para o vocabulário do feed.
func visStatus(s string) string {
	switch s {
	case "enviando":
		return "enviado"
	case "enviado":
		return "entregue"
	case "falhou":
		return "falhou"
	default:
		return s
	}
}

func humanAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "agora há pouco"
	case d < time.Hour:
		return fmt.Sprintf("há %d min", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("há %d h", int(d.Hours()))
	default:
		return fmt.Sprintf("há %d d", int(d.Hours()/24))
	}
}
