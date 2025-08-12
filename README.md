# amigo-backend

## Requirements
- Go 1.24.5+
- Make (optional)
- Docker (optional)

## Quick Start

### Run
```bash
make run
```

### Build
```bash
make build
./bin/app
```

## Docker

### Build

```bash
docker build -t amigo-backend:local .
```

### Run

```bash
docker run --rm -p 8080:8080 \
  -e PORT=8080 -e GIN_MODE=release \
  --name amigo amigo-backend:local
```

## Docker Compose

```bash
docker compose up --build
```
