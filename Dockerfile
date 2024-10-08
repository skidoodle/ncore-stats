FROM golang:alpine as builder
WORKDIR /build
COPY . .
RUN go build -o trackncore .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app/
COPY --from=builder /build/trackncore .
COPY --from=builder /build/index.html .
EXPOSE 3000
CMD ["./trackncore"]
