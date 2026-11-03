package gateway

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"encoding/json"

	"github.com/google/uuid"

	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/tenantctx"
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
	Touch(ctx context.Context, phone string) error
	WindowExpiry(ctx context.Context, phone string) *time.Time
}

// sigeAPI é o cliente do SIGE (satisfeito por *Client).
type sigeAPI interface {
	AbrirChamado(ctx context.Context, telefone, nome, referencia string, dados map[string]string, janela Janela) (*AbrirResp, error)
	ListarChamados(ctx context.Context, telefone string, limite int) ([]Chamado, error)
}

// eventQueue grava eventos para entrega com retry (satisfeito por *Outbox).
type eventQueue interface {
	Enqueue(ctx context.Context, phone, dedup string, ev map[string]any) error
}

// Service é o bot cru do gateway + o relay para o SIGE.
type Service struct {
	repo    sessionStore
	client  sigeAPI
	queue   eventQueue
	replier Replier
	// sige entrega eventos crus e midia assina a URL da mídia recebida no
	// momento da entrega (o token não vence durante os retries da fila).
	sige  func(ctx context.Context, raw []byte) error
	midia *Assinador
	// relayTTL: relay sem movimento (pessoa ou técnico) por mais que isso é
	// encerrado na próxima mensagem da pessoa, que volta ao menu. 0 = nunca.
	relayTTL time.Duration
}

func NewService(repo *Repository, client *Client, queue *Outbox, replier Replier, relayTTL time.Duration, midia *Assinador) *Service {
	return &Service{repo: repo, client: client, queue: queue, replier: replier, relayTTL: relayTTL, sige: client.Entregar, midia: midia}
}

// palavrasSair tiram a pessoa do relay (mensagem inteira, sem diferenciar caixa).
var palavrasSair = map[string]bool{"sair": true, "#sair": true, "menu": true}

const maxLista = 10 // limite de linhas da lista interativa da Meta

// HandleInbound processa uma mensagem recebida no tenant do gateway. Em relay,
// repassa ao SIGE; senão, roda o menu Abrir/Retomar.
func (s *Service) HandleInbound(ctx context.Context, in messaging.IncomingMessage) {
	from := in.From
	sess := s.repo.Load(ctx, from)

	bid := strings.TrimSpace(in.ButtonID)
	txt := strings.TrimSpace(in.Body)
	low := strings.ToLower(txt)

	if sess.Step == StepRelay {
		switch {
		case palavrasSair[low]:
			_ = s.repo.Save(ctx, from, Session{Step: StepMenu})
			s.menuOpts(ctx, from, "Você saiu do atendimento. O chamado continua registrado; para voltar a falar com a equipe, escolha *Retomar chamado*.")
		case s.relayExpirado(sess):
			_ = s.repo.Save(ctx, from, Session{Step: StepMenu})
			s.menuOpts(ctx, from, "Seu atendimento anterior foi encerrado por falta de movimento. Se a mensagem era sobre aquele chamado, escolha *Retomar chamado*.")
		default:
			s.repassar(ctx, from, sess.TicketID, in)
		}
		return
	}

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
		s.send(ctx, from, "✅ Chamado *"+numero+"* aberto! Pode mandar suas mensagens que a equipe responde por aqui.\n\nPara sair do atendimento, digite *sair*.")
	} else {
		s.send(ctx, from, "🔁 Retomando o atendimento. Pode mandar sua mensagem 👇\n\nPara sair do atendimento, digite *sair*.")
	}
}

// relayExpirado diz se o relay ficou parado mais que relayTTL.
func (s *Service) relayExpirado(sess Session) bool {
	return s.relayTTL > 0 && !sess.AtualizadoEm.IsZero() && time.Since(sess.AtualizadoEm) > s.relayTTL
}

// repassar envia ao SIGE a mensagem recebida no relay.
func (s *Service) repassar(ctx context.Context, from string, ticketID int64, in messaging.IncomingMessage) {
	ev := map[string]any{
		"ticket_id":     ticketID,
		"telefone":      from,
		"wa_message_id": in.WAMessageID,
		"tipo":          tipoMensagem(in),
		"texto":         in.Body,
		"midia_url":     nil, // preenchida na entrega (Entregar) com URL assinada
		"midia_mime":    nullIfEmpty(in.MediaMime),
		"timestamp":     in.Timestamp.UTC().Format(time.RFC3339),
		"janela_24h":    NovaJanela(s.repo.WindowExpiry(ctx, from)),
	}
	ev["evento"] = "mensagem.recebida"
	if in.MediaID != "" {
		// Campos internos (prefixo _): viram midia_url na entrega e não vão ao SIGE.
		ev["_midia_id"] = in.MediaID
		ev["_midia_nome"] = in.MediaName
	}
	// Grava na fila; o Outbox entrega com retry. Se nem gravar der (banco fora),
	// avisa a pessoa para reenviar.
	if err := s.queue.Enqueue(ctx, from, in.WAMessageID, ev); err != nil {
		log.Printf("gateway: enfileirar mensagem de %s (ticket %d): %v", from, ticketID, err)
		s.send(ctx, from, "Não consegui registrar sua mensagem agora. Reenvie em instantes, por favor.")
		return
	}
	_ = s.repo.Touch(ctx, from)
}

// Entregar é o deliver do Outbox: troca os campos internos de mídia por uma URL
// assinada recém-gerada e envia o evento ao SIGE.
func (s *Service) Entregar(ctx context.Context, raw []byte) error {
	if !strings.Contains(string(raw), `"_midia_id"`) {
		return s.sige(ctx, raw)
	}
	var ev map[string]any
	if err := json.Unmarshal(raw, &ev); err != nil {
		return err
	}
	id, _ := ev["_midia_id"].(string)
	nome, _ := ev["_midia_nome"].(string)
	mt, _ := ev["midia_mime"].(string)
	delete(ev, "_midia_id")
	delete(ev, "_midia_nome")
	if u := s.midia.URL(MidiaRef{TenantID: tenantctx.From(ctx), MediaID: id, Mime: mt, Nome: nome}); u != "" {
		ev["midia_url"] = u
	} else {
		log.Printf("gateway: GATEWAY_PUBLIC_URL vazia; mídia %s vai sem midia_url", id)
	}
	out, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return s.sige(ctx, out)
}

// AvisarFalha informa a pessoa que a mensagem não chegou (ainda) ao técnico.
// Chamado pelo Outbox com o tenant já no contexto.
func (s *Service) AvisarFalha(ctx context.Context, f Falha) {
	if f.Final {
		s.send(ctx, f.Telefone, "Não conseguimos entregar sua mensagem à equipe depois de várias tentativas. Por favor, reenvie mais tarde.")
		return
	}
	s.send(ctx, f.Telefone, "Recebemos sua mensagem, mas o sistema de chamados está instável agora. Ela será entregue à equipe assim que normalizar; não precisa reenviar.")
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
