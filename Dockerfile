FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /usr/local/bin/aroundme-api ./cmd/api

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /usr/local/bin/aroundme-api /usr/local/bin/aroundme-api
COPY --from=builder /app/migrations /app/migrations

EXPOSE 8080

CMD ["aroundme-api"]
