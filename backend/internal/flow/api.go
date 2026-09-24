package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"zapgov/backend/internal/tenantctx"
	"zapgov/backend/internal/tenants"
)

// A chave criptografada só está presente no grafo interno do servidor.
type apiData struct {
	BaseURL      string            `json:"base_url"`
	APIKey       string            `json:"api_key,omitempty"`
	EncryptedKey string            `json:"api_key_encrypted,omitempty"`
	Integration  string            `json:"integration"`
	Body         string            `json:"body"`
	Response     map[string]string `json:"response"` // campo da resposta -> variável
}

var responseVariable = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

var variablePattern = regexp.MustCompile(`\{\{([a-zA-Z0-9_]+)\}\}`)

func renderValue(value string, vars map[string]string) (string, error) {
	missing := false
	out := variablePattern.ReplaceAllStringFunc(value, func(token string) string {
		v, ok := vars[token[2:len(token)-2]]
		if !ok {
			missing = true
		}
		return v
	})
	if missing {
		return "", fmt.Errorf("variável não preenchida")
	}
	return out, nil
}

func apiPayload(d apiData, vars map[string]string) ([]byte, error) {
	var body map[string]string
	if err := json.Unmarshal([]byte(d.Body), &body); err != nil || body == nil {
		return nil, fmt.Errorf("corpo deve ser um objeto JSON com valores de texto")
	}
	for k, v := range body {
		rendered, err := renderValue(v, vars)
		if err != nil {
			return nil, err
		}
		body[k] = rendered
	}
	// Sempre usar a referência persistida desta execução, não um valor digitado.
	body["referencia_externa"] = vars["_referencia_externa"]
	body["origem"] = "whatsapp"
	if body["referencia_externa"] == "" || strings.TrimSpace(body["defeito_relatado"]) == "" {
		return nil, fmt.Errorf("defeito_relatado e referência são obrigatórios")
	}
	if p := body["prioridade"]; p != "" && p != "baixa" && p != "normal" && p != "alta" {
		return nil, fmt.Errorf("prioridade inválida")
	}
	return json.Marshal(body) // escapa aspas/quebras de linha das respostas do cidadão
}

func callAPI(ctx context.Context, raw json.RawMessage, vars map[string]string) string {
	vars["api_http_status"] = "0"
	vars["api_erro"] = ""
	var d apiData
	if json.Unmarshal(raw, &d) != nil || d.Integration != "sige_ti" {
		vars["api_erro"] = "configuracao_invalida"
		return "error"
	}
	for _, target := range d.Response {
		if target != "" && !strings.HasPrefix(target, "_") {
			delete(vars, target)
		}
	}
	base, err := url.Parse(strings.TrimRight(d.BaseURL, "/"))
	key, keyErr := tenants.OpenIntegrationSecret(d.EncryptedKey, credentialBinding(tenantctx.From(ctx), d.BaseURL))
	if tenantctx.From(ctx) <= 0 || keyErr != nil || err != nil || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || (base.Scheme != "https" && base.Scheme != "http") || key == "" {
		vars["api_erro"] = "integracao_nao_configurada"
		return "error"
	}

	payload, err := apiPayload(d, vars)
	if err != nil {
		vars["api_erro"] = "dados_invalidos"
		return "error"
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base.String(), "/")+"/integracao/ordens-servico", bytes.NewReader(payload))
	if err != nil {
		vars["api_erro"] = "configuracao_invalida"
		return "error"
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", key)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		vars["api_erro"] = "resultado_incerto"
		return "error"
	}
	defer resp.Body.Close()
	vars["api_http_status"] = strconv.Itoa(resp.StatusCode)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		switch resp.StatusCode {
		case 401:
			vars["api_erro"] = "chave_invalida"
		case 422:
			vars["api_erro"] = "dados_invalidos"
		case 503:
			vars["api_erro"] = "integracao_indisponivel"
		default:
			vars["api_erro"] = "erro_http"
		}
		return "error"
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	var result map[string]json.RawMessage
	if err != nil || len(data) > 1024*1024 || json.Unmarshal(data, &result) != nil {
		vars["api_erro"] = "resposta_invalida"
		return "error"
	}
	// Nunca anunciar abertura sem o protocolo, id e status exigidos pelo contrato.
	var number, status string
	var id int64
	if json.Unmarshal(result["numero"], &number) != nil || number == "" || json.Unmarshal(result["status"], &status) != nil || status == "" || json.Unmarshal(result["id"], &id) != nil || id <= 0 {
		vars["api_erro"] = "resposta_invalida"
		return "error"
	}
	for field, target := range d.Response {
		if target == "" || strings.HasPrefix(target, "_") || target == "api_erro" || target == "api_http_status" {
			continue
		}
		value := result[field]
		if len(value) == 0 || string(value) == "null" {
			continue
		}
		var text string
		if json.Unmarshal(value, &text) != nil {
			text = string(value)
		}
		vars[target] = text
	}
	return "success"
}
