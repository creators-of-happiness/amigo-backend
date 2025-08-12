.PHONY: run build test tidy vet fmt clean

run:
	@GIN_MODE=debug @PORT=8080 go run ./...

build:
	@go build -o bin/app .

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
