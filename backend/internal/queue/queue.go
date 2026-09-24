// Package queue enfileira disparos em massa via Asynq (Redis), para que o envio
// seja assíncrono, com retry e controle de vazão (rate limit da Meta).
package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

const TypeCampaignSend = "campaign:send"

// QueueCampaigns é a fila dedicada aos disparos.
const QueueCampaigns = "campaigns"

// CampaignSendPayload é o trabalho de enviar UMA mensagem de uma campanha.
type CampaignSendPayload struct {
	RecipientID int64    `json:"recipient_id"`
	TenantID    int64    `json:"tenant_id"`
	CampaignID  int64    `json:"campaign_id"`
	To          string   `json:"to"`
	Template    string   `json:"template"`
	Language    string   `json:"language"`
	Params      []string `json:"params"`
	Preview     string   `json:"preview"`
}

// Client enfileira tarefas (lado da API).
type Client struct {
	c *asynq.Client
}

func NewClient(redisAddr string) *Client {
	return &Client{c: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})}
}

func (c *Client) Close() error { return c.c.Close() }

// EnqueueCampaignSend agenda o envio de uma mensagem da campanha.
func (c *Client) EnqueueCampaignSend(p CampaignSendPayload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = c.c.Enqueue(
		asynq.NewTask(TypeCampaignSend, body),
		asynq.Queue(QueueCampaigns),
		asynq.MaxRetry(3),
		asynq.TaskID(fmt.Sprintf("campaign-recipient-%d", p.RecipientID)),
		asynq.Retention(24*time.Hour),
	)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}
