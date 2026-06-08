// Package obs centraliza observabilidade: log estruturado (slog), captura de
// erros (Sentry, opcional) e middlewares de log/recover.
package obs

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
)

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Setup configura o logger global (JSON) e o Sentry (se houver DSN).
// Devolve uma função de flush para o defer no main.
func Setup(logLevel, sentryDSN, env string) func() {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(logLevel)})
	slog.SetDefault(slog.New(h))

	if sentryDSN == "" {
		return func() {}
	}
	if err := sentry.Init(sentry.ClientOptions{Dsn: sentryDSN, Environment: env}); err != nil {
		slog.Error("sentry: falha ao iniciar", "err", err)
		return func() {}
	}
	slog.Info("sentry ativo", "env", env)
	return func() { sentry.Flush(2 * time.Second) }
}

func sentryOn() bool { return sentry.CurrentHub().Client() != nil }

// RequestLogger registra cada requisição em JSON (método, rota, status, latência).
func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		st := c.Response().StatusCode()
		lvl := slog.LevelInfo
		if st >= 500 {
			lvl = slog.LevelError
		} else if st >= 400 {
			lvl = slog.LevelWarn
		}
		slog.LogAttrs(context.Background(), lvl, "http",
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", st),
			slog.Int64("ms", time.Since(start).Milliseconds()),
			slog.String("ip", c.IP()),
		)
		return err
	}
}

// Recover captura panics: loga estruturado e reporta ao Sentry (se ativo).
func Recover() fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				if sentryOn() {
					sentry.CurrentHub().Recover(r)
					sentry.Flush(time.Second)
				}
				slog.Error("panic recuperado", "error", fmt.Sprint(r), "path", c.Path())
				err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "erro interno"})
			}
		}()
		return c.Next()
	}
}
