package gateway

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client fala com o SIGE: envia eventos (webhooks assinados) e lista chamados.
type Client struct {
	baseURL    string
	apiKey     string
	hmacSecret string
	http       *http.Client
}

func NewClient(baseURL, apiKey, hmacSecret string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		hmacSecret: hmacSecret,
		http:       &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
	}
}

// Janela é a fotografia da janela de 24h no momento do evento.
type Janela struct {
	Aberta   bool    `json:"aberta"`
	ExpiraEm *string `json:"expira_em"`
}

// NovaJanela monta o payload de janela a partir do prazo de expiração.
func NovaJanela(expira *time.Time) Janela {
	if expira == nil {
		return Janela{Aberta: false, ExpiraEm: nil}
	}
	iso := expira.UTC().Format(time.RFC3339)
	return Janela{Aberta: expira.After(time.Now()), ExpiraEm: &iso}
}

// Chamado é um item da lista do SIGE (GET /v1/chamados).
type Chamado struct {
	ID           int64  `json:"id"`
	Numero       string `json:"numero"`
	Resumo       string `json:"resumo"`
	Status       string `json:"status"`
	Fechado      bool   `json:"fechado"`
	AtualizadoEm string `json:"atualizado_em"`
}

// AbrirResp é a resposta do evento chamado.abrir.
type AbrirResp struct {
	TicketID int64  `json:"ticket_id"`
	Numero   string `json:"numero"`
}

// AbrirChamado envia o evento chamado.abrir e devolve o chamado criado.
func (c *Client) AbrirChamado(ctx context.Context, telefone, nome, referencia string, dados map[string]string, janela Janela) (*AbrirResp, error) {
	body := map[string]any{
		"evento":     "chamado.abrir",
		"telefone":   telefone,
		"nome":       nome,
		"referencia": referencia,
		"dados":      dados,
		"janela_24h": janela,
	}
	var out AbrirResp
	if err := c.postEvento(ctx, body, &out); err != nil {
		return nil, err
	}
	if out.TicketID <= 0 {
		return nil, fmt.Errorf("sige: resposta sem ticket_id")
	}
	return &out, nil
}

// MensagemRecebida repassa ao SIGE uma mensagem da pessoa (modo relay).
func (c *Client) MensagemRecebida(ctx context.Context, ev map[string]any) error {
	ev["evento"] = "mensagem.recebida"
	return c.postEvento(ctx, ev, nil)
}

// Status repassa a mudança de status de uma mensagem enviada.
func (c *Client) Status(ctx context.Context, waMessageID, status, erro string) error {
	return c.postEvento(ctx, map[string]any{
		"evento":        "mensagem.status",
		"wa_message_id": waMessageID,
		"status":        status,
		"erro":          erro,
	}, nil)
}

// ListarChamados lê os chamados de um telefone (GET /v1/chamados).
func (c *Client) ListarChamados(ctx context.Context, telefone string, limite int) ([]Chamado, error) {
	if limite <= 0 {
		limite = 10
	}
	u := fmt.Sprintf("%s/v1/chamados?telefone=%s&limite=%d", c.baseURL, url.QueryEscape(telefone), limite)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("sige: listar chamados HTTP %d", resp.StatusCode)
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		Chamados []Chamado `json:"chamados"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("sige: resposta inválida da lista: %w", err)
	}
	return out.Chamados, nil
}

// postEvento faz o POST assinado em /integracao/zapgov/eventos.
func (c *Client) postEvento(ctx context.Context, body map[string]any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/integracao/zapgov/eventos", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("X-Signature", c.assinar(raw))
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sige: evento HTTP %d", resp.StatusCode)
	}
	if out != nil {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if len(data) > 0 {
			if err := json.Unmarshal(data, out); err != nil {
				return fmt.Errorf("sige: resposta inválida: %w", err)
			}
		}
	}
	return nil
}

// assinar produz X-Signature = sha256=<hex(HMAC_SHA256(secret, corpo))>.
func (c *Client) assinar(body []byte) string {
	mac := hmac.New(sha256.New, []byte(c.hmacSecret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// itoa é usado pelos handlers para chaves de botão.
func itoa(n int64) string { return strconv.FormatInt(n, 10) }
