.PHONY: help build test fmt vet lint migrate-up migrate-down dev-up dev-down contracts-build contracts-test contracts-deploy run

help:
	@echo "eth-exchange-lab targets:"
	@echo "  build              Build API binary"
	@echo "  test               Run unit tests"
	@echo "  fmt                Run gofmt"
	@echo "  vet                Run go vet"
	@echo "  migrate-up         Apply database migrations"
	@echo "  dev-up             Start Docker infrastructure"
	@echo "  dev-down           Stop Docker infrastructure"
	@echo "  contracts-build    Build Solidity contracts"
	@echo "  contracts-test     Run Foundry tests"
	@echo "  contracts-deploy   Deploy contracts to local Anvil"
	@echo "  run                Run API server"

build:
	go build -o bin/api ./cmd/api

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

migrate-up:
	migrate -path migrations -database "$${POSTGRES_DSN}" up

migrate-down:
	migrate -path migrations -database "$${POSTGRES_DSN}" down 1

dev-up:
	./scripts/dev-up.sh

dev-down:
	./scripts/dev-down.sh

contracts-build:
	./scripts/contracts-build.sh

contracts-test:
	./scripts/contracts-test.sh

contracts-deploy:
	./scripts/contracts-deploy-local.sh

run:
	./scripts/backend-run.sh
