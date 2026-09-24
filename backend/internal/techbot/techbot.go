// Package techbot implementa o "modo técnico" do helpdesk pelo WhatsApp: um
// técnico de campo lista seus chamados, vê o problema de cada um e age (assumir,
// comentar, resolver) por menu/botões — sem precisar do painel web.
//
// Não usa o motor de fluxo (grafo estático): a lista de chamados é dinâmica, lida
// do banco. Roda para números isentos do bot (IsBotExempt = técnicos de campo).
package techbot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/tickets"
)

// TicketOps é o subconjunto do serviço de chamados que o modo técnico usa.
type TicketOps interface {
	List(ctx context.Context, status, prioridade, categoria, tecnico string) ([]tickets.Ticket, error)
	Get(ctx context.Context, id int64) (*tickets.Ticket, error)
	GetByNumero(ctx context.Context, numero int) (*tickets.Ticket, error)
	Assign(ctx context.Context, id int64, tecnico string) (*tickets.Ticket, error)
	SetStatus(ctx context.Context, id int64, status string) (*tickets.Ticket, error)
	AddComment(ctx context.Context, id int64, autor, body string, interno bool) (*tickets.Comment, error)
}

// Replier envia texto e menus (satisfeito por messaging.Service).
type Replier interface {
	SendText(ctx context.Context, to, body string) (*messaging.Message, error)
	SendMenu(ctx context.Context, to, text string, options []messaging.MenuOption) (*messaging.Message, error)
}

// TechResolver devolve o nome do técnico dono do número (ok=false se não é técnico).
type TechResolver func(ctx context.Context, phone string) (nome string, ok bool)

// Store guarda o passo/foco da sessão do técnico (satisfeito por *Repository).
type Store interface {
	Load(ctx context.Context, phone string) (step string, ticketID int64)
	Save(ctx context.Context, phone, step string, ticketID int64) error
	Clear(ctx context.Context, phone string) error
}

type Service struct {
	repo     Store
	tickets  TicketOps
	replier  Replier
	resolver TechResolver
}

func NewService(repo Store, ops TicketOps, replier Replier, resolver TechResolver) *Service {
	return &Service{repo: repo, tickets: ops, replier: replier, resolver: resolver}
}

const maxLista = 9 // limite de linhas na lista interativa da Meta (10) com folga

// Handle processa uma mensagem de um número isento (técnico de campo). Decide a
// resposta pelo botão tocado ou pelo texto + passo salvo. Nunca entra em pânico:
// erros viram aviso ao técnico e log.
func (s *Service) Handle(ctx context.Context, from, name, body, buttonID string) {
	nome, ok := s.resolver(ctx, from)
	if !ok {
		return // não é técnico cadastrado: ignora (só abriu a janela de 24h)
	}
	bid := strings.TrimSpace(buttonID)
	txt := strings.TrimSpace(body)
	low := strings.ToLower(txt)

	// Comandos globais: voltam ao menu principal de qualquer estado.
	if bid == "tk_menu" || low == "menu" || low == "chamados" || low == "oi" ||
		low == "0" || low == "inicio" || low == "início" || low == "voltar" {
		s.mainMenu(ctx, from, nome)
		return
	}

	switch {
	case bid == "tk_meus":
		s.listar(ctx, from, nome, "meus")
	case bid == "tk_fila":
		s.listar(ctx, from, nome, "fila")
	case bid == "tk_buscar":
		s.repo.Save(ctx, from, "search", 0)
		s.send(ctx, from, "Digite o *número* do chamado (ex.: 12).")
	case strings.HasPrefix(bid, "tk_o_"):
		s.detalhe(ctx, from, parseID(bid, "tk_o_"))
	case strings.HasPrefix(bid, "tk_as_"):
		s.assumir(ctx, from, nome, parseID(bid, "tk_as_"))
	case strings.HasPrefix(bid, "tk_re_"):
		s.resolverChamado(ctx, from, nome, parseID(bid, "tk_re_"))
	case strings.HasPrefix(bid, "tk_co_"):
		s.repo.Save(ctx, from, "note", parseID(bid, "tk_co_"))
		s.send(ctx, from, "Escreva o *comentário* para registrar no chamado.")
	default:
		s.handleTexto(ctx, from, nome, txt)
	}
}

// handleTexto trata texto livre conforme o passo salvo (comentário, busca) ou,
// no menu, um número digitado como atalho para abrir o chamado.
func (s *Service) handleTexto(ctx context.Context, from, nome, txt string) {
	step, ticketID := s.repo.Load(ctx, from)
	switch step {
	case "note":
		if txt == "" {
			s.send(ctx, from, "Comentário vazio. Escreva o texto ou digite *menu*.")
			return
		}
		if _, err := s.tickets.AddComment(ctx, ticketID, nome, txt, false); err != nil {
			s.send(ctx, from, "Não consegui registrar o comentário. Digite *menu* e tente de novo.")
			return
		}
		s.send(ctx, from, "✅ Comentário registrado.")
		s.detalhe(ctx, from, ticketID)
	case "search":
		n, err := strconv.Atoi(strings.TrimPrefix(txt, "#"))
		if err != nil || n <= 0 {
			s.send(ctx, from, "Número inválido. Digite só o número (ex.: 12) ou *menu*.")
			return
		}
		t, err := s.tickets.GetByNumero(ctx, n)
		if err != nil || t == nil {
			s.send(ctx, from, fmt.Sprintf("Não achei o chamado #%d. Tente outro número ou *menu*.", n))
			return
		}
		s.detalhe(ctx, from, t.ID)
	default:
		// No menu: número digitado abre o chamado; senão mostra o menu.
		if n, err := strconv.Atoi(strings.TrimPrefix(txt, "#")); err == nil && n > 0 {
			if t, _ := s.tickets.GetByNumero(ctx, n); t != nil {
				s.detalhe(ctx, from, t.ID)
				return
			}
		}
		s.mainMenu(ctx, from, nome)
	}
}

func (s *Service) mainMenu(ctx context.Context, from, nome string) {
	s.repo.Clear(ctx, from)
	primeiro := strings.Fields(nome)
	saud := "técnico"
	if len(primeiro) > 0 {
		saud = primeiro[0]
	}
	texto := fmt.Sprintf("👷 Olá, %s! Central de chamados do T.I.\nO que deseja?", saud)
	s.menu(ctx, from, texto, []messaging.MenuOption{
		{Key: "tk_meus", Label: "📋 Meus chamados"},
		{Key: "tk_fila", Label: "📥 Fila (sem técnico)"},
		{Key: "tk_buscar", Label: "🔎 Buscar por número"},
	})
}

// listar mostra até maxLista chamados abertos. modo "meus" = atribuídos ao
// técnico; "fila" = abertos sem técnico.
func (s *Service) listar(ctx context.Context, from, nome, modo string) {
	s.repo.Clear(ctx, from)
	var lista []tickets.Ticket
	if modo == "meus" {
		ts, err := s.tickets.List(ctx, "", "", "", nome)
		if err != nil {
			s.send(ctx, from, "Erro ao buscar seus chamados. Tente *menu*.")
			return
		}
		for _, t := range ts {
			if t.Status != "resolvido" && t.Status != "cancelado" {
				lista = append(lista, t)
			}
		}
	} else {
		ts, err := s.tickets.List(ctx, "aberto", "", "", "")
		if err != nil {
			s.send(ctx, from, "Erro ao buscar a fila. Tente *menu*.")
			return
		}
		for _, t := range ts {
			if t.Tecnico == "" {
				lista = append(lista, t)
			}
		}
	}
	if len(lista) == 0 {
		vazio := "Você não tem chamados em aberto. 🎉"
		if modo == "fila" {
			vazio = "Não há chamados aguardando na fila. 🎉"
		}
		s.send(ctx, from, vazio+"\n\nDigite *menu* para voltar.")
		return
	}
	total := len(lista)
	if len(lista) > maxLista {
		lista = lista[:maxLista]
	}
	opts := make([]messaging.MenuOption, 0, len(lista))
	for _, t := range lista {
		opts = append(opts, messaging.MenuOption{
			Key:   fmt.Sprintf("tk_o_%d", t.ID),
			Label: fmt.Sprintf("#%d %s", t.Numero, clip(t.Titulo, 18)),
		})
	}
	cab := fmt.Sprintf("Toque no chamado para ver os detalhes (%d):", total)
	if total > maxLista {
		cab = fmt.Sprintf("Mostrando %d de %d. Toque para ver:", maxLista, total)
	}
	s.menu(ctx, from, cab, opts)
}

func (s *Service) detalhe(ctx context.Context, from string, id int64) {
	t, err := s.tickets.Get(ctx, id)
	if err != nil || t == nil {
		s.send(ctx, from, "Chamado não encontrado. Digite *menu*.")
		return
	}
	s.repo.Save(ctx, from, "", id) // guarda o foco (comentário sabe o alvo)
	var b strings.Builder
	fmt.Fprintf(&b, "🎫 *Chamado #%d*\n", t.Numero)
	fmt.Fprintf(&b, "*%s*\n", t.Titulo)
	if t.Descricao != "" {
		fmt.Fprintf(&b, "%s\n", t.Descricao)
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "Status: %s · Prioridade: %s\n", statusLabel(t.Status), t.Prioridade)
	if t.Categoria != "" {
		fmt.Fprintf(&b, "Categoria: %s\n", t.Categoria)
	}
	if t.SolicitanteNome != "" || t.SolicitanteFone != "" {
		fmt.Fprintf(&b, "Solicitante: %s %s\n", t.SolicitanteNome, t.SolicitanteFone)
	}
	if t.Secretaria != "" || t.Local != "" {
		fmt.Fprintf(&b, "Local: %s %s\n", t.Secretaria, t.Local)
	}
	if t.Tecnico != "" {
		fmt.Fprintf(&b, "Técnico: %s\n", t.Tecnico)
	}

	// Ações válidas conforme o status. Resolvido/cancelado só oferecem voltar.
	var opts []messaging.MenuOption
	if t.Status != "resolvido" && t.Status != "cancelado" {
		opts = append(opts, messaging.MenuOption{Key: fmt.Sprintf("tk_as_%d", id), Label: "🙋 Assumir"})
		opts = append(opts, messaging.MenuOption{Key: fmt.Sprintf("tk_co_%d", id), Label: "💬 Comentar"})
		opts = append(opts, messaging.MenuOption{Key: fmt.Sprintf("tk_re_%d", id), Label: "✅ Resolver"})
	} else {
		opts = append(opts, messaging.MenuOption{Key: "tk_menu", Label: "⬅️ Voltar ao menu"})
	}
	s.menu(ctx, from, b.String(), opts)
}

func (s *Service) assumir(ctx context.Context, from, nome string, id int64) {
	t, err := s.tickets.Assign(ctx, id, nome)
	if err != nil {
		s.send(ctx, from, "Não consegui assumir o chamado. Digite *menu* e tente de novo.")
		return
	}
	s.send(ctx, from, fmt.Sprintf("🙋 Você assumiu o chamado #%d.", t.Numero))
	s.detalhe(ctx, from, id)
}

// resolver garante a transição: assume (aberto/aguardando → em_andamento) antes
// de fechar, já que resolvido só é válido a partir de em_andamento.
func (s *Service) resolverChamado(ctx context.Context, from, nome string, id int64) {
	t, err := s.tickets.Get(ctx, id)
	if err != nil || t == nil {
		s.send(ctx, from, "Chamado não encontrado. Digite *menu*.")
		return
	}
	if t.Status == "resolvido" {
		s.send(ctx, from, fmt.Sprintf("Chamado #%d já está resolvido.", t.Numero))
		return
	}
	if t.Status == "aberto" || t.Status == "aguardando" {
		if _, err := s.tickets.Assign(ctx, id, nome); err != nil {
			s.send(ctx, from, "Não consegui resolver o chamado. Digite *menu*.")
			return
		}
	}
	res, err := s.tickets.SetStatus(ctx, id, "resolvido")
	if err != nil {
		s.send(ctx, from, "Não consegui resolver o chamado. Digite *menu*.")
		return
	}
	s.repo.Clear(ctx, from)
	s.send(ctx, from, fmt.Sprintf("✅ Chamado #%d resolvido. O solicitante foi avisado.\n\nDigite *menu* para ver outros.", res.Numero))
}

func (s *Service) send(ctx context.Context, to, body string) {
	if _, err := s.replier.SendText(ctx, to, body); err != nil {
		log.Printf("techbot: enviar texto para %s: %v", to, err)
	}
}

func (s *Service) menu(ctx context.Context, to, text string, opts []messaging.MenuOption) {
	if _, err := s.replier.SendMenu(ctx, to, text, opts); err != nil {
		log.Printf("techbot: enviar menu para %s: %v", to, err)
	}
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

func statusLabel(s string) string {
	switch s {
	case "aberto":
		return "🆕 Aberto"
	case "em_andamento":
		return "🔧 Em andamento"
	case "aguardando":
		return "⏳ Aguardando"
	case "resolvido":
		return "✅ Resolvido"
	case "cancelado":
		return "🚫 Cancelado"
	}
	return s
}
