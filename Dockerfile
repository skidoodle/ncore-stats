FROM golang:alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o trackncore .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app/
COPY --from=builder /build/trackncore .
COPY --from=builder /build/data .
COPY --from=builder /build/index.html .
EXPOSE 3000
CMD ["./trackncore"]
