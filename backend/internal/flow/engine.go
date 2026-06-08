package flow

import (
	"context"
	"encoding/json"
	"log"
	"strings"

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
// Assinatura casa com messaging.Service.OnInbound.
func (s *Service) HandleInbound(ctx context.Context, from, name, body string) {
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
	s.run(ctx, from, g, st, body)
}

func (s *Service) run(ctx context.Context, from string, g *Graph, st State, input string) {
	vars := st.Vars
	if vars == nil {
		vars = map[string]string{}
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
		case "menu":
			in := strings.TrimSpace(input)
			t := g.target(cur, in) // casa pela chave (tecla digitada / id do botão)
			if t == "" {
				t = g.target(cur, keyForLabel(node, in)) // ou pelo rótulo (toque no botão)
			}
			if t == "" {
				s.sendMenu(ctx, from, node) // opção inválida → reenvia menu
				return
			}
			cur = t
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
			cur = g.next(cur)

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
	for k, v := range vars {
		text = strings.ReplaceAll(text, "{{"+k+"}}", v)
	}
	return text
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
