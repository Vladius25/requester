FROM golang:1.19.3-alpine AS builder
WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /go/bin/requester cmd/requester/main.go

FROM alpine
COPY --from=builder /go/bin/requester /go/bin/requester
ENTRYPOINT ["/go/bin/requester"]
