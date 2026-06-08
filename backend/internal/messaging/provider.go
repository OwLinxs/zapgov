package messaging

import (
	"context"
	"time"
)

// OutgoingMessage é uma mensagem de texto livre (válida só na janela de 24h).
type OutgoingMessage struct {
	To   string // telefone E.164 sem '+', ex: "5516998123344"
	Body string
}

// TemplateMessage inicia uma conversa com um template aprovado pela Meta
// (obrigatório fora da janela de 24h e em disparos em massa).
type TemplateMessage struct {
	To           string
	Name         string   // nome do template aprovado, ex: "hello_world"
	LanguageCode string   // ex: "pt_BR", "en_US"
	BodyParams   []string // valores das variáveis {{1}}, {{2}}… do corpo
}

// SendResult é o retorno do provider após o envio aceito.
type SendResult struct {
	WAMessageID string // wamid... atribuído pela Meta
}

// IncomingMessage é uma mensagem recebida (inbound) extraída do webhook.
type IncomingMessage struct {
	WAMessageID string
	From        string
	Name        string
	Body        string // texto ou legenda da mídia
	Timestamp   time.Time

	// Mídia (vazio quando é só texto).
	MediaType string // image|audio|video|document|sticker
	MediaID   string
	MediaMime string
	MediaName string
}

// MenuOption é uma opção do menu interativo (vira botão ou item de lista).
type MenuOption struct {
	Key   string // id que volta no webhook ao tocar (casa com a aresta do fluxo)
	Label string
}

// OutgoingMedia é uma mídia a enviar por link (URL pública).
type OutgoingMedia struct {
	To      string
	Kind    string // image|document|audio|video
	Link    string
	Caption string
	Name    string // nome do arquivo (document)
}

// StatusUpdate é uma atualização de status de uma mensagem já enviada.
type StatusUpdate struct {
	WAMessageID string
	Status      string // sent | delivered | read | failed
	Timestamp   time.Time
}

// MessageProvider abstrai o canal de envio/recebimento.
// A implementação WhatsApp vive em internal/messaging/whatsapp/.
// Manter a interface pequena permite plugar SMS/Email depois.
type MessageProvider interface {
	Send(ctx context.Context, msg OutgoingMessage) (SendResult, error)
	SendTemplate(ctx context.Context, msg TemplateMessage) (SendResult, error)
	SendMenu(ctx context.Context, to, text string, options []MenuOption) (SendResult, error)
	SendMedia(ctx context.Context, msg OutgoingMedia) (SendResult, error)
	DownloadMedia(ctx context.Context, mediaID string) (data []byte, mime string, err error)
	ParseWebhook(body []byte) ([]IncomingMessage, []StatusUpdate, error)
	VerifySignature(body []byte, signatureHeader string) bool
}
