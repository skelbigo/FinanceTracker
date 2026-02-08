APP=financetracker
DOCKER_COMPOSE?=docker compose

.PHONY: up up-cache down logs test migrate migrate-down migrate-status fmt lint tidy worker notifications-grpc

up:
	$(DOCKER_COMPOSE) up --build

up-cache:
	$(DOCKER_COMPOSE) --profile cache up --build

down:
	$(DOCKER_COMPOSE) down -v

logs:
	$(DOCKER_COMPOSE) logs -f

migrate:
	go run ./apps/gateway-http -mode=migrate -cmd=up

migrate-down:
	go run ./apps/gateway-http -mode=migrate -cmd=down

migrate-status:
	go run ./apps/gateway-http -mode=migrate -cmd=status

test:
	go test ./...

worker:
	go run ./apps/notification-service/worker

notifications-grpc:
	go run ./apps/notification-service

fmt:
	go fmt ./...

tidy:
	go mod tidy

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint is not installed. Install: https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run
