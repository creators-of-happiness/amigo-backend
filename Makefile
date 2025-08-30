.PHONY: run run-bin build test tidy vet fmt clean compose-up compose-down compose-psql compose-logs migrate-up migrate-up-1 migrate-down migrate-down-1 migrate-force migrate-version

run:
	@set -a; [ -f .env ] && . ./.env; set +a; \
	go run ./cmd/api

run-bin: build
	@set -a; [ -f .env ] && . ./.env; set +a; \
	./bin/app

build:
	@CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/app ./cmd/api

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

migrate-up:
	@docker compose run --rm --entrypoint /bin/sh migrate -c \
	'migrate -path /migrations -database "$$DATABASE_URL" up'

migrate-up-1:
	@docker compose run --rm --entrypoint /bin/sh migrate -c \
	'migrate -path /migrations -database "$$DATABASE_URL" up 1'

migrate-down:
	@docker compose run --rm --entrypoint /bin/sh migrate -c \
	'migrate -path /migrations -database "$$DATABASE_URL" down'

migrate-down-1:
	@docker compose run --rm --entrypoint /bin/sh migrate -c \
	'migrate -path /migrations -database "$$DATABASE_URL" down 1'

migrate-force:
	@if [ -z "$(v)" ]; then echo "usage: make migrate-force v=<version>"; exit 1; fi
	@docker compose run --rm --entrypoint /bin/sh migrate -c \
	'migrate -path /migrations -database "$$DATABASE_URL" force $(v)'

migrate-version:
	@docker compose run --rm --entrypoint /bin/sh migrate -c \
	'migrate -path /migrations -database "$$DATABASE_URL" version'
