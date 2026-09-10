.PHONY: build run test test-integration test-order test-inventory lint db-init db-reset

build:
	go build -o bin/order-service ./cmd/order-service
	go build -o bin/inventory-service ./cmd/inventory-service

run:
	docker compose up -d
	go run ./cmd/order-service & \
	go run ./cmd/inventory-service

test:
	go test -v ./tests/unit/...

test-integration:
	go test -v -tags=integration ./tests/integration/...

test-order:
	go test -v -tags=integration ./tests/integration/order/...

test-inventory:
	go test -v -tags=integration ./tests/integration/inventory/...

test-http:
	go test -v -tags=integration ./tests/integration/http/...

lint:
	golangci-lint run

# Database
db-init:
	docker compose up -d postgres
	sleep 3
	docker compose exec postgres psql -U postgres -d order_inventory -f /docker-entrypoint-initdb.d/init.sql

db-reset:
	docker compose down -v
	docker compose up -d postgres
	sleep 3
	docker compose exec postgres psql -U postgres -d order_inventory -f /docker-entrypoint-initdb.d/init.sql

# Kubernetes
deploy-traefik:
	kubectl apply -f deployments/traefik/

delete-traefik:
	kubectl delete -f deployments/traefik/