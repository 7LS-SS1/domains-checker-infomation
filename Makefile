GO_IMAGE ?= golang:1.26.6-alpine
GO_RACE_IMAGE ?= golang:1.26.6

.PHONY: fmt fmt-check tidy test test-race test-live-doh vet build docker-build compose-up compose-down migrate seed-admin integration-test

fmt:
	docker run --rm -v "$(CURDIR):/src" -w /src $(GO_IMAGE) gofmt -w .

fmt-check:
	docker run --rm -v "$(CURDIR):/src" -w /src $(GO_IMAGE) sh -c 'test -z "$$(gofmt -l .)"'

tidy:
	docker run --rm -v "$(CURDIR):/src" -w /src $(GO_IMAGE) go mod tidy

test:
	docker run --rm -v "$(CURDIR):/src" -w /src $(GO_IMAGE) go test ./...

test-race:
	docker run --rm -v "$(CURDIR):/src" -w /src $(GO_RACE_IMAGE) go test -race ./...

test-live-doh:
	docker run --rm -v "$(CURDIR):/src" -w /src $(GO_IMAGE) go test -tags=live ./internal/dnscheck -run TestLiveCloudflareDoH -count=1

vet:
	docker run --rm -v "$(CURDIR):/src" -w /src $(GO_IMAGE) go vet ./...

build:
	docker run --rm -v "$(CURDIR):/src" -w /src $(GO_IMAGE) go build ./cmd/...

docker-build:
	docker build --target test .

compose-up:
	docker compose up --build -d

compose-down:
	docker compose down

migrate:
	docker compose run --rm migration

seed-admin:
	docker compose --profile tools run --rm seed-admin

integration-test:
	docker compose --profile test run --rm integration-test
