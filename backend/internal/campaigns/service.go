package campaigns

import (
	"context"
	"fmt"

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
}

func NewService(repo *Repository, enq Enqueuer, checker TemplateChecker) *Service {
	return &Service{repo: repo, enq: enq, checker: checker}
}

type CreateInput struct {
	Template string
	Language string
	Params   []string
	Setor    string
	Operador string
	Preview  string
	Phones   []string
}

// Create registra a campanha e ENFILEIRA um job por destinatário.
// O envio real acontece no worker (assíncrono, com retry e vazão controlada).
func (s *Service) Create(ctx context.Context, in CreateInput) (*Campaign, error) {
	if in.Template == "" || len(in.Phones) == 0 {
		return nil, fmt.Errorf("template e ao menos um destinatário são obrigatórios")
	}
	if in.Language == "" {
		in.Language = "pt_BR"
	}
	if in.Setor == "" {
		in.Setor = "Geral"
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

	// Pula números marcados como sem WhatsApp (não desperdiça envio).
	if s.SemWhatsApp != nil {
		if skip, err := s.SemWhatsApp(ctx, in.Phones); err == nil && len(skip) > 0 {
			filtrados := in.Phones[:0]
			for _, p := range in.Phones {
				if !skip[p] {
					filtrados = append(filtrados, p)
				}
			}
			in.Phones = filtrados
		}
	}
	if len(in.Phones) == 0 {
		return nil, fmt.Errorf("todos os destinatários estão marcados como sem WhatsApp")
	}

	camp, err := s.repo.Create(ctx, Campaign{
		Template: in.Template,
		Language: in.Language,
		Setor:    in.Setor,
		Total:    len(in.Phones),
		Status:   "enviando",
		Operador: in.Operador,
	})
	if err != nil {
		return nil, fmt.Errorf("criar campanha: %w", err)
	}

	tid := tenantctx.From(ctx)
	for _, phone := range in.Phones {
		if err := s.enq.EnqueueCampaignSend(queue.CampaignSendPayload{
			TenantID:   tid,
			CampaignID: camp.ID,
			To:         phone,
			Template:   in.Template,
			Language:   in.Language,
			Params:     in.Params,
			Preview:    in.Preview,
		}); err != nil {
			// Falha ao enfileirar conta como falha imediata.
			_ = s.repo.IncFalhas(ctx, camp.ID)
		}
	}
	return camp, nil
}

func (s *Service) List(ctx context.Context, setor, status string) ([]Campaign, error) {
	return s.repo.List(ctx, setor, status)
}

func (s *Service) Get(ctx context.Context, id int64) (*Campaign, error) {
	return s.repo.Get(ctx, id)
}
