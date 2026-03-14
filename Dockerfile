# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o swagger-mcp ./cmd/swagger-mcp

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/swagger-mcp .

EXPOSE 8080

ENTRYPOINT ["/app/swagger-mcp"]
CMD ["serve"]
