include .env
export

.PHONY: doctor up down migrate-up migrate-down run-api run-scheduler run-worker test lint

doctor:        ## verify required tools are installed
	@for t in docker go migrate; do \
	  command -v $$t >/dev/null && echo "ok      $$t" || echo "MISSING $$t"; \
	done
	@docker info >/dev/null 2>&1 && echo "ok      docker daemon running" || echo "MISSING docker daemon (start Docker Desktop)"

up:            ## start postgres + redis
	docker compose up -d

down:
	docker compose down

migrate-up:    ## apply migrations (requires: brew install golang-migrate)
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

run-api:
	go run ./apps/api/cmd/api

run-scheduler:
	go run ./apps/scheduler/cmd/scheduler

run-worker:
	go run ./apps/worker/cmd/worker

test:
	go test ./... -race -cover

lint:
	golangci-lint run ./...

