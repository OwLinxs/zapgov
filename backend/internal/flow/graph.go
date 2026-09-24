// Package flow é o motor de chatbot por grafo (blocos interligados, estilo n8n).
package flow

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Node é um bloco do fluxo. Data é específico do tipo (parseado sob demanda).
type Node struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"` // start|message|menu|input|condition|action|api|handoff|end
	Data     json.RawMessage `json:"data"`
	Position struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"position"`
}

// Edge liga a saída de um nó (SourceHandle = tecla da opção / "true"/"false" / "")
// à entrada do próximo.
type Edge struct {
	Source       string `json:"source"`
	SourceHandle string `json:"sourceHandle"`
	Target       string `json:"target"`
}

type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

func Parse(raw []byte) (*Graph, error) {
	var g Graph
	if err := json.Unmarshal(raw, &g); err != nil {
		return nil, err
	}
	for _, n := range g.Nodes {
		if n.Type != "api" {
			continue
		}
		var d apiData
		if json.Unmarshal(n.Data, &d) != nil || d.Integration != "sige_ti" {
			return nil, fmt.Errorf("Chamar API: integração inválida")
		}
		var body map[string]string
		if len(d.Body) > 64*1024 || json.Unmarshal([]byte(d.Body), &body) != nil || body == nil || strings.TrimSpace(body["defeito_relatado"]) == "" {
			return nil, fmt.Errorf("Chamar API: informe um objeto JSON com valores de texto e defeito_relatado")
		}
		used := map[string]bool{}
		for _, target := range d.Response {
			if target == "" {
				continue
			}
			if !responseVariable.MatchString(target) || strings.HasPrefix(target, "_") || target == "api_erro" || target == "api_http_status" || used[target] {
				return nil, fmt.Errorf("Chamar API: variáveis de retorno inválidas ou repetidas")
			}
			used[target] = true
		}
	}
	return &g, nil
}

func (g *Graph) node(id string) *Node {
	for i := range g.Nodes {
		if g.Nodes[i].ID == id {
			return &g.Nodes[i]
		}
	}
	return nil
}

func (g *Graph) start() *Node {
	for i := range g.Nodes {
		if g.Nodes[i].Type == "start" {
			return &g.Nodes[i]
		}
	}
	return nil
}

// target devolve o destino da aresta que sai de `id` pelo handle dado.
// handle "" = saída única/padrão.
func (g *Graph) target(id, handle string) string {
	for _, e := range g.Edges {
		if e.Source == id && e.SourceHandle == handle {
			return e.Target
		}
	}
	return ""
}

// next = saída padrão (handle vazio).
func (g *Graph) next(id string) string { return g.target(id, "") }

// ---- formatos de Data por tipo de nó ----

type menuData struct {
	Text string `json:"text"`
	// Var, se preenchido, faz o menu CAPTURAR a escolha: grava o rótulo da opção
	// tocada nesta variável e segue pela saída única (não ramifica por opção).
	Var     string `json:"var"`
	Options []struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	} `json:"options"`
}

type messageData struct {
	Text string `json:"text"`
}

type inputData struct {
	Prompt string `json:"prompt"`
	Var    string `json:"var"`
}

type conditionData struct {
	Var   string `json:"var"`
	Op    string `json:"op"` // eq | contains
	Value string `json:"value"`
}

type actionData struct {
	Kind       string `json:"kind"` // assign_department | abrir_chamado
	Department string `json:"department"`
	Prioridade string `json:"prioridade"` // p/ abrir_chamado (baixa|media|alta|critica)
}

type handoffData struct {
	Department string `json:"department"`
}
