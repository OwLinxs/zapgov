# ZapGov — atalhos de desenvolvimento LOCAL.
# A infra (Postgres + Redis) sobe em Docker isolada da produção; backend e front
# rodam direto na máquina para iteração rápida. Veja deploy/DEV.md.

DC := docker compose -f docker-compose.dev.yml -p zapgov-dev

.PHONY: dev-up dev-down dev-reset api worker web test tidy

## Sobe Postgres + Redis + aplica migrações (uma vez por sessão de trabalho).
dev-up:
	$(DC) up -d
	@echo "Infra de dev no ar: Postgres :5433 · Redis :6380"

## Derruba a infra (mantém os dados).
dev-down:
	$(DC) down

## Zera o banco de dev (APAGA os dados do volume).
dev-reset:
	$(DC) down -v
	$(DC) up -d
	@echo "Banco de dev recriado e migrado."

## API local (recompila a cada save com go run). Carrega backend/.env.dev.
api:
	cd backend && set -a && . ./.env.dev && set +a && go run ./cmd/server

## Worker da fila (disparos/Asynq).
worker:
	cd backend && set -a && . ./.env.dev && set +a && go run ./cmd/worker

## Front com hot reload em http://localhost:3000.
web:
	cd web && NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev

## Testes do backend.
test:
	cd backend && go test ./...

tidy:
	cd backend && go mod tidy
