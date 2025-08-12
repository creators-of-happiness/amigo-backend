.PHONY: run build test tidy vet fmt clean

run:
	@GIN_MODE=debug @PORT=8080 go run ./...

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
