FROM golang:1.15-alpine AS builder
WORKDIR /build/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o app rss/main.go

FROM alpine:latest  
RUN apk --no-cache add ca-certificates
WORKDIR /srv/
COPY --from=builder /build/app .
VOLUME [ "/data" ]
CMD ["./app", "--config", "/etc/aldor/something/rss/config.yaml", "--data", "/data/feeds.bolt"]
