// Package whatsapp implementa o MessageProvider sobre a Meta Cloud API.
package whatsapp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"zapgov/backend/internal/messaging"
)

// Provider fala com a Meta Cloud API. Implementa messaging.MessageProvider.
type Provider struct {
	phoneNumberID string
	token         string
	appSecret     string
	apiVersion    string
	http          *http.Client
}

func New(phoneNumberID, token, appSecret, apiVersion string) *Provider {
	return &Provider{
		phoneNumberID: phoneNumberID,
		token:         token,
		appSecret:     appSecret,
		apiVersion:    apiVersion,
		http:          &http.Client{Timeout: 15 * time.Second},
	}
}

// ---------- Envio ----------

type sendPayload struct {
	MessagingProduct string      `json:"messaging_product"`
	To               string      `json:"to"`
	Type             string      `json:"type"`
	Text             textPayload `json:"text"`
	// Context cita uma mensagem anterior (responder/quote).
	Context *msgContext `json:"context,omitempty"`
}

type msgContext struct {
	MessageID string `json:"message_id"`
}

type textPayload struct {
	Body string `json:"body"`
}

type sendResponse struct {
	Messages []struct {
		ID string `json:"id"`
	} `json:"messages"`
	Error *metaError `json:"error"`
}

type metaError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// Send dispara uma mensagem de texto livre (válida dentro da janela de 24h).
func (p *Provider) Send(ctx context.Context, msg messaging.OutgoingMessage) (messaging.SendResult, error) {
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", p.apiVersion, p.phoneNumberID)

	payload := sendPayload{
		MessagingProduct: "whatsapp",
		To:               normalizePhone(msg.To),
		Type:             "text",
		Text:             textPayload{Body: msg.Body},
	}
	if msg.ReplyTo != "" {
		payload.Context = &msgContext{MessageID: msg.ReplyTo}
	}
	return p.postMessage(ctx, url, payload)
}

// postMessage serializa o payload, chama a Meta API e extrai o message id.
// Compartilhado por Send (texto) e SendTemplate.
func (p *Provider) postMessage(ctx context.Context, url string, payload any) (messaging.SendResult, error) {
	if p.phoneNumberID == "" || p.token == "" {
		return messaging.SendResult{}, messaging.ErrProviderUnavailable
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return messaging.SendResult{}, fmt.Errorf("marshal do payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return messaging.SendResult{}, fmt.Errorf("montar request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		return messaging.SendResult{}, fmt.Errorf("chamar Meta API: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var sr sendResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return messaging.SendResult{}, fmt.Errorf("decode da resposta (%d): %s", resp.StatusCode, string(body))
	}
	if resp.StatusCode >= 300 || sr.Error != nil {
		m := "erro desconhecido"
		if sr.Error != nil {
			m = sr.Error.Message
		}
		return messaging.SendResult{}, &messaging.ProviderError{Status: resp.StatusCode, Message: m}
	}
	if len(sr.Messages) == 0 {
		return messaging.SendResult{}, fmt.Errorf("resposta sem message id: %s", string(body))
	}
	return messaging.SendResult{WAMessageID: sr.Messages[0].ID}, nil
}

// ---------- Envio de template ----------

type templateSendPayload struct {
	MessagingProduct string       `json:"messaging_product"`
	To               string       `json:"to"`
	Type             string       `json:"type"`
	Template         templateBody `json:"template"`
}

type templateBody struct {
	Name       string         `json:"name"`
	Language   templateLang   `json:"language"`
	Components []templateComp `json:"components,omitempty"`
}

type templateLang struct {
	Code string `json:"code"`
}

type templateComp struct {
	Type       string          `json:"type"`
	Parameters []templateParam `json:"parameters"`
}

type templateParam struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SendTemplate dispara uma mensagem de template aprovado (inicia conversa).
func (p *Provider) SendTemplate(ctx context.Context, msg messaging.TemplateMessage) (messaging.SendResult, error) {
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", p.apiVersion, p.phoneNumberID)

	lang := msg.LanguageCode
	if lang == "" {
		lang = "pt_BR"
	}

	payload := templateSendPayload{
		MessagingProduct: "whatsapp",
		To:               normalizePhone(msg.To),
		Type:             "template",
		Template: templateBody{
			Name:     msg.Name,
			Language: templateLang{Code: lang},
		},
	}
	// Variáveis do corpo viram um componente "body" com parâmetros de texto.
	if len(msg.BodyParams) > 0 {
		params := make([]templateParam, len(msg.BodyParams))
		for i, v := range msg.BodyParams {
			params[i] = templateParam{Type: "text", Text: v}
		}
		payload.Template.Components = []templateComp{{Type: "body", Parameters: params}}
	}

	return p.postMessage(ctx, url, payload)
}

// ---------- Menu interativo (botões / lista) ----------

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// SendMenu envia o menu como botões (até 3) ou lista (até 10) interativos.
func (p *Provider) SendMenu(ctx context.Context, to, text string, options []messaging.MenuOption) (messaging.SendResult, error) {
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", p.apiVersion, p.phoneNumberID)
	if len(options) > 10 {
		options = options[:10]
	}

	var interactive map[string]any
	if len(options) <= 3 {
		buttons := make([]map[string]any, 0, len(options))
		for _, o := range options {
			buttons = append(buttons, map[string]any{
				"type":  "reply",
				"reply": map[string]any{"id": o.Key, "title": clip(o.Label, 20)},
			})
		}
		interactive = map[string]any{
			"type":   "button",
			"body":   map[string]any{"text": clip(text, 1024)},
			"action": map[string]any{"buttons": buttons},
		}
	} else {
		rows := make([]map[string]any, 0, len(options))
		for _, o := range options {
			rows = append(rows, map[string]any{"id": o.Key, "title": clip(o.Label, 24)})
		}
		interactive = map[string]any{
			"type": "list",
			"body": map[string]any{"text": clip(text, 1024)},
			"action": map[string]any{
				"button":   "Ver opções",
				"sections": []map[string]any{{"title": "Opções", "rows": rows}},
			},
		}
	}

	payload := map[string]any{
		"messaging_product": "whatsapp",
		"to":                normalizePhone(to),
		"type":              "interactive",
		"interactive":       interactive,
	}
	return p.postMessage(ctx, url, payload)
}

// ---------- Webhook ----------

// Estrutura do payload de webhook da Meta (apenas os campos usados).
type webhookEnvelope struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Contacts []struct {
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
					WaID string `json:"wa_id"`
				} `json:"contacts"`
				Messages []struct {
					From      string `json:"from"`
					ID        string `json:"id"`
					Timestamp string `json:"timestamp"`
					Type      string `json:"type"`
					Context   *struct {
						ID string `json:"id"` // wa_message_id citado (reply do cidadão)
					} `json:"context"`
					Text struct {
						Body string `json:"body"`
					} `json:"text"`
					Image    *mediaObj `json:"image"`
					Audio    *mediaObj `json:"audio"`
					Video    *mediaObj `json:"video"`
					Document *mediaObj `json:"document"`
					Sticker  *mediaObj `json:"sticker"`
					Button   *struct {
						Text    string `json:"text"`
						Payload string `json:"payload"`
					} `json:"button"`
					Interactive *struct {
						Type        string `json:"type"`
						ButtonReply *struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						} `json:"button_reply"`
						ListReply *struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						} `json:"list_reply"`
					} `json:"interactive"`
					Reaction *struct {
						MessageID string `json:"message_id"`
						Emoji     string `json:"emoji"`
					} `json:"reaction"`
				} `json:"messages"`
				Statuses []struct {
					ID          string `json:"id"`
					Status      string `json:"status"`
					Timestamp   string `json:"timestamp"`
					RecipientID string `json:"recipient_id"`
					Errors      []struct {
						Code  int    `json:"code"`
						Title string `json:"title"`
					} `json:"errors"`
				} `json:"statuses"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

// mediaObj é a forma comum dos objetos de mídia no webhook.
type mediaObj struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	Caption  string `json:"caption"`
	Filename string `json:"filename"`
}

// ParseWebhook separa mensagens recebidas, atualizações de status e reações.
func (p *Provider) ParseWebhook(body []byte) ([]messaging.IncomingMessage, []messaging.StatusUpdate, []messaging.ReactionUpdate, error) {
	var env webhookEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, nil, nil, fmt.Errorf("decode do webhook: %w", err)
	}

	var incoming []messaging.IncomingMessage
	var statuses []messaging.StatusUpdate
	var reactions []messaging.ReactionUpdate

	for _, entry := range env.Entry {
		for _, change := range entry.Changes {
			v := change.Value

			// Nome do contato (vem na lista contacts, casado por wa_id == from).
			nameByWaID := map[string]string{}
			for _, c := range v.Contacts {
				nameByWaID[c.WaID] = c.Profile.Name
			}

			for _, m := range v.Messages {
				im := messaging.IncomingMessage{
					WAMessageID: m.ID,
					From:        m.From,
					Name:        nameByWaID[m.From],
					Timestamp:   parseUnix(m.Timestamp),
				}
				if m.Context != nil {
					im.ReplyTo = m.Context.ID
				}
				switch m.Type {
				case "text":
					im.Body = m.Text.Body
				case "image":
					fillMedia(&im, "image", m.Image)
				case "audio":
					fillMedia(&im, "audio", m.Audio)
				case "video":
					fillMedia(&im, "video", m.Video)
				case "document":
					fillMedia(&im, "document", m.Document)
				case "sticker":
					fillMedia(&im, "sticker", m.Sticker)
				case "interactive":
					// toque em botão/lista interativos: o título vira o "texto"
					// e o ID fica disponível (pesquisa de satisfação usa).
					if m.Interactive != nil {
						if m.Interactive.ButtonReply != nil {
							im.Body = m.Interactive.ButtonReply.Title
							im.ButtonID = m.Interactive.ButtonReply.ID
						} else if m.Interactive.ListReply != nil {
							im.Body = m.Interactive.ListReply.Title
							im.ButtonID = m.Interactive.ListReply.ID
						}
					}
					if im.Body == "" {
						continue
					}
				case "button":
					// toque em botão de quick-reply de TEMPLATE (texto do botão).
					if m.Button != nil {
						im.Body = m.Button.Text
						im.ButtonID = m.Button.Payload
						if im.Body == "" {
							im.Body = m.Button.Payload
						}
					}
					if im.Body == "" {
						continue
					}
				case "reaction":
					// reação não vira mensagem — atualiza a mensagem alvo.
					if m.Reaction != nil {
						reactions = append(reactions, messaging.ReactionUpdate{
							TargetWAMessageID: m.Reaction.MessageID,
							Emoji:             m.Reaction.Emoji,
							From:              m.From,
						})
					}
					continue
				default:
					continue // tipos não suportados (location, contacts…)
				}
				incoming = append(incoming, im)
			}

			for _, s := range v.Statuses {
				su := messaging.StatusUpdate{
					WAMessageID: s.ID,
					Status:      s.Status,
					Timestamp:   parseUnix(s.Timestamp),
					RecipientID: s.RecipientID,
				}
				if len(s.Errors) > 0 {
					su.ErrorCode = s.Errors[0].Code
					su.ErrorTitle = s.Errors[0].Title
				}
				statuses = append(statuses, su)
			}
		}
	}
	return incoming, statuses, reactions, nil
}

func fillMedia(im *messaging.IncomingMessage, kind string, mo *mediaObj) {
	if mo == nil {
		return
	}
	im.MediaType = kind
	im.MediaID = mo.ID
	im.MediaMime = mo.MimeType
	im.MediaName = mo.Filename
	im.Body = mo.Caption
}

// ---------- Envio de mídia por link ----------

// SendMedia envia imagem/documento/áudio/vídeo a partir de uma URL pública.
// SendReaction reage com emoji a uma mensagem (emoji vazio remove a reação).
func (p *Provider) SendReaction(ctx context.Context, to, targetWAMessageID, emoji string) error {
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", p.apiVersion, p.phoneNumberID)
	payload := map[string]any{
		"messaging_product": "whatsapp",
		"to":                normalizePhone(to),
		"type":              "reaction",
		"reaction": map[string]any{
			"message_id": targetWAMessageID,
			"emoji":      emoji,
		},
	}
	_, err := p.postMessage(ctx, url, payload)
	return err
}

// UploadMedia sobe um arquivo pra Meta e devolve o media_id (válido p/ envio).
func (p *Provider) UploadMedia(ctx context.Context, data []byte, filename, mime string) (string, error) {
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/media", p.apiVersion, p.phoneNumberID)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("messaging_product", "whatsapp")
	if mime != "" {
		_ = w.WriteField("type", mime)
	}
	// part do arquivo com Content-Type correto.
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	if mime != "" {
		h.Set("Content-Type", mime)
	}
	part, err := w.CreatePart(h)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := p.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload mídia: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("Meta recusou upload (%d): %s", resp.StatusCode, string(body))
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.ID == "" {
		return "", fmt.Errorf("resposta de upload inesperada: %s", string(body))
	}
	return out.ID, nil
}

func (p *Provider) SendMedia(ctx context.Context, msg messaging.OutgoingMedia) (messaging.SendResult, error) {
	url := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", p.apiVersion, p.phoneNumberID)

	media := map[string]any{}
	if msg.ID != "" {
		media["id"] = msg.ID // upload prévio (arquivo)
	} else {
		media["link"] = msg.Link
	}
	if msg.Caption != "" && (msg.Kind == "image" || msg.Kind == "video" || msg.Kind == "document") {
		media["caption"] = msg.Caption
	}
	if msg.Kind == "document" && msg.Name != "" {
		media["filename"] = msg.Name
	}
	payload := map[string]any{
		"messaging_product": "whatsapp",
		"to":                normalizePhone(msg.To),
		"type":              msg.Kind,
		msg.Kind:            media,
	}
	return p.postMessage(ctx, url, payload)
}

// ---------- Download de mídia (proxy) ----------

// DownloadMedia resolve a URL temporária do media id e baixa os bytes (com token).
func (p *Provider) DownloadMedia(ctx context.Context, mediaID string) ([]byte, string, error) {
	metaURL := fmt.Sprintf("https://graph.facebook.com/%s/%s", p.apiVersion, mediaID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	resp, err := p.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	var meta struct {
		URL      string `json:"url"`
		MimeType string `json:"mime_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, "", err
	}
	if meta.URL == "" {
		return nil, "", fmt.Errorf("media %s sem URL (token/permissão?)", mediaID)
	}

	req2, err := http.NewRequestWithContext(ctx, http.MethodGet, meta.URL, nil)
	if err != nil {
		return nil, "", err
	}
	req2.Header.Set("Authorization", "Bearer "+p.token)
	resp2, err := p.http.Do(req2)
	if err != nil {
		return nil, "", err
	}
	defer resp2.Body.Close()

	data, err := io.ReadAll(resp2.Body)
	if err != nil {
		return nil, "", err
	}
	return data, meta.MimeType, nil
}

// VerifySignature valida o header X-Hub-Signature-256 (HMAC-SHA256 do corpo cru).
func (p *Provider) VerifySignature(body []byte, signatureHeader string) bool {
	if p.appSecret == "" || signatureHeader == "" {
		return false
	}
	expected := strings.TrimPrefix(signatureHeader, "sha256=")
	mac := hmac.New(sha256.New, []byte(p.appSecret))
	mac.Write(body)
	computed := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(computed))
}

// ---------- helpers ----------

// normalizePhone limpa o telefone e corrige o "nono dígito" brasileiro.
// O wa_id que a Meta entrega no webhook para números BR costuma vir SEM o 9 do
// celular (ex: 554691393338). Enviar de volta nesse formato falha/erra o destino,
// então reinserimos o 9 antes do envio.
func normalizePhone(s string) string {
	d := digitsOnly(s)

	// BR celular sem DDI (DDD + 9...): prefixa 55 (mesma regra do CanonPhone).
	// Restrito ao padrão de celular BR p/ não afetar números internacionais.
	if !strings.HasPrefix(d, "55") && ((len(d) == 11 && d[2] == '9') || (len(d) == 10 && d[2] >= '6' && d[2] <= '9')) {
		d = "55" + d
	}

	// Brasil: 55 + DDD(2) + assinante. Celular = assinante de 9 dígitos iniciando em 9.
	// Quando vem com 8 dígitos (total 12), falta o nono dígito → reinsere.
	if strings.HasPrefix(d, "55") {
		rest := d[2:]
		if len(rest) == 10 { // DDD(2) + 8 dígitos
			ddd, sub := rest[:2], rest[2:]
			if sub[0] >= '6' && sub[0] <= '9' { // faixa de celular
				return "55" + ddd + "9" + sub
			}
		}
	}
	return d
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func parseUnix(s string) time.Time {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Now().UTC()
	}
	return time.Unix(n, 0).UTC()
}
