include .env
export

env = local

.env:
	@ln -sf ./configs/${env}.env .env

.PHONY: deps
deps:
	@docker-compose --compatibility up -d postgres sqs

.PHONY: generate
generate:
	@go generate

.PHONY: build
build: generate
	@go build  -o ./bin/ ./cmd/...

PHONY: coverage.out
coverage.out: generate
	@go test ./... -test.failfast -test.coverprofile=coverage.out

.PHONY: coverage
coverage: coverage.out
	@go tool cover -html=coverage.out

.PHONY: tests
tests: generate deps
	@go clean -testcache
	@go test ./... -test.failfast -test.v -race
