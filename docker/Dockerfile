FROM golang:1.25-alpine AS builder

WORKDIR /app

# Настройка Go proxy для более надежной загрузки зависимостей
ENV GOPROXY=https://proxy.golang.org,direct
ENV GOSUMDB=sum.golang.org
ENV GOPRIVATE=

COPY go.mod go.sum ./
# Повторяем попытку загрузки модулей при сетевых ошибках
RUN go mod download || go mod download || go mod download
COPY . .
RUN go test -v ./...
RUN go build -o wakeUpDev main.go

FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/wakeUpDev .

ENV TZ=Europe/Moscow

ENTRYPOINT ["./wakeUpDev"]
