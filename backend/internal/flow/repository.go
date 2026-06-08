package flow

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

type Flow struct {
	ID    int64           `json:"id"`
	Nome  string          `json:"nome"`
	Graph json.RawMessage `json:"graph"`
	Ativo bool            `json:"ativo"`
}

// State é o "program counter" de uma conversa.
type State struct {
	CurrentNode string
	Vars        map[string]string
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// ActiveFlow devolve o fluxo ativo, ou (nil, nil) se não houver.
func (r *Repository) ActiveFlow(ctx context.Context) (*Flow, error) {
	var f Flow
	err := r.pool.QueryRow(ctx, `SELECT id, nome, graph, ativo FROM flows WHERE tenant_id = $1 AND ativo = true ORDER BY id LIMIT 1`, tenantctx.From(ctx)).
		Scan(&f.ID, &f.Nome, &f.Graph, &f.Ativo)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *Repository) GetState(ctx context.Context, phone string) (State, error) {
	var node string
	var rawVars []byte
	err := r.pool.QueryRow(ctx, `SELECT current_node, vars FROM conversation_flow_state WHERE tenant_id = $2 AND phone = $1`, phone, tenantctx.From(ctx)).
		Scan(&node, &rawVars)
	if errors.Is(err, pgx.ErrNoRows) {
		return State{Vars: map[string]string{}}, nil
	}
	if err != nil {
		return State{Vars: map[string]string{}}, err
	}
	vars := map[string]string{}
	_ = json.Unmarshal(rawVars, &vars)
	return State{CurrentNode: node, Vars: vars}, nil
}

func (r *Repository) SaveState(ctx context.Context, phone, node string, vars map[string]string) error {
	raw, _ := json.Marshal(vars)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO conversation_flow_state (tenant_id, phone, current_node, vars, updated_at)
		VALUES ($4, $1, $2, $3, now())
		ON CONFLICT (tenant_id, phone) DO UPDATE SET current_node = $2, vars = $3, updated_at = now()`,
		phone, node, raw, tenantctx.From(ctx),
	)
	return err
}

func (r *Repository) ClearState(ctx context.Context, phone string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM conversation_flow_state WHERE tenant_id = $2 AND phone = $1`, phone, tenantctx.From(ctx))
	return err
}

func (r *Repository) AssignDepartment(ctx context.Context, phone, dept string) error {
	_, err := r.pool.Exec(ctx, `UPDATE conversations SET setor = $2 WHERE tenant_id = $3 AND contact_phone = $1`, phone, dept, tenantctx.From(ctx))
	return err
}

// Handoff registra o setor (se houver) e os dados coletados pelo bot, para o
// atendente humano ver ao pegar a conversa.
func (r *Repository) Handoff(ctx context.Context, phone, dept string, vars map[string]string) error {
	raw, _ := json.Marshal(vars)
	_, err := r.pool.Exec(ctx, `
		UPDATE conversations
		SET setor = COALESCE(NULLIF($2,''), setor), dados = $3, status = 'aguardando'
		WHERE tenant_id = $4 AND contact_phone = $1`,
		phone, dept, raw, tenantctx.From(ctx),
	)
	return err
}

// ---- CRUD para o editor ----

func (r *Repository) List(ctx context.Context) ([]Flow, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, nome, graph, ativo FROM flows WHERE tenant_id = $1 ORDER BY id`, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Flow, error) {
		var f Flow
		err := row.Scan(&f.ID, &f.Nome, &f.Graph, &f.Ativo)
		return f, err
	})
}

func (r *Repository) SaveGraph(ctx context.Context, id int64, graph json.RawMessage) error {
	_, err := r.pool.Exec(ctx, `UPDATE flows SET graph = $2 WHERE id = $1 AND tenant_id = $3`, id, graph, tenantctx.From(ctx))
	return err
}
