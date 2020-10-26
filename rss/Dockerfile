FROM golang:1.15-alpine AS builder
WORKDIR /build/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o app rss/main.go

FROM alpine:3.12
RUN \
    apk --no-cache add ca-certificates && \
    addgroup -S -g 696 somegroup && \ 
    adduser -D -S -u 696 -G somegroup someuser
WORKDIR /srv/
COPY --from=builder /build/app .
RUN chown -R someuser:somegroup /srv
USER someuser:somegroup
VOLUME [ "/data" ]
CMD ["./app", "--config", "/etc/aldor/something/rss/config.yaml", "--data", "/data/feeds.bolt"]
