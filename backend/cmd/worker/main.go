// Worker da fila Asynq: processa os disparos em massa fora do request HTTP,
// com vazão controlada (rate limit da Meta) e retry em falhas transitórias.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"

	"github.com/hibiken/asynq"
	"golang.org/x/time/rate"

	"zapgov/backend/internal/campaigns"
	"zapgov/backend/internal/config"
	"zapgov/backend/internal/database"
	"zapgov/backend/internal/messaging"
	"zapgov/backend/internal/messaging/whatsapp"
	"zapgov/backend/internal/obs"
	"zapgov/backend/internal/queue"
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
	msgSvc.ProviderResolver = tenants.NewRepository(pool).ProviderResolver(provider)
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
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return asynq.SkipRetry // payload inválido não adianta repetir
		}

		// Isolamento: injeta o tenant do job no context (repos + provider).
		if p.TenantID > 0 {
			ctx = tenantctx.With(ctx, p.TenantID)
		}

		// Respeita a vazão antes de chamar a Meta.
		if err := limiter.Wait(ctx); err != nil {
			return err
		}

		_, err := svc.SendTemplate(ctx, p.To, p.Template, p.Language, p.Params, p.Preview)
		if err == nil {
			_ = repo.IncEnviadas(ctx, p.CampaignID)
			return nil
		}

		// LGPD: descadastrado → pula sem contar como falha nem reenviar.
		if errors.Is(err, messaging.ErrOptedOut) {
			log.Printf("campanha %d: %s está em opt-out, pulado", p.CampaignID, p.To)
			return asynq.SkipRetry
		}

		// Recusa da Meta (4xx) é permanente: conta falha e não tenta de novo.
		if isPermanent(err) {
			_ = repo.IncFalhas(ctx, p.CampaignID)
			log.Printf("campanha %d: falha permanente para %s: %v", p.CampaignID, p.To, err)
			return asynq.SkipRetry
		}

		// Erro transitório (5xx/rede): deixa o Asynq repetir com backoff.
		// Se esgotar as tentativas, contabiliza falha.
		if retried, max := retryInfo(ctx); retried >= max {
			_ = repo.IncFalhas(ctx, p.CampaignID)
		}
		return err
	}
}

func isPermanent(err error) bool {
	return strings.Contains(err.Error(), "recusou (4")
}

func retryInfo(ctx context.Context) (int, int) {
	retried, _ := asynq.GetRetryCount(ctx)
	max, _ := asynq.GetMaxRetry(ctx)
	return retried, max
}
