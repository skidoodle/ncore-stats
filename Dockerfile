FROM golang:alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ncore-stats .


FROM alpine:3
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/ncore-stats .
COPY web ./web
EXPOSE 3000

CMD ["./ncore-stats"]
