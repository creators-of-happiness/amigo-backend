.PHONY: run run-bin build test tidy vet fmt clean compose-up compose-down compose-psql compose-logs

run:
	@set -a; [ -f .env ] && . ./.env; set +a; \
	go run ./...

run-bin: build
	@set -a; [ -f .env ] && . ./.env; set +a; \
	./bin/app

build:
	@CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/app .

test:
	@go test ./...

tidy:
	@go mod tidy

vet:
	@go vet ./...

fmt:
	@gofmt -s -w .

clean:
	@rm -rf bin/

compose-up:
	@docker compose up -d --build

compose-down:
	@docker compose down -v

compose-psql:
	@set -a; [ -f .env ] && . ./.env; set +a; \
	docker compose exec db psql -U "$${POSTGRES_USER}" -d "$${POSTGRES_DB}"

compose-logs:
	@docker compose logs -f
