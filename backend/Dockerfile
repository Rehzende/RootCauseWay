FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /rootcauseway-api ./cmd/api

FROM alpine:3.20 AS runtime
RUN apk --no-cache add ca-certificates curl \
    && addgroup -S rootcauseway && adduser -S rootcauseway -G rootcauseway
WORKDIR /app
COPY --from=builder /rootcauseway-api .
COPY --from=builder /app/migrations ./migrations
RUN chown -R rootcauseway:rootcauseway /app
USER rootcauseway
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=5s --retries=3 --start-period=10s \
    CMD curl -f http://localhost:8080/health || exit 1
CMD ["./rootcauseway-api"]
