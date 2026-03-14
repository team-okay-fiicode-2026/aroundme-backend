APP_NAME=aroundme-api
DOCKER_COMPOSE ?= docker-compose

.PHONY: build run docker-up docker-down

build:
	go build -o bin/$(APP_NAME) ./cmd/api

run:
	go run ./cmd/api

docker-up:
	$(DOCKER_COMPOSE) up --build

docker-down:
	$(DOCKER_COMPOSE) down -v
