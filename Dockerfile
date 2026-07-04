FROM golang:1.22.4-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o agent .

FROM alpine:3.20
WORKDIR /app

# Create a dedicated non-root user
RUN adduser -D -u 10001 agent

COPY --from=builder /app/agent .

EXPOSE 9100

HEALTHCHECK --interval=15s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:9100/health || exit 1

USER agent
CMD ["./agent"]