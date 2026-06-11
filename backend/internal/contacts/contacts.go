// Package contacts faz o CRUD de contatos do tenant.
package contacts

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"zapgov/backend/internal/tenantctx"
)

type Contact struct {
	ID            int64      `json:"id"`
	Nome          string     `json:"nome"`
	Telefone      string     `json:"telefone"`
	CPF           string     `json:"cpf"`
	Setor         string     `json:"setor"`
	Tags          []string   `json:"tags"`
	Status        string     `json:"status"`
	Mensagens     int        `json:"mensagens"`
	UltimoContato *time.Time `json:"ultimo_contato"`
	CriadoEm      time.Time  `json:"criado_em"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func scan(row pgx.CollectableRow) (Contact, error) {
	var c Contact
	err := row.Scan(&c.ID, &c.Nome, &c.Telefone, &c.CPF, &c.Setor, &c.Tags,
		&c.Status, &c.Mensagens, &c.UltimoContato, &c.CriadoEm)
	return c, err
}

const cols = `id, nome, telefone, cpf, setor, tags, status, mensagens, ultimo_contato, criado_em`

// List filtra por setor e busca (nome/telefone/cpf).
func (r *Repository) List(ctx context.Context, setor, q string) ([]Contact, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+cols+` FROM contacts
		WHERE tenant_id = $3
		  AND ($1 = '' OR setor = $1)
		  AND ($2 = '' OR nome ILIKE '%'||$2||'%' OR telefone ILIKE '%'||$2||'%' OR cpf ILIKE '%'||$2||'%')
		ORDER BY nome ASC`,
		setor, q, tenantctx.From(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, scan)
}

func (r *Repository) Get(ctx context.Context, id int64) (*Contact, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+cols+` FROM contacts WHERE id = $1 AND tenant_id = $2`, id, tenantctx.From(ctx))
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

func (r *Repository) Create(ctx context.Context, c Contact) (*Contact, error) {
	rows, err := r.pool.Query(ctx, `
		INSERT INTO contacts (tenant_id, nome, telefone, cpf, setor, tags, status)
		VALUES ($7, $1, $2, $3, $4, $5, COALESCE(NULLIF($6,''),'ativo'))
		RETURNING `+cols,
		c.Nome, c.Telefone, c.CPF, c.Setor, c.Tags, c.Status, tenantctx.From(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := pgx.CollectOneRow(rows, scan)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// pareceTelefone: "nome" composto só de dígitos/máscara de fone (erro comum:
// atendente cola o número no campo de nome) — não vale como nome.
func pareceTelefone(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return false
	}
	dig, outros := 0, 0
	for _, r := range t {
		switch {
		case r >= '0' && r <= '9':
			dig++
		case r == ' ' || r == '(' || r == ')' || r == '-' || r == '+' || r == '.':
		default:
			outros++
		}
	}
	return dig >= 8 && outros == 0
}

// Ensure garante que o telefone exista nos contatos (cria se não houver; se já
// existir, atualiza o nome caso esteja com o placeholder "Contato ...").
func (r *Repository) Ensure(ctx context.Context, nome, telefone string) error {
	if telefone == "" {
		return nil
	}
	if nome == "" || pareceTelefone(nome) {
		nome = "Contato " + telefone
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO contacts (tenant_id, nome, telefone)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, telefone) DO UPDATE
			SET nome = EXCLUDED.nome
			WHERE contacts.nome LIKE 'Contato %'`,
		tenantctx.From(ctx), nome, telefone)
	return err
}

// MarkSemWhatsApp marca o contato como sem WhatsApp (erro 131026 da Meta).
func (r *Repository) MarkSemWhatsApp(ctx context.Context, telefone string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE contacts SET status = 'sem_whatsapp' WHERE tenant_id = $1 AND telefone = $2`,
		tenantctx.From(ctx), telefone)
	return err
}

// SemWhatsApp devolve, do conjunto informado, os telefones marcados como sem
// WhatsApp (para pular em disparos).
func (r *Repository) SemWhatsApp(ctx context.Context, telefones []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(telefones) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT telefone FROM contacts WHERE tenant_id = $1 AND status = 'sem_whatsapp' AND telefone = ANY($2)`,
		tenantctx.From(ctx), telefones)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out[t] = true
	}
	return out, rows.Err()
}

// ImportRow é uma linha de importação em massa (vinda de CSV).
type ImportRow struct {
	Nome     string
	Telefone string
	CPF      string
	Setor    string
	Tags     []string
}

// ImportBulk insere vários contatos numa transação. Telefones já existentes
// são ignorados (ON CONFLICT). Devolve quantos foram inseridos e quantos
// foram pulados por já existir.
func (r *Repository) ImportBulk(ctx context.Context, items []ImportRow) (inseridos, pulados int, err error) {
	tid := tenantctx.From(ctx)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)

	for _, it := range items {
		if it.Setor == "" {
			it.Setor = "Geral"
		}
		if it.Tags == nil {
			it.Tags = []string{}
		}
		ct, err := tx.Exec(ctx, `
			INSERT INTO contacts (tenant_id, nome, telefone, cpf, setor, tags)
			VALUES ($6, $1, $2, $3, $4, $5)
			ON CONFLICT (tenant_id, telefone) DO NOTHING`,
			it.Nome, it.Telefone, it.CPF, it.Setor, it.Tags, tid)
		if err != nil {
			return 0, 0, err
		}
		if ct.RowsAffected() == 1 {
			inseridos++
		} else {
			pulados++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return inseridos, pulados, nil
}
