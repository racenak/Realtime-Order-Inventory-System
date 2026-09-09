.PHONY: build run test lint

build:
	go build -o bin/order-service ./cmd/order-service
	go build -o bin/inventory-service ./cmd/inventory-service
	go build -o bin/websocket-service ./cmd/websocket-service

run:
	docker compose up -d
	go run ./cmd/order-service & \
	go run ./cmd/inventory-service & \
	go run ./cmd/websocket-service

test:
	go test -v ./...

lint:
	golangci-lint run

# Kubernetes
deploy-traefik:
	kubectl apply -f deployments/traefik/

delete-traefik:
	kubectl delete -f deployments/traefik/
