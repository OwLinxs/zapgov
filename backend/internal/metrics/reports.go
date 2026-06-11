package metrics

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"zapgov/backend/internal/tenantctx"
)

// SatisfacaoStats agrega a pesquisa (últimos 30 dias) para o dashboard.
type SatisfacaoStats struct {
	Media     float64 `json:"media"`
	Respostas int     `json:"respostas"`
	Enviadas  int     `json:"enviadas"`
}

func (r *Repository) Satisfacao(ctx context.Context) (SatisfacaoStats, error) {
	var s SatisfacaoStats
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(avg(nota) FILTER (WHERE nota IS NOT NULL), 0),
		       count(*) FILTER (WHERE nota IS NOT NULL), count(*)
		FROM surveys WHERE tenant_id = $1 AND enviada_em >= now() - interval '30 days'`,
		tenantctx.From(ctx)).Scan(&s.Media, &s.Respostas, &s.Enviadas)
	s.Media = round1(s.Media)
	return s, err
}

// ---- Relatórios CSV por período ----

// periodo lê from/to da query (YYYY-MM-DD); default últimos 30 dias.
func periodo(c *fiber.Ctx) (time.Time, time.Time) {
	to := time.Now().AddDate(0, 0, 1) // inclusive até hoje
	from := time.Now().AddDate(0, 0, -30)
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			to = t.AddDate(0, 0, 1) // inclusivo
		}
	}
	return from, to
}

func csvOut(c *fiber.Ctx, nome string, header []string, rows [][]string) error {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // BOM p/ Excel
	w := csv.NewWriter(&buf)
	w.Write(header)
	for _, r := range rows {
		w.Write(r)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, nome))
	return c.Send(buf.Bytes())
}

func f1(v float64) string  { return fmt.Sprintf("%.1f", v) }
func i(v int) string       { return fmt.Sprintf("%d", v) }

// reportAtendentes: desempenho por atendente no período.
func (h *Handler) reportAtendentes(c *fiber.Ctx) error {
	from, to := periodo(c)
	rows, err := h.repo.pool.Query(c.Context(), `
		SELECT s.atendente,
		       count(DISTINCT s.id) FILTER (WHERE s.encerrada_em IS NOT NULL),
		       count(m.id) FILTER (WHERE m.direction = 'out'),
		       COALESCE(avg(EXTRACT(EPOCH FROM (s.encerrada_em - s.iniciada_em))) FILTER (WHERE s.encerrada_em IS NOT NULL), 0)/60,
		       COALESCE((SELECT avg(sv.nota) FROM surveys sv
		                 WHERE sv.tenant_id = s.tenant_id AND sv.atendente = s.atendente
		                   AND sv.nota IS NOT NULL AND sv.enviada_em >= $2 AND sv.enviada_em < $3), 0)
		FROM sessions s LEFT JOIN messages m ON m.session_id = s.id
		WHERE s.tenant_id = $1 AND s.atendente <> '' AND s.iniciada_em >= $2 AND s.iniciada_em < $3
		GROUP BY s.tenant_id, s.atendente ORDER BY 2 DESC`,
		tenantctx.From(c.Context()), from, to)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		var nome string
		var atend, msgs int
		var tempo, sat float64
		if err := rows.Scan(&nome, &atend, &msgs, &tempo, &sat); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		out = append(out, []string{nome, i(atend), i(msgs), f1(tempo), f1(sat)})
	}
	return csvOut(c, "atendentes.csv",
		[]string{"Atendente", "Atendimentos", "Mensagens enviadas", "Tempo médio (min)", "Satisfação média"}, out)
}

// reportSetores: volume por departamento no período.
func (h *Handler) reportSetores(c *fiber.Ctx) error {
	from, to := periodo(c)
	rows, err := h.repo.pool.Query(c.Context(), `
		SELECT COALESCE(NULLIF(s.setor,''),'(sem setor)'),
		       count(DISTINCT s.id) FILTER (WHERE s.encerrada_em IS NOT NULL),
		       COALESCE(avg(EXTRACT(EPOCH FROM (s.encerrada_em - s.iniciada_em))) FILTER (WHERE s.encerrada_em IS NOT NULL), 0)/60,
		       COALESCE((SELECT avg(sv.nota) FROM surveys sv
		                 WHERE sv.tenant_id = s.tenant_id AND sv.setor = s.setor
		                   AND sv.nota IS NOT NULL AND sv.enviada_em >= $2 AND sv.enviada_em < $3), 0)
		FROM sessions s
		WHERE s.tenant_id = $1 AND s.iniciada_em >= $2 AND s.iniciada_em < $3
		GROUP BY s.tenant_id, s.setor ORDER BY 2 DESC`,
		tenantctx.From(c.Context()), from, to)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		var setor string
		var atend int
		var tempo, sat float64
		if err := rows.Scan(&setor, &atend, &tempo, &sat); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		out = append(out, []string{setor, i(atend), f1(tempo), f1(sat)})
	}
	return csvOut(c, "departamentos.csv",
		[]string{"Departamento", "Atendimentos", "Tempo médio (min)", "Satisfação média"}, out)
}

// reportDiario: mensagens por dia no período.
func (h *Handler) reportDiario(c *fiber.Ctx) error {
	from, to := periodo(c)
	rows, err := h.repo.pool.Query(c.Context(), `
		SELECT to_char(date_trunc('day', created_at), 'YYYY-MM-DD'),
		       count(*) FILTER (WHERE direction = 'in'),
		       count(*) FILTER (WHERE direction = 'out'),
		       count(*) FILTER (WHERE direction = 'out' AND status = 'failed')
		FROM messages
		WHERE tenant_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY 1 ORDER BY 1`,
		tenantctx.From(c.Context()), from, to)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		var dia string
		var in, outn, fail int
		if err := rows.Scan(&dia, &in, &outn, &fail); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		out = append(out, []string{dia, i(in), i(outn), i(fail)})
	}
	return csvOut(c, "diario.csv", []string{"Dia", "Recebidas", "Enviadas", "Falhas"}, out)
}

// reportSatisfacao: uma linha por pesquisa enviada no período.
func (h *Handler) reportSatisfacao(c *fiber.Ctx) error {
	from, to := periodo(c)
	rows, err := h.repo.pool.Query(c.Context(), `
		SELECT sv.enviada_em, COALESCE(co.contact_name, co.contact_phone), sv.atendente, sv.setor,
		       COALESCE(sv.nota, 0), sv.respondida_em
		FROM surveys sv
		JOIN conversations co ON co.id = sv.conversation_id
		WHERE sv.tenant_id = $1 AND sv.enviada_em >= $2 AND sv.enviada_em < $3
		ORDER BY sv.enviada_em DESC`,
		tenantctx.From(c.Context()), from, to)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		var env time.Time
		var resp *time.Time
		var contato, atendente, setor string
		var nota int
		if err := rows.Scan(&env, &contato, &atendente, &setor, &nota, &resp); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		notaS, respS := "", ""
		if nota > 0 {
			notaS = i(nota)
		}
		if resp != nil {
			respS = resp.Format("2006-01-02 15:04")
		}
		out = append(out, []string{env.Format("2006-01-02 15:04"), contato, atendente, setor, notaS, respS})
	}
	return csvOut(c, "satisfacao.csv",
		[]string{"Enviada em", "Contato", "Atendente", "Departamento", "Nota", "Respondida em"}, out)
}
