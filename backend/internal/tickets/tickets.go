// Package tickets implementa o módulo de chamados do T.I. (helpdesk).
// Domínio próprio (tabelas tickets/ticket_comments), reaproveitando a auth, a
// notificação por WhatsApp e o padrão de SLA do ZapGov.
package tickets

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

// Ticket é um chamado de suporte.
type Ticket struct {
	ID              int64      `json:"id"`
	Numero          int        `json:"numero"`
	Titulo          string     `json:"titulo"`
	Descricao       string     `json:"descricao"`
	Categoria       string     `json:"categoria"`
	Prioridade      string     `json:"prioridade"` // baixa|media|alta|critica
	Status          string     `json:"status"`     // aberto|em_andamento|aguardando|resolvido|cancelado
	SolicitanteNome string     `json:"solicitante_nome"`
	SolicitanteFone string     `json:"solicitante_fone"`
	Secretaria      string     `json:"secretaria"`
	Local           string     `json:"local"`
	Tecnico         string     `json:"tecnico"`
	ConversationID  *int64     `json:"conversation_id"`
	CriadoEm        time.Time  `json:"criado_em"`
	AtribuidoEm     *time.Time `json:"atribuido_em"`
	ResolvidoEm     *time.Time `json:"resolvido_em"`
	SLAVenc         *time.Time `json:"sla_venc"`
}

// Comment é um andamento na timeline do chamado.
type Comment struct {
	ID       int64     `json:"id"`
	TicketID int64     `json:"ticket_id"`
	Autor    string    `json:"autor"`
	Body     string    `json:"body"`
	Interno  bool      `json:"interno"`
	CriadoEm time.Time `json:"criado_em"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const cols = `id, numero, titulo, descricao, categoria, prioridade, status,
	solicitante_nome, solicitante_fone, secretaria, local, tecnico,
	conversation_id, criado_em, atribuido_em, resolvido_em, sla_venc`

func scan(row pgx.CollectableRow) (Ticket, error) {
	var t Ticket
	err := row.Scan(&t.ID, &t.Numero, &t.Titulo, &t.Descricao, &t.Categoria,
		&t.Prioridade, &t.Status, &t.SolicitanteNome, &t.SolicitanteFone,
		&t.Secretaria, &t.Local, &t.Tecnico, &t.ConversationID, &t.CriadoEm,
		&t.AtribuidoEm, &t.ResolvidoEm, &t.SLAVenc)
	return t, err
}

// Create insere um chamado com número sequencial (por tenant) numa transação.
func (r *Repository) Create(ctx context.Context, t Ticket) (*Ticket, error) {
	tid := tenantctx.From(ctx)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var numero int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(numero), 0) + 1 FROM tickets WHERE tenant_id = $1`, tid).Scan(&numero); err != nil {
		return nil, err
	}

	var out Ticket
	err = tx.QueryRow(ctx, `
		INSERT INTO tickets (tenant_id, numero, titulo, descricao, categoria, prioridade,
		                     solicitante_nome, solicitante_fone, secretaria, local,
		                     conversation_id, sla_venc)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING `+cols,
		tid, numero, t.Titulo, t.Descricao, t.Categoria, t.Prioridade,
		t.SolicitanteNome, t.SolicitanteFone, t.Secretaria, t.Local,
		t.ConversationID, t.SLAVenc,
	).Scan(&out.ID, &out.Numero, &out.Titulo, &out.Descricao, &out.Categoria,
		&out.Prioridade, &out.Status, &out.SolicitanteNome, &out.SolicitanteFone,
		&out.Secretaria, &out.Local, &out.Tecnico, &out.ConversationID, &out.CriadoEm,
		&out.AtribuidoEm, &out.ResolvidoEm, &out.SLAVenc)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &out, nil
}

// List filtra chamados (status/prioridade/categoria/tecnico opcionais) e ordena
// por SLA mais próximo de estourar (prioridade alta primeiro).
func (r *Repository) List(ctx context.Context, status, prioridade, categoria, tecnico string) ([]Ticket, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+cols+` FROM tickets
		WHERE tenant_id = $1
		  AND ($2 = '' OR status = $2)
		  AND ($3 = '' OR prioridade = $3)
		  AND ($4 = '' OR categoria = $4)
		  AND ($5 = '' OR tecnico = $5)
		ORDER BY
			CASE status WHEN 'resolvido' THEN 1 WHEN 'cancelado' THEN 2 ELSE 0 END,
			CASE prioridade WHEN 'critica' THEN 0 WHEN 'alta' THEN 1 WHEN 'media' THEN 2 ELSE 3 END,
			COALESCE(sla_venc, criado_em) ASC`,
		tenantctx.From(ctx), status, prioridade, categoria, tecnico)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := pgx.CollectRows(rows, scan)
	if out == nil {
		out = []Ticket{}
	}
	return out, err
}

func (r *Repository) Get(ctx context.Context, id int64) (*Ticket, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+cols+` FROM tickets WHERE id = $1 AND tenant_id = $2`, id, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t, err := pgx.CollectOneRow(rows, scan)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetByNumero busca o chamado pelo número humano (#N) dentro do tenant — usado
// pelo modo técnico no WhatsApp ("Buscar #12"). Devolve nil se não existir.
func (r *Repository) GetByNumero(ctx context.Context, numero int) (*Ticket, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+cols+` FROM tickets WHERE numero = $1 AND tenant_id = $2`, numero, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t, err := pgx.CollectOneRow(rows, scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetByConversation devolve o chamado ligado a uma conversa (nil se não há) —
// usado pelo inbox para sugerir artigos relacionados ao atender pelo chat.
func (r *Repository) GetByConversation(ctx context.Context, convID int64) (*Ticket, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+cols+` FROM tickets
		WHERE conversation_id = $1 AND tenant_id = $2 ORDER BY id DESC LIMIT 1`, convID, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t, err := pgx.CollectOneRow(rows, scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// SetStatus muda o status; ao resolver/cancelar carimba resolvido_em.
func (r *Repository) SetStatus(ctx context.Context, id int64, status string) (*Ticket, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE tickets SET status = $3,
			resolvido_em = CASE WHEN $3 IN ('resolvido','cancelado') THEN now() ELSE NULL END
		WHERE id = $1 AND tenant_id = $2
		RETURNING `+cols,
		id, tenantctx.From(ctx), status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t, err := pgx.CollectOneRow(rows, scan)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// SetPrioridade muda a prioridade e recalcula o prazo de SLA.
func (r *Repository) SetPrioridade(ctx context.Context, id int64, prioridade string, slaVenc time.Time) (*Ticket, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE tickets SET prioridade = $3, sla_venc = $4
		WHERE id = $1 AND tenant_id = $2
		RETURNING `+cols,
		id, tenantctx.From(ctx), prioridade, slaVenc)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t, err := pgx.CollectOneRow(rows, scan)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Assign atribui um técnico (e marca em_andamento se ainda estava aberto).
func (r *Repository) Assign(ctx context.Context, id int64, tecnico string) (*Ticket, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE tickets SET tecnico = $3, atribuido_em = COALESCE(atribuido_em, now()),
			-- assumir = começar a atender: promove de aberto/aguardando para
			-- em_andamento (resolvido/cancelado não mudam).
			status = CASE WHEN status IN ('aberto','aguardando') THEN 'em_andamento' ELSE status END
		WHERE id = $1 AND tenant_id = $2
		RETURNING `+cols,
		id, tenantctx.From(ctx), tecnico)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t, err := pgx.CollectOneRow(rows, scan)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// SyncByConversation espelha no chamado a mudança de situação da conversa
// (atendimento pelo chat). Atualização silenciosa (sem notificar o solicitante —
// o chat já cuida disso). Só mexe em chamados não-finais. tecnico vazio = mantém.
func (r *Repository) SyncByConversation(ctx context.Context, convID int64, status, tecnico string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE tickets SET
			status = $3,
			tecnico = CASE WHEN $4 <> '' THEN $4 ELSE tecnico END,
			atribuido_em = CASE WHEN $4 <> '' AND atribuido_em IS NULL THEN now() ELSE atribuido_em END,
			resolvido_em = CASE WHEN $3 IN ('resolvido','cancelado') THEN now() ELSE resolvido_em END
		WHERE conversation_id = $1 AND tenant_id = $2 AND status NOT IN ('resolvido','cancelado')`,
		convID, tenantctx.From(ctx), status, tecnico)
	return err
}

// AddComment grava um andamento na timeline.
func (r *Repository) AddComment(ctx context.Context, ticketID int64, autor, body string, interno bool) (*Comment, error) {
	var c Comment
	err := r.pool.QueryRow(ctx, `
		INSERT INTO ticket_comments (tenant_id, ticket_id, autor, body, interno)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, ticket_id, autor, body, interno, criado_em`,
		tenantctx.From(ctx), ticketID, autor, body, interno).
		Scan(&c.ID, &c.TicketID, &c.Autor, &c.Body, &c.Interno, &c.CriadoEm)
	return &c, err
}

func (r *Repository) Comments(ctx context.Context, ticketID int64) ([]Comment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, ticket_id, autor, body, interno, criado_em FROM ticket_comments
		WHERE ticket_id = $1 AND tenant_id = $2 ORDER BY criado_em ASC`,
		ticketID, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Comment, error) {
		var c Comment
		err := row.Scan(&c.ID, &c.TicketID, &c.Autor, &c.Body, &c.Interno, &c.CriadoEm)
		return c, err
	})
	if out == nil {
		out = []Comment{}
	}
	return out, err
}

// Metrics são os contadores do painel de chamados.
type Metrics struct {
	Abertos      int `json:"abertos"`
	EmAndamento  int `json:"em_andamento"`
	Aguardando   int `json:"aguardando"`
	SLAEstourado int `json:"sla_estourado"`
	Resolvidos7d int `json:"resolvidos_7d"`
}

func (r *Repository) Metrics(ctx context.Context) (Metrics, error) {
	var m Metrics
	err := r.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status = 'aberto'),
			count(*) FILTER (WHERE status = 'em_andamento'),
			count(*) FILTER (WHERE status = 'aguardando'),
			count(*) FILTER (WHERE status NOT IN ('resolvido','cancelado')
			                 AND sla_venc IS NOT NULL AND sla_venc < now()),
			count(*) FILTER (WHERE status = 'resolvido' AND resolvido_em >= now() - interval '7 days')
		FROM tickets WHERE tenant_id = $1`,
		tenantctx.From(ctx)).
		Scan(&m.Abertos, &m.EmAndamento, &m.Aguardando, &m.SLAEstourado, &m.Resolvidos7d)
	return m, err
}
