// Package templates sincroniza os templates aprovados pela Meta a partir da WABA.
package templates

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Template é a forma exposta pela API (alinhada com o front).
type Template struct {
	ID        string   `json:"id"` // id do template na Meta (necessário para editar)
	Name      string   `json:"name"`
	Language  string   `json:"language"`
	Status    string   `json:"status"`   // aprovado | pendente | rejeitado
	Category  string   `json:"category"` // Utilidade | Marketing | Autenticação
	Header    string   `json:"header"`
	Body      string   `json:"body"`
	Footer    string   `json:"footer"`
	Buttons   []string `json:"buttons"`
	Variables int      `json:"variables"`
	Motivo    string   `json:"motivo,omitempty"` // razão da rejeição, se houver
	Setores   []string `json:"setores"`          // departamentos liberados (vazio = só admin/supervisor)
	Grupo     string   `json:"grupo"`            // grupo organizacional (ex.: "Secretaria de Saúde")
}

// Client busca templates na Graph API da Meta.
type Client struct {
	wabaID     string
	token      string
	apiVersion string
	http       *http.Client
	// Resolve, se definido, devolve as credenciais Meta do tenant do context
	// (waba, token, apiVersion). Vazio => usa o padrão do .env (tenant 1).
	Resolve func(ctx context.Context) (waba, token, apiVersion string)
}

func NewClient(wabaID, token, apiVersion string) *Client {
	return &Client{
		wabaID:     wabaID,
		token:      token,
		apiVersion: apiVersion,
		http:       &http.Client{Timeout: 15 * time.Second},
	}
}

// creds resolve as credenciais do tenant atual, com fallback para o padrão.
func (c *Client) creds(ctx context.Context) (string, string, string) {
	if c.Resolve != nil {
		if w, t, v := c.Resolve(ctx); w != "" && t != "" {
			if v == "" {
				v = c.apiVersion
			}
			return w, t, v
		}
	}
	return c.wabaID, c.token, c.apiVersion
}

// ---- forma crua da resposta da Meta ----
type metaTemplate struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	Category       string `json:"category"`
	Language       string `json:"language"`
	RejectedReason string `json:"rejected_reason"`
	Components     []struct {
		Type    string `json:"type"`   // HEADER | BODY | FOOTER | BUTTONS
		Format  string `json:"format"` // TEXT, IMAGE…
		Text    string `json:"text"`
		Buttons []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"buttons"`
	} `json:"components"`
}

type metaResponse struct {
	Data  []metaTemplate `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

var varRe = regexp.MustCompile(`\{\{\s*\d+\s*\}\}`)

// List busca os templates da WABA e os mapeia para a forma da API.
func (c *Client) List(ctx context.Context) ([]Template, error) {
	waba, token, apiVer := c.creds(ctx)
	if waba == "" {
		return nil, fmt.Errorf("WHATSAPP_WABA_ID não configurado")
	}

	endpoint := fmt.Sprintf("https://graph.facebook.com/%s/%s/message_templates?limit=200&access_token=%s",
		apiVer, waba, url.QueryEscape(token))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chamar Graph API: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var mr metaResponse
	if err := json.Unmarshal(body, &mr); err != nil {
		return nil, fmt.Errorf("decode (%d): %s", resp.StatusCode, string(body))
	}
	if resp.StatusCode >= 300 || mr.Error != nil {
		m := "erro desconhecido"
		if mr.Error != nil {
			m = mr.Error.Message
		}
		return nil, fmt.Errorf("Graph API (%d): %s", resp.StatusCode, m)
	}

	out := make([]Template, 0, len(mr.Data))
	for _, t := range mr.Data {
		out = append(out, mapTemplate(t))
	}
	return out, nil
}

func mapTemplate(t metaTemplate) Template {
	tpl := Template{
		ID:       t.ID,
		Name:     t.Name,
		Language: t.Language,
		Status:   mapStatus(t.Status),
		Category: mapCategory(t.Category),
		Motivo:   normalizeReason(t.RejectedReason),
	}
	for _, comp := range t.Components {
		switch comp.Type {
		case "HEADER":
			if comp.Format == "TEXT" {
				tpl.Header = comp.Text
			}
		case "BODY":
			tpl.Body = comp.Text
			tpl.Variables = len(varRe.FindAllString(comp.Text, -1))
		case "FOOTER":
			tpl.Footer = comp.Text
		case "BUTTONS":
			for _, b := range comp.Buttons {
				tpl.Buttons = append(tpl.Buttons, b.Text)
			}
		}
	}
	return tpl
}

// CreateRequest é a definição de um novo template a enviar para aprovação.
type CreateRequest struct {
	Nome      string
	Idioma    string
	Categoria string // UTILITY | MARKETING | AUTHENTICATION
	Corpo     string
	Header    string   // opcional (texto)
	Footer    string   // opcional
	Botoes    []string // opcional: rótulos de quick-reply
	Exemplos  []string // exemplo de valor para cada variável {{1}}, {{2}}…
}

func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('_')
		}
	}
	return b.String()
}

// Create envia um novo template para aprovação da Meta.
func (c *Client) Create(ctx context.Context, req CreateRequest) error {
	waba, token, apiVer := c.creds(ctx)
	if waba == "" {
		return fmt.Errorf("WHATSAPP_WABA_ID não configurado")
	}
	components := []map[string]any{}
	if req.Header != "" {
		components = append(components, map[string]any{"type": "HEADER", "format": "TEXT", "text": req.Header})
	}
	bodyComp := map[string]any{"type": "BODY", "text": req.Corpo}
	// A Meta exige um exemplo por variável quando o corpo tem {{n}}.
	if len(req.Exemplos) > 0 {
		bodyComp["example"] = map[string]any{"body_text": [][]string{req.Exemplos}}
	}
	components = append(components, bodyComp)
	if req.Footer != "" {
		components = append(components, map[string]any{"type": "FOOTER", "text": req.Footer})
	}
	if len(req.Botoes) > 0 {
		btns := make([]map[string]any, 0, len(req.Botoes))
		for _, b := range req.Botoes {
			btns = append(btns, map[string]any{"type": "QUICK_REPLY", "text": b})
		}
		components = append(components, map[string]any{"type": "BUTTONS", "buttons": btns})
	}

	payload := map[string]any{
		"name":       sanitizeName(req.Nome),
		"language":   req.Idioma,
		"category":   req.Categoria,
		"components": components,
	}
	raw, _ := json.Marshal(payload)

	endpoint := fmt.Sprintf("https://graph.facebook.com/%s/%s/message_templates", apiVer, waba)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("chamar Graph API: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &out)
	if resp.StatusCode >= 300 || out.Error != nil {
		m := "erro desconhecido"
		if out.Error != nil {
			m = out.Error.Message
		}
		return fmt.Errorf("Meta recusou (%d): %s", resp.StatusCode, m)
	}
	return nil
}

// EditRequest edita os componentes de um template existente (por ID da Meta).
// Nome, idioma e categoria NÃO mudam — a Meta não permite em template aprovado.
type EditRequest struct {
	ID       string
	Corpo    string
	Header   string
	Footer   string
	Botoes   []string
	Exemplos []string
}

// Edit altera os componentes de um template já criado. Após editar, a Meta
// devolve o template ao estado PENDING — fica inutilizável até reaprovar.
func (c *Client) Edit(ctx context.Context, req EditRequest) error {
	_, token, apiVer := c.creds(ctx)
	if req.ID == "" {
		return fmt.Errorf("id do template é obrigatório")
	}
	if token == "" {
		return fmt.Errorf("token da Meta não configurado")
	}
	components := []map[string]any{}
	if req.Header != "" {
		components = append(components, map[string]any{"type": "HEADER", "format": "TEXT", "text": req.Header})
	}
	bodyComp := map[string]any{"type": "BODY", "text": req.Corpo}
	if len(req.Exemplos) > 0 {
		bodyComp["example"] = map[string]any{"body_text": [][]string{req.Exemplos}}
	}
	components = append(components, bodyComp)
	if req.Footer != "" {
		components = append(components, map[string]any{"type": "FOOTER", "text": req.Footer})
	}
	if len(req.Botoes) > 0 {
		btns := make([]map[string]any, 0, len(req.Botoes))
		for _, b := range req.Botoes {
			btns = append(btns, map[string]any{"type": "QUICK_REPLY", "text": b})
		}
		components = append(components, map[string]any{"type": "BUTTONS", "buttons": btns})
	}
	raw, _ := json.Marshal(map[string]any{"components": components})

	// Edição é POST direto no ID do template (não no endpoint da WABA).
	endpoint := fmt.Sprintf("https://graph.facebook.com/%s/%s", apiVer, req.ID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("chamar Graph API: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &out)
	if resp.StatusCode >= 300 || out.Error != nil {
		m := "erro desconhecido"
		if out.Error != nil {
			m = out.Error.Message
		}
		return fmt.Errorf("Meta recusou a edição (%d): %s", resp.StatusCode, m)
	}
	return nil
}

// IsUsable diz se o template pode ser disparado: aprovado E categoria Utilidade.
func (c *Client) IsUsable(ctx context.Context, name, language string) (bool, error) {
	list, err := c.List(ctx)
	if err != nil {
		return false, err
	}
	for _, t := range list {
		if t.Name == name && (language == "" || t.Language == language) {
			return t.Status == "aprovado" && t.Category == "Utilidade", nil
		}
	}
	return false, nil
}

func mapStatus(s string) string {
	switch s {
	case "APPROVED":
		return "aprovado"
	case "REJECTED":
		return "rejeitado"
	default: // PENDING, IN_APPEAL, PAUSED, DISABLED…
		return "pendente"
	}
}

func mapCategory(c string) string {
	switch c {
	case "MARKETING":
		return "Marketing"
	case "AUTHENTICATION":
		return "Autenticação"
	default: // UTILITY
		return "Utilidade"
	}
}

func normalizeReason(r string) string {
	if r == "" || r == "NONE" {
		return ""
	}
	return r
}
