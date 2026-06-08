// Package config carrega a configuração da aplicação a partir do ambiente.
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string

	// Credenciais da Meta Cloud API.
	// Nesta fatia inicial vivem no .env (1 tenant). Quando o multi-tenant entrar,
	// migram para o banco (criptografadas) e são carregadas por request.
	WhatsAppPhoneNumberID string
	WhatsAppWABAID        string
	WhatsAppToken         string
	WhatsAppAppSecret     string
	WhatsAppAPIVersion    string

	// Token arbitrário para o handshake de verificação do webhook (Meta GET).
	WebhookVerifyToken string

	// Segredo para assinar os JWT de sessão.
	JWTSecret string

	// Redis (Asynq) para a fila de disparos.
	RedisAddr string

	// Observabilidade.
	LogLevel  string // debug | info | warn | error
	SentryDSN string // vazio = Sentry desligado
	Env       string // dev | staging | production
}

// Load lê variáveis de ambiente. O .env é opcional (em produção usa-se o ambiente real).
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:                  getEnv("PORT", "8080"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		WhatsAppPhoneNumberID: os.Getenv("WHATSAPP_PHONE_NUMBER_ID"),
		WhatsAppWABAID:        os.Getenv("WHATSAPP_WABA_ID"),
		WhatsAppToken:         os.Getenv("WHATSAPP_TOKEN"),
		WhatsAppAppSecret:     os.Getenv("WHATSAPP_APP_SECRET"),
		WhatsAppAPIVersion:    getEnv("WHATSAPP_API_VERSION", "v21.0"),
		WebhookVerifyToken:    os.Getenv("WEBHOOK_VERIFY_TOKEN"),
		JWTSecret:             getEnv("JWT_SECRET", "dev-secret-troque-em-producao"),
		RedisAddr:             getEnv("REDIS_ADDR", "localhost:6379"),
		LogLevel:              getEnv("LOG_LEVEL", "info"),
		SentryDSN:             os.Getenv("SENTRY_DSN"),
		Env:                   getEnv("APP_ENV", "dev"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL é obrigatória")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
