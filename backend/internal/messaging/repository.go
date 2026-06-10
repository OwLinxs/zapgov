package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

// CanonPhone normaliza um telefone para casar opt-out independente do formato
// (dígitos; para BR reinsere o nono dígito que o wa_id às vezes omite).
func CanonPhone(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	// BR celular sem DDI (DDD + 9...): prefixa 55, p/ casar com o wa_id da resposta
	// (que vem com 55). Sem isso, envio e resposta viram 2 conversas.
	// Restrito ao padrão de celular BR p/ não afetar números internacionais.
	if !strings.HasPrefix(d, "55") && ((len(d) == 11 && d[2] == '9') || (len(d) == 10 && d[2] >= '6' && d[2] <= '9')) {
		d = "55" + d
	}
	if strings.HasPrefix(d, "55") {
		rest := d[2:]
		if len(rest) == 10 {
			ddd, sub := rest[:2], rest[2:]
			if sub[0] >= '6' && sub[0] <= '9' {
				return "55" + ddd + "9" + sub
			}
		}
	}
	return d
}

// Message é a linha persistida do histórico.
type Message struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversation_id"`
	WAMessageID    string    `json:"wa_message_id"`
	Direction      string    `json:"direction"` // in | out
	Body           string    `json:"body"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	MediaType      string    `json:"media_type"` // ''=texto; image|audio|video|document|sticker
	MediaID        string    `json:"media_id"`
	MediaMime      string    `json:"media_mime"`
	MediaName      string    `json:"media_name"`
	Autor          string    `json:"autor"` // quem escreveu (notas internas)
}

const msgCols = `id, conversation_id, wa_message_id, direction, body, status, created_at, media_type, media_id, media_mime, media_name, autor`

func scanMessage(row pgx.CollectableRow) (Message, error) {
	var m Message
	err := row.Scan(&m.ID, &m.ConversationID, &m.WAMessageID, &m.Direction, &m.Body, &m.Status,
		&m.CreatedAt, &m.MediaType, &m.MediaID, &m.MediaMime, &m.MediaName, &m.Autor)
	return m, err
}

// AddNote grava uma nota interna na conversa (não vai pro cidadão).
func (r *Repository) AddNote(ctx context.Context, convID int64, autor, body string) (*Message, error) {
	sid, err := r.ensureOpenSession(ctx, convID)
	if err != nil {
		return nil, err
	}
	waID := fmt.Sprintf("note-%d-%d", convID, time.Now().UnixNano())
	m := &Message{}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO messages (tenant_id, conversation_id, session_id, wa_message_id, direction, body, status, autor)
		VALUES ($6, $1, $2, $3, 'note', $4, 'nota', $5)
		RETURNING `+msgCols,
		convID, sid, waID, body, autor, tenantctx.From(ctx),
	).Scan(&m.ID, &m.ConversationID, &m.WAMessageID, &m.Direction, &m.Body, &m.Status,
		&m.CreatedAt, &m.MediaType, &m.MediaID, &m.MediaMime, &m.MediaName, &m.Autor)
	return m, err
}

// Conversation é uma conversa com a prévia da última mensagem (para a lista do inbox).
type Conversation struct {
	ID              int64             `json:"id"`
	ContactPhone    string            `json:"contact_phone"`
	ContactName     string            `json:"contact_name"`
	UnreadCount     int               `json:"unread_count"`
	LastMessage     string            `json:"last_message"`
	LastMsgAt       time.Time         `json:"last_msg_at"`
	WindowExpiresAt *time.Time        `json:"window_expires_at"`
	Setor           string            `json:"setor"`
	Dados           map[string]string `json:"dados"`
	Status          string            `json:"status"`
	Atendente       string            `json:"atendente"`
	OptOut          bool              `json:"opt_out"`
	Labels          []ConvLabel       `json:"labels"`
}

// ConvLabel é uma etiqueta aplicada à conversa.
type ConvLabel struct {
	ID   int64  `json:"id"`
	Nome string `json:"nome"`
	Cor  string `json:"cor"`
}

// Repository encapsula o acesso ao banco. Queries escritas à mão com pgx
// (sqlc entra quando o schema estabilizar).
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// EnsureConversation devolve o id da conversa do telefone, criando se não existir.
func (r *Repository) EnsureConversation(ctx context.Context, phone, name string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO conversations (tenant_id, contact_phone, contact_name, last_msg_at)
		VALUES ($3, $1, $2, now())
		ON CONFLICT (tenant_id, contact_phone) DO UPDATE SET
			contact_name = COALESCE(NULLIF(EXCLUDED.contact_name, ''), conversations.contact_name),
			last_msg_at = now()
		RETURNING id`,
		CanonPhone(phone), name, tenantctx.From(ctx),
	).Scan(&id)
	return id, err
}

// ensureOpenSession devolve a sessão aberta da conversa, criando uma se não houver.
func (r *Repository) ensureOpenSession(ctx context.Context, convID int64) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM sessions WHERE conversation_id = $1 AND encerrada_em IS NULL ORDER BY id DESC LIMIT 1`,
		convID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = r.pool.QueryRow(ctx,
			`INSERT INTO sessions (tenant_id, conversation_id) VALUES ($2, $1) RETURNING id`, convID, tenantctx.From(ctx)).Scan(&id)
	}
	return id, err
}

// InsertOutbound grava uma mensagem enviada (ligada à sessão aberta).
func (r *Repository) InsertOutbound(ctx context.Context, convID int64, waMessageID, body string) (*Message, error) {
	sid, err := r.ensureOpenSession(ctx, convID)
	if err != nil {
		return nil, err
	}
	m := &Message{}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO messages (tenant_id, conversation_id, session_id, wa_message_id, direction, body, status)
		VALUES ($5, $1, $2, $3, 'out', $4, 'sent')
		RETURNING `+msgCols,
		convID, sid, waMessageID, body, tenantctx.From(ctx),
	).Scan(&m.ID, &m.ConversationID, &m.WAMessageID, &m.Direction, &m.Body, &m.Status,
		&m.CreatedAt, &m.MediaType, &m.MediaID, &m.MediaMime, &m.MediaName, &m.Autor)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// InsertOutboundMedia grava uma mídia enviada (com legenda e tipo).
func (r *Repository) InsertOutboundMedia(ctx context.Context, convID int64, waMessageID, caption, kind, name string) (*Message, error) {
	sid, err := r.ensureOpenSession(ctx, convID)
	if err != nil {
		return nil, err
	}
	m := &Message{}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO messages (tenant_id, conversation_id, session_id, wa_message_id, direction, body, status, media_type, media_name)
		VALUES ($7, $1, $2, $3, 'out', $4, 'sent', $5, $6)
		RETURNING `+msgCols,
		convID, sid, waMessageID, caption, kind, name, tenantctx.From(ctx),
	).Scan(&m.ID, &m.ConversationID, &m.WAMessageID, &m.Direction, &m.Body, &m.Status,
		&m.CreatedAt, &m.MediaType, &m.MediaID, &m.MediaMime, &m.MediaName, &m.Autor)
	return m, err
}

// MediaInfo agrupa os dados de mídia de uma mensagem (vazio quando é texto).
type MediaInfo struct {
	Type string
	ID   string
	Mime string
	Name string
}

// InsertInbound grava uma mensagem recebida (idempotente por wa_message_id) e
// reabre a janela de 24h. Retorna false se já existia (evento reentregue).
func (r *Repository) InsertInbound(ctx context.Context, convID int64, waMessageID, body string, ts time.Time, media MediaInfo) (bool, error) {
	sid, err := r.ensureOpenSession(ctx, convID)
	if err != nil {
		return false, err
	}
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO messages (tenant_id, conversation_id, session_id, wa_message_id, direction, body, status, created_at,
		                      media_type, media_id, media_mime, media_name)
		VALUES ($10, $1, $2, $3, 'in', $4, 'received', $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id, wa_message_id) DO NOTHING`,
		convID, sid, waMessageID, body, ts, media.Type, media.ID, media.Mime, media.Name, tenantctx.From(ctx),
	)
	if err != nil {
		return false, err
	}
	inserted := tag.RowsAffected() > 0
	if inserted {
		_, err = r.pool.Exec(ctx, `
			UPDATE conversations
			SET unread_count = unread_count + 1,
			    last_msg_at = $2::timestamptz,
			    window_expires_at = $2::timestamptz + interval '24 hours',
			    -- nova mensagem reabre conversa já resolvida (volta ao bot)
			    status = CASE WHEN status = 'resolvido' THEN '' ELSE status END
			WHERE id = $1`,
			convID, ts,
		)
	}
	return inserted, err
}

// OptOut descadastra um telefone: marca a conversa e registra na lista canônica.
func (r *Repository) OptOut(ctx context.Context, phone, motivo string) error {
	tid := tenantctx.From(ctx)
	_, _ = r.pool.Exec(ctx, `UPDATE conversations SET opt_out = true WHERE tenant_id = $2 AND contact_phone = $1`, CanonPhone(phone), tid)
	_, err := r.pool.Exec(ctx,
		`INSERT INTO opt_outs (tenant_id, phone, motivo) VALUES ($3, $1, $2) ON CONFLICT (tenant_id, phone) DO NOTHING`,
		CanonPhone(phone), motivo, tid)
	return err
}

// OptIn recadastra um telefone.
func (r *Repository) OptIn(ctx context.Context, phone string) error {
	tid := tenantctx.From(ctx)
	_, _ = r.pool.Exec(ctx, `UPDATE conversations SET opt_out = false WHERE tenant_id = $2 AND contact_phone = $1`, CanonPhone(phone), tid)
	_, err := r.pool.Exec(ctx, `DELETE FROM opt_outs WHERE tenant_id = $2 AND phone = $1`, CanonPhone(phone), tid)
	return err
}

// IsOptedOut consulta a lista canônica (usado nos disparos em massa).
func (r *Repository) IsOptedOut(ctx context.Context, phone string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM opt_outs WHERE tenant_id = $2 AND phone = $1)`, CanonPhone(phone), tenantctx.From(ctx)).Scan(&exists)
	return exists, err
}

// ConversationOptedOut checa o flag da conversa (resposta manual).
func (r *Repository) ConversationOptedOut(ctx context.Context, phone string) (bool, error) {
	var out bool
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(opt_out, false) FROM conversations WHERE tenant_id = $2 AND contact_phone = $1`, CanonPhone(phone), tenantctx.From(ctx)).Scan(&out)
	if errors.Is(err, pgx.ErrNoRows) {
		// sem conversa: checa só a lista canônica
		return r.IsOptedOut(ctx, phone)
	}
	return out, err
}

// WindowOpen diz se a janela de atendimento de 24h está aberta para o telefone.
// Conversa inexistente = fechada (precisa iniciar com template).
func (r *Repository) WindowOpen(ctx context.Context, phone string) (bool, error) {
	var exp *time.Time
	err := r.pool.QueryRow(ctx, `SELECT window_expires_at FROM conversations WHERE tenant_id = $2 AND contact_phone = $1`, CanonPhone(phone), tenantctx.From(ctx)).Scan(&exp)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return exp != nil && exp.After(time.Now()), nil
}

// UpdateStatus avança o status de uma mensagem enviada (por wa_message_id).
func (r *Repository) UpdateStatus(ctx context.Context, waMessageID, status string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE messages SET status = $2, updated_at = now()
		WHERE wa_message_id = $1 AND direction = 'out' AND tenant_id = $3`,
		waMessageID, status, tenantctx.From(ctx),
	)
	return err
}

// ListConversations devolve as conversas (filtro opcional por setor/status),
// ordenadas pela atividade mais recente, com a prévia da última mensagem.
func (r *Repository) ListConversations(ctx context.Context, limit int, setor, status, q string, soAtendimento bool) ([]Conversation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.contact_phone, c.contact_name, c.unread_count,
		       COALESCE(m.body, '') AS last_message,
		       c.last_msg_at, c.window_expires_at, c.setor, c.dados, c.status, c.atendente, c.opt_out,
		       COALESCE((SELECT json_agg(json_build_object('id', l.id, 'nome', l.nome, 'cor', l.cor) ORDER BY l.nome)
		                 FROM conversation_labels cl JOIN labels l ON l.id = cl.label_id
		                 WHERE cl.conversation_id = c.id), '[]') AS labels
		FROM conversations c
		LEFT JOIN LATERAL (
			SELECT body FROM messages
			WHERE conversation_id = c.id
			ORDER BY created_at DESC
			LIMIT 1
		) m ON true
		WHERE c.tenant_id = $5 AND ($2 = '' OR c.setor = $2) AND ($3 = '' OR c.status = $3)
		  AND ($4 = false OR c.status <> '')
		  AND ($6 = '' OR c.contact_name ILIKE '%'||$6||'%' OR c.contact_phone ILIKE '%'||$6||'%')
		ORDER BY c.last_msg_at DESC
		LIMIT $1`,
		limit, setor, status, soAtendimento, tenantctx.From(ctx), q,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Conversation, error) {
		var c Conversation
		var dadosRaw, labelsRaw []byte
		err := row.Scan(&c.ID, &c.ContactPhone, &c.ContactName, &c.UnreadCount,
			&c.LastMessage, &c.LastMsgAt, &c.WindowExpiresAt, &c.Setor, &dadosRaw, &c.Status, &c.Atendente, &c.OptOut, &labelsRaw)
		if err == nil && len(dadosRaw) > 0 {
			c.Dados = map[string]string{}
			_ = json.Unmarshal(dadosRaw, &c.Dados)
		}
		if err == nil && len(labelsRaw) > 0 {
			_ = json.Unmarshal(labelsRaw, &c.Labels)
		}
		return c, err
	})
}

// Claim: um atendente assume a conversa (sai da fila).
func (r *Repository) Claim(ctx context.Context, id int64, atendente string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE conversations SET status = 'em_atendimento', atendente = $2 WHERE id = $1 AND tenant_id = $3`,
		id, atendente, tenantctx.From(ctx))
	return err
}

// ClaimIfQueued assume a conversa só se ela estiver na fila (aguardando).
// Usado quando o atendente responde uma conversa da fila → pega automaticamente.
func (r *Repository) ClaimIfQueued(ctx context.Context, id int64, atendente string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE conversations SET status = 'em_atendimento', atendente = $2
		WHERE id = $1 AND status = 'aguardando' AND tenant_id = $3`,
		id, atendente, tenantctx.From(ctx))
	return err
}

// Resolve: encerra o atendimento e fecha a sessão aberta (snapshot setor/atendente/dados).
func (r *Repository) Resolve(ctx context.Context, id int64) error {
	tid := tenantctx.From(ctx)
	if _, err := r.pool.Exec(ctx, `UPDATE conversations SET status = 'resolvido' WHERE id = $1 AND tenant_id = $2`, id, tid); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE sessions s
		SET encerrada_em = now(), status = 'resolvida',
		    setor = c.setor, atendente = c.atendente, dados = c.dados
		FROM conversations c
		WHERE s.conversation_id = c.id AND c.id = $1 AND c.tenant_id = $2 AND s.encerrada_em IS NULL`, id, tid)
	return err
}

// Session é um ciclo de atendimento (uma "janela" de conversa).
type Session struct {
	ID          int64             `json:"id"`
	Setor       string            `json:"setor"`
	Atendente   string            `json:"atendente"`
	Status      string            `json:"status"`
	Dados       map[string]string `json:"dados"`
	IniciadaEm  time.Time         `json:"iniciada_em"`
	EncerradaEm *time.Time        `json:"encerrada_em"`
	Mensagens   int               `json:"mensagens"`
}

// ListSessions lista as sessões de uma conversa (mais recente primeiro).
func (r *Repository) ListSessions(ctx context.Context, convID int64) ([]Session, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.setor, s.atendente, s.status, s.dados, s.iniciada_em, s.encerrada_em,
		       (SELECT count(*) FROM messages m WHERE m.session_id = s.id) AS msgs
		FROM sessions s
		WHERE s.conversation_id = $1 AND s.tenant_id = $2
		ORDER BY s.iniciada_em DESC`, convID, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Session, error) {
		var s Session
		var dadosRaw []byte
		err := row.Scan(&s.ID, &s.Setor, &s.Atendente, &s.Status, &dadosRaw, &s.IniciadaEm, &s.EncerradaEm, &s.Mensagens)
		if err == nil && len(dadosRaw) > 0 {
			s.Dados = map[string]string{}
			_ = json.Unmarshal(dadosRaw, &s.Dados)
		}
		return s, err
	})
}

// ListSessionMessages devolve as mensagens de uma sessão (ordem cronológica).
func (r *Repository) ListSessionMessages(ctx context.Context, sessionID int64) ([]Message, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+msgCols+` FROM messages WHERE session_id = $1 AND tenant_id = $2 ORDER BY created_at ASC`, sessionID, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, scanMessage)
}

// ListMessagesByConversation devolve as mensagens de uma conversa, ordem cronológica.
func (r *Repository) ListMessagesByConversation(ctx context.Context, convID int64, limit int) ([]Message, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+msgCols+` FROM messages
		WHERE conversation_id = $1 AND tenant_id = $3
		ORDER BY created_at ASC
		LIMIT $2`,
		convID, limit, tenantctx.From(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, scanMessage)
}

// SetLabels substitui as etiquetas de uma conversa pelo conjunto informado.
func (r *Repository) SetLabels(ctx context.Context, convID int64, labelIDs []int64) error {
	tid := tenantctx.From(ctx)
	if _, err := r.pool.Exec(ctx, `DELETE FROM conversation_labels WHERE conversation_id = $1 AND tenant_id = $2`, convID, tid); err != nil {
		return err
	}
	if len(labelIDs) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO conversation_labels (tenant_id, conversation_id, label_id)
		SELECT $3, $1, unnest($2::bigint[]) ON CONFLICT DO NOTHING`, convID, labelIDs, tid)
	return err
}

// Enqueue coloca uma conversa em fase de bot ('') na fila (aguardando), para o
// atendente assumir. Usado quando o chatbot está desligado.
func (r *Repository) Enqueue(ctx context.Context, convID int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE conversations SET status = 'aguardando'
		WHERE id = $1 AND tenant_id = $2 AND status = ''`,
		convID, tenantctx.From(ctx))
	return err
}

// Transfer move a conversa para outro setor e devolve à fila (aguardando,
// sem atendente), para o time do novo setor assumir.
func (r *Repository) Transfer(ctx context.Context, id int64, setor string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE conversations SET setor = $2, status = 'aguardando', atendente = ''
		WHERE id = $1 AND tenant_id = $3`,
		id, setor, tenantctx.From(ctx))
	return err
}

// AddLabel aplica UMA etiqueta à conversa sem remover as existentes (auto-tag).
func (r *Repository) AddLabel(ctx context.Context, convID, labelID int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO conversation_labels (tenant_id, conversation_id, label_id)
		VALUES ($3, $1, $2) ON CONFLICT DO NOTHING`, convID, labelID, tenantctx.From(ctx))
	return err
}

// ResolveBulk finaliza várias conversas de uma vez (fecha as sessões também).
func (r *Repository) ResolveBulk(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tid := tenantctx.From(ctx)
	_, err := r.pool.Exec(ctx, `
		UPDATE sessions s
		SET encerrada_em = now(), status = 'resolvida',
		    setor = c.setor, atendente = c.atendente, dados = c.dados
		FROM conversations c
		WHERE s.conversation_id = c.id AND s.encerrada_em IS NULL AND c.id = ANY($1) AND c.tenant_id = $2`, ids, tid)
	if err != nil {
		return 0, err
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE conversations SET status = 'resolvido' WHERE id = ANY($1) AND tenant_id = $2 AND status <> 'resolvido'`, ids, tid)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// AutoResolveExpired finaliza conversas ativas (fila/atendimento) cuja janela de
// 24h expirou — o cidadão sumiu e nem dá pra responder. Fecha as sessões também.
func (r *Repository) AutoResolveExpired(ctx context.Context) (int, error) {
	// fecha as sessões abertas dessas conversas (snapshot setor/atendente/dados).
	// Colunas qualificadas por c. para evitar ambiguidade com sessions.status.
	_, err := r.pool.Exec(ctx, `
		UPDATE sessions s
		SET encerrada_em = now(), status = 'resolvida',
		    setor = c.setor, atendente = c.atendente, dados = c.dados
		FROM conversations c
		WHERE s.conversation_id = c.id AND s.encerrada_em IS NULL
		  AND c.status IN ('aguardando','em_atendimento')
		  AND c.window_expires_at IS NOT NULL AND c.window_expires_at < now()`)
	if err != nil {
		return 0, err
	}

	tag, err := r.pool.Exec(ctx, `
		UPDATE conversations SET status = 'resolvido'
		WHERE status IN ('aguardando','em_atendimento')
		  AND window_expires_at IS NOT NULL AND window_expires_at < now()`)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// ConversationStatus devolve o status atual da conversa ('', aguardando, em_atendimento, resolvido).
func (r *Repository) ConversationStatus(ctx context.Context, convID int64) (string, error) {
	var status string
	err := r.pool.QueryRow(ctx, `SELECT status FROM conversations WHERE id = $1 AND tenant_id = $2`, convID, tenantctx.From(ctx)).Scan(&status)
	return status, err
}

// PhoneByID devolve o telefone de uma conversa.
func (r *Repository) PhoneByID(ctx context.Context, id int64) (string, error) {
	var phone string
	err := r.pool.QueryRow(ctx, `SELECT contact_phone FROM conversations WHERE id = $1 AND tenant_id = $2`, id, tenantctx.From(ctx)).Scan(&phone)
	return phone, err
}

// ConversationContact devolve telefone e nome do contato de uma conversa.
func (r *Repository) ConversationContact(ctx context.Context, id int64) (string, string, error) {
	var phone, name string
	err := r.pool.QueryRow(ctx, `SELECT contact_phone, COALESCE(contact_name, '') FROM conversations WHERE id = $1 AND tenant_id = $2`, id, tenantctx.From(ctx)).
		Scan(&phone, &name)
	return phone, name, err
}

// MarkAbsenceIfDue marca o envio da auto-resposta de ausência e devolve true se
// ela deve ser enviada agora (no máximo 1x a cada 8h por conversa). Atômico.
func (r *Repository) MarkAbsenceIfDue(ctx context.Context, convID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE conversations SET last_ausencia_at = now()
		WHERE id = $1 AND tenant_id = $2
		  AND (last_ausencia_at IS NULL OR last_ausencia_at < now() - interval '8 hours')`,
		convID, tenantctx.From(ctx))
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// MarkRead zera o contador de não-lidas de uma conversa.
func (r *Repository) MarkRead(ctx context.Context, convID int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE conversations SET unread_count = 0 WHERE id = $1 AND tenant_id = $2`, convID, tenantctx.From(ctx))
	return err
}

// ListMessages devolve o histórico mais recente (para conferência/inbox).
func (r *Repository) ListMessages(ctx context.Context, limit int) ([]Message, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+msgCols+` FROM messages
		WHERE tenant_id = $2
		ORDER BY created_at DESC
		LIMIT $1`,
		limit, tenantctx.From(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, scanMessage)
}
