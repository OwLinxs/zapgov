// Worker da fila Asynq: processa os disparos em massa fora do request HTTP,
// com vazão controlada (rate limit da Meta) e retry em falhas transitórias.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/hibiken/asynq"
	"golang.org/x/time/rate"

	"zapgov/backend/internal/campaigns"
	"zapgov/backend/internal/config"
	"zapgov/backend/internal/database"
	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/messaging/whatsapp"
	"zapgov/backend/internal/obs"
	"zapgov/backend/internal/queue"
	"zapgov/backend/internal/realtime"
	"zapgov/backend/internal/tenantctx"
	"zapgov/backend/internal/tenants"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	defer obs.Setup(cfg.LogLevel, cfg.SentryDSN, cfg.Env)()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	provider := whatsapp.New(cfg.WhatsAppPhoneNumberID, cfg.WhatsAppToken, cfg.WhatsAppAppSecret, cfg.WhatsAppAPIVersion)
	msgSvc := messaging.NewService(messaging.NewRepository(pool), provider)
	// Envio multi-tenant na fila: usa as credenciais do tenant do job.
	tenantRepo := tenants.NewRepository(pool)
	if err := tenantRepo.SealExisting(ctx); err != nil {
		log.Fatalf("credenciais: %v", err)
	}
	msgSvc.ProviderResolver = tenantRepo.ProviderResolver(provider)
	hub := realtime.NewHub()
	defer hub.ConnectRedis(ctx, cfg.RedisAddr)()
	msgSvc.OnEvent = func(tid, cid int64, kind, name, preview string) {
		hub.Publish(realtime.Event{TenantID: tid, ConversationID: cid, Type: kind, Name: name, Preview: preview})
	}
	campRepo := campaigns.NewRepository(pool)

	// Vazão: a Meta aceita ~80 msg/s. Ficamos folgados em 40/s.
	limiter := rate.NewLimiter(rate.Limit(40), 40)

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr},
		asynq.Config{
			Concurrency: 20,
			Queues:      map[string]int{queue.QueueCampaigns: 10},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TypeCampaignSend, handleCampaignSend(msgSvc, campRepo, limiter))

	log.Printf("ZapGov worker iniciado (redis %s)", cfg.RedisAddr)
	if err := srv.Run(mux); err != nil {
		log.Fatalf("worker: %v", err)
	}
}

func handleCampaignSend(svc *messaging.Service, repo *campaigns.Repository, limiter *rate.Limiter) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var p queue.CampaignSendPayload
		if json.Unmarshal(t.Payload(), &p) != nil || p.TenantID <= 0 || p.RecipientID <= 0 {
			log.Printf("worker: tarefa sem identidade de destinatário; conferir campanha legada")
			return asynq.SkipRetry
		}
		ctx = tenantctx.With(ctx, p.TenantID)
		trusted, err := repo.DeliveryPayload(ctx, p.RecipientID, p.CampaignID)
		if err != nil {
			return err
		}
		p = trusted
		if err := limiter.Wait(ctx); err != nil {
			return err
		}
		claimed, err := repo.ClaimDelivery(ctx, p.RecipientID, p.CampaignID)
		if err != nil {
			return err
		}
		if !claimed {
			return nil
		}
		msg, sendErr := svc.SendTemplate(ctx, p.To, p.Template, p.Language, p.Params, p.Preview)
		state := deliveryState(sendErr)
		if state == "retry" {
			retried, max := retryInfo(ctx)
			if retried < max {
				if err := repo.RetryDelivery(ctx, p.RecipientID); err != nil {
					return err
				}
				return sendErr
			}
			state = "failed"
		}
		var waID, detail string
		var msgID int64
		if msg != nil {
			waID = msg.WAMessageID
			msgID = msg.ID
		}
		if sendErr != nil {
			detail = sendErr.Error()
		}
		if err := repo.FinishDelivery(ctx, p.RecipientID, p.CampaignID, state, waID, msgID, detail); err != nil {
			return err
		}
		return nil
	}
}

func deliveryState(err error) string {
	if err == nil {
		return "sent"
	}
	if errors.Is(err, messaging.ErrOptedOut) {
		return "skipped"
	}
	if errors.Is(err, messaging.ErrProviderUnavailable) {
		return "failed"
	}
	var refusal *messaging.ProviderError
	if errors.As(err, &refusal) {
		if refusal.Status == 429 {
			return "retry"
		}
		if refusal.Status >= 400 && refusal.Status < 500 {
			return "failed"
		}
	}
	// Network failures, provider 5xx and persistence failures may follow acceptance.
	return "uncertain"
}
func retryInfo(ctx context.Context) (int, int) {
	retried, _ := asynq.GetRetryCount(ctx)
	max, _ := asynq.GetMaxRetry(ctx)
	return retried, max
}
