FROM golang:1.18-alpine AS builder
RUN apk add --no-cache git
WORKDIR /go/src/github.com/pokt-foundation

COPY . /go/src/github.com/pokt-foundation/rate-limiter

WORKDIR /go/src/github.com/pokt-foundation/rate-limiter
RUN CGO_ENABLED=0 GOOS=linux go build -a -o bin ./main.go

FROM alpine:3.16.0
WORKDIR /app
COPY --from=builder /go/src/github.com/pokt-foundation/rate-limiter/bin ./
ENTRYPOINT ["/app/bin"]
