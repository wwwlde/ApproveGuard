
FROM golang:1.21.0 as builder

WORKDIR /build/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o ./ApproveGuard --ldflags '-w -s -extldflags "-static" -X main.version=0.0.1' .

FROM alpine:3.17 as alpine

RUN apk add -U --no-cache ca-certificates

FROM scratch as app

LABEL org.opencontainers.image.source=https://github.com/wwwlde/ApproveGuard
LABEL org.opencontainers.image.description="ApproveGuard container image"
LABEL org.opencontainers.image.licenses=GPL3

WORKDIR /app
COPY --from=builder /build/ApproveGuard .
COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/app/ApproveGuard"]
