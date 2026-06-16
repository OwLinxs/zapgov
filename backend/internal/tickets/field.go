package tickets

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"zapgov/backend/internal/tenantctx"
)

// FieldTech é um técnico de campo (contato de WhatsApp, sem login) que recebe
// ordens de serviço encaminhadas dos chamados.
type FieldTech struct {
	ID       int64  `json:"id"`
	Nome     string `json:"nome"`
	Telefone string `json:"telefone"`
	Ativo    bool   `json:"ativo"`
}

func scanTech(row pgx.CollectableRow) (FieldTech, error) {
	var t FieldTech
	err := row.Scan(&t.ID, &t.Nome, &t.Telefone, &t.Ativo)
	return t, err
}

func (r *Repository) FieldTechs(ctx context.Context) ([]FieldTech, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, nome, telefone, ativo FROM field_techs
		WHERE tenant_id = $1 AND ativo ORDER BY nome`, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := pgx.CollectRows(rows, scanTech)
	if out == nil {
		out = []FieldTech{}
	}
	return out, err
}

func (r *Repository) GetFieldTech(ctx context.Context, id int64) (*FieldTech, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, nome, telefone, ativo FROM field_techs WHERE id = $1 AND tenant_id = $2`, id, tenantctx.From(ctx))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t, err := pgx.CollectOneRow(rows, scanTech)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Repository) CreateFieldTech(ctx context.Context, nome, telefone string) (*FieldTech, error) {
	rows, err := r.pool.Query(ctx, `
		INSERT INTO field_techs (tenant_id, nome, telefone) VALUES ($1, $2, $3)
		RETURNING id, nome, telefone, ativo`, tenantctx.From(ctx), nome, telefone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t, err := pgx.CollectOneRow(rows, scanTech)
	return &t, err
}

// DeleteFieldTech desativa o técnico (preserva histórico de quem foi acionado).
func (r *Repository) DeleteFieldTech(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE field_techs SET ativo = false WHERE id = $1 AND tenant_id = $2`, id, tenantctx.From(ctx))
	return err
}

// ordemServico monta a mensagem de WhatsApp enviada ao técnico de campo.
// Função pura — testável sem banco.
func ordemServico(t *Ticket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🔧 *Chamado #%d*", t.Numero)
	if t.Categoria != "" {
		fmt.Fprintf(&b, " — %s", t.Categoria)
	}
	b.WriteString("\n")
	if t.Local != "" {
		fmt.Fprintf(&b, "📍 %s\n", t.Local)
	}
	if t.Secretaria != "" {
		fmt.Fprintf(&b, "🏢 %s\n", t.Secretaria)
	}
	fmt.Fprintf(&b, "📋 %s\n", t.Titulo)
	if t.Descricao != "" {
		fmt.Fprintf(&b, "%s\n", t.Descricao)
	}
	if t.SolicitanteNome != "" || t.SolicitanteFone != "" {
		fmt.Fprintf(&b, "👤 %s", t.SolicitanteNome)
		if t.SolicitanteFone != "" {
			fmt.Fprintf(&b, " (%s)", t.SolicitanteFone)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// Dispatch encaminha o chamado a um técnico de campo: envia a ordem de serviço
// no WhatsApp dele, atribui o chamado e registra na timeline.
func (s *Service) Dispatch(ctx context.Context, ticketID, techID int64) (*FieldTech, error) {
	t, err := s.repo.Get(ctx, ticketID)
	if err != nil {
		return nil, fmt.Errorf("chamado não encontrado")
	}
	tech, err := s.repo.GetFieldTech(ctx, techID)
	if err != nil {
		return nil, fmt.Errorf("técnico não encontrado")
	}
	if s.OnDispatch != nil {
		s.OnDispatch(ctx, tech.Telefone, ordemServico(t), t)
	}
	// Atribui ao técnico e marca em andamento (silencioso — sem notificar o
	// solicitante aqui; o aviso a ele segue pelo chat).
	_, _ = s.repo.Assign(ctx, ticketID, tech.Nome)
	_, _ = s.repo.AddComment(ctx, ticketID, "Sistema",
		fmt.Sprintf("Encaminhado para %s (técnico de campo) via WhatsApp.", tech.Nome), true)
	return tech, nil
}
