package tickets

import (
	"context"
	"fmt"
	"time"
)

// Service contém a regra de negócio dos chamados, separada do HTTP e do banco.
type Service struct {
	repo *Repository

	// SLAHoras, se definido, devolve as horas de SLA por prioridade (config do
	// tenant). nil = usa os padrões de defaultSLAHoras.
	SLAHoras func(ctx context.Context) map[string]int

	// OnAssigned/OnResolved, se definidos, notificam o solicitante (WhatsApp).
	// Desacoplados do pacote messaging via callback (mesmo padrão do projeto).
	OnAssigned func(ctx context.Context, t *Ticket)
	OnResolved func(ctx context.Context, t *Ticket)

	// OnDispatch, se definido, envia a ordem de serviço no WhatsApp do técnico de
	// campo (fone). Retorna erro se não conseguiu entregar (ex.: janela fechada) —
	// aí o encaminhamento falha em vez de fingir que enviou.
	OnDispatch func(ctx context.Context, fone, texto string, t *Ticket) error
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// defaultSLAHoras é o fallback quando a config não está disponível.
var defaultSLAHoras = map[string]int{"critica": 4, "alta": 8, "media": 24, "baixa": 72}

// prioridadeValida limita o campo a valores conhecidos.
func prioridadeValida(p string) bool {
	switch p {
	case "baixa", "media", "alta", "critica":
		return true
	}
	return false
}

// transicaoValida diz se a mudança de status é permitida. Função pura.
// Fluxo: aberto → em_andamento → aguardando ⇄ em_andamento → resolvido.
// Cancelar é possível de qualquer estado não-final; reabrir resolvido volta a
// em_andamento.
func transicaoValida(de, para string) bool {
	if de == para {
		return true
	}
	switch de {
	case "aberto":
		return para == "em_andamento" || para == "aguardando" || para == "cancelado"
	case "em_andamento":
		return para == "aguardando" || para == "resolvido" || para == "cancelado"
	case "aguardando":
		return para == "em_andamento" || para == "resolvido" || para == "cancelado"
	case "resolvido":
		return para == "em_andamento" // reabrir
	case "cancelado":
		return false // final
	}
	return false
}

// slaVenc calcula o prazo a partir da prioridade e da config (ou padrão).
// Função pura — recebe o mapa de horas explícito p/ ser testável.
func slaVenc(base time.Time, prioridade string, horas map[string]int) time.Time {
	h, ok := horas[prioridade]
	if !ok || h <= 0 {
		h = defaultSLAHoras[prioridade]
		if h <= 0 {
			h = 24
		}
	}
	return base.Add(time.Duration(h) * time.Hour)
}

func (s *Service) slaHoras(ctx context.Context) map[string]int {
	if s.SLAHoras != nil {
		if m := s.SLAHoras(ctx); len(m) > 0 {
			return m
		}
	}
	return defaultSLAHoras
}

// CreateInput são os campos para abrir um chamado.
type CreateInput struct {
	Titulo          string
	Descricao       string
	Categoria       string
	Prioridade      string
	SolicitanteNome string
	SolicitanteFone string
	Secretaria      string
	Local           string
	ConversationID  *int64
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Ticket, error) {
	if in.Titulo == "" {
		return nil, fmt.Errorf("título é obrigatório")
	}
	if in.Prioridade == "" {
		in.Prioridade = "media"
	}
	if !prioridadeValida(in.Prioridade) {
		return nil, fmt.Errorf("prioridade inválida")
	}
	venc := slaVenc(time.Now(), in.Prioridade, s.slaHoras(ctx))
	t := Ticket{
		Titulo:          in.Titulo,
		Descricao:       in.Descricao,
		Categoria:       in.Categoria,
		Prioridade:      in.Prioridade,
		SolicitanteNome: in.SolicitanteNome,
		SolicitanteFone: in.SolicitanteFone,
		Secretaria:      in.Secretaria,
		Local:           in.Local,
		ConversationID:  in.ConversationID,
		SLAVenc:         &venc,
	}
	return s.repo.Create(ctx, t)
}

func (s *Service) List(ctx context.Context, status, prioridade, categoria, tecnico string) ([]Ticket, error) {
	return s.repo.List(ctx, status, prioridade, categoria, tecnico)
}

func (s *Service) Get(ctx context.Context, id int64) (*Ticket, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) GetByConversation(ctx context.Context, convID int64) (*Ticket, error) {
	return s.repo.GetByConversation(ctx, convID)
}

func (s *Service) Comments(ctx context.Context, id int64) ([]Comment, error) {
	return s.repo.Comments(ctx, id)
}

func (s *Service) AddComment(ctx context.Context, id int64, autor, body string, interno bool) (*Comment, error) {
	if body == "" {
		return nil, fmt.Errorf("comentário vazio")
	}
	return s.repo.AddComment(ctx, id, autor, body, interno)
}

// SetStatus valida a transição e atualiza; notifica ao resolver.
func (s *Service) SetStatus(ctx context.Context, id int64, status string) (*Ticket, error) {
	atual, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !transicaoValida(atual.Status, status) {
		return nil, fmt.Errorf("transição de '%s' para '%s' não permitida", atual.Status, status)
	}
	t, err := s.repo.SetStatus(ctx, id, status)
	if err != nil {
		return nil, err
	}
	if status == "resolvido" && s.OnResolved != nil && t.SolicitanteFone != "" {
		s.OnResolved(ctx, t)
	}
	return t, nil
}

// Assign atribui o técnico e notifica o solicitante.
func (s *Service) Assign(ctx context.Context, id int64, tecnico string) (*Ticket, error) {
	if tecnico == "" {
		return nil, fmt.Errorf("técnico é obrigatório")
	}
	t, err := s.repo.Assign(ctx, id, tecnico)
	if err != nil {
		return nil, err
	}
	if s.OnAssigned != nil && t.SolicitanteFone != "" {
		s.OnAssigned(ctx, t)
	}
	return t, nil
}

func (s *Service) Metrics(ctx context.Context) (Metrics, error) {
	return s.repo.Metrics(ctx)
}

// SetPrioridade muda a prioridade (gestão pelo kanban) e recalcula o SLA.
func (s *Service) SetPrioridade(ctx context.Context, id int64, prioridade string) (*Ticket, error) {
	if !prioridadeValida(prioridade) {
		return nil, fmt.Errorf("prioridade inválida")
	}
	atual, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	venc := slaVenc(atual.CriadoEm, prioridade, s.slaHoras(ctx))
	return s.repo.SetPrioridade(ctx, id, prioridade, venc)
}

// SyncFromConversation espelha no chamado a situação da conversa atendida pelo
// chat (assumida → em_andamento, resolvida → resolvido, etc).
func (s *Service) SyncFromConversation(ctx context.Context, convID int64, status, tecnico string) error {
	return s.repo.SyncByConversation(ctx, convID, status, tecnico)
}
