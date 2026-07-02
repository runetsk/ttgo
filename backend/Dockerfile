FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -tags "sqlite_fts5" -o ttgo ./cmd/server/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 1000 ttgo && adduser -D -u 1000 -G ttgo ttgo \
    && mkdir -p /data /app/uploads \
    && chown -R ttgo:ttgo /data /app
WORKDIR /app
COPY --from=builder /app/ttgo .
USER ttgo
EXPOSE 8080
CMD ["./ttgo"]
