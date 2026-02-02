FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copiar go mod files
COPY go.mod ./
RUN go mod download

# Copiar código fuente
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o catalog-service main.go

# Runtime
FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/catalog-service .

EXPOSE 8085

CMD ["./catalog-service"]
