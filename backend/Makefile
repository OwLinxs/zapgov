DB_URL ?= postgres://zapgov:zapgov@localhost:5432/zapgov?sslmode=disable

.PHONY: run build tidy migrate-up migrate-down up down logs

run:        ## roda a API localmente (precisa de Postgres no ar)
	go run ./cmd/server

worker:     ## roda o worker da fila (precisa de Redis no ar)
	go run ./cmd/worker

build:
	go build -o bin/server ./cmd/server
	go build -o bin/worker ./cmd/worker

tidy:
	go mod tidy

test:       ## roda os testes unitários
	go test ./...

vet:
	go vet ./...

migrate-up:  ## aplica migrations (requer golang-migrate instalado localmente)
	migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path migrations -database "$(DB_URL)" down 1

up:         ## sobe tudo via docker-compose (postgres + migrate + app)
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f app
