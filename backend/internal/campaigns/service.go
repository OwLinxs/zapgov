package campaigns

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"zapgov/backend/internal/queue"
	"zapgov/backend/internal/tenantctx"
)

// Enqueuer enfileira o envio de cada mensagem (satisfeito por queue.Client).
type Enqueuer interface {
	EnqueueCampaignSend(p queue.CampaignSendPayload) error
}

// TemplateChecker valida se um template pode ser disparado (satisfeito por templates.Client).
type TemplateChecker interface {
	IsUsable(ctx context.Context, name, language string) (bool, error)
}

type Service struct {
	repo    *Repository
	enq     Enqueuer
	checker TemplateChecker
	// SemWhatsApp, se definido, devolve quais telefones estão marcados como
	// sem WhatsApp (erro 131026) — pulados no disparo.
	SemWhatsApp func(ctx context.Context, phones []string) (map[string]bool, error)
	// OnDateInTemplate, se definido, agenda lembrete de véspera quando uma
	// variável do destinatário contém data futura (só no modo personalizado).
	OnDateInTemplate func(ctx context.Context, phone, nome string, data time.Time)
}

func NewService(repo *Repository, enq Enqueuer, checker TemplateChecker) *Service {
	return &Service{repo: repo, enq: enq, checker: checker}
}

type CreateInput struct {
	Template string
	Language string
	Setor    string
	Operador string
	Body     string // corpo cru do template (p/ preview por destinatário)

	// Modo comunicado: mesmas variáveis pra todos.
	Params []string
	Phones []string

	// Modo personalizado: variáveis por destinatário (se presente, vence Phones).
	Recipients []Recipient

	// Agendamento (nil/passado = envia agora).
	ScheduledAt *time.Time
}

var varRe = regexp.MustCompile(`\{\{(\d+)\}\}`)

// render substitui {{n}} pelos parâmetros do destinatário.
func render(body string, params []string) string {
	return varRe.ReplaceAllStringFunc(body, func(m string) string {
		n, _ := strconv.Atoi(varRe.FindStringSubmatch(m)[1])
		if n >= 1 && n <= len(params) && params[n-1] != "" {
			return params[n-1]
		}
		return m
	})
}

// dataBR detecta DD/MM/AAAA nas variáveis (lembrete automático).
var dataBR = regexp.MustCompile(`\b(\d{2})/(\d{2})/(\d{4})\b`)

func acharData(params []string) (time.Time, bool) {
	for _, p := range params {
		if m := dataBR.FindString(p); m != "" {
			if t, err := time.Parse("02/01/2006", m); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// Create registra a campanha (e os destinatários). Envio imediato enfileira já;
// agendado fica para o despachante (DispatchDue).
func (s *Service) Create(ctx context.Context, in CreateInput) (*Campaign, error) {
	if in.Template == "" {
		return nil, fmt.Errorf("template é obrigatório")
	}
	if in.Language == "" {
		in.Language = "pt_BR"
	}
	if in.Setor == "" {
		in.Setor = "Geral"
	}

	// Normaliza destinatários: personalizado (Recipients) ou comunicado (Phones).
	personalizado := len(in.Recipients) > 0
	recs := in.Recipients
	if !personalizado {
		for _, p := range in.Phones {
			recs = append(recs, Recipient{Phone: p, Params: in.Params})
		}
	}
	if len(recs) == 0 {
		return nil, fmt.Errorf("ao menos um destinatário é obrigatório")
	}

	// Só dispara template APROVADO e na categoria UTILIDADE.
	if s.checker != nil {
		ok, err := s.checker.IsUsable(ctx, in.Template, in.Language)
		if err != nil {
			return nil, fmt.Errorf("verificar template: %w", err)
		}
		if !ok {
			return nil, fmt.Errorf("o template precisa estar aprovado e na categoria Utilidade para ser disparado")
		}
	}

	// Pula números marcados como sem WhatsApp.
	if s.SemWhatsApp != nil {
		phones := make([]string, len(recs))
		for i, r := range recs {
			phones[i] = r.Phone
		}
		if skip, err := s.SemWhatsApp(ctx, phones); err == nil && len(skip) > 0 {
			filtrados := recs[:0]
			for _, r := range recs {
				if !skip[r.Phone] {
					filtrados = append(filtrados, r)
				}
			}
			recs = filtrados
		}
	}
	if len(recs) == 0 {
		return nil, fmt.Errorf("todos os destinatários estão marcados como sem WhatsApp")
	}

	// Agendado se a data é futura; senão envia agora.
	status := "enviando"
	var schedAt *time.Time
	if in.ScheduledAt != nil && in.ScheduledAt.After(time.Now()) {
		status = "agendado"
		schedAt = in.ScheduledAt
	}

	camp, err := s.repo.Create(ctx, Campaign{
		Template:      in.Template,
		Language:      in.Language,
		Setor:         in.Setor,
		Total:         len(recs),
		Status:        status,
		Operador:      in.Operador,
		Personalizado: personalizado,
		ScheduledAt:   schedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("criar campanha: %w", err)
	}
	if err := s.repo.AddRecipients(ctx, camp.ID, recs); err != nil {
		return nil, fmt.Errorf("salvar destinatários: %w", err)
	}

	if status == "enviando" {
		s.enqueueAll(ctx, camp, in.Body, recs)
	}
	return camp, nil
}

// enqueueAll enfileira um job por destinatário (com preview e lembrete).
func (s *Service) enqueueAll(ctx context.Context, camp *Campaign, body string, recs []Recipient) {
	tid := tenantctx.From(ctx)
	for _, rec := range recs {
		preview := render(body, rec.Params)
		if body == "" {
			preview = "[modelo: " + camp.Template + "]"
		}
		if err := s.enq.EnqueueCampaignSend(queue.CampaignSendPayload{
			TenantID:   tid,
			CampaignID: camp.ID,
			To:         rec.Phone,
			Template:   camp.Template,
			Language:   camp.Language,
			Params:     rec.Params,
			Preview:    preview,
		}); err != nil {
			_ = s.repo.IncFalhas(ctx, camp.ID)
			continue
		}
		// Lembrete automático: só no personalizado (datas individuais).
		if camp.Personalizado && s.OnDateInTemplate != nil {
			if data, ok := acharData(rec.Params); ok {
				s.OnDateInTemplate(ctx, rec.Phone, rec.Nome, data)
			}
		}
	}
}

// DispatchDue enfileira campanhas agendadas vencidas. Chamado pelo agendador.
// O corpo do template não foi persistido → preview usa "[modelo: nome]".
func (s *Service) DispatchDue(ctx context.Context, limit int) (int, error) {
	due, err := s.repo.DueScheduled(ctx, limit)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, camp := range due {
		tctx := tenantctx.With(ctx, camp.TenantID)
		// transição atômica (evita despachar 2x se houver corrida)
		ok, err := s.repo.SetStatus(tctx, camp.ID, "agendado", "enviando")
		if err != nil || !ok {
			continue
		}
		recs, err := s.repo.Recipients(tctx, camp.ID)
		if err != nil {
			continue
		}
		c := camp
		s.enqueueAll(tctx, &c, "", recs)
		n++
	}
	return n, nil
}

// Cancel cancela uma campanha agendada (não enfileirada ainda).
func (s *Service) Cancel(ctx context.Context, id int64) error {
	ok, err := s.repo.SetStatus(ctx, id, "agendado", "cancelado")
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("só campanhas agendadas podem ser canceladas")
	}
	return nil
}

func (s *Service) List(ctx context.Context, setor, status string) ([]Campaign, error) {
	return s.repo.List(ctx, setor, status)
}

func (s *Service) Get(ctx context.Context, id int64) (*Campaign, error) {
	return s.repo.Get(ctx, id)
}
