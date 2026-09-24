// Package settings expõe as configurações da organização (single-tenant),
// webhooks, chave de API e consumo (faturamento).
package settings

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

type Org struct {
	Nome         string `json:"nome"`
	UF           string `json:"uf"`
	Subdominio   string `json:"subdominio"`
	Fuso         string `json:"fuso"`
	Plano        string `json:"plano"`
	LimiteMensal int    `json:"limite_mensal"`
}

type Webhook struct {
	ID       int64     `json:"id"`
	URL      string    `json:"url"`
	Eventos  []string  `json:"eventos"`
	Ativo    bool      `json:"ativo"`
	CriadoEm time.Time `json:"criado_em"`
}

type Billing struct {
	Plano     string `json:"plano"`
	Limite    int    `json:"limite_mensal"`
	Enviadas  int    `json:"enviadas"`
	Entregues int    `json:"entregues"`
	Lidas     int    `json:"lidas"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// ensure garante uma linha de org_settings para o tenant (criada sob demanda
// quando um tenant é provisionado). id = tenant_id por convenção.
// ensure garante a linha de org_settings do tenant. Auto-corretivo: cria a linha
// só se ainda não existir, escolhendo um id LIVRE (max+1) — antes forçava
// id = tenant_id, que colidia com o id de outra linha e a criação falhava calada,
// deixando todo GET de settings em 500 (tela em skeleton). Agora loga a causa.
func (r *Repository) ensure(ctx context.Context, tid int64) {
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO org_settings (id, tenant_id)
		SELECT COALESCE((SELECT max(id) FROM org_settings), 0) + 1, $1
		WHERE NOT EXISTS (SELECT 1 FROM org_settings WHERE tenant_id = $1)`, tid); err != nil {
		log.Printf("settings: ensure org_settings tenant %d falhou: %v", tid, err)
	}
}

func (r *Repository) GetOrg(ctx context.Context) (Org, error) {
	tid := tenantctx.From(ctx)
	r.ensure(ctx, tid)
	var o Org
	err := r.pool.QueryRow(ctx, `SELECT nome, uf, subdominio, fuso, plano, limite_mensal FROM org_settings WHERE tenant_id = $1`, tid).
		Scan(&o.Nome, &o.UF, &o.Subdominio, &o.Fuso, &o.Plano, &o.LimiteMensal)
	return o, err
}

func (r *Repository) UpdateOrg(ctx context.Context, nome, uf, fuso string) error {
	_, err := r.pool.Exec(ctx, `UPDATE org_settings SET nome = $1, uf = $2, fuso = $3 WHERE tenant_id = $4`, nome, uf, fuso, tenantctx.From(ctx))
	return err
}

// Encerramento: mensagem automática enviada ao finalizar o atendimento.
type Encerramento struct {
	Ativo bool   `json:"ativo"`
	Msg   string `json:"msg"`
}

func (r *Repository) GetEncerramento(ctx context.Context) (Encerramento, error) {
	tid := tenantctx.From(ctx)
	r.ensure(ctx, tid)
	var e Encerramento
	err := r.pool.QueryRow(ctx, `SELECT encerramento_ativo, encerramento_msg FROM org_settings WHERE tenant_id = $1`, tid).
		Scan(&e.Ativo, &e.Msg)
	return e, err
}

func (r *Repository) SetEncerramento(ctx context.Context, e Encerramento) error {
	_, err := r.pool.Exec(ctx, `UPDATE org_settings SET encerramento_ativo = $1, encerramento_msg = $2 WHERE tenant_id = $3`, e.Ativo, e.Msg, tenantctx.From(ctx))
	return err
}

// Roteamento de atendimento (escopo por setor + auto-etiqueta + SLA da fila).
type Roteamento struct {
	PorSetor    bool `json:"por_setor"`
	PorEtiqueta bool `json:"por_etiqueta"`
	SLAMinutos  int  `json:"sla_minutos"`
}

func (r *Repository) GetRoteamento(ctx context.Context) (Roteamento, error) {
	tid := tenantctx.From(ctx)
	r.ensure(ctx, tid)
	var rt Roteamento
	err := r.pool.QueryRow(ctx, `SELECT roteamento_setor, etiqueta_atendente, sla_minutos FROM org_settings WHERE tenant_id = $1`, tid).
		Scan(&rt.PorSetor, &rt.PorEtiqueta, &rt.SLAMinutos)
	return rt, err
}

// GetVisibilidade devolve os grupos de setores que se enxergam. Cada grupo é
// uma lista de nomes de setor; setor fora de qualquer grupo é isolado.
func (r *Repository) GetVisibilidade(ctx context.Context) ([][]string, error) {
	tid := tenantctx.From(ctx)
	r.ensure(ctx, tid)
	var b []byte
	if err := r.pool.QueryRow(ctx, `SELECT visibilidade_setores FROM org_settings WHERE tenant_id = $1`, tid).Scan(&b); err != nil {
		return nil, err
	}
	grupos := [][]string{}
	if len(b) > 0 {
		_ = json.Unmarshal(b, &grupos)
	}
	return grupos, nil
}

// SetVisibilidade regrava os grupos de visibilidade de setores.
func (r *Repository) SetVisibilidade(ctx context.Context, grupos [][]string) error {
	if grupos == nil {
		grupos = [][]string{}
	}
	b, err := json.Marshal(grupos)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `UPDATE org_settings SET visibilidade_setores = $1::jsonb WHERE tenant_id = $2`, b, tenantctx.From(ctx))
	return err
}

func (r *Repository) SetRoteamento(ctx context.Context, rt Roteamento) error {
	if rt.SLAMinutos <= 0 {
		rt.SLAMinutos = 15
	}
	_, err := r.pool.Exec(ctx, `UPDATE org_settings SET roteamento_setor = $1, etiqueta_atendente = $2, sla_minutos = $3 WHERE tenant_id = $4`,
		rt.PorSetor, rt.PorEtiqueta, rt.SLAMinutos, tenantctx.From(ctx))
	return err
}

// Pesquisa de satisfação pós-atendimento.
type Pesquisa struct {
	Ativo    bool   `json:"ativo"`
	Msg      string `json:"msg"`
	Obrigado string `json:"obrigado"`
}

func (r *Repository) GetPesquisa(ctx context.Context) (Pesquisa, error) {
	tid := tenantctx.From(ctx)
	r.ensure(ctx, tid)
	var p Pesquisa
	err := r.pool.QueryRow(ctx, `SELECT pesquisa_ativo, pesquisa_msg, pesquisa_obrigado FROM org_settings WHERE tenant_id = $1`, tid).
		Scan(&p.Ativo, &p.Msg, &p.Obrigado)
	return p, err
}

func (r *Repository) SetPesquisa(ctx context.Context, p Pesquisa) error {
	_, err := r.pool.Exec(ctx, `UPDATE org_settings SET pesquisa_ativo = $1, pesquisa_msg = $2, pesquisa_obrigado = $3 WHERE tenant_id = $4`,
		p.Ativo, p.Msg, p.Obrigado, tenantctx.From(ctx))
	return err
}

// Lembrete: agendamento automático de lembrete na véspera (data detectada nas
// variáveis do template enviado).
type Lembrete struct {
	Ativo    bool   `json:"ativo"`
	Template string `json:"template"` // template aprovado usado no lembrete
	Idioma   string `json:"idioma"`
	Hora     string `json:"hora"` // "HH:MM" no fuso da org
	Fuso     string `json:"fuso"` // leitura
}

func (r *Repository) GetLembrete(ctx context.Context) (Lembrete, error) {
	tid := tenantctx.From(ctx)
	r.ensure(ctx, tid)
	var l Lembrete
	err := r.pool.QueryRow(ctx, `
		SELECT lembrete_ativo, lembrete_template, lembrete_idioma, lembrete_hora, fuso
		FROM org_settings WHERE tenant_id = $1`, tid).
		Scan(&l.Ativo, &l.Template, &l.Idioma, &l.Hora, &l.Fuso)
	return l, err
}

func (r *Repository) SetLembrete(ctx context.Context, l Lembrete) error {
	if l.Idioma == "" {
		l.Idioma = "pt_BR"
	}
	if l.Hora == "" {
		l.Hora = "09:00"
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE org_settings SET lembrete_ativo = $1, lembrete_template = $2, lembrete_idioma = $3, lembrete_hora = $4
		WHERE tenant_id = $5`,
		l.Ativo, l.Template, l.Idioma, l.Hora, tenantctx.From(ctx))
	return err
}

// GetChatbot diz se o chatbot está ativo. Desligado: inbound vai direto pra fila.
func (r *Repository) GetChatbot(ctx context.Context) (bool, error) {
	tid := tenantctx.From(ctx)
	r.ensure(ctx, tid)
	var on bool
	err := r.pool.QueryRow(ctx, `SELECT chatbot_ativo FROM org_settings WHERE tenant_id = $1`, tid).Scan(&on)
	return on, err
}

func (r *Repository) SetChatbot(ctx context.Context, on bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE org_settings SET chatbot_ativo = $1 WHERE tenant_id = $2`, on, tenantctx.From(ctx))
	return err
}

// GetAssinatura diz se a assinatura do atendente vai no fim das mensagens.
func (r *Repository) GetAssinatura(ctx context.Context) (bool, error) {
	tid := tenantctx.From(ctx)
	r.ensure(ctx, tid)
	var on bool
	err := r.pool.QueryRow(ctx, `SELECT assinatura_ativo FROM org_settings WHERE tenant_id = $1`, tid).Scan(&on)
	return on, err
}

func (r *Repository) SetAssinatura(ctx context.Context, on bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE org_settings SET assinatura_ativo = $1 WHERE tenant_id = $2`, on, tenantctx.From(ctx))
	return err
}

// Ausencia: auto-resposta de fim de semana / feriado / fora de horário.
type Ausencia struct {
	Ativo     bool     `json:"ativo"`
	FimSemana string   `json:"fim_semana"`
	Feriado   string   `json:"feriado"`
	Feriados  []string `json:"feriados"` // 'YYYY-MM-DD' ou 'MM-DD' (recorrente)
	Fuso      string   `json:"fuso"`     // somente leitura (vem da config da org)
	// Horário comercial (auto-resposta fora do expediente em dia útil).
	HorarioAtivo  bool   `json:"horario_ativo"`
	HorarioInicio string `json:"horario_inicio"` // "HH:MM"
	HorarioFim    string `json:"horario_fim"`    // "HH:MM"
	HorarioDias   string `json:"horario_dias"`   // "1,2,3,4,5" (0=Dom..6=Sáb)
	HorarioMsg    string `json:"horario_msg"`
}

func (r *Repository) GetAusencia(ctx context.Context) (Ausencia, error) {
	tid := tenantctx.From(ctx)
	r.ensure(ctx, tid)
	var a Ausencia
	err := r.pool.QueryRow(ctx, `
		SELECT ausencia_ativo, ausencia_fim_semana, ausencia_feriado, ausencia_feriados, fuso,
		       horario_ativo, horario_inicio, horario_fim, horario_dias, horario_msg
		FROM org_settings WHERE tenant_id = $1`, tid).
		Scan(&a.Ativo, &a.FimSemana, &a.Feriado, &a.Feriados, &a.Fuso,
			&a.HorarioAtivo, &a.HorarioInicio, &a.HorarioFim, &a.HorarioDias, &a.HorarioMsg)
	if a.Feriados == nil {
		a.Feriados = []string{}
	}
	return a, err
}

// MensagemAusencia decide, para um instante (já no fuso da org), qual
// auto-resposta cabe: feriado > fim de semana > fora do horário comercial.
// Função pura — vive aqui para ser testável sem banco/HTTP.
func MensagemAusencia(a Ausencia, now time.Time) (string, bool) {
	// Feriado / fim de semana.
	if a.Ativo {
		md, ymd := now.Format("01-02"), now.Format("2006-01-02")
		for _, f := range a.Feriados {
			if (f == md || f == ymd) && a.Feriado != "" {
				return a.Feriado, true
			}
		}
		if wd := now.Weekday(); wd == time.Saturday || wd == time.Sunday {
			if a.FimSemana != "" {
				return a.FimSemana, true
			}
		}
	}
	// Horário comercial: fora do expediente em dia útil configurado.
	if a.HorarioAtivo && a.HorarioMsg != "" {
		wd := strconv.Itoa(int(now.Weekday()))
		if strings.Contains(","+a.HorarioDias+",", ","+wd+",") {
			cur := now.Format("15:04")
			if cur < a.HorarioInicio || cur >= a.HorarioFim {
				return a.HorarioMsg, true
			}
		}
	}
	return "", false
}

func (r *Repository) SetAusencia(ctx context.Context, a Ausencia) error {
	if a.Feriados == nil {
		a.Feriados = []string{}
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE org_settings SET ausencia_ativo = $1, ausencia_fim_semana = $2, ausencia_feriado = $3, ausencia_feriados = $4,
			horario_ativo = $6, horario_inicio = $7, horario_fim = $8, horario_dias = $9, horario_msg = $10
		WHERE tenant_id = $5`,
		a.Ativo, a.FimSemana, a.Feriado, a.Feriados, tenantctx.From(ctx),
		a.HorarioAtivo, a.HorarioInicio, a.HorarioFim, a.HorarioDias, a.HorarioMsg)
	return err
}

// EnsureApiKey gera a chave na primeira vez (se vazia) e a devolve.
func (r *Repository) EnsureApiKey(ctx context.Context) (string, error) {
	tid := tenantctx.From(ctx)
	r.ensure(ctx, tid)
	var key string
	if err := r.pool.QueryRow(ctx, `SELECT api_key FROM org_settings WHERE tenant_id = $1`, tid).Scan(&key); err != nil {
		return "", err
	}
	if key == "" {
		return r.RegenApiKey(ctx)
	}
	return key, nil
}

func (r *Repository) RegenApiKey(ctx context.Context) (string, error) {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	key := "zg_live_" + hex.EncodeToString(b)
	_, err := r.pool.Exec(ctx, `UPDATE org_settings SET api_key = $1 WHERE tenant_id = $2`, key, tenantctx.From(ctx))
	return key, err
}

func (r *Repository) ListWebhooks(ctx context.Context) ([]Webhook, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, url, eventos, ativo, criado_em FROM webhook_endpoints WHERE tenant_id = $1 ORDER BY id`, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Webhook, error) {
		var w Webhook
		err := row.Scan(&w.ID, &w.URL, &w.Eventos, &w.Ativo, &w.CriadoEm)
		return w, err
	})
}

func (r *Repository) CreateWebhook(ctx context.Context, url string, eventos []string) (*Webhook, error) {
	var w Webhook
	err := r.pool.QueryRow(ctx, `
		INSERT INTO webhook_endpoints (tenant_id, url, eventos) VALUES ($3, $1, $2)
		RETURNING id, url, eventos, ativo, criado_em`, url, eventos, tenantctx.From(ctx)).
		Scan(&w.ID, &w.URL, &w.Eventos, &w.Ativo, &w.CriadoEm)
	return &w, err
}

func (r *Repository) DeleteWebhook(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM webhook_endpoints WHERE id = $1 AND tenant_id = $2`, id, tenantctx.From(ctx))
	return err
}

// BillingUsage conta o consumo do mês corrente a partir das mensagens.
func (r *Repository) BillingUsage(ctx context.Context) (Billing, error) {
	var b Billing
	err := r.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE direction = 'out'),
			count(*) FILTER (WHERE direction = 'out' AND status IN ('delivered','read')),
			count(*) FILTER (WHERE direction = 'out' AND status = 'read')
		FROM messages
		WHERE tenant_id = $1 AND created_at >= date_trunc('month', now())`, tenantctx.From(ctx)).
		Scan(&b.Enviadas, &b.Entregues, &b.Lidas)
	if err != nil {
		return b, err
	}
	org, err := r.GetOrg(ctx)
	if err == nil {
		b.Plano = org.Plano
		b.Limite = org.LimiteMensal
	}
	return b, nil
}
