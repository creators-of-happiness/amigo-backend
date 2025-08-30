FROM golang:1.24 AS deps
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

FROM golang:1.24 AS build
WORKDIR /app
COPY --from=deps /go/pkg/mod /go/pkg/mod
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/api /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
