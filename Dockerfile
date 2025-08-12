FROM golang:1.24 AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/app .

FROM gcr.io/distroless/static-debian12:nonroot
ENV PORT=8080 GIN_MODE=release
WORKDIR /
COPY --from=build /app/app /app
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app"]
