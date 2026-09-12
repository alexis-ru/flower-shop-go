FROM golang:1.27-alpine AS builder

WORKDIR /build
COPY go.mod ./
COPY main.go ./
COPY web/ ./web/

RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o vasilyok .

RUN apk add --no-cache file && file vasilyok | grep "statically linked" || \
    (echo "Бинарник не статический!" && exit 1)

FROM alpine:3.20

COPY --from=builder /build/vasilyok /usr/local/bin/vasilyok
COPY config.ini /etc/vasilyok/config.ini

RUN chmod +x /usr/local/bin/vasilyok

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/vasilyok", "/etc/vasilyok/config.ini"]
