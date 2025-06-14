FROM golang:alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ENV CGO_ENABLED=1
RUN go build -o ncore-stats .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app/
COPY --from=builder /build/trackncore .
COPY --from=builder /build/data .
COPY --from=builder /build/index.html .
COPY --from=builder /build/script.js .
COPY --from=builder /build/style.css .
EXPOSE 3000
CMD ["./ncore-stats"]
