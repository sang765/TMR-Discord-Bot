FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o tmr-bot .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /home/container

COPY --from=builder /app/tmr-bot .
COPY --from=builder /app/config ./config

EXPOSE 8080

CMD ["./tmr-bot"]
