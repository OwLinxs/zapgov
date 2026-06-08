package chatbot

import (
	"context"
	"log"
	"strings"

	"zapgov/backend/internal/messaging"
)

// Replier envia a resposta do bot ao cidadão (satisfeito por messaging.Service).
type Replier interface {
	SendText(ctx context.Context, to, body string) (*messaging.Message, error)
}

type Service struct {
	repo    *Repository
	replier Replier
}

func NewService(repo *Repository, replier Replier) *Service {
	return &Service{repo: repo, replier: replier}
}

// HandleInbound é chamado para cada mensagem recebida (callback do webhook).
// Decide a resposta do bot e a envia. Assinatura casa com messaging.Service.OnInbound.
func (s *Service) HandleInbound(ctx context.Context, from, name, body string) {
	cfg, err := s.repo.GetConfig(ctx)
	if err != nil || !cfg.Enabled {
		return // bot desligado: deixa para atendimento humano
	}

	opts, err := s.repo.ListOptions(ctx, true)
	if err != nil || len(opts) == 0 {
		return
	}

	escolha := strings.TrimSpace(body)
	for _, o := range opts {
		if strings.EqualFold(escolha, o.Chave) {
			reply := o.Resposta
			if o.DeptNome != "" {
				reply += "\n\n📋 Encaminhado para: " + o.DeptNome
			}
			s.send(ctx, from, reply)
			return
		}
	}

	// Não casou nenhuma opção → menu de boas-vindas.
	s.send(ctx, from, buildMenu(cfg.Welcome, opts))
}

func (s *Service) send(ctx context.Context, to, body string) {
	if _, err := s.replier.SendText(ctx, to, body); err != nil {
		log.Printf("chatbot: falha ao responder %s: %v", to, err)
	}
}

func buildMenu(welcome string, opts []Option) string {
	var b strings.Builder
	b.WriteString(welcome)
	b.WriteString("\n")
	for _, o := range opts {
		b.WriteString("\n")
		b.WriteString(o.Chave)
		b.WriteString(" - ")
		b.WriteString(o.Label)
	}
	return b.String()
}

// ---- pass-through para o handler ----

func (s *Service) Config(ctx context.Context) (Config, error)     { return s.repo.GetConfig(ctx) }
func (s *Service) SaveConfig(ctx context.Context, c Config) error { return s.repo.UpdateConfig(ctx, c) }
func (s *Service) Options(ctx context.Context) ([]Option, error) {
	return s.repo.ListOptions(ctx, false)
}
func (s *Service) AddOption(ctx context.Context, o Option) (*Option, error) {
	return s.repo.CreateOption(ctx, o)
}
func (s *Service) RemoveOption(ctx context.Context, id int64) error {
	return s.repo.DeleteOption(ctx, id)
}
