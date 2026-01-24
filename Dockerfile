# Build Stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -o monitor ./cmd/monitor

# Final Stage
FROM alpine:latest

WORKDIR /app

# Install certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

COPY --from=builder /app/monitor .
COPY config.json .

# Expose Web Port
EXPOSE 8080

# Volumes for data and logs
VOLUME ["/app/data", "/app/logs"]

CMD ["./monitor"]
