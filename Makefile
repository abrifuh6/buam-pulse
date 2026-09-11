include .env
export

.PHONY: doctor stripe-setup dev-up dev-down dev-logs dev-ps dev-restart up down migrate-up migrate-down run-api run-scheduler run-worker test lint docker-build

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
	METRICS_PORT=9091 go run ./apps/scheduler/cmd/scheduler

run-worker:
	go run ./apps/worker/cmd/worker

test:
	go test ./... -race -cover

lint:
	golangci-lint run ./...


IMAGE_PREFIX ?= pulse
TAG ?= dev
SERVICES = api scheduler worker migrate notifier retention
ALL_IMAGES = api scheduler worker migrate notifier retention web status

docker-build:  ## build all service images
	@for s in $(SERVICES); do \
	  docker build --build-arg SERVICE=$$s -t $(IMAGE_PREFIX)-$$s:$(TAG) . || exit 1; \
	done
	docker images | grep '^$(IMAGE_PREFIX)-'


KIND_CLUSTER ?= desktop

kind-load:     ## push local images into the Docker Desktop cluster nodes
	@for s in $(ALL_IMAGES); do kind load docker-image $(IMAGE_PREFIX)-$$s:$(TAG) --name $(KIND_CLUSTER); done


NS ?= pulse-dev

deploy-dev:    ## build, load, and helm install/upgrade into the dev cluster
	$(MAKE) docker-build docker-build-web kind-load
	helm upgrade --install pulse deploy/helm/pulse -n $(NS) --create-namespace \
	  -f deploy/helm/pulse/values-dev.yaml --wait --timeout 3m
	kubectl get pods -n $(NS)

undeploy-dev:
	helm uninstall pulse -n $(NS)


WEB_APPS = web status

docker-build-web:  ## build the two frontend images
	@for a in $(WEB_APPS); do \
	  docker build -f Dockerfile.web --build-arg APP=$$a -t $(IMAGE_PREFIX)-$$a:$(TAG) . || exit 1; \
	done
ingress-forward:  ## expose the dev ingress on localhost:80 (needs sudo; runs in foreground)
	sudo kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 80:80

run-notifier:
	METRICS_PORT=9092 go run ./apps/notifier/cmd/notifier

stripe-setup:  ## create Stripe products/prices and store their IDs
	go run ./apps/stripe-setup/cmd/stripe-setup

run-retention:
	METRICS_PORT=9093 go run ./apps/retention/cmd/retention

COMPOSE_DEV = docker compose -f docker-compose.yml -f docker-compose.dev.yml

dev-up:        ## start the whole stack (postgres, redis, mailpit + all Go services)
	$(COMPOSE_DEV) up -d --build
	@echo
	@$(COMPOSE_DEV) ps
	@echo
	@echo "API        http://localhost:8080"
	@echo "Mail       http://localhost:8025"
	@echo "Dashboard  cd apps/web && npm run dev     → http://localhost:5173"
	@echo "Status     cd apps/status && npm run dev  → http://localhost:5174"

dev-down:      ## stop everything
	$(COMPOSE_DEV) down

dev-logs:      ## tail all service logs (make dev-logs S=worker for one)
	$(COMPOSE_DEV) logs -f $(S)

dev-ps:
	$(COMPOSE_DEV) ps

dev-restart:   ## restart one service, e.g. make dev-restart S=worker
	$(COMPOSE_DEV) restart $(S)
