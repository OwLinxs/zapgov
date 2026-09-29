// Package config carrega a configuração da aplicação a partir do ambiente.
package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

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

	// Gateway WhatsApp <-> SIGE (relay bidirecional de chamados). Ver deploy/GATEWAY.md.
	// Vazio/0 = gateway desligado (o tenant segue no motor de fluxo normal).
	GatewayTenantID      int64  // tenant cujo número usa o gateway (T.I.)
	GatewayInboundAPIKey string // X-API-Key que o SIGE usa ao chamar /v1/*
	SigeBaseURL          string // base do SIGE (ZapGov chama /v1/chamados e /integracao/zapgov/eventos)
	SigeAPIKey           string // X-API-Key que o ZapGov envia ao SIGE
	SigeHMACSecret       string // assina os webhooks ZapGov->SIGE (X-Signature)
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
		GatewayTenantID:       int64(getEnvInt("GATEWAY_TENANT_ID", 0)),
		GatewayInboundAPIKey:  os.Getenv("GATEWAY_INBOUND_API_KEY"),
		SigeBaseURL:           os.Getenv("SIGE_BASE_URL"),
		SigeAPIKey:            os.Getenv("SIGE_API_KEY"),
		SigeHMACSecret:        os.Getenv("SIGE_HMAC_SECRET"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL é obrigatória")
	}
	if cfg.Env == "production" && (cfg.JWTSecret == "dev-secret-troque-em-producao" || len(cfg.JWTSecret) < 32) {
		return nil, fmt.Errorf("JWT_SECRET deve ter ao menos 32 bytes e não pode ser o padrão em produção")
	}
	key := os.Getenv("META_CREDENTIALS_KEY")
	if key != "" || cfg.Env == "production" {
		decoded, err := hex.DecodeString(key)
		if err != nil || len(decoded) != 32 {
			return nil, fmt.Errorf("META_CREDENTIALS_KEY deve conter 64 caracteres hexadecimais")
		}
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// GatewayEnabled diz se o gateway WhatsApp<->SIGE está configurado.
func (c *Config) GatewayEnabled() bool {
	return c.GatewayTenantID > 0 && c.GatewayInboundAPIKey != "" &&
		c.SigeBaseURL != "" && c.SigeAPIKey != "" && c.SigeHMACSecret != ""
}
