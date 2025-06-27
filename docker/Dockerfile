FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go test -v ./...
RUN go build -o wakeUpDev main.go

FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/wakeUpDev .

ENV TZ=Europe/Moscow

ENTRYPOINT ["./wakeUpDev"]
