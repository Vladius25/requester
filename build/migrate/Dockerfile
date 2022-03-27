FROM golang:1.19.3-alpine AS builder
ENV CGO_ENABLED 0
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build  -o /app/bin/migrate /app/cmd/migrate

FROM busybox:1.33.1-musl

COPY --from=builder /etc/ssl/certs /etc/ssl/certs
COPY --from=builder /app/bin/migrate /app/bin/migrate
COPY --from=builder /app/migrations /app/migrations

WORKDIR /app
CMD /app/bin/migrate up
