FROM golang:1.26.1-alpine AS builder

RUN apk update && apk add --no-cache git ca-certificates tzdata

RUN addgroup -S -g 10001 appgroup && \
    adduser -S -u 10001 -G appgroup appuser

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ncore-stats .

RUN mkdir -p /app/data && chown -R 10001:10001 /app/data

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group
COPY --from=builder --chown=10001:10001 /build/ncore-stats /app/ncore-stats
COPY --from=builder --chown=10001:10001 /build/web /app/web
COPY --from=builder --chown=10001:10001 /app/data /app/data

WORKDIR /app
USER 10001
EXPOSE 3000

CMD ["./ncore-stats"]
