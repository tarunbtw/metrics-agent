.PHONY: build run test lint docker-build

## build: compile the agent binary
build:
	go build -o agent .

## run: run the agent locally (reads .env if present)
run: build
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi && ./agent

## test: run all tests with race detector and coverage
test:
	go test ./... -race -cover

## lint: run golangci-lint
lint:
	golangci-lint run

## docker-build: build the Docker image
docker-build:
	docker build -t metrics-agent:local .

## help: show this message
help:
	@grep -E '^##' Makefile | sed 's/## /  /'
