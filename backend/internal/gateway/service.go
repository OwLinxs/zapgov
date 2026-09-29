package gateway

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"zapgov/backend/internal/messaging"
)

// Replier envia mensagens/menus no WhatsApp (satisfeito por messaging.Service).
type Replier interface {
	SendText(ctx context.Context, to, body string) (*messaging.Message, error)
	SendMenu(ctx context.Context, to, text string, options []messaging.MenuOption) (*messaging.Message, error)
}

// sessionStore guarda o estado da conversa (satisfeito por *Repository).
type sessionStore interface {
	Load(ctx context.Context, phone string) Session
	Save(ctx context.Context, phone string, s Session) error
	Clear(ctx context.Context, phone string) error
	WindowExpiry(ctx context.Context, phone string) *time.Time
}

// sigeAPI é o cliente do SIGE (satisfeito por *Client).
type sigeAPI interface {
	AbrirChamado(ctx context.Context, telefone, nome, referencia string, dados map[string]string, janela Janela) (*AbrirResp, error)
	MensagemRecebida(ctx context.Context, ev map[string]any) error
	ListarChamados(ctx context.Context, telefone string, limite int) ([]Chamado, error)
}

// Service é o bot cru do gateway + o relay para o SIGE.
type Service struct {
	repo    sessionStore
	client  sigeAPI
	replier Replier
}

func NewService(repo *Repository, client *Client, replier Replier) *Service {
	return &Service{repo: repo, client: client, replier: replier}
}

const maxLista = 10 // limite de linhas da lista interativa da Meta

// HandleInbound processa uma mensagem recebida no tenant do gateway. Em relay,
// repassa ao SIGE; senão, roda o menu Abrir/Retomar.
func (s *Service) HandleInbound(ctx context.Context, in messaging.IncomingMessage) {
	from := in.From
	sess := s.repo.Load(ctx, from)

	if sess.Step == StepRelay {
		s.repassar(ctx, from, sess.TicketID, in)
		return
	}

	bid := strings.TrimSpace(in.ButtonID)
	txt := strings.TrimSpace(in.Body)
	low := strings.ToLower(txt)

	// Voltar ao menu de qualquer passo.
	if bid == "gw_menu" || low == "menu" || low == "oi" || low == "0" || low == "início" || low == "inicio" {
		s.menu(ctx, from)
		return
	}

	switch {
	case bid == "gw_abrir" || low == "1" || low == "abrir chamado":
		s.iniciarAbertura(ctx, from)
	case bid == "gw_retomar" || low == "2" || low == "retomar chamado":
		s.retomar(ctx, from)
	case strings.HasPrefix(bid, "gw_open_"):
		s.iniciarRelay(ctx, from, parseID(bid, "gw_open_"), "")
	default:
		switch sess.Step {
		case StepAbrindo:
			if txt == "" {
				s.send(ctx, from, "Me manda a descrição do problema em texto, por favor.")
				return
			}
			s.abrir(ctx, from, in.Name, txt, sess.Referencia)
		default:
			s.menu(ctx, from)
		}
	}
}

func (s *Service) menu(ctx context.Context, from string) {
	_ = s.repo.Save(ctx, from, Session{Step: StepMenu})
	s.menuOpts(ctx, from, "👋 Olá! Como posso ajudar?")
}

func (s *Service) menuOpts(ctx context.Context, from, texto string) {
	s.sendMenu(ctx, from, texto, []messaging.MenuOption{
		{Key: "gw_abrir", Label: "🆕 Abrir chamado"},
		{Key: "gw_retomar", Label: "🔁 Retomar chamado"},
	})
}

func (s *Service) iniciarAbertura(ctx context.Context, from string) {
	// Referência estável desta abertura (idempotência em reenvios/retries).
	_ = s.repo.Save(ctx, from, Session{Step: StepAbrindo, Referencia: "wa-" + uuid.NewString()})
	s.send(ctx, from, "Descreva seu problema em uma mensagem 👇")
}

// retomar lista os chamados do telefone; sem chamados, cai para abrir um novo.
func (s *Service) retomar(ctx context.Context, from string) {
	chamados, err := s.client.ListarChamados(ctx, from, maxLista)
	if err != nil {
		log.Printf("gateway: listar chamados de %s: %v", from, err)
		s.send(ctx, from, "Não consegui buscar seus chamados agora. Tente novamente em instantes ou digite *menu*.")
		return
	}
	if len(chamados) == 0 {
		_ = s.repo.Save(ctx, from, Session{Step: StepAbrindo, Referencia: "wa-" + uuid.NewString()})
		s.send(ctx, from, "Você não tem chamados. Descreva seu problema que eu abro um novo 👇")
		return
	}
	opts := make([]messaging.MenuOption, 0, len(chamados))
	for _, c := range chamados {
		opts = append(opts, messaging.MenuOption{
			Key:   "gw_open_" + itoa(c.ID),
			Label: clip("#"+c.Numero+" "+c.Resumo, 24),
		})
	}
	_ = s.repo.Save(ctx, from, Session{Step: StepListando})
	s.sendMenu(ctx, from, "Escolha o chamado para continuar:", opts)
}

// abrir cria o chamado no SIGE e entra em relay.
func (s *Service) abrir(ctx context.Context, from, nome, descricao, referencia string) {
	if referencia == "" {
		referencia = "wa-" + uuid.NewString()
	}
	janela := NovaJanela(s.repo.WindowExpiry(ctx, from))
	resp, err := s.client.AbrirChamado(ctx, from, nome, referencia, map[string]string{"descricao": descricao}, janela)
	if err != nil {
		log.Printf("gateway: abrir chamado de %s: %v", from, err)
		// Mantém o passo 'abrindo' (a referência é reutilizada no reenvio → idempotente).
		s.send(ctx, from, "Não consegui abrir o chamado agora. Reenvie a mensagem em instantes.")
		return
	}
	s.iniciarRelay(ctx, from, resp.TicketID, resp.Numero)
}

// iniciarRelay vincula a conversa ao chamado e confirma.
func (s *Service) iniciarRelay(ctx context.Context, from string, ticketID int64, numero string) {
	if ticketID <= 0 {
		s.send(ctx, from, "Chamado inválido. Digite *menu* para recomeçar.")
		return
	}
	_ = s.repo.Save(ctx, from, Session{Step: StepRelay, TicketID: ticketID})
	if numero != "" {
		s.send(ctx, from, "✅ Chamado *"+numero+"* aberto! Pode mandar suas mensagens que a equipe responde por aqui.")
	} else {
		s.send(ctx, from, "🔁 Retomando o atendimento. Pode mandar sua mensagem 👇")
	}
}

// repassar envia ao SIGE a mensagem recebida no relay.
func (s *Service) repassar(ctx context.Context, from string, ticketID int64, in messaging.IncomingMessage) {
	ev := map[string]any{
		"ticket_id":     ticketID,
		"telefone":      from,
		"wa_message_id": in.WAMessageID,
		"tipo":          tipoMensagem(in),
		"texto":         in.Body,
		"midia_url":     nil, // Fase 3: URL assinada da mídia recebida
		"midia_mime":    nullIfEmpty(in.MediaMime),
		"timestamp":     in.Timestamp.UTC().Format(time.RFC3339),
		"janela_24h":    NovaJanela(s.repo.WindowExpiry(ctx, from)),
	}
	if err := s.client.MensagemRecebida(ctx, ev); err != nil {
		// Fase 2 mete numa fila/retry; por ora loga (a mensagem já está salva no ZapGov).
		log.Printf("gateway: repassar mensagem de %s (ticket %d): %v", from, ticketID, err)
	}
}

func (s *Service) send(ctx context.Context, to, body string) {
	if _, err := s.replier.SendText(ctx, to, body); err != nil {
		log.Printf("gateway: enviar texto para %s: %v", to, err)
	}
}

func (s *Service) sendMenu(ctx context.Context, to, text string, opts []messaging.MenuOption) {
	if _, err := s.replier.SendMenu(ctx, to, text, opts); err != nil {
		log.Printf("gateway: enviar menu para %s: %v", to, err)
	}
}

// tipoMensagem mapeia o tipo de mídia da Meta para o vocabulário do evento.
func tipoMensagem(in messaging.IncomingMessage) string {
	switch in.MediaType {
	case "image", "sticker":
		return "imagem"
	case "audio":
		return "audio"
	case "video":
		return "video"
	case "document":
		return "documento"
	}
	return "texto"
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func parseID(bid, prefix string) int64 {
	id, _ := strconv.ParseInt(strings.TrimPrefix(bid, prefix), 10, 64)
	return id
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
