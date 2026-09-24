package flow

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/google/uuid"

	"zapgov/backend/internal/messaging"
)

const maxSteps = 25 // trava anti-loop por turno

// Replier envia mensagens do bot (satisfeito por messaging.Service).
type Replier interface {
	SendText(ctx context.Context, to, body string) (*messaging.Message, error)
	SendMenu(ctx context.Context, to, text string, options []messaging.MenuOption) (*messaging.Message, error)
}

type Service struct {
	repo    *Repository
	replier Replier
}

func NewService(repo *Repository, replier Replier) *Service {
	return &Service{repo: repo, replier: replier}
}

// HandleInbound executa o fluxo para uma mensagem recebida.
// Assinatura casa com messaging.Service.OnInbound. buttonID = chave do botão/
// lista tocado (vazio em texto livre).
func (s *Service) HandleInbound(ctx context.Context, from, name, body, buttonID string) {
	// Canonicaliza o telefone (reinsere o 9 do BR) para casar com a chave das
	// conversas. Sem isso, o Handoff/AssignDepartment não acham a conversa e o
	// status nunca sai de bot → o menu reaparece a cada mensagem.
	from = messaging.CanonPhone(from)

	fl, err := s.repo.ActiveFlow(ctx)
	if err != nil || fl == nil {
		return // sem fluxo ativo → atendimento humano
	}
	g, err := Parse(fl.Graph)
	if err != nil {
		log.Printf("flow: grafo inválido: %v", err)
		return
	}
	st, err := s.repo.GetState(ctx, from)
	if err != nil {
		log.Printf("flow: estado de %s: %v", from, err)
		return
	}
	s.run(ctx, from, name, g, st, body, buttonID)
}

func (s *Service) run(ctx context.Context, from, name string, g *Graph, st State, input, buttonID string) {
	vars := st.Vars
	if vars == nil {
		vars = map[string]string{}
	}
	// Nome do perfil do WhatsApp fica disponível p/ blocos (ex.: abrir_chamado).
	if name != "" || vars["_nome"] == "" {
		vars["_nome"] = name
	}

	vars["_telefone"] = from
	if vars["_referencia_externa"] == "" {
		vars["_referencia_externa"] = "wa-" + uuid.NewString()
	}
	cur := st.CurrentNode

	if cur == "" {
		// Primeira interação: começa no start (input "oi" só dá o pontapé).
		start := g.start()
		if start == nil {
			return
		}
		cur = g.next(start.ID)
	} else {
		// Estamos num nó de espera (menu/input): consome a resposta.
		node := g.node(cur)
		if node == nil {
			_ = s.repo.ClearState(ctx, from)
			return
		}
		switch node.Type {
		case "api":
			// Uma chamada interrompida retoma com a mesma referência persistida.
		case "menu":
			var md menuData
			_ = json.Unmarshal(node.Data, &md)
			// Casa primeiro pela chave do botão (não trunca); senão pelo texto.
			opt := matchMenuOption(md, buttonID, input)
			if opt == nil {
				s.sendMenu(ctx, from, node) // opção inválida → reenvia menu
				return
			}
			if md.Var != "" {
				// Menu de captura: grava o rótulo escolhido e segue pela saída única.
				vars[md.Var] = opt.Label
				cur = g.next(cur)
			} else {
				cur = g.target(cur, opt.Key) // ramifica pela opção escolhida
			}
		case "input":
			var d inputData
			_ = json.Unmarshal(node.Data, &d)
			if d.Var != "" {
				vars[d.Var] = input
			}
			cur = g.next(cur)
		default:
			cur = g.next(cur)
		}
	}

	// Avança executando nós até bater num bloqueante (menu/input) ou fim.
	for i := 0; i < maxSteps && cur != ""; i++ {
		node := g.node(cur)
		if node == nil {
			break
		}
		switch node.Type {
		case "start":
			cur = g.next(cur)

		case "message":
			var d messageData
			_ = json.Unmarshal(node.Data, &d)
			s.send(ctx, from, render(d.Text, vars))
			cur = g.next(cur)

		case "action":
			var d actionData
			_ = json.Unmarshal(node.Data, &d)
			if d.Kind == "assign_department" && d.Department != "" {
				_ = s.repo.AssignDepartment(ctx, from, d.Department)
			}
			if d.Kind == "abrir_chamado" {
				if d.Prioridade != "" {
					vars["prioridade"] = d.Prioridade
				}
				s.abrirChamado(ctx, from, vars)
				_ = s.repo.ClearState(ctx, from)
				return
			}
			cur = g.next(cur)

		case "api":
			// Persistir antes do efeito externo permite repetir com idempotência.
			if err := s.repo.SaveState(ctx, from, cur, vars); err != nil {
				return
			}
			handle := callAPI(ctx, node.Data, vars)
			if err := s.repo.SaveData(ctx, from, vars); err != nil {
				return
			}
			next := g.target(cur, handle)
			if next == "" && handle == "error" {
				_ = s.repo.Handoff(ctx, from, "", vars)
				s.send(ctx, from, "Não foi possível confirmar o chamado agora. Um atendente vai conferir por aqui.")
			}
			cur = next

		case "condition":
			var d conditionData
			_ = json.Unmarshal(node.Data, &d)
			cur = g.target(cur, boolStr(evalCond(vars[d.Var], d.Op, d.Value)))

		case "menu":
			s.sendMenu(ctx, from, node)
			_ = s.repo.SaveState(ctx, from, cur, vars)
			return

		case "input":
			var d inputData
			_ = json.Unmarshal(node.Data, &d)
			s.send(ctx, from, render(d.Prompt, vars))
			_ = s.repo.SaveState(ctx, from, cur, vars)
			return

		case "handoff":
			var d handoffData
			_ = json.Unmarshal(node.Data, &d)
			// Grava setor + dados coletados (variáveis) para o atendente.
			_ = s.repo.Handoff(ctx, from, d.Department, vars)
			msg := "✅ Tudo certo! Um atendente vai continuar por aqui."
			if d.Department != "" {
				msg = "✅ Encaminhado para *" + d.Department + "*. Um atendente vai continuar por aqui."
			}
			s.send(ctx, from, msg)
			_ = s.repo.ClearState(ctx, from)
			return

		case "end":
			_ = s.repo.ClearState(ctx, from)
			return

		default:
			cur = g.next(cur)
		}
	}

	// Beco sem saída ou estouro de passos: reseta para não travar a conversa.
	_ = s.repo.ClearState(ctx, from)
}

// Fluxos antigos mantêm os dados coletados e entram no atendimento humano.
func (s *Service) abrirChamado(ctx context.Context, from string, vars map[string]string) {
	if err := s.repo.Handoff(ctx, from, "", vars); err != nil {
		log.Printf("flow: encaminhar atendimento: %v", err)
	}
}

func (s *Service) send(ctx context.Context, to, body string) {
	if body == "" {
		return
	}
	if _, err := s.replier.SendText(ctx, to, body); err != nil {
		log.Printf("flow: falha ao enviar para %s: %v", to, err)
	}
}

func (s *Service) sendMenu(ctx context.Context, to string, node *Node) {
	var d menuData
	_ = json.Unmarshal(node.Data, &d)
	if len(d.Options) == 0 {
		s.send(ctx, to, d.Text)
		return
	}
	opts := make([]messaging.MenuOption, len(d.Options))
	for i, o := range d.Options {
		opts[i] = messaging.MenuOption{Key: o.Key, Label: o.Label}
	}
	if _, err := s.replier.SendMenu(ctx, to, d.Text, opts); err != nil {
		log.Printf("flow: enviar menu para %s: %v", to, err)
	}
}

// matchMenuOption acha a opção escolhida: prioriza a chave do botão/lista tocado
// (buttonID — confiável, não trunca); senão casa por chave digitada ou rótulo.
func matchMenuOption(md menuData, buttonID, input string) *struct {
	Key   string `json:"key"`
	Label string `json:"label"`
} {
	bid := strings.TrimSpace(buttonID)
	if bid != "" {
		for i := range md.Options {
			if strings.EqualFold(strings.TrimSpace(md.Options[i].Key), bid) {
				return &md.Options[i]
			}
		}
	}
	in := strings.TrimSpace(input)
	for i := range md.Options {
		o := &md.Options[i]
		if strings.EqualFold(strings.TrimSpace(o.Key), in) || strings.EqualFold(strings.TrimSpace(o.Label), in) {
			return o
		}
	}
	return nil
}

// keyForLabel acha a chave da opção cujo rótulo casa com o texto (toque no botão).
func keyForLabel(node *Node, input string) string {
	var d menuData
	_ = json.Unmarshal(node.Data, &d)
	for _, o := range d.Options {
		if strings.EqualFold(strings.TrimSpace(o.Label), strings.TrimSpace(input)) {
			return o.Key
		}
	}
	return ""
}

// render substitui {{var}} pelos valores capturados.
func render(text string, vars map[string]string) string {
	return variablePattern.ReplaceAllStringFunc(text, func(token string) string {
		if value, ok := vars[token[2:len(token)-2]]; ok {
			return value
		}
		return token
	})
}

func evalCond(got, op, want string) bool {
	switch op {
	case "contains":
		return strings.Contains(strings.ToLower(got), strings.ToLower(want))
	default: // eq
		return strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(want))
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
